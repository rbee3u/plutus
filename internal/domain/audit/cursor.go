package audit

import (
	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/shared/cursor"
)

// cursorCodec is the audit list cursor's wire identity: version 1 signed with
// the audit key. The wire format lives in shared/cursor.
var cursorCodec = cursor.Codec{Version: 1, Key: []byte("plutus-audit-cursor-v1")}

type listCursor struct {
	PortfolioID uuid.UUID
	OccurredAt  int64
	ID          uuid.UUID
}

func encodeCursor(c listCursor) (string, error) {
	return cursorCodec.Encode(cursor.Cursor{PortfolioID: c.PortfolioID, TimeMicro: c.OccurredAt, ID: c.ID})
}

func decodeCursor(raw string, portfolioID uuid.UUID) (listCursor, error) {
	decoded, err := cursorCodec.Decode(raw, portfolioID)
	if err != nil {
		return listCursor{}, ErrInvalidCursor
	}
	return listCursor{PortfolioID: decoded.PortfolioID, OccurredAt: decoded.TimeMicro, ID: decoded.ID}, nil
}
