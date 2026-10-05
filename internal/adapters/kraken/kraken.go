// Package kraken is the read-only Kraken adapter for the Separate wallets and
// Unified Wallet Account Structures (ADR 0001).
//
// It calls only balance, position, account, and market-data endpoints of
// Kraken's Spot REST API and Futures REST API, plus one bounded balances
// snapshot of the Futures WebSocket feed. Raw responses are decoded inside
// this package and never cross the adapter boundary; the one exception is
// Record, the explicit response recorder of ADR 0009. Field mappings are
// documented in docs/adapters/kraken/mapping.md.
package kraken

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/httpclient"
	"github.com/rbee3u/plutus/internal/decimal"
)

// Account Types of the kraken_separate and kraken_unified Account Structures. Multi-M exists only
// in Separate wallets: in Unified Wallet the multi-collateral wallet is Main itself.
const (
	TypeMain               adapters.AccountType = "main"
	TypeEarn               adapters.AccountType = "earn"
	TypeMultiM             adapters.AccountType = "multi_m"
	TypeCoinM              adapters.AccountType = "coin_m"
	TypeDerivativesHolding adapters.AccountType = "derivatives_holding"
)

// priceQuotes are the Spot quotes a Balance is priced in, in order of
// preference: USD, then the two stablecoins ADR 0003 values at 1 USD.
var priceQuotes = []string{"USD", "USDT", "USDC"}

// structureWords names each Account Structure the way Kraken's flag reports it, for the detail of
// a structure mismatch.
var structureWords = map[adapters.AccountStructure]string{
	adapters.StructureKrakenUnified:  "unified_balances = true (Unified Wallet)",
	adapters.StructureKrakenSeparate: "unified_balances = false (Separate wallets)",
}

// Options configures an Adapter. Zero values select production defaults.
type Options struct {
	SpotHost     string
	FuturesHost  string
	WebSocketURL string
	HTTP         *http.Client
	Now          func() time.Time
	// HTTPFactory creates a client with the Adapter timeout when HTTP is nil.
	HTTPFactory func(time.Duration) *http.Client
}

// Adapter is the Kraken adapter. It is safe for concurrent use.
type Adapter struct {
	spotHost        string
	futuresHost     string
	wsURL           string
	http            *http.Client
	clock           func() time.Time
	nonces          *nonces
	captureSnapshot func([]byte)
}

// New constructs a Kraken adapter; nil options select production defaults.
func New(in *Options) *Adapter {
	opts := Options{}
	if in != nil {
		opts = *in
	}
	if opts.SpotHost == "" {
		opts.SpotHost = DefaultSpotHost
	}
	if opts.FuturesHost == "" {
		opts.FuturesHost = DefaultFuturesHost
	}
	if opts.WebSocketURL == "" {
		opts.WebSocketURL = DefaultWebSocketURL
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
		spotHost:    strings.TrimRight(opts.SpotHost, "/"),
		futuresHost: strings.TrimRight(opts.FuturesHost, "/"),
		wsURL:       opts.WebSocketURL,
		http:        opts.HTTP,
		clock:       opts.Now,
		nonces:      &nonces{},
	}
}

func (a *Adapter) client(creds adapters.Credentials) *client {
	return &client{a: a, creds: creds}
}

func (a *Adapter) now() time.Time { return a.clock().UTC() }

func supported(structure adapters.AccountStructure) bool {
	return adapters.ValidStructure(adapters.PlatformKraken, structure)
}

// structureOf reads the Account Structure from the documented unified_balances flag of the
// balances snapshot. A missing flag is unknown; it is never read as Separate wallets.
func structureOf(s *snapshot) (adapters.AccountStructure, error) {
	if s.Flex.UnifiedBalances == nil {
		return "", adapters.Malformed("Kraken "+epBalancesSnapshot.String(), "flex_futures.unified_balances")
	}
	if *s.Flex.UnifiedBalances {
		return adapters.StructureKrakenUnified, nil
	}
	return adapters.StructureKrakenSeparate, nil
}

