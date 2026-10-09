// Package fake is a deterministic offline Adapter used by seeds, tests, and local lanes.
//
// Values derive from the API key and the current time so that consecutive cycles drift
// smoothly. The Margin Mode of a structure that has one is picked by the key. Keys with special
// prefixes reproduce failure classes and Platform-side changes:
//
//	invalid-*        authentication error (Connection becomes invalid)
//	forbidden-*      authorization error
//	ratelimit-*      rate-limited error on every call
//	flaky-*          temporary error on the first attempt of each minute, success afterwards
//	empty-*          success with no balances or positions
//	unpriced-*       adds a TOKENX Balance that has no price
//	switch-*         a different Margin Mode every minute, as if the owner kept switching it
//	restructured-*   the account reports another Account Structure (unsupported)
//
// The Platform account a key reaches is the key without its prefix: "flaky-alice" reaches the
// same account as "alice", and "bob" reaches another one, which validation then refuses.
package fake

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// Adapter is the deterministic fake.
type Adapter struct {
	platform adapters.Platform
	now      func() time.Time
	mu       sync.Mutex
	flaky    map[string]int64
}

// New constructs a fake adapter for one Platform.
func New(platform adapters.Platform) *Adapter {
	return &Adapter{platform: platform, now: time.Now, flaky: map[string]int64{}}
}

// WithClock overrides the clock (tests and seeds).
func (a *Adapter) WithClock(now func() time.Time) *Adapter {
	a.now = now
	return a
}

// Validate checks the key prefix and returns a deterministic external ref and Margin Mode.
func (a *Adapter) Validate(ctx context.Context, creds adapters.Credentials, structure adapters.AccountStructure) (adapters.ValidationResult, error) {
	if err := a.check(ctx, creds, structure); err != nil {
		return adapters.ValidationResult{}, err
	}
	key := scenarioKey(creds)
	if d, anchor, matched, err := parseDemoKey(key); matched {
		if err != nil {
			return adapters.ValidationResult{}, err
		}
		if d.Platform != a.platform || d.Structure != structure {
			return adapters.ValidationResult{}, adapters.UnsupportedStructure(a.platform, structure)
		}
		if err := demoValidate(d, anchor, a.now()); err != nil {
			return adapters.ValidationResult{}, err
		}
		return adapters.ValidationResult{ExternalRef: ExternalRef(key), MarginMode: demoMode(d, anchor, a.now())}, nil
	}
	return adapters.ValidationResult{ExternalRef: ExternalRef(key), MarginMode: MarginMode(structure, key, a.now())}, nil
}

// behaviorPrefixes are the key prefixes that select a behavior rather than an account.
var behaviorPrefixes = []string{"invalid-", "forbidden-", "ratelimit-", "flaky-", "empty-", "unpriced-", "switch-", "restructured-"}

// ExternalRef is the reference of the Platform account a key reaches: the key without its
// behavior prefix, so one account can be tried with every behavior.
func ExternalRef(apiKey string) string {
	if d, _, matched, err := parseDemoKey(apiKey); matched && err == nil {
		return "demo-" + d.ID
	}
	for _, prefix := range behaviorPrefixes {
		if rest, ok := strings.CutPrefix(apiKey, prefix); ok {
			apiKey = rest
			break
		}
	}
	return "uid-" + shortHash(apiKey)
}

// Collect produces one report per requested Account Type with drifting balances and prices.
func (a *Adapter) Collect(ctx context.Context, creds adapters.Credentials, structure adapters.AccountStructure, accounts []adapters.AccountType) (adapters.Batch, error) {
	if err := a.check(ctx, creds, structure); err != nil {
		return adapters.Batch{}, err
	}
	now := a.now().UTC().Truncate(time.Second)
	key := scenarioKey(creds)
	if d, anchor, matched, err := parseDemoKey(key); matched {
		if err != nil {
			return adapters.Batch{}, err
		}
		if d.Platform != a.platform || d.Structure != structure {
			return adapters.Batch{}, adapters.UnsupportedStructure(a.platform, structure)
		}
		return DemoBatch(d, anchor, now, accounts)
	}
	mode := MarginMode(structure, key, now)
	batch := adapters.Batch{ObservedAt: now, MarginMode: mode}
	scenario := Scenario(key, now)
	for _, symbol := range []string{"BTC", "ETH", "SOL", "BNB"} {
		batch.Prices = append(batch.Prices, adapters.Price{
			AssetSymbol: symbol,
			Price:       scenario.Price(symbol),
			QuoteSymbol: "USDT",
			ObservedAt:  now,
		})
	}
	for _, accountType := range accounts {
		if !adapters.ValidAccountType(structure, accountType) {
			batch.Reports = append(batch.Reports, adapters.FailedReport(accountType, adapters.UnknownAccountType(a.platform, structure, accountType)))
			continue
		}
		batch.Reports = append(batch.Reports, scenario.Report(mode, accountType, now, strings.HasPrefix(key, "empty-")))
	}
	return batch, nil
}

