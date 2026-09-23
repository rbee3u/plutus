package catalog

import "errors"

var (
	// ErrNotFound is returned when the listing or instrument is missing.
	ErrNotFound = errors.New("catalog entry not found")
	// ErrInvalid is returned when a symbol, filter, or page is malformed.
	ErrInvalid = errors.New("catalog request is invalid")
	// ErrPrecondition is returned when If-Match does not match the listing version.
	ErrPrecondition = errors.New("listing version mismatch")
)