// checkStructure confirms the account still has the Identity's Account
// Structure. The structure is declared once, while the owner can switch
// wallets inside Kraken at any time (ADR 0001).
func checkStructure(s *snapshot, structure adapters.AccountStructure) error {
	actual, err := structureOf(s)
	if err != nil {
		return err
	}
	if actual != structure {
		return adapters.StructureMismatch(adapters.PlatformKraken, structureWords[actual], structure)
	}
	return nil
}

// Validate confirms the keys reach one Kraken customer account with the Identity's Account
// Structure and returns that account's reference: the IBAN-like customer reference both key
// groups report, or the Futures accountUid for a Futures-only Identity whose key check has no
// iiban, which Kraken documents as nullable. The structure is read through the Futures balances
// feed, so a Spot-only (Earn) Identity has no structure check.
func (a *Adapter) Validate(ctx context.Context, creds adapters.Credentials, structure adapters.AccountStructure) (adapters.ValidationResult, error) {
	if !supported(structure) {
		return adapters.ValidationResult{}, adapters.UnsupportedStructure(adapters.PlatformKraken, structure)
	}
	return a.validate(ctx, a.client(creds), structure)
}

func (a *Adapter) validate(ctx context.Context, c *client, structure adapters.AccountStructure) (adapters.ValidationResult, error) {
	if err := adapters.CheckCredentials(adapters.PlatformKraken, structure, nil, c.creds); err != nil {
		return adapters.ValidationResult{}, err
	}
	var spotRef, futuresRef string
	if c.creds.APIKey != "" {
		var key struct {
			IBAN string `json:"iban"`
		}
		if err := c.call(ctx, epAPIKeyInfo, nil, &key); err != nil {
			return adapters.ValidationResult{}, err
		}
		if spotRef = normalizeRef(key.IBAN); spotRef == "" {
			return adapters.ValidationResult{}, adapters.Malformed("Kraken "+epAPIKeyInfo.String(), "iban")
		}
	}
	if c.creds.FuturesKey() != "" {
		var key struct {
			IBAN       string `json:"iiban"`
			AccountUID string `json:"accountUid"`
		}
		if err := c.call(ctx, epFuturesKey, nil, &key); err != nil {
			return adapters.ValidationResult{}, err
		}
		futuresRef = normalizeRef(key.IBAN)
		if futuresRef == "" && spotRef == "" {
			if futuresRef = strings.TrimSpace(key.AccountUID); futuresRef == "" {
				return adapters.ValidationResult{}, adapters.Malformed("Kraken "+epFuturesKey.String(), "iiban and accountUid")
			}
		}
		s, err := c.balancesSnapshot(ctx)
		if err != nil {
			return adapters.ValidationResult{}, err
		}
		if err := checkStructure(&s, structure); err != nil {
			return adapters.ValidationResult{}, err
		}
	}
	if spotRef != "" && futuresRef != "" && spotRef != futuresRef {
		return adapters.ValidationResult{}, adapters.NewError(adapters.ClassAuthentication, "The Kraken Spot and Futures keys belong to different customer accounts.")
	}
	if spotRef != "" {
		return adapters.ValidationResult{ExternalRef: spotRef}, nil
	}
	return adapters.ValidationResult{ExternalRef: futuresRef}, nil
}

// normalizeRef removes the spaces Kraken may print inside a customer reference.
func normalizeRef(ref string) string { return strings.ToUpper(strings.Join(strings.Fields(ref), "")) }

// run holds what one Collect call shares across its Accounts.
type run struct {
	earnMetadata map[string]earnStrategy
	a            *Adapter
	c            *client
	structure    adapters.AccountStructure
	assets       assetNames
	prices       spotPrices
	// snapshot is the balances snapshot; it is read only when an Account other than Earn is
	// requested, because only those Accounts depend on the wallet structure.
	snapshot *snapshot
	// open caches the open positions and their contracts, shared by Main or Multi-M and Coin-M.
	open *openBook
}

