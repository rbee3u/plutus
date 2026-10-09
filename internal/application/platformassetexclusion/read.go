// Package platformassetexclusion reads the Portfolio policy that was effective when a Collection Cycle began.
package platformassetexclusion

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// ActiveForJob returns the Asset PlatformAssets ignored at the start of a Collection Cycle.
// A rule saved while a cycle is running first applies to the following cycle.
func ActiveForJob(ctx context.Context, db *sql.DB, portfolioID, jobID uuid.UUID) (map[uuid.UUID]bool, error) {
	q := sqlcgen.New(db)
	job, err := q.GetJob(ctx, sqlcgen.GetJobParams{PortfolioID: portfolioID[:], ID: jobID[:]})
	if err != nil {
		return nil, err
	}
	rows, err := q.ListPlatformAssetExclusionsBefore(ctx, sqlcgen.ListPlatformAssetExclusionsBeforeParams{PortfolioID: portfolioID[:], EffectiveAt: job.CreatedAt})
	if err != nil {
		return nil, err
	}
	active := map[uuid.UUID]bool{}
	seen := map[uuid.UUID]bool{}
	for _, row := range rows {
		id, err := uuid.FromBytes(row.PlatformAssetID)
		if err != nil {
			return nil, err
		}
		if !seen[id] {
			seen[id] = true
			active[id] = row.Ignored
		}
	}
	return active, nil
}
