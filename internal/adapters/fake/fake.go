// Package fake is a deterministic offline Adapter used by seeds, tests, and local lanes.
//
// Values derive from the API key and the current time so that consecutive cycles drift
// smoothly. Keys with special prefixes reproduce failure classes:
//
//	invalid-*    authentication error (Connection becomes invalid)
//	forbidden-*  authorization error
//	ratelimit-*  rate-limited error on every call
//	flaky-*      temporary error on the first attempt of each minute, success afterwards
//	empty-*      success with no balances or positions
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

// Capabilities lists the Account Types per Account Mode.
func (a *Adapter) Capabilities() map[adapters.AccountMode][]adapters.AccountType {
	out := map[adapters.AccountMode][]adapters.AccountType{}
	for _, info := range (&adapters.Registry{}).Platforms() {
		if info.ID != a.platform {
			continue
		}
		for _, mode := range info.Modes {
			for _, t := range mode.AccountTypes {
				out[mode.ID] = append(out[mode.ID], t.ID)
			}
		}
	}
	return out
}

// Validate checks the key prefix and returns a deterministic external ref.
func (a *Adapter) Validate(ctx context.Context, creds adapters.Credentials, mode adapters.AccountMode) (adapters.ValidationResult, error) {
	if err := ctx.Err(); err != nil {
		return adapters.ValidationResult{}, err
	}
	if !adapters.ValidMode(a.platform, mode) {
		return adapters.ValidationResult{}, adapters.NewError(adapters.ClassUnsupported, "The account mode is not supported by this platform.")
	}
	if err := a.keyError(creds.APIKey); err != nil {
		return adapters.ValidationResult{}, err
	}
	return adapters.ValidationResult{ExternalRef: "uid-" + shortHash(creds.APIKey)}, nil
}

// Collect produces one report per requested Account Type with drifting balances and prices.
func (a *Adapter) Collect(ctx context.Context, creds adapters.Credentials, mode adapters.AccountMode, accounts []adapters.AccountType) (adapters.Batch, error) {
	if err := ctx.Err(); err != nil {
		return adapters.Batch{}, err
	}
	if err := a.keyError(creds.APIKey); err != nil {
		return adapters.Batch{}, err
	}
	now := a.now().UTC().Truncate(time.Second)
	batch := adapters.Batch{ObservedAt: now}
	scenario := Scenario(creds.APIKey, now)
	for _, symbol := range []string{"BTC", "ETH", "SOL", "BNB"} {
		batch.Prices = append(batch.Prices, adapters.Price{
			AssetSymbol: symbol,
			Price:       scenario.Price(symbol),
			QuoteSymbol: "USDT",
			ObservedAt:  now,
		})
	}
	for _, accountType := range accounts {
		if !adapters.ValidAccountType(mode, accountType) {
			batch.Reports = append(batch.Reports, adapters.AccountReport{
				AccountType: accountType,
				Status:      adapters.StatusFailed,
				ErrorClass:  adapters.ClassUnsupported,
				ErrorDetail: "The platform reports this account type as not enabled.",
			})
			continue
		}
		batch.Reports = append(batch.Reports, scenario.Report(accountType, now, strings.HasPrefix(creds.APIKey, "empty-")))
	}
	return batch, nil
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

// Report builds the deterministic report for one Account Type.
func (s ScenarioValues) Report(accountType adapters.AccountType, at time.Time, empty bool) adapters.AccountReport {
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
	case "portfolio_margin", "trading", "unified":
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
	case "spot":
		report.Balances = []adapters.Balance{
			{Symbol: "USDC", Name: "USD Coin", Equity: fs(round(2, 800*f)), AvailableToWithdraw: str(round(2, 800*f)), AvailableToTrade: str(round(2, 800*f))},
			{Symbol: "SOL", Name: "Solana", Equity: fs(round(3, 12*f)), AvailableToWithdraw: str(round(3, 12*f)), AvailableToTrade: str(round(3, 12*f))},
		}
		if s.unpriced {
			report.Balances = append(report.Balances, UnpricedBalance(f))
		}
	case "funding":
		report.Balances = []adapters.Balance{
			{Symbol: "USDT", Name: "Tether", Equity: fs(round(2, 1200*f)), AvailableToWithdraw: str(round(2, 1200*f)), AvailableToTrade: str(0)},
			{Symbol: "BNB", Name: "BNB", Equity: fs(round(3, 3*f)), AvailableToWithdraw: str(round(3, 3*f)), AvailableToTrade: str(0)},
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
