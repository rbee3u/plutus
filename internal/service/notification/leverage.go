// Package notification evaluates the Overview high-leverage condition and
// delivers one Telegram message per completed collection cycle.
package notification

import (
	"math/big"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/financial"
	"github.com/rbee3u/plutus/internal/service/valuation"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// highLeverageThreshold is the strict display threshold shared with Overview.
const highLeverageThreshold = 3

// supportedLeverageScopes mirrors the supported unified account scopes in the
// frontend real-leverage rule.
var supportedLeverageScopes = map[string]map[string]bool{
	"binance": {"portfolio_margin": true},
	"okx":     {"trading": true},
	"bybit":   {"unified": true},
	"kraken":  {"main": true},
}

// leverageAlert is one account above the threshold at a valuation point.
type leverageAlert struct {
	AccountName      string
	AccountType      string
	IdentityName     string
	Platform         string
	Leverage         *big.Rat
	AdjustedEquity   *big.Rat
	Notional         *big.Rat
	Stale            bool
	ReportObservedAt *time.Time
}

// supportsRealLeverage reports whether an account scope has an adjusted-equity
// basis for real leverage.
func supportsRealLeverage(platform adapter.Platform, accountType string) bool {
	types, ok := supportedLeverageScopes[string(platform)]
	return ok && types[accountType]
}

// evaluate applies the Overview condition to one account: a known source
// report, positive adjusted equity, every position notional known, and an
// unrounded ratio strictly above three. A stale known value still qualifies.
func evaluate(account valuation.AccountValuation, positions []financial.PositionRow) (leverageAlert, bool) {
	if !supportsRealLeverage(adapter.Platform(account.Platform), account.AccountType) {
		return leverageAlert{}, false
	}
	if account.SourceReportID == nil {
		return leverageAlert{}, false
	}
	equity, ok := parseRatio(account.AdjustedEquity)
	if !ok || equity.Sign() <= 0 {
		return leverageAlert{}, false
	}
	notional := decimal.Zero()
	for i := range positions {
		value, ok := parseRatio(positions[i].NotionalUSD)
		if !ok || value.Sign() < 0 {
			return leverageAlert{}, false
		}
		notional = decimal.Add(notional, value)
	}
	leverage := new(big.Rat).Quo(notional, equity)
	if leverage.Cmp(big.NewRat(highLeverageThreshold, 1)) <= 0 {
		return leverageAlert{}, false
	}
	return leverageAlert{
		AccountName:      account.AccountName,
		AccountType:      account.AccountType,
		IdentityName:     account.IdentityName,
		Platform:         adapter.PlatformName(adapter.Platform(account.Platform)),
		Leverage:         leverage,
		AdjustedEquity:   equity,
		Notional:         notional,
		Stale:            account.Freshness == valuation.FreshnessStale,
		ReportObservedAt: account.ReportObservedAt,
	}, true
}

// parseRatio keeps an unknown value distinct from zero.
func parseRatio(value *string) (*big.Rat, bool) {
	if value == nil {
		return nil, false
	}
	return decimal.Parse(*value)
}
