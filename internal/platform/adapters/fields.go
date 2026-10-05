package adapters

import (
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// Decimal parses a required decimal field. An absent or malformed value is a normalization
// failure naming where it came from, never a zero: read as zero, a coin would disappear under the
// Complete Set rule.
func Decimal(where, field, value string) (*big.Rat, error) {
	if !decimal.Valid(value) {
		return nil, NewError(ClassNormalization, fmt.Sprintf("%s: field %s is not a decimal.", where, field))
	}
	rat, _ := decimal.Parse(value)
	return rat, nil
}

// OptionalDecimal parses a field a Platform may leave empty: an absent value reads as zero, a
// present non-decimal is a normalization failure.
func OptionalDecimal(where, field, value string) (*big.Rat, error) {
	if strings.TrimSpace(value) == "" {
		return decimal.Zero(), nil
	}
	return Decimal(where, field, value)
}

// Malformed is the normalization failure of a response that lacks a required part.
func Malformed(where, what string) error {
	return NewError(ClassNormalization, fmt.Sprintf("%s: %s is missing or malformed.", where, what))
}

// Plain returns a valid decimal field as reported, compacted. An absent or malformed value stays
// nil, so an unknown measure is never stored as zero.
func Plain(value string) *string {
	value = strings.TrimSpace(value)
	if !decimal.Valid(value) {
		return nil
	}
	s := decimal.Compact(value)
	return &s
}

// NonZero is Plain for fields a Platform fills with "0" when they do not apply, such as the
// liquidation price of a position that cannot be liquidated on its own.
func NonZero(value string) *string {
	out := Plain(value)
	if out == nil {
		return nil
	}
	if rat, ok := decimal.Parse(*out); !ok || rat.Sign() == 0 {
		return nil
	}
	return out
}

// Reported formats a parsed value only when the Platform actually reported the field.
func Reported(raw string, value *big.Rat) *string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return decimal.FormatPtr(value)
}

// Ratio returns numerator ÷ denominator rounded to eight fraction digits, or nil when either is
// unknown or the denominator is zero.
func Ratio(numerator, denominator *big.Rat) *string {
	if numerator == nil || denominator == nil || denominator.Sign() == 0 {
		return nil
	}
	s := decimal.Compact(new(big.Rat).Quo(numerator, denominator).FloatString(8))
	return &s
}

// Totals sums coin amounts by symbol, for Accounts that report one coin across several rows.
type Totals map[string]*big.Rat

// Add adds amount to symbol. A nonzero amount without a symbol is a normalization failure naming
// where it came from: dropped, it would vanish from the Complete Set.
func (t Totals) Add(where, symbol string, amount *big.Rat) error {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		if amount == nil || amount.Sign() == 0 {
			return nil
		}
		return Malformed(where, "the coin of a nonzero amount")
	}
	t[symbol] = decimal.Add(t[symbol], amount)
	return nil
}

// Balances returns one Balance per nonzero total, ordered by symbol, with only its equity.
func (t Totals) Balances() []Balance {
	out := make([]Balance, 0, len(t))
	for _, symbol := range SortedKeys(t) {
		if t[symbol].Sign() == 0 {
			continue
		}
		out = append(out, Balance{Symbol: symbol, Equity: decimal.Format(t[symbol])})
	}
	return out
}

// SortedKeys returns the keys of m in ascending order.
func SortedKeys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// USDEquivalent mirrors ADR 0003: USD, USDT, and USDC value at 1 USD without a
// price row.
func USDEquivalent(symbol string) bool {
	switch strings.ToUpper(strings.TrimSpace(symbol)) {
	case "USD", "USDT", "USDC":
		return true
	default:
		return false
	}
}

// QuoteLookup returns the Platform's last price of symbol in quote, if it has one.
type QuoteLookup func(symbol, quote string) (string, bool)

// QuotePrices picks one price row for every Balance symbol that needs one: the first quote in
// quotes that lookup prices. USD equivalents get no row; a symbol no quote prices gets none either
// and is valued as no_price.
func QuotePrices(reports []AccountReport, quotes []string, lookup QuoteLookup, at time.Time) []Price {
	var symbols []string
	for _, report := range reports {
		for _, balance := range report.Balances {
			symbols = append(symbols, balance.Symbol)
		}
	}
	return SymbolPrices(symbols, quotes, lookup, at)
}

// SymbolPrices chooses public prices for explicit assets, including manually held assets.
func SymbolPrices(symbols []string, quotes []string, lookup QuoteLookup, at time.Time) []Price {
	if lookup == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []Price
	for _, input := range symbols {
		symbol := strings.ToUpper(strings.TrimSpace(input))
		if symbol == "" || seen[symbol] || USDEquivalent(symbol) {
			continue
		}
		seen[symbol] = true
		for _, quote := range quotes {
			if price, ok := lookup(symbol, quote); ok {
				out = append(out, Price{AssetSymbol: symbol, Price: price, QuoteSymbol: quote, ObservedAt: at})
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AssetSymbol < out[j].AssetSymbol })
	return out
}

// USDPrice returns the USD value of one unit of symbol: 1 for a USD equivalent, otherwise the
// first quote in quotes that lookup prices, where every quote is itself a USD equivalent.
func USDPrice(symbol string, quotes []string, lookup QuoteLookup) (*big.Rat, bool) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if USDEquivalent(symbol) {
		return big.NewRat(1, 1), true
	}
	if lookup == nil {
		return nil, false
	}
	for _, quote := range quotes {
		if price, ok := lookup(symbol, quote); ok {
			if rat, ok := decimal.Parse(price); ok && rat.Sign() > 0 {
				return rat, true
			}
		}
	}
	return nil, false
}
