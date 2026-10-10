package assetexclusion

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

type policyEvent struct {
	at      time.Time
	ignored bool
}

// History retains all versions in descending version order for each asset.
// A newer version wins only when it predates the cycle's creation time.
type History struct {
	assets map[uuid.UUID][]policyEvent
}

func LoadHistory(ctx context.Context, db *sql.DB, portfolioID uuid.UUID) (History, error) {
	rows, err := sqlcgen.New(db).ListAssetExclusionHistory(ctx, portfolioID[:])
	if err != nil {
		return History{}, err
	}
	h := History{assets: map[uuid.UUID][]policyEvent{}}
	for _, row := range rows {
		id, err := uuid.FromBytes(row.AssetID)
		if err != nil {
			return History{}, err
		}
		h.assets[id] = append(h.assets[id], policyEvent{at: row.EffectiveAt, ignored: row.Ignored})
	}
	return h, nil
}

func (h History) At(at time.Time) map[uuid.UUID]bool {
	active := map[uuid.UUID]bool{}
	for id, events := range h.assets {
		for _, event := range events {
			if event.at.Before(at) {
				active[id] = event.ignored
				break
			}
		}
	}
	return active
}
