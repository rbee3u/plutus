package platformassetexclusion

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

// History retains all versions in descending version order for each platform asset.
// A newer version wins only when it predates the cycle's creation time.
type History struct {
	platformAssets map[uuid.UUID][]policyEvent
}

func LoadHistory(ctx context.Context, db *sql.DB, portfolioID uuid.UUID) (History, error) {
	rows, err := sqlcgen.New(db).ListPlatformAssetExclusionHistory(ctx, portfolioID[:])
	if err != nil {
		return History{}, err
	}
	h := History{platformAssets: map[uuid.UUID][]policyEvent{}}
	for _, row := range rows {
		id, err := uuid.FromBytes(row.PlatformAssetID)
		if err != nil {
			return History{}, err
		}
		h.platformAssets[id] = append(h.platformAssets[id], policyEvent{at: row.EffectiveAt, ignored: row.Ignored})
	}
	return h, nil
}

func (h History) At(at time.Time) map[uuid.UUID]bool {
	active := map[uuid.UUID]bool{}
	for id, events := range h.platformAssets {
		for _, event := range events {
			if event.at.Before(at) {
				active[id] = event.ignored
				break
			}
		}
	}
	return active
}
