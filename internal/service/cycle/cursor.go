package cycle

import (
	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/shared/cursor"
)

// cursorCodec is the cycle list cursor's wire identity. The wire format lives
// in shared/cursor.
var cursorCodec = cursor.Codec{Version: 1, Key: []byte("plutus-cycle-cursor-v1")}

func decodeCursor(raw string, portfolioID uuid.UUID) (cursor.Cursor, error) {
	decoded, err := cursorCodec.Decode(raw, portfolioID)
	if err != nil {
		return cursor.Cursor{}, ErrInvalidCursor
	}
	return decoded, nil
}
