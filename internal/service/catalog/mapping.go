package catalog

import (
	"context"
	"strings"

	"golang.org/x/text/unicode/norm"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/service/audit"
)

// NormalizeSymbol upper-cases and validates an asset, canonical asset, or futures contract symbol.
func NormalizeSymbol(raw string) (string, error) {
	symbol := norm.NFC.String(strings.ToUpper(norm.NFC.String(strings.TrimSpace(raw))))
	if symbol == "" || len(symbol) > maxSymbol || !symbolPattern.MatchString(symbol) {
		return "", ErrInvalid
	}
	return symbol, nil
}

// CanonicalAssetSymbolFor derives the automatic canonical asset symbol from an asset
// symbol (ADR 0003).
func CanonicalAssetSymbolFor(assetSymbol string) string {
	upper := norm.NFC.String(strings.ToUpper(norm.NFC.String(strings.TrimSpace(assetSymbol))))
	// Wrappers such as LDBTC remain distinct; only case and whitespace normalize.
	return upper
}

// MappingInput overrides the canonical asset symbol of an asset.
type MappingInput struct {
	ActorUserID          uuid.UUID
	PortfolioID          uuid.UUID
	RequestID            string
	CanonicalAssetSymbol string
	MappingVersion       uint64
}

// PutMapping commits a mapping and its audit event together.
func (s *Service) PutMapping(ctx context.Context, id uuid.UUID, in MappingInput) (Asset, error) {
	canonicalAssetSymbol, err := NormalizeSymbol(in.CanonicalAssetSymbol)
	if err != nil {
		return Asset{}, err
	}
	current, err := s.GetAsset(ctx, in.PortfolioID, id)
	if err != nil {
		return Asset{}, err
	}
	if current.MappingVersion != in.MappingVersion {
		return current, ErrPrecondition
	}
	source := MappingManual
	if canonicalAssetSymbol == CanonicalAssetSymbolFor(current.Symbol) {
		source = MappingAuto
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Asset{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	if _, err := q.LockPortfolio(ctx, in.PortfolioID[:]); err != nil {
		return Asset{}, err
	}
	n, err := q.UpdateAssetMapping(ctx, sqlcgen.UpdateAssetMappingParams{
		PortfolioID:          in.PortfolioID[:],
		CanonicalAssetSymbol: canonicalAssetSymbol,
		ID:                   id[:],
		MappingVersion:       in.MappingVersion,
	})
	if err != nil {
		if mysql.IsDuplicate(err) {
			return Asset{}, ErrInvalid
		}
		return Asset{}, err
	}
	if n == 0 {
		return current, ErrPrecondition
	}
	updated := current
	updated.CanonicalAssetSymbol = canonicalAssetSymbol
	updated.MappingSource = source
	updated.MappingVersion++
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
		return Asset{}, err
	}
	if err := tx.Commit(); err != nil {
		return Asset{}, err
	}
	if err := s.withExclusion(ctx, in.PortfolioID, &updated); err != nil {
		return Asset{}, err
	}
	return updated, nil
}

type assetAudit struct {
	Platform             string `json:"platform"`
	Symbol               string `json:"symbol"`
	CanonicalAssetSymbol string `json:"canonicalAssetSymbol"`
	MappingSource        string `json:"mappingSource"`
	MappingVersion       uint64 `json:"mappingVersion"`
}

func auditView(asset *Asset) assetAudit {
	return assetAudit{Platform: asset.Platform, Symbol: asset.Symbol, CanonicalAssetSymbol: asset.CanonicalAssetSymbol, MappingSource: asset.MappingSource, MappingVersion: asset.MappingVersion}
}
