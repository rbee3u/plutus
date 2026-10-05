package jobs

import "github.com/rbee3u/plutus/internal/shared/apperr"

var (
	// ErrNotFound is returned when the job or portfolio is missing.
	ErrNotFound = apperr.NotFound("job not found")
	// ErrInvalidCursor is returned when a list cursor is malformed or bound to another Portfolio.
	ErrInvalidCursor = apperr.Invalid("job cursor is invalid")
	// ErrInvalidLimit is returned when a list limit is out of range.
	ErrInvalidLimit = apperr.Invalid("job list limit is invalid")
	// ErrLostLease is returned when a worker no longer owns the claimed job.
	ErrLostLease = apperr.Internal("job lease is no longer owned")
)
