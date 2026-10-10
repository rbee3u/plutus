package cycles

import "github.com/rbee3u/plutus/internal/shared/apperr"

var (
	// ErrNotFound is returned when the cycle or Portfolio is missing.
	ErrNotFound = apperr.NotFound("collection cycle not found")
	// ErrInvalidCursor is returned when a list cursor is malformed or bound to another Portfolio.
	ErrInvalidCursor = apperr.Invalid("collection cycle cursor is invalid")
	// ErrInvalidLimit is returned when a list limit is out of range.
	ErrInvalidLimit = apperr.Invalid("collection cycle list limit is invalid")
	// ErrLostLease is returned when a worker no longer owns the claimed cycle.
	ErrLostLease = apperr.Internal("collection cycle lease is no longer owned")
)