// check refuses a structure of another Platform, reproduces a changed structure, and applies the
// key's failure class.
func (a *Adapter) check(ctx context.Context, creds adapters.Credentials, structure adapters.AccountStructure) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !adapters.ValidStructure(a.platform, structure) {
		return adapters.UnsupportedStructure(a.platform, structure)
	}
	if err := a.credentialError(creds); err != nil {
		return err
	}
	if strings.HasPrefix(scenarioKey(creds), "restructured-") {
		return adapters.StructureMismatch(a.platform, "another account structure", structure)
	}
	return nil
}

// MarginMode is the Margin Mode the fake reports for a key within an Account Structure: one of the
// structure's modes picked by the key, a different one every minute for "switch-" keys, and empty
// for structures that have none.
func MarginMode(structure adapters.AccountStructure, apiKey string, at time.Time) adapters.MarginMode {
	modes := adapters.MarginModes(structure)
	if len(modes) == 0 {
		return ""
	}
	sum := sha256.Sum256([]byte(apiKey))
	index := binary.BigEndian.Uint64(sum[:8]) % uint64(len(modes))
	if strings.HasPrefix(apiKey, "switch-") {
		index = (index + uint64(at.UTC().Unix()/60)) % uint64(len(modes)) // #nosec G115 -- Modulo scenario rotation intentionally uses the timestamp bit pattern.
	}
	return modes[index].ID
}

func (a *Adapter) keyError(apiKey string) error {
	switch {
	case strings.HasPrefix(apiKey, "invalid-"):
		return adapters.NewError(adapters.ClassAuthentication, "The platform rejected the API key.")
	case strings.HasPrefix(apiKey, "forbidden-"):
		return adapters.NewError(adapters.ClassAuthorization, "The API key lacks read permission.")
	case strings.HasPrefix(apiKey, "ratelimit-"):
		return adapters.NewError(adapters.ClassRateLimited, "The platform rate limit was exceeded.")
	case strings.HasPrefix(apiKey, "flaky-"):
		minute := a.now().UTC().Unix() / 60
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.flaky[apiKey] != minute {
			a.flaky[apiKey] = minute
			return adapters.NewError(adapters.ClassTemporary, "The platform returned a temporary error.")
		}
	}
	return nil
}

// ScenarioValues is the deterministic account state for one key at one instant.
type ScenarioValues struct {
	seed uint64
	at   time.Time
	// unpriced adds a TOKENX Balance with no market price to spot reports (keys prefixed "unpriced-").
	unpriced bool
}

// Scenario derives deterministic values from the key and time.
func Scenario(apiKey string, at time.Time) ScenarioValues {
	sum := sha256.Sum256([]byte(apiKey))
	return ScenarioValues{seed: binary.BigEndian.Uint64(sum[:8]), at: at.UTC(), unpriced: strings.HasPrefix(apiKey, "unpriced-")}
}

// Price returns a slowly drifting USDT price for a symbol; TOKENX has no price.
func (s ScenarioValues) Price(symbol string) string {
	base := map[string]float64{"BTC": 65000, "ETH": 3200, "SOL": 150, "BNB": 600}[symbol]
	if base == 0 {
		return ""
	}
	hours := float64(s.at.Unix()) / 3600
	drift := 1 + 0.06*math.Sin(hours/37) + 0.02*math.Sin(hours/5+float64(symbol[0]))
	return fmt.Sprintf("%.2f", base*drift)
}

