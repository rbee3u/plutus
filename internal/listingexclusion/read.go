// Package listingexclusion reads the Portfolio policy that was effective when a Collection Cycle began.
package listingexclusion

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// ActiveForJob returns the Asset Listings ignored at the start of a Collection Cycle.
// A rule saved while a cycle is running first applies to the following cycle.
func ActiveForJob(ctx context.Context, db *sql.DB, portfolioID, jobID uuid.UUID) (map[uuid.UUID]bool, error) {
	var startedAt time.Time
	err := db.QueryRowContext(ctx, "SELECT created_at FROM jobs WHERE portfolio_id = ? AND id = ?", portfolioID[:], jobID[:]).Scan(&startedAt)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `
		SELECT listing_id, ignored
		FROM portfolio_listing_exclusion_events
		WHERE portfolio_id = ? AND effective_at < ?
		ORDER BY listing_id, version DESC`, portfolioID[:], startedAt)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	active := map[uuid.UUID]bool{}
	seen := map[uuid.UUID]bool{}
	for rows.Next() {
		var raw []byte
		var ignored bool
		if err := rows.Scan(&raw, &ignored); err != nil {
			return nil, err
		}
		id, err := uuid.FromBytes(raw)
		if err != nil {
			return nil, err
		}
		if !seen[id] {
			seen[id] = true
			active[id] = ignored
		}
	}
	return active, rows.Err()
}
