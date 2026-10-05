package catalog

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/domain/audit"
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

// AssetSymbolFor derives the automatic Asset symbol from a Platform listing
// symbol (ADR 0003).
func AssetSymbolFor(symbol string) string {
	upper := strings.ToUpper(strings.TrimSpace(symbol))
	// Platform-specific wrappers such as "LDBTC" or "BETHUSDT" are left as-is; only the
	// common lowercase and whitespace differences are removed so that USDT on every Platform
	// merges into one Asset.
	return upper
}

// MappingInput overrides the Asset symbol of a listing.
type MappingInput struct {
	ActorUserID uuid.UUID
	PortfolioID uuid.UUID
	RequestID   string
	AssetSymbol string
	Version     uint64
}

// PutMapping sets the listing's Asset symbol. Equal to the listing symbol restores automatic mapping.
// It returns the updated listing and the Portfolios whose points hold this listing.
func (s *Service) PutMapping(ctx context.Context, id uuid.UUID, in MappingInput) (Listing, []uuid.UUID, error) {
	asset, err := NormalizeSymbol(in.AssetSymbol)
	if err != nil {
		return Listing{}, nil, err
	}
	current, err := s.GetListing(ctx, id)
	if err != nil {
		return Listing{}, nil, err
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
		return Listing{}, nil, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	n, err := q.UpdateAssetListingMapping(ctx, sqlcgen.UpdateAssetListingMappingParams{
		AssetSymbol:   asset,
		MappingSource: source,
		UpdatedAt:     now,
		ID:            id[:],
		Version:       in.Version,
	})
	if err != nil {
		if mysql.IsDuplicate(err) {
			return Listing{}, nil, ErrInvalid
		}
		return Listing{}, nil, err
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
		Action:       "catalog.listing.map",
		ResourceType: "asset_listing",
		ResourceID:   id,
		RequestID:    in.RequestID,
		Before:       auditView(&current),
		After:        auditView(&updated),
	}); err != nil {
		return Listing{}, nil, err
	}
	if err := tx.Commit(); err != nil {
		return Listing{}, nil, err
	}
	holders, err := s.q.ListPortfoliosHoldingListing(ctx, id[:])
	if err != nil {
		return Listing{}, nil, err
	}
	affected := make([]uuid.UUID, 0, len(holders))
	for _, raw := range holders {
		pid, err := uuid.FromBytes(raw)
		if err != nil {
			return Listing{}, nil, err
		}
		affected = append(affected, pid)
	}
	return updated, affected, nil
}
