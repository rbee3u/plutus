// Package okx is the read-only OKX adapter for the Unified account, Account
// Structure okx_unified, in all four Margin Modes OKX offers: Spot mode,
// Futures mode, Multi-currency margin, and Portfolio margin.
//
// It calls only balance, position, account, and market-data endpoints. Raw
// responses are decoded inside this package and never cross the adapter
// boundary; the one exception is Record, the explicit response recorder of ADR
// 0009. Field mappings are documented in docs/adapters/okx/mapping.md.
package okx

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/httpclient"
	"github.com/rbee3u/plutus/internal/decimal"
)

// Account Types of the okx_unified Account Structure, the same in every Margin Mode.
const (
	TypeDualInvestment                      = adapters.TypeDualInvestment
	TypeTrading        adapters.AccountType = "trading"
	TypeFunding        adapters.AccountType = "funding"
	TypeEarn           adapters.AccountType = "earn"
)

// marginModes maps OKX's account level, the acctLv of account/config, to its Margin Mode. OKX
// calls the levels account modes; all four are the same Unified account with the same Accounts.
var marginModes = map[string]adapters.MarginMode{
	"1": adapters.MarginOKXSpot,
	"2": adapters.MarginOKXFutures,
	"3": adapters.MarginOKXMCM,
	"4": adapters.MarginOKXPM,
}

// quotes are the spot quote coins prices are read in, in order of preference.
// Both value at 1 USD (ADR 0003).
var quotes = []string{"USDT", "USDC"}

// pageInterval spaces two pages of an endpoint OKX limits to one request per second.
const pageInterval = 1100 * time.Millisecond

// Options configures an Adapter. Zero values select production defaults.
type Options struct {
	Host string
	HTTP *http.Client
	Now  func() time.Time
	// HTTPFactory creates a client with the Adapter timeout when HTTP is nil.
	HTTPFactory func(time.Duration) *http.Client
}

// Adapter is the OKX adapter. It is safe for concurrent use.
type Adapter struct {
	host        string
	http        *http.Client
	clock       *clock
	instruments *instrumentCache
	pace        time.Duration
}

// New constructs an OKX adapter; nil options select production defaults.
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
	return &Adapter{
		host:        strings.TrimSuffix(opts.Host, "/"),
		http:        opts.HTTP,
		clock:       &clock{now: opts.Now},
		instruments: newInstrumentCache(opts.Now),
		pace:        pageInterval,
	}
}

func (a *Adapter) client(creds adapters.Credentials) *client {
	return &client{http: a.http, host: a.host, clock: a.clock, creds: creds}
}

func (a *Adapter) now() time.Time { return a.clock.now().UTC() }