// Report builds the deterministic report for one Account Type under a Margin Mode (empty for
// structures without one). Like the live adapters, a Margin Mode that does not report a figure
// leaves it unknown instead of zero.
func (s ScenarioValues) Report(mode adapters.MarginMode, accountType adapters.AccountType, at time.Time, empty bool) adapters.AccountReport {
	report := s.report(accountType, at, empty)
	if report.Status != adapters.StatusOK || empty {
		return report
	}
	switch mode {
	case adapters.MarginOKXFutures:
		// Futures mode reports total equity but no account-level margin figures.
		report.AdjustedEquity, report.AvailableEquity, report.InitialMarginRatio, report.MaintenanceMarginRatio, report.UnifiedMarginRatio = nil, nil, nil, nil, nil
	case adapters.MarginBybitIsolated:
		// Isolated Margin has no account-wide figures; the value is calculated from Balances.
		report.Equity, report.AdjustedEquity, report.AvailableEquity, report.InitialMarginRatio, report.MaintenanceMarginRatio, report.UnifiedMarginRatio = nil, nil, nil, nil, nil, nil
	case adapters.MarginBybitPM, adapters.MarginOKXPM:
		// Portfolio Margin positions carry no leverage, position margin, or liquidation price.
		for i := range report.Positions {
			report.Positions[i].Leverage, report.Positions[i].Margin, report.Positions[i].LiquidationPrice = nil, nil, nil
		}
	case adapters.MarginOKXSpot, adapters.MarginOKXMCM, adapters.MarginBybitCross:
		// These Margin Modes report every figure.
	default:
		// Structures without a Margin Mode keep the base report.
	}
	for i := range report.Positions {
		position := &report.Positions[i]
		position.PositionMode = "one_way"
		if i > 0 {
			position.PositionMode = "hedged"
		}
		position.MarginMode, position.MarginCurrency, position.MarginBasis = "cross", position.FuturesContract.SettlementSymbol, "initial"
		if mode == adapters.MarginBybitIsolated {
			position.MarginMode = "isolated"
		}
		if accountType == "portfolio_margin" || mode == adapters.MarginOKXPM || mode == adapters.MarginBybitPM {
			position.Margin = nil
		}
	}
	return report
}

