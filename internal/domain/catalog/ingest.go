package catalog

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// EnsureListing upserts a listing inside q and returns its ID and current Asset symbol.
func (s *Service) EnsureListing(ctx context.Context, q *sqlcgen.Queries, platform adapters.Platform, symbol, name string, seenAt time.Time) (uuid.UUID, string, error) {
	normalized, err := NormalizeSymbol(symbol)
	if err != nil {
		return uuid.Nil, "", err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, "", err
	}
	var nullName sql.NullString
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		nullName = sql.NullString{String: trimmed, Valid: true}
	}
	if err := q.UpsertAssetListing(ctx, sqlcgen.UpsertAssetListingParams{
		ID:          id[:],
		Platform:    string(platform),
		Symbol:      normalized,
		AssetSymbol: AssetSymbolFor(normalized),
		Name:        nullName,
		SeenAt:      seenAt.UTC(),
	}); err != nil {
		return uuid.Nil, "", err
	}
	row, err := q.GetAssetListingByPlatformSymbol(ctx, sqlcgen.GetAssetListingByPlatformSymbolParams{Platform: string(platform), Symbol: normalized})
	if err != nil {
		return uuid.Nil, "", err
	}
	stored, err := uuid.FromBytes(row.ID)
	if err != nil {
		return uuid.Nil, "", err
	}
	return stored, row.AssetSymbol, nil
}

// EnsureInstrument upserts an instrument inside q and returns its ID.
func (s *Service) EnsureInstrument(ctx context.Context, q *sqlcgen.Queries, platform adapters.Platform, in *adapters.Instrument, seenAt time.Time) (uuid.UUID, error) {
	symbol, err := NormalizeSymbol(in.Symbol)
	if err != nil {
		return uuid.Nil, err
	}
	if in.Kind == "" {
		return uuid.Nil, ErrInvalid
	}
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, err
	}
	params := sqlcgen.UpsertInstrumentParams{
		ID:               id[:],
		Platform:         string(platform),
		Symbol:           symbol,
		Kind:             in.Kind,
		BaseSymbol:       optionalString(in.BaseSymbol),
		QuoteSymbol:      optionalString(in.QuoteSymbol),
		SettlementSymbol: optionalString(in.SettlementSymbol),
		SeenAt:           seenAt.UTC(),
	}
	if in.ExpiryAt != nil {
		params.ExpiryAt = sql.NullTime{Time: in.ExpiryAt.UTC(), Valid: true}
	}
	if in.ContractSize != nil && decimal.Valid(*in.ContractSize) {
		params.ContractSize = sql.NullString{String: *in.ContractSize, Valid: true}
	}
	if err := q.UpsertInstrument(ctx, params); err != nil {
		return uuid.Nil, err
	}
	row, err := q.GetInstrumentByPlatformSymbol(ctx, sqlcgen.GetInstrumentByPlatformSymbolParams{Platform: string(platform), Symbol: symbol})
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.FromBytes(row.ID)
}

// RecordPrice upserts one Platform price inside q. Prices are stored in their
// quote symbol; valuation treats USDT, USDC, and USD quotes as USD (ADR 0003).
func (s *Service) RecordPrice(ctx context.Context, q *sqlcgen.Queries, platform adapters.Platform, price adapters.Price) error {
	asset, err := NormalizeSymbol(price.AssetSymbol)
	if err != nil {
		return err
	}
	quote, err := NormalizeSymbol(price.QuoteSymbol)
	if err != nil {
		return err
	}
	if !decimal.Valid(price.Price) {
		return ErrInvalid
	}
	return q.UpsertPrice(ctx, sqlcgen.UpsertPriceParams{
		Platform:    string(platform),
		AssetSymbol: asset,
		ObservedAt:  price.ObservedAt.UTC(),
		Price:       price.Price,
		QuoteSymbol: quote,
	})
}
