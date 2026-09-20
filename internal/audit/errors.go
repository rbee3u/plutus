package audit

import "errors"

var (
	// ErrInvalidCursor is returned when a list cursor is malformed.
	ErrInvalidCursor = errors.New("audit cursor is invalid")
	// ErrInvalidPage is returned when a list limit is out of range.
	ErrInvalidPage = errors.New("page is invalid")
)
