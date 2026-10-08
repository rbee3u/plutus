package valuation

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"time"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

type priceResolver interface {
	Lookup(context.Context, string, string, time.Time) (*big.Rat, bool, error)
	LookupPlatform(context.Context, string, string, time.Time) (*big.Rat, bool, error)
}

type priceKey struct {
	platform string
	at       time.Time
}

// PriceBook resolves stored prices with an operation-scoped cache.
type PriceBook struct {
	q     *sqlcgen.Queries
	cache map[priceKey]map[string]pricePoint
}

type pricePoint struct {
	value      *big.Rat
	observedAt time.Time
}

// Lookup prefers own-Platform prices, then falls back across Platforms.
// This fallback is the unresolved pricing gap in docs/architecture.md.
func (b *PriceBook) Lookup(ctx context.Context, platform, assetSymbol string, at time.Time) (*big.Rat, bool, error) {
	return b.lookupWithPolicy(ctx, platform, assetSymbol, at, false)
}

// LookupPlatform requires an eligible own-Platform price.
func (b *PriceBook) LookupPlatform(ctx context.Context, platform, assetSymbol string, at time.Time) (*big.Rat, bool, error) {
	return b.lookupWithPolicy(ctx, platform, assetSymbol, at, true)
}

func (b *PriceBook) lookupWithPolicy(ctx context.Context, platform, assetSymbol string, at time.Time, ownPlatform bool) (*big.Rat, bool, error) {
	switch assetSymbol {
	case "USD", "USDT", "USDC":
		return big.NewRat(1, 1), true, nil
	}
	key := priceKey{platform: platform, at: at}
	book, ok := b.cache[key]
	if !ok {
		rows, err := b.q.ListLatestPricesByPlatformAt(ctx, sqlcgen.ListLatestPricesByPlatformAtParams{Platform: platform, At: at})
		if err != nil {
			return nil, false, err
		}
		book = map[string]pricePoint{}
		for i := range rows {
			if !USDQuote(rows[i].QuoteSymbol) {
				continue
			}
			if value, ok := decimal.Parse(rows[i].Price); ok {
				book[rows[i].AssetSymbol] = pricePoint{value: value, observedAt: rows[i].ObservedAt}
			}
		}
		b.cache[key] = book
	}
	if p, ok := book[assetSymbol]; ok && at.Sub(p.observedAt) <= maxPriceAge {
		return p.value, true, nil
	}
	if ownPlatform {
		return nil, false, nil
	}
	row, err := b.q.GetNearestPriceAt(ctx, sqlcgen.GetNearestPriceAtParams{AssetSymbol: assetSymbol, FilterPlatform: 0, At: at})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !USDQuote(row.QuoteSymbol) || at.Sub(row.ObservedAt) > maxPriceAge {
		return nil, false, nil
	}
	value, ok := decimal.Parse(row.Price)
	return value, ok, nil
}

// USDQuote reports whether the quote is treated as USD.
func USDQuote(symbol string) bool {
	switch symbol {
	case "USD", "USDT", "USDC":
		return true
	}
	return false
}

// NewPriceBook creates a cache for one valuation or financial read operation.
func NewPriceBook(q *sqlcgen.Queries) *PriceBook {
	return &PriceBook{q: q, cache: map[priceKey]map[string]pricePoint{}}
}
