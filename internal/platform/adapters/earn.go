package adapters

import (
	"fmt"
	"math/big"

	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// CheckEarnComplete detects unitemized Earn value, including auxiliary Dual
// holdings. A shortfall above max(1% of |platformUSD|, 1 USD) is unsupported;
// the tolerance absorbs price differences. Missing prices skip this comparison
// and leave financial completeness to valuation. See ADR 0005.
func CheckEarnComplete(platform Platform, platformUSD *big.Rat, balances []Balance, price func(symbol string) (*big.Rat, bool)) error {
	if platformUSD == nil {
		return NewError(ClassNormalization, PlatformName(platform)+" returned no Earn valuation.")
	}
	itemized := decimal.Zero()
	for _, balance := range balances {
		amount, ok := decimal.Parse(balance.Equity)
		if !ok {
			return NewError(ClassNormalization, fmt.Sprintf("Earn balance %s is not a decimal.", balance.AssetSymbol))
		}
		unit, ok := price(balance.AssetSymbol)
		if !ok {
			return nil
		}
		itemized = decimal.Add(itemized, decimal.Mul(amount, unit))
	}
	tolerance := new(big.Rat).Quo(decimal.Abs(platformUSD), big.NewRat(100, 1))
	if floor := big.NewRat(1, 1); tolerance.Cmp(floor) < 0 {
		tolerance = floor
	}
	if decimal.Sub(platformUSD, itemized).Cmp(tolerance) <= 0 {
		return nil
	}
	return NewError(ClassUnsupported, fmt.Sprintf(
		"%s values Earn at %s USD, but Plutus can itemize only %s USD: an Earn product Plutus does not read is held, so Earn is kept stale instead of losing that value.",
		PlatformName(platform), platformUSD.FloatString(2), itemized.FloatString(2)))
}
