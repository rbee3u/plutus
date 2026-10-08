package jobs

import (
	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/shared/cursor"
)

// cursorCodec is the job list cursor's wire identity: version 2 signed with
// the job key. The wire format lives in shared/cursor. Collection cycles share
// this codec through the jobs package.
var cursorCodec = cursor.Codec{Version: 2, Key: []byte("plutus-job-cursor-v2")}

func decodeCursor(raw string, portfolioID uuid.UUID) (cursor.Cursor, error) {
	decoded, err := cursorCodec.Decode(raw, portfolioID)
	if err != nil {
		return cursor.Cursor{}, ErrInvalidCursor
	}
	return decoded, nil
}
