package valuation

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"slices"
	"sort"
	"time"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

type priceAsset struct {
	platform, symbol string
}

type historicalPrice struct {
	at    time.Time
	value *big.Rat
	quote string
}

type priceLookup struct {
	priceAsset
	at time.Time
}

type priceResult struct {
	value *big.Rat
	found bool
}

type historicalPrices struct {
	q        *sqlcgen.Queries
	platform map[priceAsset][]historicalPrice
	global   map[string][]historicalPrice
	cache    map[priceLookup]priceResult
}

func newHistoricalPrices(q *sqlcgen.Queries, rows []sqlcgen.Price) *historicalPrices {
	h := &historicalPrices{q: q, platform: map[priceAsset][]historicalPrice{}, global: map[string][]historicalPrice{}, cache: map[priceLookup]priceResult{}}
	for _, row := range rows {
		value, _ := decimal.Parse(row.Price)
		p := historicalPrice{at: row.ObservedAt, value: value, quote: row.QuoteSymbol}
		key := priceAsset{platform: row.Platform, symbol: row.AssetSymbol}
		h.platform[key] = append(h.platform[key], p)
		h.global[row.AssetSymbol] = append(h.global[row.AssetSymbol], p)
	}
	for _, series := range h.global {
		slices.SortStableFunc(series, func(a, b historicalPrice) int { return a.at.Compare(b.at) })
	}
	return h
}

func latestPrice(series []historicalPrice, at time.Time) int {
	return sort.Search(len(series), func(i int) bool { return series[i].at.After(at) }) - 1
}

func eligiblePrice(p historicalPrice, at time.Time) bool {
	return p.value != nil && USDQuote(p.quote) && at.Sub(p.at) <= maxPriceAge
}

// LookupPlatform enforces Manual pricing without the ordinary cross-Platform fallback.
func (h *historicalPrices) LookupPlatform(_ context.Context, platform, symbol string, at time.Time) (*big.Rat, bool, error) {
	switch symbol {
	case "USD", "USDT", "USDC":
		return big.NewRat(1, 1), true, nil
	}
	series := h.platform[priceAsset{platform: platform, symbol: symbol}]
	if i := latestPrice(series, at); i >= 0 && eligiblePrice(series[i], at) {
		return series[i].value, true, nil
	}
	return nil, false, nil
}

func (h *historicalPrices) Lookup(ctx context.Context, platform, symbol string, at time.Time) (*big.Rat, bool, error) {
	switch symbol {
	case "USD", "USDT", "USDC":
		return big.NewRat(1, 1), true, nil
	}
	key := priceLookup{priceAsset: priceAsset{platform: platform, symbol: symbol}, at: at}
	if p, ok := h.cache[key]; ok {
		return p.value, p.found, nil
	}
	local := h.platform[key.priceAsset]
	if i := latestPrice(local, at); i >= 0 && eligiblePrice(local[i], at) {
		p := priceResult{value: local[i].value, found: true}
		h.cache[key] = p
		return p.value, true, nil
	}
	// Preserve the existing cross-Platform fallback, including invalid latest
	// quotes. This optimization does not change the documented pricing policy
	// discrepancy. Missing prices do not issue another full-table query.
	global := h.global[symbol]
	i := latestPrice(global, at)
	p := priceResult{}
	if i >= 0 && i > 0 && global[i-1].at.Equal(global[i].at) {
		// The existing SQL has no Platform tie-break. Let the same database
		// query choose tied fallback rows rather than imposing a new policy.
		row, err := h.q.GetNearestPriceAt(ctx, sqlcgen.GetNearestPriceAtParams{AssetSymbol: symbol, FilterPlatform: 0, At: at})
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, false, err
		}
		if err == nil {
			value, _ := decimal.Parse(row.Price)
			candidate := historicalPrice{at: row.ObservedAt, value: value, quote: row.QuoteSymbol}
			if eligiblePrice(candidate, at) {
				p = priceResult{value: value, found: true}
			}
		}
	} else if i >= 0 && eligiblePrice(global[i], at) {
		p = priceResult{value: global[i].value, found: true}
	}
	h.cache[key] = p
	return p.value, p.found, nil
}
