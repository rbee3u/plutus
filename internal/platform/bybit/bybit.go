// Package bybit is the read-only Bybit adapter for the global Unified Trading
// Account 2.0 in its three Margin Modes: Isolated Margin, Cross Margin, and
// Portfolio Margin.
//
// It calls only balance, position, account, and market-data endpoints. Raw
// responses are decoded inside this package and never cross the adapter
// boundary; the one exception is Record, the explicit response recorder of ADR
// 0009. Field mappings are documented in docs/platform/bybit/mapping.md.
package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/platform/httpclient"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// Account Types of the bybit_uta_2_0 Account Structure.
const (
	TypeDualInvestment                      = adapters.TypeDualInvestment
	TypeUnified        adapters.AccountType = "unified"
	TypeFunding        adapters.AccountType = "funding"
	TypeEarn           adapters.AccountType = "earn"
)

// accountStatuses names every unifiedMarginStatus Bybit documents, so a refusal can say what the
// account is. Only 5 and 6, UTA 2.0 and its Pro version, have the bybit_uta_2_0 structure; Bybit
// has migrated classic accounts and UTA 1.0 to it, and they stay refused.
var accountStatuses = map[int]string{
	1: "a classic account",
	3: "Unified Trading Account 1.0",
	4: "Unified Trading Account 1.0 Pro",
	5: "Unified Trading Account 2.0",
	6: "Unified Trading Account 2.0 Pro",
}

// marginModes maps Bybit's account-wide marginMode to the Margin Modes of bybit_uta_2_0.
var marginModes = map[string]adapters.MarginMode{
	"ISOLATED_MARGIN":  adapters.MarginBybitIsolated,
	"REGULAR_MARGIN":   adapters.MarginBybitCross,
	"PORTFOLIO_MARGIN": adapters.MarginBybitPM,
}

// quotes are the spot quotes a coin is priced in, in order of preference. Both
// value at 1 USD (ADR 0003).
var quotes = []string{"USDT", "USDC"}

// Options configures an Adapter. Zero values select production defaults.
type Options struct {
	Host string
	HTTP *http.Client
	Now  func() time.Time
	// HTTPFactory creates a client with the Adapter timeout when HTTP is nil.
	HTTPFactory func(time.Duration) *http.Client
}

// Adapter is the Bybit adapter. It is safe for concurrent use.
type Adapter struct {
	host  string
	http  *http.Client
	clock *clock
}

