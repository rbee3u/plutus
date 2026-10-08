package audit

import (
	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/shared/cursor"
)

// cursorCodec is the audit list cursor's wire identity: version 1 signed with
// the audit key. The wire format lives in shared/cursor.
var cursorCodec = cursor.Codec{Version: 1, Key: []byte("plutus-audit-cursor-v1")}

func decodeCursor(raw string, portfolioID uuid.UUID) (cursor.Cursor, error) {
	decoded, err := cursorCodec.Decode(raw, portfolioID)
	if err != nil {
		return cursor.Cursor{}, ErrInvalidCursor
	}
	return decoded, nil
}
