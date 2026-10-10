// Package assetexclusion reads the Portfolio policy that was effective when a Collection Cycle began.
package assetexclusion

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// ActiveForCycle returns the Assets excluded at the start of a Collection Cycle.
// A rule saved while a cycle is running first applies to the following cycle.
func ActiveForCycle(ctx context.Context, db *sql.DB, portfolioID, cycleID uuid.UUID) (map[uuid.UUID]bool, error) {
	q := mysql.Queries(ctx, db)
	cycle, err := q.GetCollectionCycle(ctx, sqlcgen.GetCollectionCycleParams{PortfolioID: portfolioID[:], ID: cycleID[:]})
	if err != nil {
		return nil, err
	}
	rows, err := q.ListAssetExclusionsBefore(ctx, sqlcgen.ListAssetExclusionsBeforeParams{PortfolioID: portfolioID[:], EffectiveAt: cycle.CreatedAt})
	if err != nil {
		return nil, err
	}
	excluded := map[uuid.UUID]bool{}
	for _, row := range rows {
		id, err := uuid.FromBytes(row.AssetID)
		if err != nil {
			return nil, err
		}
		if _, decided := excluded[id]; !decided {
			excluded[id] = row.Excluded
		}
	}
	return excluded, nil
}
