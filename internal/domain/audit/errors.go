package audit

import "github.com/rbee3u/plutus/internal/shared/apperr"

var (
	// ErrInvalidCursor is returned when a list cursor is malformed.
	ErrInvalidCursor = apperr.Invalid("audit cursor is invalid")
	// ErrInvalidPage is returned when a list limit is out of range.
	ErrInvalidPage = apperr.Invalid("page is invalid")
)
