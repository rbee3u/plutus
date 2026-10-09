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

// EnsurePlatformAsset upserts a platform asset inside q and returns its ID and current Asset symbol.
func (s *Service) EnsurePlatformAsset(ctx context.Context, q *sqlcgen.Queries, platform adapters.Platform, symbol, name string, seenAt time.Time) (uuid.UUID, string, error) {
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
	if err := q.UpsertPlatformAsset(ctx, sqlcgen.UpsertPlatformAssetParams{
		ID:          id[:],
		Platform:    string(platform),
		Symbol:      normalized,
		AssetSymbol: AssetSymbolFor(normalized),
		Name:        nullName,
		SeenAt:      seenAt.UTC(),
	}); err != nil {
		return uuid.Nil, "", err
	}
	row, err := q.GetPlatformAssetByPlatformSymbol(ctx, sqlcgen.GetPlatformAssetByPlatformSymbolParams{Platform: string(platform), Symbol: normalized})
	if err != nil {
		return uuid.Nil, "", err
	}
	stored, err := uuid.FromBytes(row.ID)
	if err != nil {
		return uuid.Nil, "", err
	}
	return stored, row.AssetSymbol, nil
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
		ID:                            id[:],
		Platform:                      string(platform),
		Symbol:                        symbol,
		Kind:                          in.Kind,
		BasePlatformAssetSymbol:       optionalSymbolColumn(in.BaseSymbol),
		QuotePlatformAssetSymbol:      optionalSymbolColumn(in.QuoteSymbol),
		SettlementPlatformAssetSymbol: optionalSymbolColumn(in.SettlementSymbol),
		SeenAt:                        seenAt.UTC(),
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
		Platform:         string(platform),
		AssetSymbol:      asset,
		ObservedAt:       price.ObservedAt.UTC(),
		Price:            price.Price,
		QuoteAssetSymbol: quote,
	})
}

func optionalSymbolColumn(value string) sql.NullString {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: trimmed, Valid: true}
}
