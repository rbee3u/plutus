package valuation

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

func canonicalManualState(raw []byte) string {
	if len(raw) == 0 {
		return "{}"
	}
	var amounts map[string]string
	if json.Unmarshal(raw, &amounts) != nil {
		return string(raw)
	}
	encoded, err := json.Marshal(amounts)
	if err != nil {
		return string(raw)
	}
	return string(encoded)
}

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