// wait holds the next page of a rate-limited endpoint for the adapter's pace.
func (a *Adapter) wait(ctx context.Context) error {
	if a.pace <= 0 {
		return nil
	}
	timer := time.NewTimer(a.pace)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type accountConfig struct {
	UID     string `json:"uid"`
	MainUID string `json:"mainUid"`
	AcctLv  string `json:"acctLv"`
}

// config reads the account configuration: the account's own UID and its Margin
// Mode. The User can switch the level inside OKX at any time, so every
// validation and every Collection Run reads it afresh (ADR 0001).
func (a *Adapter) config(ctx context.Context, c *client) (accountConfig, adapters.MarginMode, error) {
	var rows []accountConfig
	if err := c.call(ctx, epConfig, &rows); err != nil {
		return accountConfig{}, "", err
	}
	if len(rows) == 0 {
		return accountConfig{}, "", adapters.Malformed(epConfig.String(), "the account configuration")
	}
	mode, err := marginModeOf(rows[0].AcctLv)
	if err != nil {
		return accountConfig{}, "", err
	}
	return rows[0], mode, nil
}

// marginModeOf maps an account level. A level OKX adds later is refused by name instead of being
// collected with a mapping written for another level.
func marginModeOf(level string) (adapters.MarginMode, error) {
	level = strings.TrimSpace(level)
	if level == "" {
		return "", adapters.Malformed(epConfig.String(), "acctLv")
	}
	mode, ok := marginModes[level]
	if !ok {
		return "", adapters.UnknownMarginMode(adapters.PlatformOKX, "acctLv "+level)
	}
	return mode, nil
}

// Validate confirms the key reaches an OKX account and returns the account's UID and Margin Mode.
// A sub-account key is accepted: OKX reports the sub-account's own UID and level for it.
func (a *Adapter) Validate(ctx context.Context, creds adapters.Credentials, structure adapters.AccountStructure) (adapters.ValidationResult, error) {
	if structure != adapters.StructureOKXUnified {
		return adapters.ValidationResult{}, adapters.UnsupportedStructure(adapters.PlatformOKX, structure)
	}
	cfg, mode, err := a.config(ctx, a.client(creds))
	if err != nil {
		return adapters.ValidationResult{}, err
	}
	uid := strings.TrimSpace(cfg.UID)
	if uid == "" {
		return adapters.ValidationResult{}, adapters.Malformed(epConfig.String(), "uid")
	}
	return adapters.ValidationResult{ExternalRef: uid, MarginMode: mode}, nil
}

// Collect returns one Account Report per requested Account Type, the Margin Mode read at the start
// of the run, and the USDT/USDC prices of the reported Balances. An authentication or
// authorization failure fails the whole batch; any other failure fails only its Account, unless
// every Account failed with a retryable class.
func (a *Adapter) Collect(ctx context.Context, creds adapters.Credentials, structure adapters.AccountStructure, accounts []adapters.AccountType) (adapters.Batch, error) {
	if structure != adapters.StructureOKXUnified {
		return adapters.Batch{}, adapters.UnsupportedStructure(adapters.PlatformOKX, structure)
	}
	c := a.client(creds)
	_, mode, err := a.config(ctx, c)
	if err != nil {
		return adapters.Batch{}, err
	}
	// Prices come first so their observation time precedes every report of the batch, and the
	// valuation's "latest price at or before the report" picks them.
	lookup, pricedAt := a.tickers(ctx, c)
	usdPrice := func(symbol string) (*big.Rat, bool) { return adapters.USDPrice(symbol, quotes, lookup) }
	earnPair := adapters.CollectEarnPair(func(ctx context.Context) (adapters.AccountReport, adapters.AccountReport, error) {
		return a.earn(ctx, c, usdPrice)
	})
	reports, err := adapters.CollectAccounts(ctx, accounts, func(ctx context.Context, accountType adapters.AccountType) (adapters.AccountReport, error) {
		switch accountType {
		case TypeTrading:
			return a.trading(ctx, c)
		case TypeFunding:
			return a.funding(ctx, c)
		case TypeEarn, TypeDualInvestment:
			return earnPair(ctx, accountType)
		default:
			return adapters.AccountReport{}, adapters.UnknownAccountType(adapters.PlatformOKX, structure, accountType)
		}
	})
	if err != nil {
		return adapters.Batch{}, err
	}
	return adapters.Batch{
		ObservedAt: a.now(),
		MarginMode: mode,
		Reports:    reports,
		Prices:     adapters.QuotePrices(reports, quotes, lookup, pricedAt),
	}, nil
}

type tradingAccount struct {
	TotalEq  string           `json:"totalEq"`
	AdjEq    string           `json:"adjEq"`
	AvailEq  string           `json:"availEq"`
	IMR      string           `json:"imr"`
	MMR      string           `json:"mmr"`
	MgnRatio string           `json:"mgnRatio"`
	Details  []tradingBalance `json:"details"`
}

type tradingBalance struct {
	Ccy      string `json:"ccy"`
	Eq       string `json:"eq"`
	AvailEq  string `json:"availEq"`
	AvailBal string `json:"availBal"`
	Liab     string `json:"liab"`
	Interest string `json:"interest"`
	Upl      string `json:"upl"`
}

// trading maps the trading account: its USD equity and margin figures, its
// per-coin equities, and its open positions. One mapping serves all four
// Margin Modes: OKX returns "" for every field that does not apply to the
// account's level (docs/adapters/okx/mapping.md lists them per mode), and an
// empty field stays unknown rather than becoming zero.
func (a *Adapter) trading(ctx context.Context, c *client) (adapters.AccountReport, error) {
	var rows []tradingAccount
	if err := c.call(ctx, epBalance, &rows); err != nil {
		return adapters.AccountReport{}, err
	}
	if len(rows) == 0 {
		return adapters.AccountReport{}, adapters.Malformed(epBalance.String(), "the trading account")
	}
	account := &rows[0]
	report, err := mapTradingAccount(account)
	if err != nil {
		return adapters.AccountReport{}, err
	}
	report.ObservedAt = a.now()
	if report.Balances, err = mapTradingBalances(account.Details); err != nil {
		return adapters.AccountReport{}, err
	}
	if report.Positions, err = a.positions(ctx, c); err != nil {
		return adapters.AccountReport{}, err
	}
	return report, nil
}

// mapTradingAccount reads the account-level figures. Futures mode reports none of them but
// totalEq, and Spot mode no availEq.
func mapTradingAccount(in *tradingAccount) (adapters.AccountReport, error) {
	where := epBalance.String()
	if _, err := adapters.Decimal(where, "totalEq", in.TotalEq); err != nil {
		return adapters.AccountReport{}, err
	}
	parsed, err := optionalFields(where, "",
		field{"adjEq", in.AdjEq}, field{"availEq", in.AvailEq}, field{"imr", in.IMR}, field{"mmr", in.MMR}, field{"mgnRatio", in.MgnRatio})
	if err != nil {
		return adapters.AccountReport{}, err
	}
	report := adapters.AccountReport{
		AccountType:        TypeTrading,
		Status:             adapters.StatusOK,
		Equity:             adapters.Plain(in.TotalEq),
		AdjustedEquity:     adapters.Plain(in.AdjEq),
		AvailableEquity:    adapters.Plain(in.AvailEq),
		UnifiedMarginRatio: adapters.Plain(in.MgnRatio),
	}
	// OKX reports the margin requirements as USD amounts, like Binance, so the stored ratios are
	// derived against the adjusted equity that OKX itself divides by.
	if adjusted := parsed["adjEq"]; adjusted.Sign() > 0 {
		if strings.TrimSpace(in.IMR) != "" {
			report.InitialMarginRatio = adapters.Ratio(parsed["imr"], adjusted)
		}
		if strings.TrimSpace(in.MMR) != "" {
			report.MaintenanceMarginRatio = adapters.Ratio(parsed["mmr"], adjusted)
		}
	}
	return report, nil
}

// mapTradingBalances maps details[] rows. Equity is OKX's own `eq`, never recomputed: the sum of
// the rows' USD equities is exactly the account's totalEq, while the composition of `eq` is not
// documented. An empty `eq` is unknown and fails the Account; read as zero, it would drop the coin
// under the Complete Set rule. Liabilities are reported positive and stored positive.
func mapTradingBalances(rows []tradingBalance) ([]adapters.Balance, error) {
	where := epBalance.String()
	out := make([]adapters.Balance, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		equity, err := adapters.Decimal(where, "eq of "+row.Ccy, row.Eq)
		if err != nil {
			return nil, err
		}
		parsed, err := optionalFields(where, row.Ccy,
			field{"availEq", row.AvailEq}, field{"availBal", row.AvailBal}, field{"liab", row.Liab},
			field{"interest", row.Interest}, field{"upl", row.Upl})
		if err != nil {
			return nil, err
		}
		nonzero := equity.Sign() != 0
		for _, value := range parsed {
			nonzero = nonzero || value.Sign() != 0
		}
		if !nonzero || strings.TrimSpace(row.Ccy) == "" {
			continue
		}
		out = append(out, adapters.Balance{
			Symbol:              row.Ccy,
			Equity:              decimal.Format(equity),
			AvailableToTrade:    adapters.Reported(row.AvailEq, parsed["availEq"]),
			AvailableToWithdraw: adapters.Reported(row.AvailBal, parsed["availBal"]),
			Liability:           adapters.Reported(row.Liab, decimal.Abs(parsed["liab"])),
			Interest:            adapters.Reported(row.Interest, parsed["interest"]),
			UnrealizedPnl:       adapters.Reported(row.Upl, parsed["upl"]),
		})
	}
	return out, nil
}

// field is one named value of a response row.
type field struct{ name, value string }

// optionalFields parses fields that OKX may leave empty, by name; an empty one reads as zero and a
// present non-decimal fails. coin, when set, names the row in the error.
func optionalFields(where, coin string, fields ...field) (map[string]*big.Rat, error) {
	out := make(map[string]*big.Rat, len(fields))
	for _, f := range fields {
		name := f.name
		if coin != "" {
			name += " of " + coin
		}
		value, err := adapters.OptionalDecimal(where, name, f.value)
		if err != nil {
			return nil, err
		}
		out[f.name] = value
	}
	return out, nil
}

type fundingBalance struct {
	Ccy      string `json:"ccy"`
	Bal      string `json:"bal"`
	AvailBal string `json:"availBal"`
}

// funding maps the funding account, which OKX reports without any USD valuation. Like `eq`, an
// empty `bal` fails the Account instead of reading as zero.
func (a *Adapter) funding(ctx context.Context, c *client) (adapters.AccountReport, error) {
	var rows []fundingBalance
	if err := c.call(ctx, epFunding, &rows); err != nil {
		return adapters.AccountReport{}, err
	}
	where := epFunding.String()
	balances := make([]adapters.Balance, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		total, err := adapters.Decimal(where, "bal of "+row.Ccy, row.Bal)
		if err != nil {
			return adapters.AccountReport{}, err
		}
		available, err := adapters.OptionalDecimal(where, "availBal of "+row.Ccy, row.AvailBal)
		if err != nil {
			return adapters.AccountReport{}, err
		}
		if total.Sign() == 0 || strings.TrimSpace(row.Ccy) == "" {
			continue
		}
		balances = append(balances, adapters.Balance{
			Symbol:              row.Ccy,
			Equity:              decimal.Format(total),
			AvailableToWithdraw: adapters.Reported(row.AvailBal, available),
		})
	}
	return adapters.AccountReport{AccountType: TypeFunding, Status: adapters.StatusOK, ObservedAt: a.now(), Balances: balances}, nil
}

type savingsBalance struct {
	Ccy string `json:"ccy"`
	Amt string `json:"amt"`
}

type stakingOrder struct {
	InvestData []struct {
		Ccy string `json:"ccy"`
		Amt string `json:"amt"`
	} `json:"investData"`
}

// earn maps the Earn bucket, one Balance per coin: Simple Earn flexible
// balances, the principal of active on-chain earn orders, and the principal of
// dual investment orders still invested. An empty amount fails the Account.
// ETH and SOL staking are deliberately not collected: OKX reports those
// balances account-wide, and the BETH and OKSOL coins they produce are already
// held in the trading or funding account. Last, the Earn check compares the
// Balances with OKX's own valuation of Earn (ADR 0005), so unitemized value
// fails selected Earn and Dual Accounts instead of vanishing.
func (a *Adapter) earn(ctx context.Context, c *client, usdPrice func(symbol string) (*big.Rat, bool)) (adapters.AccountReport, adapters.AccountReport, error) {
	totals := adapters.Totals{}
	var savings []savingsBalance
	if err := c.call(ctx, epSavings, &savings); err != nil {
		return adapters.AccountReport{}, adapters.AccountReport{}, err
	}
	for i := range savings {
		amount, err := adapters.Decimal(epSavings.String(), "amt of "+savings[i].Ccy, savings[i].Amt)
		if err != nil {
			return adapters.AccountReport{}, adapters.AccountReport{}, err
		}
		if err := totals.Add(epSavings.String(), savings[i].Ccy, amount); err != nil {
			return adapters.AccountReport{}, adapters.AccountReport{}, err
		}
	}
	var staking []stakingOrder
	if err := c.call(ctx, epStaking, &staking); err != nil {
		return adapters.AccountReport{}, adapters.AccountReport{}, err
	}
	for i := range staking {
		for _, invested := range staking[i].InvestData {
			amount, err := adapters.Decimal(epStaking.String(), "investData.amt of "+invested.Ccy, invested.Amt)
			if err != nil {
				return adapters.AccountReport{}, adapters.AccountReport{}, err
			}
			if err := totals.Add(epStaking.String(), invested.Ccy, amount); err != nil {
				return adapters.AccountReport{}, adapters.AccountReport{}, err
			}
		}
	}
	orders, err := a.dualInvestmentOrders(ctx, c)
	if err != nil {
		return adapters.AccountReport{}, adapters.AccountReport{}, err
	}
	positions, err := dualPositions(orders)
	if err != nil {
		return adapters.AccountReport{}, adapters.AccountReport{}, err
	}
	balances := totals.Balances()
	dualTotals := adapters.Totals{}
	for i := range positions {
		p := &positions[i]
		if err := dualTotals.Add("Dual Investment", p.InvestSymbol, decimal.MustParse(p.Principal)); err != nil {
			return adapters.AccountReport{}, adapters.AccountReport{}, err
		}
	}
	dualBalances := dualTotals.Balances()
	combined := append(append([]adapters.Balance{}, balances...), dualBalances...)
	valuation, err := a.earnValuation(ctx, c)
	if err != nil {
		return adapters.AccountReport{}, adapters.AccountReport{}, err
	}
	if err := adapters.CheckEarnComplete(adapters.PlatformOKX, valuation, combined, usdPrice); err != nil {
		return adapters.AccountReport{}, adapters.AccountReport{}, err
	}
	return adapters.AccountReport{AccountType: TypeEarn, Status: adapters.StatusOK, ObservedAt: a.now(), Balances: balances}, adapters.AccountReport{AccountType: TypeDualInvestment, Status: adapters.StatusOK, ObservedAt: a.now(), Balances: dualBalances, DualPositions: positions}, nil
}

// earnValuation reads OKX's own valuation of the Earn bucket in USDT, which Plutus counts as USD.
func (a *Adapter) earnValuation(ctx context.Context, c *client) (*big.Rat, error) {
	var rows []struct {
		Details struct {
			Earn string `json:"earn"`
		} `json:"details"`
	}
	if err := c.call(ctx, epValuation, &rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 || strings.TrimSpace(rows[0].Details.Earn) == "" {
		return nil, adapters.Malformed(epValuation.String(), "the earn valuation")
	}
	return adapters.Decimal(epValuation.String(), "details.earn", rows[0].Details.Earn)
}

type openPosition struct {
	InstType string `json:"instType"`
	InstID   string `json:"instId"`
	PosSide  string `json:"posSide"`
	Pos      string `json:"pos"`
	MgnMode  string `json:"mgnMode"`
	AvgPx    string `json:"avgPx"`
	MarkPx   string `json:"markPx"`
	LiqPx    string `json:"liqPx"`
	Upl      string `json:"upl"`
	Lever    string `json:"lever"`
	Margin   string `json:"margin"`
}

// positions maps the open contract positions of the trading account. MARGIN rows are skipped:
// they are borrow-backed spot exposure whose value is already in the Balances' equity. An
// instrument kind Plutus cannot store fails the Account instead of disappearing.
func (a *Adapter) positions(ctx context.Context, c *client) ([]adapters.Position, error) {
	var rows []openPosition
	if err := c.call(ctx, epPositions, &rows); err != nil {
		return nil, err
	}
	out := make([]adapters.Position, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		instType := strings.ToUpper(strings.TrimSpace(row.InstType))
		if instType == instTypeMargin {
			continue
		}
		market, ok := markets[instType]
		if !ok {
			return nil, adapters.NewError(adapters.ClassNormalization,
				fmt.Sprintf("OKX reports an open %s position in %s, which Plutus cannot store.", instType, row.InstID))
		}
		amount, err := adapters.Decimal(epPositions.String(), "pos of "+row.InstID, row.Pos)
		if err != nil {
			return nil, err
		}
		// OKX emits zero-size placeholder rows when a position was moved out of isolated margin.
		if amount.Sign() == 0 {
			continue
		}
		instrument, err := a.instruments.lookup(ctx, c, market, row.InstID)
		if err != nil {
			return nil, err
		}
		position, err := mapPosition(row, amount, &instrument)
		if err != nil {
			return nil, err
		}
		out = append(out, position)
	}
	return out, nil
}

func mapPosition(row *openPosition, amount *big.Rat, instrument *adapters.Instrument) (adapters.Position, error) {
	if _, err := optionalFields(epPositions.String(), row.InstID,
		field{"avgPx", row.AvgPx}, field{"markPx", row.MarkPx}, field{"liqPx", row.LiqPx},
		field{"upl", row.Upl}, field{"lever", row.Lever}, field{"margin", row.Margin}); err != nil {
		return adapters.Position{}, err
	}
	// In net mode one row carries the netted position and its direction is the sign of pos; in
	// long/short mode OKX reports one row per side with a positive size.
	side := adapters.SideLong
	switch strings.ToLower(strings.TrimSpace(row.PosSide)) {
	case "short":
		side = adapters.SideShort
	case "long":
	default:
		if amount.Sign() < 0 {
			side = adapters.SideShort
		}
	}
	return adapters.Position{
		PlatformPositionRef: fmt.Sprintf("%s:%s", row.InstID, strings.ToLower(strings.TrimSpace(row.PosSide))),
		Instrument:          *instrument,
		Side:                side,
		Quantity:            decimal.Format(decimal.Abs(amount)),
		QuantityUnit:        adapters.UnitContracts,
		EntryPrice:          adapters.Plain(row.AvgPx),
		MarkPrice:           adapters.Plain(row.MarkPx),
		LiquidationPrice:    adapters.NonZero(row.LiqPx),
		UnrealizedPnl:       adapters.Plain(row.Upl),
		// Portfolio margin reports no leverage for cross positions and fills the field with "0".
		Leverage: adapters.NonZero(row.Lever),
		// Only isolated positions carry their own margin; cross positions leave it empty.
		Margin: adapters.Plain(row.Margin),
	}, nil
}

// tickers returns a lookup of every spot last price and the time it was read. A failure yields
// no prices rather than failing the batch: valuation falls back to the latest price within 24
// hours, and the Earn check is skipped.
func (a *Adapter) tickers(ctx context.Context, c *client) (adapters.QuoteLookup, time.Time) {
	var rows []struct {
		InstID string `json:"instId"`
		Last   string `json:"last"`
	}
	if err := c.call(ctx, epTickers, &rows); err != nil {
		return nil, time.Time{}
	}
	last := make(map[string]string, len(rows))
	for _, row := range rows {
		if decimal.Valid(row.Last) {
			last[row.InstID] = row.Last
		}
	}
	return spotLookup(last), a.now()
}

// spotLookup prices a coin through its <symbol>-<quote> spot pair.
func spotLookup(last map[string]string) adapters.QuoteLookup {
	return func(symbol, quote string) (string, bool) {
		price, ok := last[symbol+"-"+quote]
		return price, ok
	}
}