// Collect returns one Account Report per requested Account Type and the USD, USDT, or USDC
// prices of the reported Balances. The Account Structure is re-checked through the balances
// snapshot before any Account is read, so a wallet switched inside Kraken fails the batch instead
// of being collected under a stale label. An authentication failure fails the whole batch; any
// other failure fails only its Account, unless every Account failed with a retryable class.
func (a *Adapter) Collect(ctx context.Context, creds adapters.Credentials, structure adapters.AccountStructure, accounts []adapters.AccountType) (adapters.Batch, error) {
	if !supported(structure) {
		return adapters.Batch{}, adapters.UnsupportedStructure(adapters.PlatformKraken, structure)
	}
	return a.collect(ctx, a.client(creds), structure, accounts)
}

func (a *Adapter) collect(ctx context.Context, c *client, structure adapters.AccountStructure, accounts []adapters.AccountType) (adapters.Batch, error) {
	if err := adapters.CheckCredentials(adapters.PlatformKraken, structure, accounts, c.creds); err != nil {
		return adapters.Batch{}, err
	}
	r := &run{a: a, c: c, structure: structure}
	// Market data comes first so its observation time precedes every report of the batch, and
	// the valuation's "latest price at or before the report" picks it.
	assets, assetsErr := a.assetNames(ctx, c)
	r.assets = assets
	r.prices = a.spotPrices(ctx, c, assets)
	pricedAt := a.now()
	if needsSnapshot(accounts) {
		s, err := c.balancesSnapshot(ctx)
		if err != nil {
			return adapters.Batch{}, err
		}
		if err := checkStructure(&s, structure); err != nil {
			return adapters.Batch{}, err
		}
		r.snapshot = &s
	}
	reports, err := adapters.CollectAccounts(ctx, accounts, func(ctx context.Context, accountType adapters.AccountType) (adapters.AccountReport, error) {
		if !adapters.ValidAccountType(structure, accountType) {
			return adapters.AccountReport{}, adapters.UnknownAccountType(adapters.PlatformKraken, structure, accountType)
		}
		if assetsErr != nil {
			return adapters.AccountReport{}, assetsErr
		}
		return r.account(ctx, accountType)
	})
	if err != nil {
		return adapters.Batch{}, err
	}
	return adapters.Batch{
		ObservedAt: a.now(),
		Reports:    reports,
		Prices:     adapters.QuotePrices(reports, priceQuotes, r.prices.lookup, pricedAt),
	}, nil
}

// needsSnapshot reports whether an Account other than Earn is requested.
func needsSnapshot(accounts []adapters.AccountType) bool {
	for _, accountType := range accounts {
		if accountType != TypeEarn {
			return true
		}
	}
	return false
}

func (r *run) account(ctx context.Context, accountType adapters.AccountType) (adapters.AccountReport, error) {
	switch accountType { //nolint:exhaustive // Kraken supports only its declared Account Types, not Dual Investment.
	case TypeMain:
		if r.structure == adapters.StructureKrakenUnified {
			return r.multiCollateral(ctx, TypeMain)
		}
		return r.separateMain(ctx)
	case TypeMultiM:
		return r.multiCollateral(ctx, TypeMultiM)
	case TypeCoinM:
		return r.coinM(ctx)
	case TypeDerivativesHolding:
		return r.derivativesHolding()
	case TypeEarn:
		return r.earn(ctx)
	default:
		return adapters.AccountReport{}, adapters.UnknownAccountType(adapters.PlatformKraken, r.structure, accountType)
	}
}

// snapshot is the balances_snapshot message of the Futures balances feed.
type snapshot struct {
	Timestamp int64                   `json:"timestamp"`
	Holding   map[string]number       `json:"holding"`
	Futures   map[string]singleWallet `json:"futures"`
	Flex      multiWallet             `json:"flex_futures"`
	// FuturesUPnLCashValue is the USD value of every single-collateral wallet with its
	// unrealized profit.
	FuturesUPnLCashValue number `json:"futures_upnl_cash_value"`
}

