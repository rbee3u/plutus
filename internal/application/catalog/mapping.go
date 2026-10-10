package catalog

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/audit"
	"github.com/rbee3u/plutus/internal/application/jobs"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// NormalizeSymbol upper-cases and validates an Asset, Canonical Asset, or Futures Contract symbol.
func NormalizeSymbol(raw string) (string, error) {
	symbol := strings.ToUpper(strings.TrimSpace(raw))
	if symbol == "" || len(symbol) > maxSymbol || !symbolPattern.MatchString(symbol) {
		return "", ErrInvalid
	}
	return symbol, nil
}

// CanonicalAssetSymbolFor derives the automatic Canonical Asset Symbol from an Asset
// symbol (ADR 0003).
func CanonicalAssetSymbolFor(assetSymbol string) string {
	upper := strings.ToUpper(strings.TrimSpace(assetSymbol))
	// Wrappers such as LDBTC remain distinct; only case and whitespace normalize.
	return upper
}

// MappingInput overrides the Canonical Asset Symbol of an asset.
type MappingInput struct {
	ActorUserID     uuid.UUID
	PortfolioID     uuid.UUID
	RequestID       string
	CanonicalSymbol string
	Version         uint64
}

// PutMapping updates an asset and schedules rebuilds for its affected Portfolios.
// Mapping commits before rebuild enqueueing; failures do not undo that update.
func (s *Service) PutMapping(ctx context.Context, id uuid.UUID, in MappingInput) (Asset, error) {
	item, affected, err := s.putMapping(ctx, id, in)
	if err != nil {
		return Asset{}, err
	}
	if err := s.withExclusion(ctx, in.PortfolioID, &item); err != nil {
		return Asset{}, err
	}
	for _, portfolioID := range affected {
		if _, _, err := s.jobs.EnqueueRecalc(ctx, portfolioID, jobs.RecalcPayload{Reason: "asset mapping changed", AssetID: item.ID.String()}, &in.ActorUserID); err != nil {
			return Asset{}, err
		}
	}
	return item, nil
}

// putMapping commits the mapping and identifies Portfolios with derived values.
func (s *Service) putMapping(ctx context.Context, id uuid.UUID, in MappingInput) (Asset, []uuid.UUID, error) {
	canonicalSymbol, err := NormalizeSymbol(in.CanonicalSymbol)
	if err != nil {
		return Asset{}, nil, err
	}
	current, err := s.GetAsset(ctx, id)
	if err != nil {
		return Asset{}, nil, err
	}
	if current.Version != in.Version {
		return current, nil, ErrPrecondition
	}
	source := MappingManual
	if canonicalSymbol == CanonicalAssetSymbolFor(current.Symbol) {
		source = MappingAuto
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Asset{}, nil, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	n, err := q.UpdateAssetMapping(ctx, sqlcgen.UpdateAssetMappingParams{
		CanonicalSymbol: canonicalSymbol,
		MappingSource:   source,
		UpdatedAt:       now,
		ID:              id[:],
		Version:         in.Version,
	})
	if err != nil {
		if mysql.IsDuplicate(err) {
			return Asset{}, nil, ErrInvalid
		}
		return Asset{}, nil, err
	}
	if n == 0 {
		return current, nil, ErrPrecondition
	}
	updated := current
	updated.CanonicalSymbol = canonicalSymbol
	updated.MappingSource = source
	updated.Version++
	updated.UpdatedAt = now
	if err := audit.Record(ctx, q, now, &audit.Entry{
		PortfolioID:  in.PortfolioID,
		ActorUserID:  in.ActorUserID,
		Action:       "catalog.asset.map",
		ResourceType: "asset",
		ResourceID:   id,
		RequestID:    in.RequestID,
		Before:       auditView(&current),
		After:        auditView(&updated),
	}); err != nil {
		return Asset{}, nil, err
	}
	if err := tx.Commit(); err != nil {
		return Asset{}, nil, err
	}
	holders, err := s.q.ListPortfoliosHoldingAsset(ctx, id[:])
	if err != nil {
		return Asset{}, nil, err
	}
	affected := make([]uuid.UUID, 0, len(holders))
	for _, raw := range holders {
		pid, err := uuid.FromBytes(raw)
		if err != nil {
			return Asset{}, nil, err
		}
		affected = append(affected, pid)
	}
	return updated, affected, nil
}

type assetAudit struct {
	Platform        string `json:"platform"`
	Symbol          string `json:"symbol"`
	CanonicalSymbol string `json:"canonicalSymbol"`
	MappingSource   string `json:"mappingSource"`
	Version         uint64 `json:"version"`
}

func auditView(asset *Asset) assetAudit {
	return assetAudit{Platform: asset.Platform, Symbol: asset.Symbol, CanonicalSymbol: asset.CanonicalSymbol, MappingSource: asset.MappingSource, Version: asset.Version}
}
