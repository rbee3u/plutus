package catalog

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// EnsureAsset upserts an asset inside q and returns its ID and current Canonical Asset Symbol.
func (s *Service) EnsureAsset(ctx context.Context, q *sqlcgen.Queries, portfolioID uuid.UUID, platform adapters.Platform, symbol string, observedAt time.Time) (uuid.UUID, string, error) {
	normalized, err := NormalizeSymbol(symbol)
	if err != nil {
		return uuid.Nil, "", err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, "", err
	}
	if err := q.UpsertAsset(ctx, sqlcgen.UpsertAssetParams{
		PortfolioID:          portfolioID[:],
		ID:                   id[:],
		Platform:             string(platform),
		Symbol:               normalized,
		CanonicalAssetSymbol: CanonicalAssetSymbolFor(normalized),
		ObservedAt:           observedAt.UTC(),
	}); err != nil {
		return uuid.Nil, "", err
	}
	row, err := q.GetAssetByPlatformSymbol(ctx, sqlcgen.GetAssetByPlatformSymbolParams{PortfolioID: portfolioID[:], Platform: string(platform), Symbol: normalized})
	if err != nil {
		return uuid.Nil, "", err
	}
	stored, err := uuid.FromBytes(row.ID)
	if err != nil {
		return uuid.Nil, "", err
	}
	return stored, row.CanonicalAssetSymbol, nil
}

// EnsureFuturesContract returns the Futures Contract whose terms this position
// observed. A -> B -> A reuses A; observation bounds are metadata, not
// effective dates.
func (s *Service) EnsureFuturesContract(ctx context.Context, q *sqlcgen.Queries, portfolioID uuid.UUID, platform adapters.Platform, in *adapters.FuturesContract, observedAt time.Time) (uuid.UUID, error) {
	symbol, err := NormalizeSymbol(in.Symbol)
	if err != nil {
		return uuid.Nil, err
	}
	terms, raw, err := NormalizeFuturesTerms(in)
	if err != nil {
		return uuid.Nil, err
	}
	hash := sha256.Sum256(raw)
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, err
	}
	params := sqlcgen.UpsertFuturesContractParams{PortfolioID: portfolioID[:], ID: id[:], Platform: string(platform), Symbol: symbol, TermsHash: hash[:], Kind: terms.Kind, BaseAssetSymbol: optionalSymbolColumn(terms.BaseAssetSymbol), QuoteAssetSymbol: optionalSymbolColumn(terms.QuoteAssetSymbol), SettlementAssetSymbol: optionalSymbolColumn(terms.SettlementAssetSymbol), ObservedAt: observedAt.UTC()}
	if terms.ExpiryAt != nil {
		params.ExpiryAt = sql.NullTime{Time: *terms.ExpiryAt, Valid: true}
	}
	if terms.ContractSize != nil {
		params.ContractSize = sql.NullString{String: *terms.ContractSize, Valid: true}
	}
	if err := q.UpsertFuturesContract(ctx, params); err != nil {
		return uuid.Nil, err
	}
	row, err := q.GetFuturesContractByTerms(ctx, sqlcgen.GetFuturesContractByTermsParams{PortfolioID: portfolioID[:], Platform: string(platform), Symbol: symbol, TermsHash: hash[:]})
	if err != nil {
		return uuid.Nil, err
	}
	stored := adapters.FuturesContract{Kind: row.Kind, BaseAssetSymbol: row.BaseAssetSymbol.String, QuoteAssetSymbol: row.QuoteAssetSymbol.String, SettlementAssetSymbol: row.SettlementAssetSymbol.String}
	if row.ExpiryAt.Valid {
		stored.ExpiryAt = &row.ExpiryAt.Time
	}
	if row.ContractSize.Valid {
		value, _ := decimal.Parse(row.ContractSize.String)
		stored.ContractSize = decimal.FormatPtr(value)
	}
	storedRaw, err := json.Marshal(stored)
	if err != nil {
		return uuid.Nil, err
	}
	if !bytes.Equal(raw, storedRaw) {
		return uuid.Nil, fmt.Errorf("contract terms hash does not match stored content")
	}
	return uuid.FromBytes(row.ID)
}

// RecordPrice retains a USD-equivalent retrieval and its source market.
// Other quotes cannot value anything and return ErrInvalid.
func (s *Service) RecordPrice(ctx context.Context, q *sqlcgen.Queries, portfolioID uuid.UUID, platform adapters.Platform, price adapters.Price) error {
	symbol, err := NormalizeSymbol(price.CanonicalAssetSymbol)
	if err != nil {
		return err
	}
	quote, err := NormalizeSymbol(price.QuoteCanonicalAssetSymbol)
	if err != nil {
		return err
	}
	value, ok := decimal.Parse(price.Price)
	if !adapters.USDEquivalent(quote) || !decimal.Valid(price.Price) || !ok || value.Sign() <= 0 || price.RetrievedAt.IsZero() || price.SourceMarketRef == "" || len(price.SourceMarketRef) > 255 {
		return ErrInvalid
	}
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	params := sqlcgen.InsertPriceObservationParams{PortfolioID: portfolioID[:], ID: id[:], Platform: string(platform), RequestedCanonicalAssetSymbol: symbol, SourceMarketRef: price.SourceMarketRef, QuoteCanonicalAssetSymbol: quote, Price: price.Price, RetrievedAt: price.RetrievedAt.UTC()}
	if price.SourceObservedAt != nil {
		params.SourceObservedAt = sql.NullTime{Time: price.SourceObservedAt.UTC(), Valid: true}
	}
	return q.InsertPriceObservation(ctx, params)
}

func optionalSymbolColumn(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

// NormalizeFuturesTerms defines canonical bytes used for immutable term identity.
func NormalizeFuturesTerms(in *adapters.FuturesContract) (adapters.FuturesContract, []byte, error) {
	var err error
	terms := *in
	terms.Symbol = ""
	switch terms.Kind {
	case adapters.KindLinearPerpetual, adapters.KindInversePerpetual:
		if terms.ExpiryAt != nil {
			return adapters.FuturesContract{}, nil, ErrInvalid
		}
	case adapters.KindLinearDelivery, adapters.KindInverseDelivery:
		if terms.ExpiryAt == nil {
			return adapters.FuturesContract{}, nil, ErrInvalid
		}
	default:
		return adapters.FuturesContract{}, nil, ErrInvalid
	}
	for _, value := range []*string{&terms.BaseAssetSymbol, &terms.QuoteAssetSymbol, &terms.SettlementAssetSymbol} {
		if *value == "" {
			continue
		}
		*value, err = NormalizeSymbol(*value)
		if err != nil {
			return adapters.FuturesContract{}, nil, err
		}
	}
	if terms.ContractSize != nil {
		size, ok := decimal.Parse(*terms.ContractSize)
		if !decimal.Valid(*terms.ContractSize) || !ok || size.Sign() <= 0 {
			return adapters.FuturesContract{}, nil, ErrInvalid
		}
		terms.ContractSize = decimal.FormatPtr(size)
	}
	if terms.ExpiryAt != nil {
		at := terms.ExpiryAt.UTC().Truncate(time.Microsecond)
		terms.ExpiryAt = &at
	}
	raw, err := json.Marshal(terms)
	if err != nil {
		return adapters.FuturesContract{}, nil, err
	}
	return terms, raw, nil
}