// observedAt is Kraken's own time of the balance state, or now when the snapshot has none.
func (s *snapshot) observedAt(now time.Time) time.Time {
	if s.Timestamp > 0 {
		return time.UnixMilli(s.Timestamp).UTC()
	}
	return now
}

// multiWallet is the multi-collateral wallet: Multi-M in Separate wallets, Main in Unified Wallet.
type multiWallet struct {
	Isolated          map[string]isolatedWallet     `json:"isolated"`
	UnifiedBalances   *bool                         `json:"unified_balances"`
	Currencies        map[string]collateralCurrency `json:"currencies"`
	PortfolioValue    number                        `json:"portfolio_value"`
	MarginEquity      number                        `json:"margin_equity"`
	AvailableMargin   number                        `json:"available_margin"`
	InitialMargin     number                        `json:"initial_margin"`
	MaintenanceMargin number                        `json:"maintenance_margin"`
}

type isolatedWallet struct {
	InitialWithoutOrders number `json:"initial_margin_without_orders"`
}

// collateralCurrency is one currency of the multi-collateral wallet.
type collateralCurrency struct {
	Quantity  number `json:"quantity"`
	Available number `json:"available"`
}

// singleWallet is one single-collateral (Coin-M) wallet, valued in its own unit.
type singleWallet struct {
	Unit           string `json:"unit"`
	PortfolioValue number `json:"portfolio_value"`
}

// multiCollateral maps the multi-collateral wallet: Main in Unified Wallet, Multi-M in Separate
// wallets. Its equity is portfolio_value, which already holds unrealized profit and funding;
// margin_equity is the haircut-adjusted collateral Kraken margins against. Each currency is a
// Balance in coin units; the wallet's derivative profit is not split among currencies.
func (r *run) multiCollateral(ctx context.Context, accountType adapters.AccountType) (adapters.AccountReport, error) {
	where := "Kraken " + epBalancesSnapshot.String()
	wallet := &r.snapshot.Flex
	if _, err := adapters.Decimal(where, "flex_futures.portfolio_value", string(wallet.PortfolioValue)); err != nil {
		return adapters.AccountReport{}, err
	}
	if wallet.Currencies == nil {
		return adapters.AccountReport{}, adapters.Malformed(where, "flex_futures.currencies")
	}
	figures := map[string]number{"margin_equity": wallet.MarginEquity, "initial_margin": wallet.InitialMargin, "maintenance_margin": wallet.MaintenanceMargin}
	parsed := map[string]*big.Rat{}
	for name, value := range figures {
		rat, err := adapters.OptionalDecimal(where, "flex_futures."+name, string(value))
		if err != nil {
			return adapters.AccountReport{}, err
		}
		parsed[name] = rat
	}
	report := adapters.AccountReport{
		AccountType:     accountType,
		Status:          adapters.StatusOK,
		ObservedAt:      r.snapshot.observedAt(r.a.now()),
		Equity:          adapters.Plain(string(wallet.PortfolioValue)),
		AdjustedEquity:  adapters.Plain(string(wallet.MarginEquity)),
		AvailableEquity: adapters.Plain(string(wallet.AvailableMargin)),
	}
	// Kraken reports the margin requirements as USD amounts, like Binance and OKX, so the stored
	// ratios are derived against the adjusted equity.
	if adjusted := parsed["margin_equity"]; adjusted.Sign() > 0 {
		if wallet.InitialMargin != "" {
			report.InitialMarginRatio = adapters.Ratio(parsed["initial_margin"], adjusted)
		}
		if wallet.MaintenanceMargin != "" {
			report.MaintenanceMarginRatio = adapters.Ratio(parsed["maintenance_margin"], adjusted)
		}
	}
	for _, code := range adapters.SortedKeys(wallet.Currencies) {
		row := wallet.Currencies[code]
		quantity, err := adapters.Decimal(where, "flex_futures.currencies."+code+".quantity", string(row.Quantity))
		if err != nil {
			return adapters.AccountReport{}, err
		}
		if quantity.Sign() == 0 {
			continue
		}
		symbol, err := r.assets.symbol(where, code)
		if err != nil {
			return adapters.AccountReport{}, err
		}
		report.Balances = append(report.Balances, adapters.Balance{
			Symbol:           symbol,
			Equity:           decimal.Format(quantity),
			AvailableToTrade: adapters.Plain(string(row.Available)),
		})
	}
	positions, err := r.positions(ctx, true)
	if err != nil {
		return adapters.AccountReport{}, err
	}
	report.Positions = positions
	return report, nil
}

