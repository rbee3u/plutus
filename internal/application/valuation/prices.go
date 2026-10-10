package valuation

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

type priceResolver interface {
	Resolve(context.Context, string, string, time.Time) (PriceSelection, error)
}

// PriceSelection identifies the exact evidence adopted by a valuation.
type PriceSelection struct {
	Value         *big.Rat
	ObservationID *uuid.UUID
	Method        string
}

type priceKey struct {
	platform string
	at       time.Time
}

// PriceBook caches only eligible observations from the requested Platform.
type PriceBook struct {
	q            *sqlcgen.Queries
	portfolioID  uuid.UUID
	cache        map[priceKey]map[string]PriceSelection
	observations map[uuid.UUID]*big.Rat
}

func (b *PriceBook) Resolve(ctx context.Context, platform, symbol string, at time.Time) (PriceSelection, error) {
	if USDQuote(symbol) {
		return PriceSelection{Value: big.NewRat(1, 1), Method: "usd_parity"}, nil
	}
	if err := b.Preload(ctx, platform, []string{symbol}, at); err != nil {
		return PriceSelection{}, err
	}
	return b.cache[priceKey{platform: platform, at: at}][symbol], nil
}

// Preload bounds price selection to the symbols actually used by this report.
func (b *PriceBook) Preload(ctx context.Context, platform string, symbols []string, at time.Time) error {
	key := priceKey{platform: platform, at: at}
	book := b.cache[key]
	if book == nil {
		book = map[string]PriceSelection{}
		b.cache[key] = book
	}
	missing := []string{}
	for _, symbol := range symbols {
		if USDQuote(symbol) {
			continue
		}
		if _, ok := book[symbol]; ok {
			continue
		}
		book[symbol] = PriceSelection{Method: "unavailable"}
		missing = append(missing, symbol)
	}
	for start := 0; start < len(missing); start += 256 {
		rows, err := b.q.ListLatestPricesByPlatformAt(ctx, sqlcgen.ListLatestPricesByPlatformAtParams{PortfolioID: b.portfolioID[:], Platform: platform, At: at, PriceCutoff: sql.NullTime{Time: at, Valid: true}, Symbols: missing[start:min(start+256, len(missing))]})
		if err != nil {
			return err
		}
		for _, row := range rows {
			value, valid := decimal.Parse(row.Price)
			if !valid || value.Sign() <= 0 {
				continue
			}
			id, err := uuid.FromBytes(row.ID)
			if err != nil {
				return err
			}
			book[row.RequestedCanonicalAssetSymbol] = PriceSelection{Value: value, ObservationID: &id, Method: "market_price"}
			b.observations[id] = value
		}
	}
	return nil
}

// Lookup returns the eligible price, retaining unknown as an explicit false flag.
func (b *PriceBook) Lookup(ctx context.Context, platform, symbol string, at time.Time) (*big.Rat, bool, error) {
	p, err := b.Resolve(ctx, platform, symbol, at)
	return p.Value, p.Value != nil, err
}

func (b *PriceBook) LookupPlatform(ctx context.Context, platform, symbol string, at time.Time) (*big.Rat, bool, error) {
	return b.Lookup(ctx, platform, symbol, at)
}

func USDQuote(symbol string) bool { return symbol == "USD" || symbol == "USDT" || symbol == "USDC" }

func NewPriceBook(q *sqlcgen.Queries, portfolioID uuid.UUID) *PriceBook {
	return &PriceBook{q: q, portfolioID: portfolioID, cache: map[priceKey]map[string]PriceSelection{}, observations: map[uuid.UUID]*big.Rat{}}
}

// Observation reads the adopted immutable price; it never selects a replacement.
func (b *PriceBook) Observation(ctx context.Context, id uuid.UUID) (*big.Rat, error) {
	if value, ok := b.observations[id]; ok {
		return value, nil
	}
	row, err := b.q.GetPriceObservation(ctx, sqlcgen.GetPriceObservationParams{PortfolioID: b.portfolioID[:], ID: id[:]})
	if err != nil {
		return nil, err
	}
	value, ok := decimal.Parse(row.Price)
	if !ok || value.Sign() <= 0 {
		return nil, errors.New("invalid adopted price observation")
	}
	b.observations[id] = value
	return value, nil
}

// PreloadObservations reads adopted evidence once per batch, never per balance.
func (b *PriceBook) PreloadObservations(ctx context.Context, accounts []AccountValuation) error {
	ids := [][]byte{}
	seen := map[uuid.UUID]bool{}
	for _, account := range accounts {
		for _, line := range account.BalanceValuations {
			if line.PriceObservationID != nil {
				id := *line.PriceObservationID
				if !seen[id] {
					seen[id] = true
					ids = append(ids, id[:])
				}
			}
		}
	}
	for start := 0; start < len(ids); start += 256 {
		rows, err := b.q.ListPriceObservationsByIDs(ctx, sqlcgen.ListPriceObservationsByIDsParams{PortfolioID: b.portfolioID[:], Ids: ids[start:min(start+256, len(ids))]})
		if err != nil {
			return err
		}
		for _, row := range rows {
			id, err := uuid.FromBytes(row.ID)
			if err != nil {
				return err
			}
			value, ok := decimal.Parse(row.Price)
			if !ok || value.Sign() <= 0 {
				return errors.New("invalid adopted price observation")
			}
			b.observations[id] = value
		}
	}
	return nil
}
