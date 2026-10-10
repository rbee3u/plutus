package valuation

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

func identityWasPresent(previous map[uuid.UUID]previousAccount, identityID uuid.UUID) bool {
	for _, account := range previous {
		if account.identityID == identityID {
			return true
		}
	}
	return false
}

func manualTime(at sql.NullTime) *time.Time {
	if !at.Valid {
		return nil
	}
	return &at.Time
}