// coinM maps the single-collateral wallets: one Balance per settlement unit in coin units, and
// futures_upnl_cash_value, already in USD with unrealized profit, as the equity.
func (r *run) coinM(ctx context.Context) (adapters.AccountReport, error) {
	where := "Kraken " + epBalancesSnapshot.String()
	if r.snapshot.Futures == nil {
		return adapters.AccountReport{}, adapters.Malformed(where, "futures")
	}
	if _, err := adapters.Decimal(where, "futures_upnl_cash_value", string(r.snapshot.FuturesUPnLCashValue)); err != nil {
		return adapters.AccountReport{}, err
	}
	totals := adapters.Totals{}
	for _, name := range adapters.SortedKeys(r.snapshot.Futures) {
		wallet := r.snapshot.Futures[name]
		value, err := adapters.Decimal(where, "futures."+name+".portfolio_value", string(wallet.PortfolioValue))
		if err != nil {
			return adapters.AccountReport{}, err
		}
		symbol, err := r.assets.symbol(where, wallet.Unit)
		if err != nil {
			return adapters.AccountReport{}, err
		}
		if err := totals.Add(where, symbol, value); err != nil {
			return adapters.AccountReport{}, err
		}
	}
	positions, err := r.positions(ctx, false)
	if err != nil {
		return adapters.AccountReport{}, err
	}
	return adapters.AccountReport{
		AccountType: TypeCoinM,
		Status:      adapters.StatusOK,
		ObservedAt:  r.snapshot.observedAt(r.a.now()),
		Equity:      adapters.Plain(string(r.snapshot.FuturesUPnLCashValue)),
		Balances:    totals.Balances(),
		Positions:   positions,
	}, nil
}

// derivativesHolding maps the holding wallet, which only stores coins for the derivatives
// wallets and never trades.
func (r *run) derivativesHolding() (adapters.AccountReport, error) {
	where := "Kraken " + epBalancesSnapshot.String()
	if r.snapshot.Holding == nil {
		return adapters.AccountReport{}, adapters.Malformed(where, "holding")
	}
	totals := adapters.Totals{}
	for _, code := range adapters.SortedKeys(r.snapshot.Holding) {
		quantity, err := adapters.Decimal(where, "holding."+code, string(r.snapshot.Holding[code]))
		if err != nil {
			return adapters.AccountReport{}, err
		}
		symbol, err := r.assets.symbol(where, code)
		if err != nil {
			return adapters.AccountReport{}, err
		}
		if err := totals.Add(where, symbol, quantity); err != nil {
			return adapters.AccountReport{}, err
		}
	}
	return adapters.AccountReport{
		AccountType: TypeDerivativesHolding,
		Status:      adapters.StatusOK,
		ObservedAt:  r.snapshot.observedAt(r.a.now()),
		Balances:    totals.Balances(),
	}, nil
}

// extendedBalance is one BalanceEx row.
type extendedBalance struct {
	Balance    number `json:"balance"`
	CreditUsed number `json:"credit_used"`
}

