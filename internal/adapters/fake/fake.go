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
	"strings"
	"sync"
	"time"

	"github.com/rbee3u/plutus/internal/adapters"
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
	return adapters.ValidationResult{ExternalRef: ExternalRef(key), MarginMode: MarginMode(structure, key, a.now())}, nil
}

// behaviorPrefixes are the key prefixes that select a behavior rather than an account.
var behaviorPrefixes = []string{"invalid-", "forbidden-", "ratelimit-", "flaky-", "empty-", "unpriced-", "switch-", "restructured-"}

// ExternalRef is the reference of the Platform account a key reaches: the key without its
// behavior prefix, so one account can be tried with every behavior.
func ExternalRef(apiKey string) string {
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
		index = (index + uint64(at.UTC().Unix()/60)) % uint64(len(modes))
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
	case adapters.MarginBybitPM:
		// Portfolio Margin positions carry no leverage, position margin, or liquidation price.
		for i := range report.Positions {
			report.Positions[i].Leverage, report.Positions[i].Margin, report.Positions[i].LiquidationPrice = nil, nil, nil
		}
	default:
		// The other Margin Modes report every figure.
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
	btcPrice := mustFloat(s.Price("BTC"))
	ethPrice := mustFloat(s.Price("ETH"))
	switch accountType {
	case "portfolio_margin", "trading", "unified", "main", "multi_m", "coin_m":
		equity := round(2, usdt+btc*btcPrice+eth*ethPrice-1200*f)
		report.Equity = str(equity)
		report.AdjustedEquity = str(round(2, equity*0.97))
		report.AvailableEquity = str(round(2, equity*0.55))
		report.InitialMarginRatio = str(round(4, 0.18+0.02*math.Sin(hours/7)))
		report.MaintenanceMarginRatio = str(round(4, 0.09+0.01*math.Sin(hours/7)))
		report.UnifiedMarginRatio = str(round(4, 3.1+0.4*math.Sin(hours/13+f)))
		report.Balances = []adapters.Balance{
			{Symbol: "USDT", Name: "Tether", Equity: fs(usdt), AvailableToWithdraw: str(round(2, usdt*0.5)), AvailableToTrade: str(round(2, usdt*0.9))},
			{Symbol: "BTC", Name: "Bitcoin", Equity: fs(btc), AvailableToWithdraw: str(round(6, btc*0.5)), AvailableToTrade: str(btc)},
			{Symbol: "ETH", Name: "Ethereum", Equity: fs(eth), AvailableToWithdraw: str(eth), AvailableToTrade: str(eth), Liability: str(round(4, 0.4*f)), Interest: str(round(6, 0.0012*f))},
		}
		short := round(3, 0.5*f)
		entry := round(2, btcPrice*0.98)
		report.Positions = []adapters.Position{{
			PlatformPositionRef: "BTCUSDT-perp-short",
			Instrument:          adapters.Instrument{Symbol: "BTCUSDT", Kind: adapters.KindLinearPerp, BaseSymbol: "BTC", QuoteSymbol: "USDT", SettlementSymbol: "USDT", ContractSize: str(1)},
			Side:                adapters.SideShort,
			Quantity:            fs(short),
			QuantityUnit:        adapters.UnitBase,
			EntryPrice:          str(entry),
			MarkPrice:           str(btcPrice),
			LiquidationPrice:    str(round(2, btcPrice*1.35)),
			UnrealizedPnl:       str(round(2, (entry-btcPrice)*short)),
			Leverage:            str(3),
			Margin:              str(round(2, short*btcPrice/3)),
		}, {
			PlatformPositionRef: "ETHUSDT-perp-long",
			Instrument:          adapters.Instrument{Symbol: "ETHUSDT", Kind: adapters.KindLinearPerp, BaseSymbol: "ETH", QuoteSymbol: "USDT", SettlementSymbol: "USDT", ContractSize: str(1)},
			Side:                adapters.SideLong,
			Quantity:            fs(round(2, 4*f)),
			QuantityUnit:        adapters.UnitBase,
			EntryPrice:          str(round(2, ethPrice*1.02)),
			MarkPrice:           str(ethPrice),
			LiquidationPrice:    str(round(2, ethPrice*0.6)),
			UnrealizedPnl:       str(round(2, (ethPrice-ethPrice*1.02)*4*f)),
			Leverage:            str(2),
			Margin:              str(round(2, 4*f*ethPrice/2)),
		}}
	case "cross_margin":
		// Binance Classic Cross Margin: net assets per coin, a BTC loan, and the Margin Level.
		report.UnifiedMarginRatio = str(round(4, 4.2+0.5*math.Sin(hours/9+f)))
		report.Balances = []adapters.Balance{
			{Symbol: "USDT", Name: "Tether", Equity: fs(round(2, 3000*f*wave)), AvailableToWithdraw: str(round(2, 2000*f)), AvailableToTrade: str(round(2, 3000*f))},
			{Symbol: "BTC", Name: "Bitcoin", Equity: fs(round(6, 0.02*f)), Liability: str(round(6, 0.01*f)), Interest: str(round(8, 0.00001*f))},
		}
	case "isolated_margin":
		// Binance Classic Isolated Margin: coins summed across pairs, with the pair's loan.
		report.Balances = []adapters.Balance{
			{Symbol: "SOL", Name: "Solana", Equity: fs(round(3, 8*f)), Liability: str(round(3, 2*f)), Interest: str(round(6, 0.001*f))},
			{Symbol: "USDT", Name: "Tether", Equity: fs(round(2, 400*f))},
		}
	case "um_futures":
		// Binance Classic USDⓈ-M Futures in Multi-Assets Mode: margin balance and account figures.
		short := round(3, 0.2*f)
		entry := round(2, btcPrice*1.01)
		margin := round(2, 4000*f*wave)
		report.AdjustedEquity = str(margin)
		report.AvailableEquity = str(round(2, margin*0.7))
		report.InitialMarginRatio = str(round(4, 0.12+0.02*math.Sin(hours/7)))
		report.MaintenanceMarginRatio = str(round(4, 0.04+0.01*math.Sin(hours/7)))
		report.Balances = []adapters.Balance{
			{Symbol: "USDT", Name: "Tether", Equity: fs(margin), AvailableToWithdraw: str(round(2, margin*0.6)), AvailableToTrade: str(round(2, margin*0.7)), UnrealizedPnl: str(round(2, (entry-btcPrice)*short))},
		}
		report.Positions = []adapters.Position{{
			PlatformPositionRef: "BTCUSDT-BOTH",
			Instrument:          adapters.Instrument{Symbol: "BTCUSDT", Kind: adapters.KindLinearPerp, BaseSymbol: "BTC", QuoteSymbol: "USDT", SettlementSymbol: "USDT", ContractSize: str(1)},
			Side:                adapters.SideShort,
			Quantity:            fs(short),
			QuantityUnit:        adapters.UnitBase,
			EntryPrice:          str(entry),
			MarkPrice:           str(btcPrice),
			LiquidationPrice:    str(round(2, btcPrice*1.6)),
			UnrealizedPnl:       str(round(2, (entry-btcPrice)*short)),
			Leverage:            str(5),
			Margin:              str(round(2, short*btcPrice/5)),
		}}
	case "cm_futures":
		// Binance Classic COIN-M Futures: BTC margin and an inverse perpetual in contracts.
		contracts := math.Round(20 * f)
		entry := round(1, btcPrice*0.99)
		report.Balances = []adapters.Balance{
			{Symbol: "BTC", Name: "Bitcoin", Equity: fs(round(6, 0.05*f*wave)), AvailableToWithdraw: str(round(6, 0.03*f)), UnrealizedPnl: str(round(8, contracts*100*(1/entry-1/btcPrice)))},
		}
		report.Positions = []adapters.Position{{
			PlatformPositionRef: "BTCUSD_PERP-BOTH",
			Instrument:          adapters.Instrument{Symbol: "BTCUSD_PERP", Kind: adapters.KindInversePerp, BaseSymbol: "BTC", QuoteSymbol: "USD", SettlementSymbol: "BTC", ContractSize: str(100)},
			Side:                adapters.SideLong,
			Quantity:            fs(contracts),
			QuantityUnit:        adapters.UnitContracts,
			EntryPrice:          str(entry),
			MarkPrice:           str(btcPrice),
			LiquidationPrice:    str(round(1, btcPrice*0.55)),
			UnrealizedPnl:       str(round(8, contracts*100*(1/entry-1/btcPrice))),
			Leverage:            str(2),
		}}
	case "spot":
		report.Balances = []adapters.Balance{
			{Symbol: "USDC", Name: "USD Coin", Equity: fs(round(2, 800*f)), AvailableToWithdraw: str(round(2, 800*f)), AvailableToTrade: str(round(2, 800*f))},
			{Symbol: "SOL", Name: "Solana", Equity: fs(round(3, 12*f)), AvailableToWithdraw: str(round(3, 12*f)), AvailableToTrade: str(round(3, 12*f))},
		}
		if s.unpriced {
			report.Balances = append(report.Balances, UnpricedBalance(f))
		}
	case "funding", "derivatives_holding":
		report.Balances = []adapters.Balance{
			{Symbol: "USDT", Name: "Tether", Equity: fs(round(2, 1200*f)), AvailableToWithdraw: str(round(2, 1200*f)), AvailableToTrade: str(0)},
			{Symbol: "BNB", Name: "BNB", Equity: fs(round(3, 3*f)), AvailableToWithdraw: str(round(3, 3*f)), AvailableToTrade: str(0)},
		}
	case adapters.TypeDualInvestment:
		report.Balances = []adapters.Balance{{Symbol: "USDT", Equity: "1000"}, {Symbol: "BTC", Equity: "0.02"}}
		expiry := at.UTC().Truncate(24 * time.Hour).Add(56 * time.Hour)
		for i, spec := range []struct{ base, invest, amount, direction, strike string }{
			{"BTC", "USDT", "400", "buy_low", "62000"},
			{"ETH", "USDT", "250", "buy_low", "3000"},
			{"BTC", "USDT", "150", "buy_low", "62000"},
			{"BTC", "USDT", "200", "buy_low", "64000"},
			{"BTC", "BTC", "0.02", "sell_high", "72000"},
		} {
			maturity := expiry.Add(time.Duration([]int{0, 1, 0, 2, 1}[i]) * 24 * time.Hour)
			report.DualPositions = append(report.DualPositions, adapters.DualPosition{PlatformPositionRef: fmt.Sprintf("dual-%d", i+1), ProductRef: adapters.DualText(spec.base + "-" + spec.strike), OrderRef: adapters.DualText(fmt.Sprintf("order-%d", i+1)), BaseSymbol: adapters.DualText(spec.base), QuoteSymbol: adapters.DualText("USDT"), InvestSymbol: spec.invest, Principal: spec.amount, Direction: adapters.DualText(spec.direction), TargetPrice: adapters.DualText(spec.strike), ExpiryAt: &maturity, AnnualizedRate: adapters.DualText("0.18"), RateKind: adapters.DualText("apr"), Status: "active"})
		}
	case "earn":
		report.Balances = []adapters.Balance{
			{Symbol: "ETH", Name: "Ethereum", Equity: fs(round(4, 1.5*f)), AvailableToWithdraw: str(0), AvailableToTrade: str(0), Interest: str(round(6, 0.0031*f))},
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
	return adapters.Balance{Symbol: "TOKENX", Name: "Token X", Equity: fs(round(2, 1500*factor))}
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

func fs(value float64) string {
	return fmt.Sprintf("%.8f", value)
}

func str(value float64) *string {
	s := fs(value)
	return &s
}

func mustFloat(value string) float64 {
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
