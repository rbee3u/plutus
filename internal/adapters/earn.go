package adapters

import (
	"fmt"
	"math/big"

	"github.com/rbee3u/plutus/internal/decimal"
)

// CheckEarnComplete is the runtime completeness check of an Earn Account (ADR
// 0005). A Platform keeps adding Earn products, and a product Plutus cannot
// read would otherwise vanish from a successful Earn report without a trace.
// So the adapter also reads the Platform's own USD valuation of Earn,
// platformUSD, and compares it with the Balances it itemized, each valued with
// price (the USD value of one unit of a symbol).
//
// The check passes when the Platform's valuation is at most max(1% of it, 1
// USD) above the itemized value; the tolerance absorbs the difference between
// the Platform's prices and the adapter's. It also passes when a Balance has
// no price, because no comparison is possible and valuation already marks that
// Account incomplete. Otherwise it fails with ClassUnsupported, naming both
// amounts, so the Earn Account goes Stale instead of silently losing value.
func CheckEarnComplete(platform Platform, platformUSD *big.Rat, balances []Balance, price func(symbol string) (*big.Rat, bool)) error {
	if platformUSD == nil {
		return NewError(ClassNormalization, PlatformName(platform)+" returned no Earn valuation.")
	}
	itemized := decimal.Zero()
	for _, balance := range balances {
		amount, ok := decimal.Parse(balance.Equity)
		if !ok {
			return NewError(ClassNormalization, fmt.Sprintf("Earn balance %s is not a decimal.", balance.Symbol))
		}
		unit, ok := price(balance.Symbol)
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
