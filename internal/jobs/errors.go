package jobs

import "errors"

var (
	// ErrNotFound is returned when the job or portfolio is missing.
	ErrNotFound = errors.New("job not found")
	// ErrInvalidCursor is returned when a list cursor is malformed or bound to another Portfolio.
	ErrInvalidCursor = errors.New("job cursor is invalid")
	// ErrInvalidLimit is returned when a list limit is out of range.
	ErrInvalidLimit = errors.New("job list limit is invalid")
	// ErrLostLease is returned when a worker no longer owns the claimed job.
	ErrLostLease = errors.New("job lease is no longer owned")
)
