// Package binance implements read-only Binance access for classic trading and
// portfolio margin. Field mappings: docs/platform/binance/mapping.md. Runtime
// responses stay private; RecordRequest is the explicit recording path.
package binance

import (
	"context"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/platform/httpclient"
)

// Account types. Spot, funding, and earn belong to both account structures; portfolio margin
// replaces the four classic margin and futures wallets with portfolio_margin.
const (
	TypeDualInvestment                      = adapter.TypeDualInvestment
	TypeSpot            adapter.AccountType = "spot"
	TypeFunding         adapter.AccountType = "funding"
	TypeEarn            adapter.AccountType = "earn"
	TypePortfolioMargin adapter.AccountType = "portfolio_margin"
	TypeCrossMargin     adapter.AccountType = "cross_margin"
	TypeIsolatedMargin  adapter.AccountType = "isolated_margin"
	TypeUMFutures       adapter.AccountType = "um_futures"
	TypeCMFutures       adapter.AccountType = "cm_futures"
)

// pageSize is the largest page the Simple Earn and dual investment position endpoints accept.
const pageSize = 100

// Options configures an adapter. Zero values select production defaults.
type Options struct {
	Hosts Hosts
	HTTP  *http.Client
	Now   func() time.Time
	// HTTPFactory creates a client with the adapter timeout when HTTP is nil.
	HTTPFactory func(time.Duration) *http.Client
}

// Adapter is the Binance adapter. It is safe for concurrent use.
type Adapter struct {
	hosts            Hosts
	http             *http.Client
	clock            *clock
	futuresContracts *futuresContractCache
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
		factory := opts.HTTPFactory
		if factory == nil {
			factory = httpclient.NewIPv4
		}
		opts.HTTP = factory(requestTimeout)
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Adapter{
		hosts:            opts.Hosts,
		http:             opts.HTTP,
		clock:            &clock{now: opts.Now},
		futuresContracts: newFuturesContractCache(opts.Now),
	}
}

func (a *Adapter) client(creds adapter.Credentials) *client {
	return &client{http: a.http, hosts: a.hosts, clock: a.clock, creds: creds}
}

func (a *Adapter) now() time.Time { return a.clock.now().UTC() }

func (a *Adapter) Validate(ctx context.Context, creds adapter.Credentials, structure adapter.AccountStructure) (adapter.ValidationResult, error) {
	if !adapter.ValidStructure(adapter.PlatformBinance, structure) {
		return adapter.ValidationResult{}, adapter.UnsupportedStructure(adapter.PlatformBinance, structure)
	}
	c := a.client(creds)
	if _, err := checkStructure(ctx, c, structure); err != nil {
		return adapter.ValidationResult{}, err
	}
	var spot struct {
		UID int64 `json:"uid"`
	}
	if err := c.call(ctx, epSpotAccount, spotAccountQuery(), &spot); err != nil {
		return adapter.ValidationResult{}, err
	}
	if spot.UID <= 0 {
		return adapter.ValidationResult{}, adapter.NewError(adapter.ClassNormalization, "Binance returned no account UID.")
	}
	return adapter.ValidationResult{ExternalRef: strconv.FormatInt(spot.UID, 10)}, nil
}

// spotAccountQuery reads the spot account for its UID without listing every coin ever held; the
// recorder sends the same query.
func spotAccountQuery() url.Values {
	return url.Values{"omitZeroBalances": {"true"}}
}

func (a *Adapter) Collect(ctx context.Context, creds adapter.Credentials, structure adapter.AccountStructure, accounts []adapter.AccountType) (adapter.Batch, error) {
	// Market terms are reused only within this collection run.
	run := *a
	run.futuresContracts = newFuturesContractCache(a.now)
	a = &run

	if !adapter.ValidStructure(adapter.PlatformBinance, structure) {
		return adapter.Batch{}, adapter.UnsupportedStructure(adapter.PlatformBinance, structure)
	}
	c := a.client(creds)
	info, err := checkStructure(ctx, c, structure)
	if err != nil {
		return adapter.Batch{}, err
	}
	// Read prices before reports to satisfy at-or-before valuation lookup.
	prices, pricedAt := a.lastPrices(ctx, c)
	earnPair := adapter.CollectEarnPair(func(ctx context.Context) (adapter.AccountReport, adapter.AccountReport, error) {
		return a.earn(ctx, c, prices)
	})
	reports, err := adapter.CollectAccounts(ctx, accounts, a.now, func(ctx context.Context, accountType adapter.AccountType) (adapter.AccountReport, error) {
		if accountType == TypeEarn || accountType == TypeDualInvestment {
			return earnPair(ctx, accountType)
		}
		return a.collectAccount(ctx, c, structure, &info, accountType)
	})
	if err != nil {
		return adapter.Batch{}, err
	}
	return adapter.Batch{CollectedAt: a.now(), Reports: reports, Prices: adapter.QuotePrices(reports, quotes, prices.quote, pricedAt)}, nil
}

