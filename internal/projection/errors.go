package projection

import "errors"

var (
	// ErrTemporary means the rebuild can be retried.
	ErrTemporary = errors.New("projection rebuild is temporarily unavailable")
	// ErrFailed means the generation was stored as failed without activation.
	ErrFailed = errors.New("projection rebuild failed")
	// ErrInvalidPage is returned when offset pagination is out of range.
	ErrInvalidPage = errors.New("page is invalid")
	// ErrInvalidGroup is returned when a grouping dimension is not on the whitelist.
	ErrInvalidGroup = errors.New("grouping is not supported")
	// ErrInvalidCursor is returned when a signed list cursor is malformed.
	ErrInvalidCursor = errors.New("projection cursor is invalid")
	// ErrInvalidQuery is returned when a time window or sampling parameter is invalid.
	ErrInvalidQuery = errors.New("query is invalid")
)
