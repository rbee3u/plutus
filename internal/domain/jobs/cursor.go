package jobs

import (
	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/shared/cursor"
)

// cursorCodec is the job list cursor's wire identity: version 2 signed with
// the job key. The wire format lives in shared/cursor. Collection cycles share
// this codec through the jobs package.
var cursorCodec = cursor.Codec{Version: 2, Key: []byte("plutus-job-cursor-v2")}

type listCursor struct {
	PortfolioID uuid.UUID
	CreatedAt   int64
	ID          uuid.UUID
}

func encodeCursor(c listCursor) (string, error) {
	return cursorCodec.Encode(cursor.Cursor{PortfolioID: c.PortfolioID, TimeMicro: c.CreatedAt, ID: c.ID})
}

func decodeCursor(raw string, portfolioID uuid.UUID) (listCursor, error) {
	decoded, err := cursorCodec.Decode(raw, portfolioID)
	if err != nil {
		return listCursor{}, ErrInvalidCursor
	}
	return listCursor{PortfolioID: decoded.PortfolioID, CreatedAt: decoded.TimeMicro, ID: decoded.ID}, nil
}
