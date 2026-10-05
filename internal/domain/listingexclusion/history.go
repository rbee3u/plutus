package listingexclusion

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type policyEvent struct {
	at      time.Time
	ignored bool
}

// History retains all versions in descending version order for each listing.
// A newer version wins only when it predates the cycle's creation time.
type History struct {
	listings map[uuid.UUID][]policyEvent
}

func LoadHistory(ctx context.Context, db *sql.DB, portfolioID uuid.UUID) (History, error) {
	rows, err := db.QueryContext(ctx, `SELECT listing_id, effective_at, ignored
		FROM portfolio_listing_exclusion_events WHERE portfolio_id = ?
		ORDER BY listing_id, version DESC`, portfolioID[:])
	if err != nil {
		return History{}, err
	}
	defer func() { _ = rows.Close() }()
	h := History{listings: map[uuid.UUID][]policyEvent{}}
	for rows.Next() {
		var raw []byte
		var event policyEvent
		if err := rows.Scan(&raw, &event.at, &event.ignored); err != nil {
			return History{}, err
		}
		id, err := uuid.FromBytes(raw)
		if err != nil {
			return History{}, err
		}
		h.listings[id] = append(h.listings[id], event)
	}
	return h, rows.Err()
}

func (h History) At(at time.Time) map[uuid.UUID]bool {
	active := map[uuid.UUID]bool{}
	for id, events := range h.listings {
		for _, event := range events {
			if event.at.Before(at) {
				active[id] = event.ignored
				break
			}
		}
	}
	return active
}