func (s ScenarioValues) report(accountType adapters.AccountType, at time.Time, empty bool) adapters.AccountReport {
	report := adapters.AccountReport{AccountType: accountType, Status: adapters.StatusOK, ObservedAt: at}
	if empty {
		return report
	}
	f := s.factor(accountType)
	hours := float64(at.Unix()) / 3600
	wave := 1 + 0.03*math.Sin(hours/11+f)
	usdt := round(2, 5000*f*wave)
	btc := round(6, 0.25*f*wave)
	eth := round(4, 2.5*f)
	btcPrice := floatValue(s.Price("BTC"))
	ethPrice := floatValue(s.Price("ETH"))
	//nolint:exhaustive // Manual is application-owned and never enters private Adapter collection.
	switch accountType {
	case "portfolio_margin", "trading", "unified", "main", "multi_m", "coin_m":
		equity := round(2, usdt+btc*btcPrice+eth*ethPrice-1200*f)
		report.Equity = amountPtr(equity)
		report.AdjustedEquity = amountPtr(round(2, equity*0.97))
		report.AvailableEquity = amountPtr(round(2, equity*0.55))
		report.InitialMarginRatio = amountPtr(round(4, 0.18+0.02*math.Sin(hours/7)))
		report.MaintenanceMarginRatio = amountPtr(round(4, 0.09+0.01*math.Sin(hours/7)))
		report.UnifiedMarginRatio = amountPtr(round(4, 3.1+0.4*math.Sin(hours/13+f)))
		report.Balances = []adapters.Balance{
			{PlatformAssetSymbol: "USDT", Name: "Tether", Equity: amountString(usdt), AvailableToWithdraw: amountPtr(round(2, usdt*0.5)), AvailableToTrade: amountPtr(round(2, usdt*0.9))},
			{PlatformAssetSymbol: "BTC", Name: "Bitcoin", Equity: amountString(btc), AvailableToWithdraw: amountPtr(round(6, btc*0.5)), AvailableToTrade: amountPtr(btc)},
			{PlatformAssetSymbol: "ETH", Name: "Ethereum", Equity: amountString(eth), AvailableToWithdraw: amountPtr(eth), AvailableToTrade: amountPtr(eth), Liability: amountPtr(round(4, 0.4*f)), Interest: amountPtr(round(6, 0.0012*f))},
		}
		short := round(3, 0.5*f)
		entry := round(2, btcPrice*0.98)
		report.Positions = []adapters.Position{{
			PlatformPositionRef: "BTCUSDT-perp-short",
			FuturesContract:     adapters.FuturesContract{Symbol: "BTCUSDT", Kind: adapters.KindLinearPerpetual, BaseSymbol: "BTC", QuoteSymbol: "USDT", SettlementSymbol: "USDT", ContractSize: amountPtr(1)},
			Side:                adapters.SideShort,
			Quantity:            amountString(short),
			QuantityUnit:        adapters.UnitBase,
			EntryPrice:          amountPtr(entry),
			MarkPrice:           amountPtr(btcPrice),
			LiquidationPrice:    amountPtr(round(2, btcPrice*1.35)),
			UnrealizedPnl:       amountPtr(round(2, (entry-btcPrice)*short)),
			Leverage:            amountPtr(3),
			Margin:              amountPtr(round(2, short*btcPrice/3)),
		}, {
			PlatformPositionRef: "ETHUSDT-perp-long",
			FuturesContract:     adapters.FuturesContract{Symbol: "ETHUSDT", Kind: adapters.KindLinearPerpetual, BaseSymbol: "ETH", QuoteSymbol: "USDT", SettlementSymbol: "USDT", ContractSize: amountPtr(1)},
			Side:                adapters.SideLong,
			Quantity:            amountString(round(2, 4*f)),
			QuantityUnit:        adapters.UnitBase,
			EntryPrice:          amountPtr(round(2, ethPrice*1.02)),
			MarkPrice:           amountPtr(ethPrice),
			LiquidationPrice:    amountPtr(round(2, ethPrice*0.6)),
			UnrealizedPnl:       amountPtr(round(2, (ethPrice-ethPrice*1.02)*4*f)),
			Leverage:            amountPtr(2),
			Margin:              amountPtr(round(2, 4*f*ethPrice/2)),
		}}
	case "cross_margin":
		// Binance Classic Cross Margin: net assets per coin, a BTC loan, and the Margin Level.
		report.UnifiedMarginRatio = amountPtr(round(4, 4.2+0.5*math.Sin(hours/9+f)))
		report.Balances = []adapters.Balance{
			{PlatformAssetSymbol: "USDT", Name: "Tether", Equity: amountString(round(2, 3000*f*wave)), AvailableToWithdraw: amountPtr(round(2, 2000*f)), AvailableToTrade: amountPtr(round(2, 3000*f))},
			{PlatformAssetSymbol: "BTC", Name: "Bitcoin", Equity: amountString(round(6, 0.02*f)), Liability: amountPtr(round(6, 0.01*f)), Interest: amountPtr(round(8, 0.00001*f))},
		}
	case "isolated_margin":
		// Binance Classic Isolated Margin: coins summed across pairs, with the pair's loan.
		report.Balances = []adapters.Balance{
			{PlatformAssetSymbol: "SOL", Name: "Solana", Equity: amountString(round(3, 8*f)), Liability: amountPtr(round(3, 2*f)), Interest: amountPtr(round(6, 0.001*f))},
			{PlatformAssetSymbol: "USDT", Name: "Tether", Equity: amountString(round(2, 400*f))},
		}
	case "um_futures":
		// Binance Classic USDⓈ-M Futures in Multi-Assets Mode: margin balance and account figures.
		short := round(3, 0.2*f)
		entry := round(2, btcPrice*1.01)
		margin := round(2, 4000*f*wave)
		report.AdjustedEquity = amountPtr(margin)
		report.AvailableEquity = amountPtr(round(2, margin*0.7))
		report.InitialMarginRatio = amountPtr(round(4, 0.12+0.02*math.Sin(hours/7)))
		report.MaintenanceMarginRatio = amountPtr(round(4, 0.04+0.01*math.Sin(hours/7)))
		report.Balances = []adapters.Balance{
			{PlatformAssetSymbol: "USDT", Name: "Tether", Equity: amountString(margin), AvailableToWithdraw: amountPtr(round(2, margin*0.6)), AvailableToTrade: amountPtr(round(2, margin*0.7)), UnrealizedPnl: amountPtr(round(2, (entry-btcPrice)*short))},
		}
		report.Positions = []adapters.Position{{
			PlatformPositionRef: "BTCUSDT-BOTH",
			FuturesContract:     adapters.FuturesContract{Symbol: "BTCUSDT", Kind: adapters.KindLinearPerpetual, BaseSymbol: "BTC", QuoteSymbol: "USDT", SettlementSymbol: "USDT", ContractSize: amountPtr(1)},
			Side:                adapters.SideShort,
			Quantity:            amountString(short),
			QuantityUnit:        adapters.UnitBase,
			EntryPrice:          amountPtr(entry),
			MarkPrice:           amountPtr(btcPrice),
			LiquidationPrice:    amountPtr(round(2, btcPrice*1.6)),
			UnrealizedPnl:       amountPtr(round(2, (entry-btcPrice)*short)),
			Leverage:            amountPtr(5),
			Margin:              amountPtr(round(2, short*btcPrice/5)),
		}}
	case "cm_futures":
		// Binance Classic COIN-M Futures: BTC margin and an inverse perpetual in contracts.
		contracts := math.Round(20 * f)
		entry := round(1, btcPrice*0.99)
		report.Balances = []adapters.Balance{
			{PlatformAssetSymbol: "BTC", Name: "Bitcoin", Equity: amountString(round(6, 0.05*f*wave)), AvailableToWithdraw: amountPtr(round(6, 0.03*f)), UnrealizedPnl: amountPtr(round(8, contracts*100*(1/entry-1/btcPrice)))},
		}
		report.Positions = []adapters.Position{{
			PlatformPositionRef: "BTCUSD_PERP-BOTH",
			FuturesContract:     adapters.FuturesContract{Symbol: "BTCUSD_PERP", Kind: adapters.KindInversePerpetual, BaseSymbol: "BTC", QuoteSymbol: "USD", SettlementSymbol: "BTC", ContractSize: amountPtr(100)},
			Side:                adapters.SideLong,
			Quantity:            amountString(contracts),
			QuantityUnit:        adapters.UnitContracts,
			EntryPrice:          amountPtr(entry),
			MarkPrice:           amountPtr(btcPrice),
			LiquidationPrice:    amountPtr(round(1, btcPrice*0.55)),
			UnrealizedPnl:       amountPtr(round(8, contracts*100*(1/entry-1/btcPrice))),
			Leverage:            amountPtr(2),
		}}
	case "spot":
		report.Balances = []adapters.Balance{
			{PlatformAssetSymbol: "USDC", Name: "USD Coin", Equity: amountString(round(2, 800*f)), AvailableToWithdraw: amountPtr(round(2, 800*f)), AvailableToTrade: amountPtr(round(2, 800*f))},
			{PlatformAssetSymbol: "SOL", Name: "Solana", Equity: amountString(round(3, 12*f)), AvailableToWithdraw: amountPtr(round(3, 12*f)), AvailableToTrade: amountPtr(round(3, 12*f))},
		}
		if s.unpriced {
			report.Balances = append(report.Balances, UnpricedBalance(f))
		}
	case "funding", "derivatives_holding":
		report.Balances = []adapters.Balance{
			{PlatformAssetSymbol: "USDT", Name: "Tether", Equity: amountString(round(2, 1200*f)), AvailableToWithdraw: amountPtr(round(2, 1200*f)), AvailableToTrade: amountPtr(0)},
			{PlatformAssetSymbol: "BNB", Name: "BNB", Equity: amountString(round(3, 3*f)), AvailableToWithdraw: amountPtr(round(3, 3*f)), AvailableToTrade: amountPtr(0)},
		}
	case adapters.TypeDualInvestment:
		report.Balances = []adapters.Balance{{PlatformAssetSymbol: "USDT", Equity: "1000"}, {PlatformAssetSymbol: "BTC", Equity: "0.02"}}
		expiry := at.UTC().Truncate(24 * time.Hour).Add(56 * time.Hour)
		for i, spec := range []struct{ base, invest, amount, direction, strike string }{
			{"BTC", "USDT", "400", "buy_low", "62000"},
			{"ETH", "USDT", "250", "buy_low", "3000"},
			{"BTC", "USDT", "150", "buy_low", "62000"},
			{"BTC", "USDT", "200", "buy_low", "64000"},
			{"BTC", "BTC", "0.02", "sell_high", "72000"},
		} {
			maturity := expiry.Add(time.Duration([]int{0, 1, 0, 2, 1}[i]) * 24 * time.Hour)
			subscribed := maturity.Add(-72 * time.Hour)
			report.DualPositions = append(report.DualPositions, adapters.DualPosition{PlatformPositionRef: fmt.Sprintf("dual-%d", i+1), ProductRef: adapters.OptionalText(spec.base + "-" + spec.strike), OrderRef: adapters.OptionalText(fmt.Sprintf("order-%d", i+1)), BaseSymbol: adapters.OptionalText(spec.base), QuoteSymbol: adapters.OptionalText("USDT"), InvestSymbol: spec.invest, Principal: spec.amount, Direction: adapters.OptionalText(spec.direction), TargetPrice: adapters.OptionalText(spec.strike), ExpiryAt: &maturity, SubscribedAt: &subscribed, AnnualizedRate: adapters.OptionalText("0.18"), RateKind: adapters.OptionalText("apr"), Status: "active"})
		}
	case "earn":
		report.Balances = []adapters.Balance{
			{PlatformAssetSymbol: "ETH", Name: "Ethereum", Equity: amountString(round(4, 1.5*f)), AvailableToWithdraw: amountPtr(0), AvailableToTrade: amountPtr(0), Interest: amountPtr(round(6, 0.0031*f))},
		}
		report.EarnDetailsCollected = true
		for i, kind := range []string{"flexible", "fixed"} {
			p := adapters.NewEarnPosition(fmt.Sprintf("earn-%d", i+1), kind, "position", "principal")
			p.ProductRef = adapters.OptionalText(fmt.Sprintf("ETH-%s", kind))
			p.ProductName = adapters.OptionalText("Demo Earn " + kind)
			p.Assets = append(p.Assets, adapters.EarnAmount{PlatformAssetSymbol: "ETH", Amount: decimal.Format(new(big.Rat).Quo(decimal.ParseOrZero(report.Balances[0].Equity), big.NewRat(2, 1)))})
			p.AddRate("0.045", "apr", "estimated", "Estimated APR", nil)
			p.AddReward("ETH", amountString(round(6, 0.00155*f)), "cumulative")
			p.Status = adapters.OptionalText("Active")
			if kind == "fixed" {
				subscribed := at.UTC().Truncate(24 * time.Hour).Add(-72 * time.Hour)
				maturity := subscribed.Add(30 * 24 * time.Hour)
				p.SubscribedAt = &subscribed
				p.MaturityAt = &maturity
			} else {
				p.Granularity = "product"
			}
			report.EarnPositions = append(report.EarnPositions, p)
		}
	default:
		report.Status = adapters.StatusFailed
		report.ErrorClass = adapters.ClassUnsupported
		report.ErrorDetail = "The platform reports this account type as not enabled."
	}
	return report
}

