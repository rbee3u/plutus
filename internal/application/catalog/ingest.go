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

// EnsureAsset upserts an asset inside q and returns its ID and current Canonical Asset Symbol.
func (s *Service) EnsureAsset(ctx context.Context, q *sqlcgen.Queries, platform adapters.Platform, symbol, name string, seenAt time.Time) (uuid.UUID, string, error) {
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
	if err := q.UpsertAsset(ctx, sqlcgen.UpsertAssetParams{
		ID:              id[:],
		Platform:        string(platform),
		Symbol:          normalized,
		CanonicalSymbol: CanonicalAssetSymbolFor(normalized),
		Name:            nullName,
		SeenAt:          seenAt.UTC(),
	}); err != nil {
		return uuid.Nil, "", err
	}
	row, err := q.GetAssetByPlatformSymbol(ctx, sqlcgen.GetAssetByPlatformSymbolParams{Platform: string(platform), Symbol: normalized})
	if err != nil {
		return uuid.Nil, "", err
	}
	stored, err := uuid.FromBytes(row.ID)
	if err != nil {
		return uuid.Nil, "", err
	}
	return stored, row.CanonicalSymbol, nil
}

// EnsureFuturesContract upserts a futures contract inside q and returns its ID.
func (s *Service) EnsureFuturesContract(ctx context.Context, q *sqlcgen.Queries, platform adapters.Platform, in *adapters.FuturesContract, seenAt time.Time) (uuid.UUID, error) {
	symbol, err := NormalizeSymbol(in.Symbol)
	if err != nil {
		return uuid.Nil, err
	}
	if in.Kind != adapters.KindLinearPerpetual && in.Kind != adapters.KindInversePerpetual &&
		in.Kind != adapters.KindLinearDelivery && in.Kind != adapters.KindInverseDelivery {
		return uuid.Nil, ErrInvalid
	}
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, err
	}
	params := sqlcgen.UpsertFuturesContractParams{
		ID:                    id[:],
		Platform:              string(platform),
		Symbol:                symbol,
		Kind:                  in.Kind,
		BaseAssetSymbol:       optionalSymbolColumn(in.BaseAssetSymbol),
		QuoteAssetSymbol:      optionalSymbolColumn(in.QuoteAssetSymbol),
		SettlementAssetSymbol: optionalSymbolColumn(in.SettlementAssetSymbol),
		SeenAt:                seenAt.UTC(),
	}
	if in.ExpiryAt != nil {
		params.ExpiryAt = sql.NullTime{Time: in.ExpiryAt.UTC(), Valid: true}
	}
	if in.ContractSize != nil && decimal.Valid(*in.ContractSize) {
		params.ContractSize = sql.NullString{String: *in.ContractSize, Valid: true}
	}
	if err := q.UpsertFuturesContract(ctx, params); err != nil {
		return uuid.Nil, err
	}
	row, err := q.GetFuturesContractByPlatformSymbol(ctx, sqlcgen.GetFuturesContractByPlatformSymbolParams{Platform: string(platform), Symbol: symbol})
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.FromBytes(row.ID)
}

// RecordPrice upserts one Platform price inside q. Prices are stored in their
// Quote Canonical Asset Symbol; valuation treats USD, USDT, and USDC as USD (ADR 0003).
func (s *Service) RecordPrice(ctx context.Context, q *sqlcgen.Queries, platform adapters.Platform, price adapters.Price) error {
	canonicalAssetSymbol, err := NormalizeSymbol(price.CanonicalAssetSymbol)
	if err != nil {
		return err
	}
	quoteCanonicalAssetSymbol, err := NormalizeSymbol(price.QuoteCanonicalAssetSymbol)
	if err != nil {
		return err
	}
	if !decimal.Valid(price.Price) {
		return ErrInvalid
	}
	return q.UpsertPrice(ctx, sqlcgen.UpsertPriceParams{
		Platform:                  string(platform),
		CanonicalAssetSymbol:      canonicalAssetSymbol,
		ObservedAt:                price.ObservedAt.UTC(),
		Price:                     price.Price,
		QuoteCanonicalAssetSymbol: quoteCanonicalAssetSymbol,
	})
}

func optionalSymbolColumn(value string) sql.NullString {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: trimmed, Valid: true}
}