// allocatedSuffixes mark BalanceEx rows of funds allocated to Earn: .B yield-bearing products,
// .S staking, .M opt-in rewards. Earn reports them, so Main leaves them out. .F (Kraken Rewards,
// shown as Auto Earn) earns on Main balances and stays in Main.
var allocatedSuffixes = []string{".B", ".S", ".M"}

// separateMain maps Main in Separate wallets: its BalanceEx coins, and an equity that adds the
// net result of open Spot Margin positions (TradeBalance `n`) to the coins valued at their USD
// prices. TradeBalance `e` is never used, because it values the whole balance with Kraken's own
// rules. Spot Margin positions are not itemized, as with OKX.
func (r *run) separateMain(ctx context.Context) (adapters.AccountReport, error) {
	where := "Kraken " + epBalanceEx.String()
	var rows map[string]extendedBalance
	if err := r.c.call(ctx, epBalanceEx, nil, &rows); err != nil {
		return adapters.AccountReport{}, err
	}
	if rows == nil {
		return adapters.AccountReport{}, adapters.Malformed(where, "the balance list")
	}
	observedAt := r.a.now()
	totals := adapters.Totals{}
	for _, code := range adapters.SortedKeys(rows) {
		row := rows[code]
		used, err := adapters.OptionalDecimal(where, "credit_used of "+code, string(row.CreditUsed))
		if err != nil {
			return adapters.AccountReport{}, err
		}
		if used.Sign() != 0 {
			return adapters.AccountReport{}, adapters.NewError(adapters.ClassUnsupported, "Kraken Main uses credit, which Plutus does not support.")
		}
		amount, err := adapters.Decimal(where, "balance of "+code, string(row.Balance))
		if err != nil {
			return adapters.AccountReport{}, err
		}
		symbol, inMain, err := r.assets.mainSymbol(where, code)
		if err != nil {
			return adapters.AccountReport{}, err
		}
		if inMain {
			if err := totals.Add(where, symbol, amount); err != nil {
				return adapters.AccountReport{}, err
			}
		}
	}
	var trade struct {
		Net number `json:"n"`
	}
	if err := r.c.call(ctx, epTradeBalance, url.Values{"asset": {"ZUSD"}}, &trade); err != nil {
		return adapters.AccountReport{}, err
	}
	net, err := adapters.Decimal("Kraken "+epTradeBalance.String(), "n", string(trade.Net))
	if err != nil {
		return adapters.AccountReport{}, err
	}
	report := adapters.AccountReport{AccountType: TypeMain, Status: adapters.StatusOK, ObservedAt: observedAt, Balances: totals.Balances()}
	equity, complete := net, true
	for _, balance := range report.Balances {
		price, ok := adapters.USDPrice(balance.Symbol, priceQuotes, r.prices.lookup)
		if !ok {
			complete = false
			continue
		}
		equity = decimal.Add(equity, decimal.Mul(decimal.MustParse(balance.Equity), price))
	}
	switch {
	case complete:
		report.Equity = decimal.FormatPtr(equity)
	case net.Sign() != 0:
		// Without an equity the value is calculated from the Balances, which cannot hold the
		// margin result; valued incomplete, it would silently lose that profit or loss.
		return adapters.AccountReport{}, adapters.NewError(adapters.ClassNormalization, "Kraken Main has an open Spot Margin result and a balance without a price, so its net equity cannot be determined.")
	}
	return report, nil
}

// earnInMain lists the lock types Kraken documents. flex strategies are Kraken Rewards, which earn
// on Main balances and stay in Main; the other lock types hold funds allocated to Earn.
var earnInMain = map[string]bool{"flex": true, "instant": false, "bonded": false, "timed": false}

// maxStrategyPages bounds the strategy pages one collection follows.
const maxStrategyPages = 1000

