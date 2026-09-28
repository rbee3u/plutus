// Package binance is the read-only Binance adapter for both Binance Account Structures: Classic
// Trading (binance_classic), where Cross Margin, Isolated Margin, USDⓈ-M Futures, and COIN-M
// Futures are separate wallets, and Portfolio Margin (binance_pm), which merges those four into
// one portfolio_margin Account. Spot, Funding, and Earn exist in both. Binance has no Margin Mode.
//
// It calls only balance, position, account, and market-data endpoints. Raw responses are decoded
// inside this package and never cross the adapter boundary; the one exception is Record, the
// offline fixture recorder of ADR 0023. Field mappings are documented in docs/adapters/binance.md.
package binance

import (
	"context"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/httpclient"
	"github.com/rbee3u/plutus/internal/decimal"
)

// Account Types. Spot, Funding, and Earn belong to both Account Structures; Portfolio Margin
// replaces the four Classic margin and futures wallets with portfolio_margin.
const (
	TypeDualInvestment                       = adapters.TypeDualInvestment
	TypeSpot            adapters.AccountType = "spot"
	TypeFunding         adapters.AccountType = "funding"
	TypeEarn            adapters.AccountType = "earn"
	TypePortfolioMargin adapters.AccountType = "portfolio_margin"
	TypeCrossMargin     adapters.AccountType = "cross_margin"
	TypeIsolatedMargin  adapters.AccountType = "isolated_margin"
	TypeUMFutures       adapters.AccountType = "um_futures"
	TypeCMFutures       adapters.AccountType = "cm_futures"
)

// quotes are the spot quote coins prices are read in, in order of preference. Both value at 1 USD
// (ADR 0017).
var quotes = []string{"USDT", "USDC"}

// pageSize is the largest page the Simple Earn and dual investment position endpoints accept.
const pageSize = 100

// Options configures an Adapter. Zero values select production defaults.
type Options struct {
	Hosts Hosts
	HTTP  *http.Client
	Now   func() time.Time
}

// Adapter is the Binance adapter. It is safe for concurrent use.
type Adapter struct {
	hosts       Hosts
	http        *http.Client
	clock       *clock
	instruments *instrumentCache
}

