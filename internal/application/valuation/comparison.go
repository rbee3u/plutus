package valuation

import (
	"context"
	"database/sql"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// persistComparisonEvidence runs inside the Point's first-commit transaction.
// Reads of old summaries must never depend on details that retention can remove.
func (s *Service) persistComparisonEvidence(ctx context.Context, q *sqlcgen.Queries, point *Point) error {
	adjustments, err := s.manualBalanceChangesByPoints(ctx, point.PortfolioID, []Point{*point})
	if err != nil {
		return err
	}
	for _, adjustment := range adjustments[point.CycleID] {
		if err := q.InsertManualBalanceChange(ctx, sqlcgen.InsertManualBalanceChangeParams{PortfolioID: point.PortfolioID[:], CycleID: point.CycleID[:], AccountID: adjustment.AccountID[:]}); err != nil {
			return err
		}
	}
	return persistCalculatedAssetChanges(ctx, q, point)
}

func persistCalculatedAssetChanges(ctx context.Context, q *sqlcgen.Queries, point *Point) error {
	prior, err := q.ListLatestCalculatedAssetChanges(ctx, point.PortfolioID[:])
	if err != nil {
		return err
	}
	before := make(map[uuid.UUID]sqlcgen.CalculatedAssetChange, len(prior))
	for _, row := range prior {
		id, err := uuid.FromBytes(row.AssetID)
		if err != nil {
			return err
		}
		before[id] = row
	}
	current, err := q.ListPointCalculatedAssets(ctx, sqlcgen.ListPointCalculatedAssetsParams{PortfolioID: point.PortfolioID[:], CycleID: point.CycleID[:]})
	if err != nil {
		return err
	}
	insert := func(asset []byte, treatment string, canonical sql.NullString) error {
		return q.InsertCalculatedAssetChange(ctx, sqlcgen.InsertCalculatedAssetChangeParams{
			PortfolioID: point.PortfolioID[:], AssetID: asset, EvaluatedAt: point.EvaluatedAt, CycleID: point.CycleID[:],
			Treatment: treatment, CanonicalAssetSymbol: canonical,
		})
	}
	for _, row := range current {
		id, err := uuid.FromBytes(row.AssetID)
		if err != nil {
			return err
		}
		treatment, canonical := "included", sql.NullString{String: row.CanonicalAssetSymbol, Valid: true}
		if row.Excluded {
			treatment, canonical = "excluded", sql.NullString{}
		}
		old, found := before[id]
		delete(before, id)
		if found && old.Treatment == treatment && old.CanonicalAssetSymbol == canonical {
			continue
		}
		if err := insert(row.AssetID, treatment, canonical); err != nil {
			return err
		}
	}
	ids := make([]uuid.UUID, 0, len(before))
	for id, old := range before {
		if old.Treatment != "absent" {
			ids = append(ids, id)
		}
	}
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	for _, id := range ids {
		if err := insert(id[:], "absent", sql.NullString{}); err != nil {
			return err
		}
	}
	return nil
}
