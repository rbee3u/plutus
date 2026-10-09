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

// NormalizeSymbol upper-cases and validates a Platform symbol.
func NormalizeSymbol(raw string) (string, error) {
	symbol := strings.ToUpper(strings.TrimSpace(raw))
	if symbol == "" || len(symbol) > maxSymbol || !symbolPattern.MatchString(symbol) {
		return "", ErrInvalid
	}
	return symbol, nil
}

// AssetSymbolFor derives the automatic Asset symbol from a Platform asset
// symbol (ADR 0003).
func AssetSymbolFor(symbol string) string {
	upper := strings.ToUpper(strings.TrimSpace(symbol))
	// Wrappers such as LDBTC remain distinct; only case and whitespace normalize.
	return upper
}

// MappingInput overrides the Asset symbol of a platform asset.
type MappingInput struct {
	ActorUserID uuid.UUID
	PortfolioID uuid.UUID
	RequestID   string
	AssetSymbol string
	Version     uint64
}

// PutMapping updates a platform asset and schedules rebuilds for its affected Portfolios.
// Mapping commits before rebuild enqueueing; failures do not undo that update.
func (s *Service) PutMapping(ctx context.Context, id uuid.UUID, in MappingInput) (PlatformAsset, error) {
	item, affected, err := s.putMapping(ctx, id, in)
	if err != nil {
		return PlatformAsset{}, err
	}
	if err := s.withExclusion(ctx, in.PortfolioID, &item); err != nil {
		return PlatformAsset{}, err
	}
	for _, portfolioID := range affected {
		if _, _, err := s.jobs.EnqueueRecalc(ctx, portfolioID, jobs.RecalcPayload{Reason: "platform asset mapping changed", PlatformAssetID: item.ID.String()}, &in.ActorUserID); err != nil {
			return PlatformAsset{}, err
		}
	}
	return item, nil
}

// putMapping commits the mapping and identifies Portfolios with derived values.
func (s *Service) putMapping(ctx context.Context, id uuid.UUID, in MappingInput) (PlatformAsset, []uuid.UUID, error) {
	asset, err := NormalizeSymbol(in.AssetSymbol)
	if err != nil {
		return PlatformAsset{}, nil, err
	}
	current, err := s.GetPlatformAsset(ctx, id)
	if err != nil {
		return PlatformAsset{}, nil, err
	}
	if current.Version != in.Version {
		return current, nil, ErrPrecondition
	}
	source := MappingManual
	if asset == AssetSymbolFor(current.Symbol) {
		source = MappingAuto
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PlatformAsset{}, nil, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	n, err := q.UpdatePlatformAssetMapping(ctx, sqlcgen.UpdatePlatformAssetMappingParams{
		AssetSymbol:   asset,
		MappingSource: source,
		UpdatedAt:     now,
		ID:            id[:],
		Version:       in.Version,
	})
	if err != nil {
		if mysql.IsDuplicate(err) {
			return PlatformAsset{}, nil, ErrInvalid
		}
		return PlatformAsset{}, nil, err
	}
	if n == 0 {
		return current, nil, ErrPrecondition
	}
	updated := current
	updated.AssetSymbol = asset
	updated.MappingSource = source
	updated.Version++
	updated.UpdatedAt = now
	if err := audit.Record(ctx, q, now, &audit.Entry{
		PortfolioID:  in.PortfolioID,
		ActorUserID:  in.ActorUserID,
		Action:       "catalog.platform_asset.map",
		ResourceType: "platform_asset",
		ResourceID:   id,
		RequestID:    in.RequestID,
		Before:       auditView(&current),
		After:        auditView(&updated),
	}); err != nil {
		return PlatformAsset{}, nil, err
	}
	if err := tx.Commit(); err != nil {
		return PlatformAsset{}, nil, err
	}
	holders, err := s.q.ListPortfoliosHoldingPlatformAsset(ctx, id[:])
	if err != nil {
		return PlatformAsset{}, nil, err
	}
	affected := make([]uuid.UUID, 0, len(holders))
	for _, raw := range holders {
		pid, err := uuid.FromBytes(raw)
		if err != nil {
			return PlatformAsset{}, nil, err
		}
		affected = append(affected, pid)
	}
	return updated, affected, nil
}

type platformAssetAudit struct {
	Platform      string `json:"platform"`
	Symbol        string `json:"symbol"`
	AssetSymbol   string `json:"assetSymbol"`
	MappingSource string `json:"mappingSource"`
	Version       uint64 `json:"version"`
}

func auditView(l *PlatformAsset) platformAssetAudit {
	return platformAssetAudit{Platform: l.Platform, Symbol: l.Symbol, AssetSymbol: l.AssetSymbol, MappingSource: l.MappingSource, Version: l.Version}
}