// New constructs a Binance adapter; nil options select production defaults.
func New(in *Options) *Adapter {
	opts := Options{}
	if in != nil {
		opts = *in
	}
	if opts.Hosts == (Hosts{}) {
		opts.Hosts = DefaultHosts
	}
	if opts.HTTP == nil {
		opts.HTTP = httpclient.NewIPv4(requestTimeout)
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Adapter{
		hosts:       opts.Hosts,
		http:        opts.HTTP,
		clock:       &clock{now: opts.Now},
		instruments: newInstrumentCache(opts.Now),
	}
}

func (a *Adapter) client(creds adapters.Credentials) *client {
	return &client{http: a.http, hosts: a.hosts, clock: a.clock, creds: creds}
}

func (a *Adapter) now() time.Time { return a.clock.now().UTC() }

// Validate confirms that the key reaches an account with the Identity's Account Structure and
// returns the account's UID.
func (a *Adapter) Validate(ctx context.Context, creds adapters.Credentials, structure adapters.AccountStructure) (adapters.ValidationResult, error) {
	if !adapters.ValidStructure(adapters.PlatformBinance, structure) {
		return adapters.ValidationResult{}, adapters.UnsupportedStructure(adapters.PlatformBinance, structure)
	}
	c := a.client(creds)
	if _, err := checkStructure(ctx, c, structure); err != nil {
		return adapters.ValidationResult{}, err
	}
	var spot struct {
		UID int64 `json:"uid"`
	}
	if err := c.call(ctx, epSpotAccount, spotAccountQuery(), &spot); err != nil {
		return adapters.ValidationResult{}, err
	}
	if spot.UID <= 0 {
		return adapters.ValidationResult{}, adapters.NewError(adapters.ClassNormalization, "Binance returned no account UID.")
	}
	return adapters.ValidationResult{ExternalRef: strconv.FormatInt(spot.UID, 10)}, nil
}

// spotAccountQuery reads the spot account for its UID without listing every coin ever held; the
// recorder sends the same query.
func spotAccountQuery() url.Values {
	return url.Values{"omitZeroBalances": {"true"}}
}

// Collect confirms the Account Structure, then returns one Account Report per requested Account
// Type and the USDT/USDC prices of the reported Balances. An authentication failure on any call
// fails the whole batch; any other failure fails only its Account, unless every Account failed
// with a retryable class.
func (a *Adapter) Collect(ctx context.Context, creds adapters.Credentials, structure adapters.AccountStructure, accounts []adapters.AccountType) (adapters.Batch, error) {
	if !adapters.ValidStructure(adapters.PlatformBinance, structure) {
		return adapters.Batch{}, adapters.UnsupportedStructure(adapters.PlatformBinance, structure)
	}
	c := a.client(creds)
	info, err := checkStructure(ctx, c, structure)
	if err != nil {
		return adapters.Batch{}, err
	}
	// Prices come next so their observation time precedes every report of the batch, and the
	// valuation's "latest price at or before the report" picks them.
	prices, pricedAt := a.lastPrices(ctx, c)
	earnPair := adapters.CollectEarnPair(func(ctx context.Context) (adapters.AccountReport, adapters.AccountReport, error) {
		return a.earn(ctx, c, prices)
	})
	reports, err := adapters.CollectAccounts(ctx, accounts, func(ctx context.Context, accountType adapters.AccountType) (adapters.AccountReport, error) {
		if accountType == TypeEarn || accountType == TypeDualInvestment {
			return earnPair(ctx, accountType)
		}
		return a.collectAccount(ctx, c, structure, &info, accountType)
	})
	if err != nil {
		return adapters.Batch{}, err
	}
	return adapters.Batch{ObservedAt: a.now(), Reports: reports, Prices: adapters.QuotePrices(reports, quotes, prices.quote, pricedAt)}, nil
}

func (a *Adapter) collectAccount(ctx context.Context, c *client, structure adapters.AccountStructure, info *accountInfo, accountType adapters.AccountType) (adapters.AccountReport, error) {
	if !adapters.ValidAccountType(structure, accountType) {
		return adapters.AccountReport{}, adapters.UnknownAccountType(adapters.PlatformBinance, structure, accountType)
	}
	switch accountType {
	case TypeSpot:
		return a.wallet(ctx, c, TypeSpot, epUserAsset)
	case TypeFunding:
		return a.wallet(ctx, c, TypeFunding, epFundingAsset)
	case TypePortfolioMargin:
		return a.portfolioMargin(ctx, c)
	case TypeCrossMargin, TypeIsolatedMargin:
		if info.marginClosed() {
			return a.closed(accountType), nil
		}
		if accountType == TypeCrossMargin {
			return a.crossMargin(ctx, c)
		}
		return a.isolatedMargin(ctx, c)
	case TypeUMFutures, TypeCMFutures:
		if info.futuresClosed() {
			return a.closed(accountType), nil
		}
		if accountType == TypeUMFutures {
			return a.umFutures(ctx, c)
		}
		return a.cmFutures(ctx, c)
	default:
		return adapters.AccountReport{}, adapters.UnknownAccountType(adapters.PlatformBinance, structure, accountType)
	}
}

// closed is the report of a Classic wallet Binance says is not opened: it cannot hold anything,
// so the Account is reported empty instead of failing on an endpoint that refuses it.
func (a *Adapter) closed(accountType adapters.AccountType) adapters.AccountReport {
	return adapters.AccountReport{AccountType: accountType, Status: adapters.StatusOK, ObservedAt: a.now()}
}

// accountInfo is /sapi/v1/account/info: the account's Account Structure and whether its Classic
// margin and futures wallets are opened.
type accountInfo struct {
	MarginEnabled   *bool `json:"isMarginEnabled"`
	FuturesEnabled  *bool `json:"isFutureEnabled"`
	PortfolioMargin *bool `json:"isPortfolioMarginRetailEnabled"`
}

// structure maps isPortfolioMarginRetailEnabled, Binance's Portfolio Margin enable status: true is
// Portfolio Margin, false is Classic Trading. Portfolio Margin Pro is institutional only and not
// offered.
func (info *accountInfo) structure() (adapters.AccountStructure, error) {
	if info.PortfolioMargin == nil {
		return "", adapters.Malformed(epAccountInfo.String(), "isPortfolioMarginRetailEnabled")
	}
	if *info.PortfolioMargin {
		return adapters.StructureBinancePM, nil
	}
	return adapters.StructureBinanceClassic, nil
}

// marginClosed and futuresClosed report a Classic wallet Binance says is not opened. An absent
// flag counts as opened, so the wallet is read instead of being reported empty on a guess.
func (info *accountInfo) marginClosed() bool {
	return info.MarginEnabled != nil && !*info.MarginEnabled
}

func (info *accountInfo) futuresClosed() bool {
	return info.FuturesEnabled != nil && !*info.FuturesEnabled
}

// checkStructure reads the account's Account Structure. The User can switch between Classic
// Trading and Portfolio Margin inside Binance at any time, so every validation and every
// Collection Run checks it first (ADR 0025); a different structure fails before any account is
// read.
func checkStructure(ctx context.Context, c *client, configured adapters.AccountStructure) (accountInfo, error) {
	var info accountInfo
	if err := c.call(ctx, epAccountInfo, nil, &info); err != nil {
		return accountInfo{}, err
	}
	actual, err := info.structure()
	if err != nil {
		return accountInfo{}, err
	}
	if actual != configured {
		return accountInfo{}, adapters.StructureMismatch(adapters.PlatformBinance, adapters.StructureName(actual), configured)
	}
	return info, nil
}

// lastPrices holds every spot last price by pair symbol, such as BTCUSDT.
type lastPrices map[string]string

// quote returns the last price of symbol in quote; it is the batch's adapters.QuoteLookup.
func (p lastPrices) quote(symbol, quote string) (string, bool) {
	price, ok := p[symbol+quote]
	return price, ok
}

// usd returns the USD value of one unit of symbol, for the Earn completeness check.
func (p lastPrices) usd(symbol string) (*big.Rat, bool) {
	return adapters.USDPrice(symbol, quotes, p.quote)
}

// lastPrices returns every spot last price and when it arrived. A failure yields no prices rather
// than failing the batch: valuation falls back to the latest price within 24 hours, and the Earn
// completeness check is skipped for coins without a price.
func (a *Adapter) lastPrices(ctx context.Context, c *client) (lastPrices, time.Time) {
	var rows []struct {
		Symbol string `json:"symbol"`
		Price  string `json:"price"`
	}
	if err := c.call(ctx, epTickerPrice, nil, &rows); err != nil {
		return nil, time.Time{}
	}
	out := make(lastPrices, len(rows))
	for _, row := range rows {
		// A delisted pair stays listed with price 0, which is not a value.
		if price, ok := decimal.Parse(row.Price); ok && decimal.Valid(row.Price) && price.Sign() > 0 {
			out[row.Symbol] = row.Price
		}
	}
	return out, a.now()
}

// amounts parses a row's optional decimal fields and reports whether any of them is nonzero.
func amounts(where string, fields map[string]string) (map[string]*big.Rat, bool, error) {
	parsed := make(map[string]*big.Rat, len(fields))
	nonzero := false
	for name, value := range fields {
		rat, err := adapters.OptionalDecimal(where, name, value)
		if err != nil {
			return nil, false, err
		}
		parsed[name] = rat
		nonzero = nonzero || rat.Sign() != 0
	}
	return parsed, nonzero, nil
}

// coin returns the coin of a row that holds an amount. Such a row without a coin is a
// normalization failure, never skipped: skipping it would make its amount disappear.
func coin(where, asset string) (string, error) {
	asset = strings.TrimSpace(asset)
	if asset == "" {
		return "", adapters.Malformed(where, "the asset of a nonzero row")
	}
	return asset, nil
}