// New constructs a Bybit adapter; nil options select production defaults.
func New(in *Options) *Adapter {
	opts := Options{}
	if in != nil {
		opts = *in
	}
	if opts.Host == "" {
		opts.Host = DefaultHost
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
	return &Adapter{host: strings.TrimSuffix(opts.Host, "/"), http: opts.HTTP, clock: &clock{now: opts.Now}}
}

func (a *Adapter) client(creds adapters.Credentials) *client {
	return &client{http: a.http, host: a.host, clock: a.clock, creds: creds}
}

func (a *Adapter) now() time.Time { return a.clock.now().UTC() }

// Validate confirms the key reaches a UTA 2.0 account and returns the account's UID and Margin
// Mode.
func (a *Adapter) Validate(ctx context.Context, creds adapters.Credentials, structure adapters.AccountStructure) (adapters.ValidationResult, error) {
	return a.validate(ctx, a.client(creds), structure)
}

func (a *Adapter) validate(ctx context.Context, c *client, structure adapters.AccountStructure) (adapters.ValidationResult, error) {
	mode, err := a.marginMode(ctx, c, structure)
	if err != nil {
		return adapters.ValidationResult{}, err
	}
	var key struct {
		UserID json.Number `json:"userID"`
	}
	if err := c.call(ctx, epAPIKey, nil, &key); err != nil {
		return adapters.ValidationResult{}, err
	}
	uid := strings.TrimSpace(key.UserID.String())
	if uid == "" {
		return adapters.ValidationResult{}, adapters.Malformed(epAPIKey.String(), "userID")
	}
	return adapters.ValidationResult{ExternalRef: uid, MarginMode: mode}, nil
}

// marginMode confirms the account still has the Identity's Account Structure
// (ADR 0001) and returns its Margin Mode (ADR 0001). The structure is chosen
// once by the User, while Bybit lets the Margin Mode change at any time, so
// both are read at validation and at the start of every Collection Run.
func (a *Adapter) marginMode(ctx context.Context, c *client, structure adapters.AccountStructure) (adapters.MarginMode, error) {
	if structure != adapters.StructureBybitUTA2 {
		return "", adapters.UnsupportedStructure(adapters.PlatformBybit, structure)
	}
	var info struct {
		UnifiedMarginStatus *int   `json:"unifiedMarginStatus"`
		MarginMode          string `json:"marginMode"`
	}
	if err := c.call(ctx, epAccountInfo, nil, &info); err != nil {
		return "", err
	}
	if info.UnifiedMarginStatus == nil {
		return "", adapters.Malformed(epAccountInfo.String(), "unifiedMarginStatus")
	}
	if status := *info.UnifiedMarginStatus; status != 5 && status != 6 {
		name, ok := accountStatuses[status]
		if !ok {
			name = "an undocumented account status"
		}
		return "", adapters.StructureMismatch(adapters.PlatformBybit, fmt.Sprintf("%s (unifiedMarginStatus %d)", name, status), structure)
	}
	reported := strings.TrimSpace(info.MarginMode)
	if reported == "" {
		return "", adapters.Malformed(epAccountInfo.String(), "marginMode")
	}
	mode, ok := marginModes[reported]
	if !ok {
		return "", adapters.UnknownMarginMode(adapters.PlatformBybit, reported)
	}
	return mode, nil
}

// Collect returns one Account Report per requested Account Type, the Margin Mode, and the
// USDT/USDC prices of the reported Balances. The structure and Margin Mode are read first, so an
// account changed inside Bybit fails the batch instead of being collected under a stale label.
func (a *Adapter) Collect(ctx context.Context, creds adapters.Credentials, structure adapters.AccountStructure, accounts []adapters.AccountType) (adapters.Batch, error) {
	return a.collect(ctx, a.client(creds), structure, accounts)
}

func (a *Adapter) collect(ctx context.Context, c *client, structure adapters.AccountStructure, accounts []adapters.AccountType) (adapters.Batch, error) {
	mode, err := a.marginMode(ctx, c, structure)
	if err != nil {
		return adapters.Batch{}, err
	}
	// Prices come first so their observation time precedes every report of the batch, and the
	// valuation's "latest price at or before the report" picks them.
	prices, pricedAt := a.tickers(ctx, c)
	earnPair := adapters.CollectEarnPair(func(ctx context.Context) (adapters.AccountReport, adapters.AccountReport, error) {
		enrich := false
		for _, selected := range accounts {
			if selected == TypeEarn {
				enrich = true
			}
		}
		return a.earn(ctx, c, prices, enrich)
	})
	reports, err := adapters.CollectAccounts(ctx, accounts, func(ctx context.Context, accountType adapters.AccountType) (adapters.AccountReport, error) {
		//nolint:exhaustive // Manual is application-owned and never enters private Adapter collection.
		switch accountType {
		case TypeUnified:
			return a.unified(ctx, c, mode)
		case TypeFunding:
			return a.funding(ctx, c)
		case TypeEarn, TypeDualInvestment:
			return earnPair(ctx, accountType)
		default:
			return adapters.AccountReport{}, adapters.UnknownAccountType(adapters.PlatformBybit, structure, accountType)
		}
	})
	if err != nil {
		return adapters.Batch{}, err
	}
	return adapters.Batch{
		ObservedAt: a.now(),
		MarginMode: mode,
		Reports:    reports,
		Prices:     adapters.QuotePrices(reports, quotes, prices.lookup, pricedAt),
	}, nil
}

type walletAccount struct {
	AccountType           string       `json:"accountType"`
	TotalEquity           string       `json:"totalEquity"`
	TotalAvailableBalance string       `json:"totalAvailableBalance"`
	AccountIMRate         string       `json:"accountIMRate"`
	AccountMMRate         string       `json:"accountMMRate"`
	Coins                 []walletCoin `json:"coin"`
}

type walletCoin struct {
	Coin            string `json:"coin"`
	Equity          string `json:"equity"`
	BorrowAmount    string `json:"borrowAmount"`
	AccruedInterest string `json:"accruedInterest"`
	UnrealisedPnl   string `json:"unrealisedPnl"`
}

// unified maps the Unified Trading Account: its account-wide figures, its per-coin equities, and
// its open contract positions.
func (a *Adapter) unified(ctx context.Context, c *client, mode adapters.MarginMode) (adapters.AccountReport, error) {
	var result struct {
		List []walletAccount `json:"list"`
	}
	if err := c.call(ctx, epWallet, nil, &result); err != nil {
		return adapters.AccountReport{}, err
	}
	if len(result.List) != 1 || result.List[0].AccountType != "UNIFIED" || result.List[0].Coins == nil {
		return adapters.AccountReport{}, adapters.Malformed(epWallet.String(), "the UNIFIED account and its coin list")
	}
	account := &result.List[0]
	report, err := mapUnifiedAccount(account, mode)
	if err != nil {
		return adapters.AccountReport{}, err
	}
	report.ObservedAt = a.now()
	if report.Balances, err = mapUnifiedBalances(account.Coins); err != nil {
		return adapters.AccountReport{}, err
	}
	if report.Positions, err = a.positions(ctx, c, mode); err != nil {
		return adapters.AccountReport{}, err
	}
	return report, nil
}

// mapUnifiedAccount reads the account-wide figures, all in USD. Bybit documents them as not
// applicable to Isolated Margin, so there they are ignored and the value is calculated from the
// Balances. In Cross and Portfolio Margin an empty field stays unknown rather than becoming zero.
// totalMarginBalance is not haircut-adjusted equity and is not mapped.
func mapUnifiedAccount(in *walletAccount, mode adapters.MarginMode) (adapters.AccountReport, error) {
	report := adapters.AccountReport{AccountType: TypeUnified, Status: adapters.StatusOK}
	if mode == adapters.MarginBybitIsolated {
		return report, nil
	}
	for _, field := range []struct {
		name, value string
		dst         **string
	}{
		{"totalEquity", in.TotalEquity, &report.Equity},
		{"totalAvailableBalance", in.TotalAvailableBalance, &report.AvailableEquity},
		{"accountIMRate", in.AccountIMRate, &report.InitialMarginRatio},
		{"accountMMRate", in.AccountMMRate, &report.MaintenanceMarginRatio},
	} {
		if _, err := adapters.OptionalDecimal(epWallet.String(), field.name, field.value); err != nil {
			return adapters.AccountReport{}, err
		}
		*field.dst = adapters.Plain(field.value)
	}
	return report, nil
}

// mapUnifiedBalances maps coin[] rows. Equity is Bybit's own per-coin `equity`, which already
// nets spot borrowing and includes unrealized PnL, so neither is added or subtracted again. An
// empty `equity` fails the Account: read as zero, it would drop the coin under the Complete Set
// rule. Bybit reports no full coin name, so Name stays empty.
func mapUnifiedBalances(rows []walletCoin) ([]adapters.Balance, error) {
	where := epWallet.String()
	out := make([]adapters.Balance, 0, len(rows))
	seen := map[string]bool{}
	for i := range rows {
		row := &rows[i]
		symbol := strings.TrimSpace(row.Coin)
		if symbol == "" || seen[symbol] {
			return nil, adapters.Malformed(where, "a unique coin symbol")
		}
		seen[symbol] = true
		equity, err := adapters.Decimal(where, "equity of "+symbol, row.Equity)
		if err != nil {
			return nil, err
		}
		parsed := map[string]*big.Rat{}
		nonzero := equity.Sign() != 0
		for name, value := range map[string]string{
			"borrowAmount": row.BorrowAmount, "accruedInterest": row.AccruedInterest, "unrealisedPnl": row.UnrealisedPnl,
		} {
			rat, err := adapters.OptionalDecimal(where, name+" of "+symbol, value)
			if err != nil {
				return nil, err
			}
			parsed[name] = rat
			nonzero = nonzero || rat.Sign() != 0
		}
		if !nonzero {
			continue
		}
		out = append(out, adapters.Balance{
			Symbol:        symbol,
			Equity:        decimal.Format(equity),
			Liability:     adapters.Reported(row.BorrowAmount, decimal.Abs(parsed["borrowAmount"])),
			Interest:      adapters.Reported(row.AccruedInterest, parsed["accruedInterest"]),
			UnrealizedPnl: adapters.Reported(row.UnrealisedPnl, parsed["unrealisedPnl"]),
		})
	}
	return out, nil
}

type fundingCoin struct {
	Coin            string `json:"coin"`
	WalletBalance   string `json:"walletBalance"`
	TransferBalance string `json:"transferBalance"`
}

// funding maps the Funding account, which Bybit reports without any USD valuation, so the value
// is calculated from the Balances. Like `equity`, an empty `walletBalance` fails the Account
// instead of reading as zero.
func (a *Adapter) funding(ctx context.Context, c *client) (adapters.AccountReport, error) {
	var result struct {
		AccountType string        `json:"accountType"`
		Balance     []fundingCoin `json:"balance"`
	}
	if err := c.call(ctx, epFunding, nil, &result); err != nil {
		return adapters.AccountReport{}, err
	}
	where := epFunding.String()
	if result.AccountType != "FUND" || result.Balance == nil {
		return adapters.AccountReport{}, adapters.Malformed(where, "the FUND balance list")
	}
	balances := make([]adapters.Balance, 0, len(result.Balance))
	seen := map[string]bool{}
	for i := range result.Balance {
		row := &result.Balance[i]
		symbol := strings.TrimSpace(row.Coin)
		if symbol == "" || seen[symbol] {
			return adapters.AccountReport{}, adapters.Malformed(where, "a unique coin symbol")
		}
		seen[symbol] = true
		total, err := adapters.Decimal(where, "walletBalance of "+symbol, row.WalletBalance)
		if err != nil {
			return adapters.AccountReport{}, err
		}
		transferable, err := adapters.OptionalDecimal(where, "transferBalance of "+symbol, row.TransferBalance)
		if err != nil {
			return adapters.AccountReport{}, err
		}
		if total.Sign() == 0 {
			continue
		}
		balances = append(balances, adapters.Balance{
			Symbol:              symbol,
			Equity:              decimal.Format(total),
			AvailableToWithdraw: adapters.Reported(row.TransferBalance, transferable),
		})
	}
	return adapters.AccountReport{AccountType: TypeFunding, Status: adapters.StatusOK, ObservedAt: a.now(), Balances: balances}, nil
}

// spotPrices are Bybit spot last prices by pair symbol, such as BTCUSDT.
type spotPrices map[string]string

func (p spotPrices) lookup(symbol, quote string) (string, bool) {
	price, ok := p[symbol+quote]
	return price, ok
}

// usd returns the USD value of one unit of symbol, for the Earn check.
func (p spotPrices) usd(symbol string) (*big.Rat, bool) {
	return adapters.USDPrice(symbol, quotes, p.lookup)
}

// tickers reads every spot last price in one call. A failure yields no prices rather than failing
// the batch: valuation falls back to the latest price within 24 hours.
func (a *Adapter) tickers(ctx context.Context, c *client) (spotPrices, time.Time) {
	var result struct {
		List []struct {
			Symbol    string `json:"symbol"`
			LastPrice string `json:"lastPrice"`
		} `json:"list"`
	}
	if err := c.call(ctx, epTickers, nil, &result); err != nil {
		return spotPrices{}, time.Time{}
	}
	prices := make(spotPrices, len(result.List))
	for _, row := range result.List {
		if price, ok := decimal.Parse(row.LastPrice); ok && decimal.Valid(row.LastPrice) && price.Sign() > 0 {
			prices[strings.TrimSpace(row.Symbol)] = decimal.Compact(strings.TrimSpace(row.LastPrice))
		}
	}
	return prices, a.now()
}

// maxPages bounds a cursor-paged read, so a Platform that keeps answering with new cursors
// cannot hold a Collection Run forever.
const maxPages = 1000

// pages reads every page of a cursor-paged list. A missing list, a cursor seen before, or a list
// that does not end within maxPages is a failure, never a shorter Complete Set.
func pages[T any](ctx context.Context, c *client, ep endpoint, params url.Values) ([]T, error) {
	query := url.Values{}
	for key, values := range params {
		query[key] = values
	}
	var rows []T
	seen := map[string]bool{}
	for page := 0; page < maxPages; page++ {
		var result struct {
			List   []T    `json:"list"`
			Cursor string `json:"nextPageCursor"`
		}
		if err := c.call(ctx, ep, query, &result); err != nil {
			return nil, err
		}
		if result.List == nil {
			return nil, adapters.Malformed(ep.String(), "the list")
		}
		rows = append(rows, result.List...)
		if result.Cursor == "" {
			return rows, nil
		}
		if seen[result.Cursor] {
			return nil, adapters.NewError(adapters.ClassNormalization, fmt.Sprintf("%s: Bybit repeated a page cursor, so the list may be incomplete.", ep))
		}
		seen[result.Cursor] = true
		query.Set("cursor", result.Cursor)
	}
	return nil, adapters.NewError(adapters.ClassNormalization, fmt.Sprintf("%s: the list did not end within %d pages.", ep, maxPages))
}

// parseMillis reads a positive Unix time in milliseconds.
func parseMillis(value string) (time.Time, bool) {
	millis, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || millis <= 0 {
		return time.Time{}, false
	}
	return time.UnixMilli(millis).UTC(), true
}
