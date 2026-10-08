package adapters

import (
	"fmt"
	"maps"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// OptionalText preserves an absent platform field as null.
func OptionalText(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

// OptionalMillis reads milliseconds since epoch without treating missing terms as zero.
func OptionalMillis(where, name, value string) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil //nolint:nilnil // An absent product term remains null without failing known principal.
	}
	ms, err := strconv.ParseInt(value, 10, 64)
	if err != nil || ms <= 0 {
		return nil, Malformed(where, name)
	}
	t := time.UnixMilli(ms).UTC()
	if t.Year() > 9999 {
		return nil, Malformed(where, name)
	}
	return &t, nil
}

// Decimal rejects absent or malformed required values with a normalization error.
func Decimal(where, field, value string) (*big.Rat, error) {
	if !decimal.Valid(value) {
		return nil, NewError(ClassNormalization, fmt.Sprintf("%s: field %s is not a decimal.", where, field))
	}
	rat, _ := decimal.Parse(value)
	return rat, nil
}

// OptionalDecimal returns zero for absence and rejects malformed values.
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

// Plain compacts valid decimals; absent or malformed values stay nil.
func Plain(value string) *string {
	value = strings.TrimSpace(value)
	if !decimal.Valid(value) {
		return nil
	}
	return new(decimal.Compact(value))
}

// NonZero is Plain for fields whose zero means inapplicable, such as liquidation price.
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

// Ratio rounds to eight fractional digits; unknown inputs or a zero denominator
// return nil.
func Ratio(numerator, denominator *big.Rat) *string {
	if numerator == nil || denominator == nil || denominator.Sign() == 0 {
		return nil
	}
	return new(decimal.Compact(new(big.Rat).Quo(numerator, denominator).FloatString(8)))
}

// Totals sums coin amounts by symbol, for Accounts that report one coin across several rows.
type Totals map[string]*big.Rat

// Add aggregates by symbol and rejects nonzero amounts without a symbol.
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
	for _, symbol := range slices.Sorted(maps.Keys(t)) {
		if t[symbol].Sign() == 0 {
			continue
		}
		out = append(out, Balance{Symbol: symbol, Equity: decimal.Format(t[symbol])})
	}
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

// QuotePrices selects the first available quote per Balance symbol. USD
// equivalents and unpriced symbols emit no row; the latter remain unvalued.
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
	slices.SortFunc(out, func(a, b Price) int { return strings.Compare(a.AssetSymbol, b.AssetSymbol) })
	return out
}

// USDPrice returns 1 for USD equivalents or the first positive quoted price.
// Every supplied quote must itself be USD-equivalent.
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