func (a *Adapter) collectAccount(ctx context.Context, c *client, structure adapter.AccountStructure, info *accountInfo, accountType adapter.AccountType) (adapter.AccountReport, error) {
	if !adapter.ValidAccountType(structure, accountType) {
		return adapter.AccountReport{}, adapter.UnknownAccountType(adapter.PlatformBinance, structure, accountType)
	}
	// Earn and dual investment are collected together before this helper is called.
	switch accountType { //nolint:exhaustive // Shared product accounts are handled by earnPair.
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
		return adapter.AccountReport{}, adapter.UnknownAccountType(adapter.PlatformBinance, structure, accountType)
	}
}

// closed is the report of a classic wallet Binance says is not opened: it cannot hold anything,
// so the account is reported empty instead of failing on an endpoint that refuses it.
func (a *Adapter) closed(accountType adapter.AccountType) adapter.AccountReport {
	return adapter.AccountReport{AccountType: accountType, Status: adapter.StatusSucceeded, CollectedAt: a.now()}
}

// accountInfo is /sapi/v1/account/info: the account's account structure and whether its classic
// margin and futures wallets are opened.
type accountInfo struct {
	MarginEnabled   *bool `json:"isMarginEnabled"`
	FuturesEnabled  *bool `json:"isFutureEnabled"`
	PortfolioMargin *bool `json:"isPortfolioMarginRetailEnabled"`
}

// structure maps isPortfolioMarginRetailEnabled, Binance's portfolio margin enable status: true is
// portfolio margin, false is classic trading. Portfolio Margin Pro is institutional only and not
// offered.
func (info *accountInfo) structure() (adapter.AccountStructure, error) {
	if info.PortfolioMargin == nil {
		return "", adapter.Malformed(epAccountInfo.String(), "isPortfolioMarginRetailEnabled")
	}
	if *info.PortfolioMargin {
		return adapter.StructureBinancePM, nil
	}
	return adapter.StructureBinanceClassic, nil
}

// marginClosed and futuresClosed report a classic wallet Binance says is not opened. An absent
// flag counts as opened, so the wallet is read instead of being reported empty on a guess.
func (info *accountInfo) marginClosed() bool {
	return info.MarginEnabled != nil && !*info.MarginEnabled
}

func (info *accountInfo) futuresClosed() bool {
	return info.FuturesEnabled != nil && !*info.FuturesEnabled
}

// checkStructure reads the account's account structure. The user can switch
// between classic trading and portfolio margin inside Binance at any time, so
// every validation and every collection run checks it first (ADR 0001); a
// different structure fails before any account is read.
func checkStructure(ctx context.Context, c *client, configured adapter.AccountStructure) (accountInfo, error) {
	var info accountInfo
	if err := c.call(ctx, epAccountInfo, nil, &info); err != nil {
		return accountInfo{}, err
	}
	actual, err := info.structure()
	if err != nil {
		return accountInfo{}, err
	}
	if actual != configured {
		return accountInfo{}, adapter.StructureMismatch(adapter.PlatformBinance, adapter.StructureName(actual), configured)
	}
	return info, nil
}

// amounts parses a row's optional decimal fields and reports whether any of them is nonzero.
func amounts(where string, fields map[string]string) (map[string]*big.Rat, bool, error) {
	parsed := make(map[string]*big.Rat, len(fields))
	nonzero := false
	for name, value := range fields {
		rat, err := adapter.OptionalDecimal(where, name, value)
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
		return "", adapter.Malformed(where, "the asset of a nonzero row")
	}
	return asset, nil
}