// earn maps allocated Earn: every allocation of an instant, bonded, or timed strategy, one Balance
// per coin. A nonzero allocation whose strategy is unknown or has an undocumented lock type fails
// Earn: dropping it would erase real holdings under the Complete Set rule.
func (r *run) earn(ctx context.Context) (adapters.AccountReport, error) {
	strategies, err := r.earnStrategies(ctx)
	if err != nil {
		return adapters.AccountReport{}, err
	}
	where := "Kraken " + epEarnAllocations.String()
	var allocations struct {
		Items      []earnAllocation `json:"items"`
		NextCursor *string          `json:"next_cursor"`
	}
	if err := r.c.call(ctx, epEarnAllocations, url.Values{"converted_asset": {"USD"}, "hide_zero_allocations": {"true"}}, &allocations); err != nil {
		return adapters.AccountReport{}, err
	}
	// Kraken documents that allocations are not paged; a cursor would mean a partial list.
	if allocations.Items == nil || (allocations.NextCursor != nil && *allocations.NextCursor != "") {
		return adapters.AccountReport{}, adapters.Malformed(where, "the complete allocation list")
	}
	totals := adapters.Totals{}
	positions := []adapters.EarnPosition{}
	for i := range allocations.Items {
		row := &allocations.Items[i]
		amount, err := adapters.Decimal(where, "amount_allocated.total.native of "+row.NativeAsset, string(row.Allocated.Total.Native))
		if err != nil {
			return adapters.AccountReport{}, err
		}
		// Kraken can return past allocations after their strategies are no longer available.
		// A zero total has no holdings to classify, even if the filter above is ignored.
		if amount.Sign() == 0 {
			continue
		}
		lockType, ok := strategies[row.StrategyID]
		if !ok {
			return adapters.AccountReport{}, adapters.Malformed(where, "the strategy of an allocation")
		}
		inMain, known := earnInMain[lockType]
		if !known {
			return adapters.AccountReport{}, adapters.NewError(adapters.ClassNormalization, fmt.Sprintf("%s: strategy lock type %q is not documented.", where, lockType))
		}
		if inMain {
			continue
		}
		symbol, err := r.assets.symbol(where, row.NativeAsset)
		if err != nil {
			return adapters.AccountReport{}, err
		}
		if err := totals.Add(where, symbol, amount); err != nil {
			return adapters.AccountReport{}, err
		}
		positions = append(positions, row.position(symbol, decimal.Format(amount), r.earnMetadata[row.StrategyID]))
	}
	balances := totals.Balances()
	if err := adapters.CheckEarnPositions(positions, balances); err != nil {
		return adapters.AccountReport{}, err
	}
	return adapters.AccountReport{AccountType: TypeEarn, Status: adapters.StatusOK, ObservedAt: r.a.now(), Balances: balances, EarnDetailsCollected: true, EarnPositions: positions}, nil
}

// earnStrategies returns the lock type of every strategy, following next_cursor until Kraken
// stops returning one. A repeated cursor would loop forever and fails Earn instead.
func (r *run) earnStrategies(ctx context.Context) (map[string]string, error) {
	where := "Kraken " + epEarnStrategies.String()
	out := map[string]string{}
	r.earnMetadata = map[string]earnStrategy{}
	seen := map[string]bool{}
	var params url.Values
	for range maxStrategyPages {
		var page struct {
			Items      []earnStrategy `json:"items"`
			NextCursor *string        `json:"next_cursor"`
		}
		if err := r.c.call(ctx, epEarnStrategies, params, &page); err != nil {
			return nil, err
		}
		if page.Items == nil {
			return nil, adapters.Malformed(where, "the strategy list")
		}
		for i := range page.Items {
			strategy := &page.Items[i]
			if strategy.ID == "" || strategy.LockType.Type == "" {
				return nil, adapters.Malformed(where, "a strategy id or lock type")
			}
			out[strategy.ID] = strategy.LockType.Type
			r.earnMetadata[strategy.ID] = *strategy
		}
		if page.NextCursor == nil || *page.NextCursor == "" {
			return out, nil
		}
		if seen[*page.NextCursor] {
			return nil, adapters.Malformed(where, "the strategy pagination")
		}
		seen[*page.NextCursor] = true
		params = url.Values{"cursor": {*page.NextCursor}}
	}
	return nil, adapters.Malformed(where, "the strategy pagination")
}