// UnpricedBalance is the TOKENX Balance the fake never prices; seeds use it for the no-price case.
func UnpricedBalance(factor float64) adapters.Balance {
	return adapters.Balance{PlatformAssetSymbol: "TOKENX", Name: "Token X", Equity: amountString(round(2, 1500*factor))}
}

func (s ScenarioValues) factor(accountType adapters.AccountType) float64 {
	h := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", s.seed, accountType)))
	return 0.5 + float64(binary.BigEndian.Uint16(h[:2]))/65535*1.5
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:6])
}

func round(places int, value float64) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(value*p) / p
}

func amountString(value float64) string {
	return fmt.Sprintf("%.8f", value)
}

func amountPtr(value float64) *string {
	return new(amountString(value))
}

func floatValue(value string) float64 {
	var f float64
	_, _ = fmt.Sscanf(value, "%g", &f)
	return f
}

func (a *Adapter) credentialError(creds adapters.Credentials) error {
	if creds.APIKey != "" {
		if err := a.keyError(creds.APIKey); err != nil {
			return err
		}
	}
	if a.platform == adapters.PlatformKraken && creds.FuturesKey() != "" {
		return a.keyError(creds.FuturesKey())
	}
	return nil
}

func scenarioKey(creds adapters.Credentials) string {
	if creds.APIKey != "" {
		return creds.APIKey
	}
	return creds.FuturesKey()
}