// assetNames maps Kraken asset codes such as XXBT and ZUSD to the public names Assets lists for
// them (XBT, USD).
type assetNames map[string]string

// aliases are the legacy Kraken names that differ from the symbols Plutus
// merges by (ADR 0003).
var aliases = map[string]string{"XBT": "BTC", "XXBT": "BTC", "XDG": "DOGE", "XXDG": "DOGE", "ZUSD": "USD"}

// assetNames reads the public asset list.
func (a *Adapter) assetNames(ctx context.Context, c *client) (assetNames, error) {
	var rows map[string]struct {
		Altname string `json:"altname"`
	}
	if err := c.call(ctx, epAssets, nil, &rows); err != nil {
		return nil, err
	}
	out := make(assetNames, len(rows))
	for code, row := range rows {
		out[code] = row.Altname
	}
	return out, nil
}

// symbol normalizes a Kraken asset code: its public name when Assets lists the code, then the
// aliases above. Genuine suffixes such as .T (tokenized assets) are kept.
func (n assetNames) symbol(where, code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", adapters.Malformed(where, "an asset code")
	}
	if name := n[code]; name != "" {
		code = name
	}
	code = strings.ToUpper(code)
	if alias, ok := aliases[code]; ok {
		return alias, nil
	}
	return code, nil
}

// mainSymbol normalizes a BalanceEx code and reports whether the row belongs to Main.
func (n assetNames) mainSymbol(where, code string) (string, bool, error) {
	for _, suffix := range allocatedSuffixes {
		if strings.HasSuffix(code, suffix) {
			return "", false, nil
		}
	}
	symbol, err := n.symbol(where, strings.TrimSuffix(code, ".F"))
	return symbol, true, err
}

// spotPrices holds the last Spot price of each asset per quote.
type spotPrices map[string]map[string]string

// lookup is the adapters.QuoteLookup of the Spot prices.
func (p spotPrices) lookup(symbol, quote string) (string, bool) {
	price, ok := p[symbol][quote]
	return price, ok
}

// spotPrices returns the last Spot price of every asset with a USD, USDT, or USDC pair. A failed
// call yields no prices rather than failing the batch: valuation falls back to the latest price
// within 24 hours.
func (a *Adapter) spotPrices(ctx context.Context, c *client, assets assetNames) spotPrices {
	var pairs map[string]struct {
		Base  string `json:"base"`
		Quote string `json:"quote"`
	}
	if c.call(ctx, epAssetPairs, nil, &pairs) != nil {
		return nil
	}
	var tickers map[string]struct {
		Close []number `json:"c"`
	}
	if c.call(ctx, epSpotTicker, nil, &tickers) != nil {
		return nil
	}
	out := spotPrices{}
	for _, name := range adapters.SortedKeys(pairs) {
		base, err := assets.symbol("", pairs[name].Base)
		if err != nil {
			continue
		}
		quote, err := assets.symbol("", pairs[name].Quote)
		if err != nil || !isPriceQuote(quote) {
			continue
		}
		ticker, ok := tickers[name]
		if !ok || len(ticker.Close) == 0 || out[base][quote] != "" {
			continue
		}
		if last, ok := decimal.Parse(string(ticker.Close[0])); ok && decimal.Valid(string(ticker.Close[0])) && last.Sign() > 0 {
			if out[base] == nil {
				out[base] = map[string]string{}
			}
			out[base][quote] = string(ticker.Close[0])
		}
	}
	return out
}

func isPriceQuote(symbol string) bool {
	for _, quote := range priceQuotes {
		if symbol == quote {
			return true
		}
	}
	return false
}
