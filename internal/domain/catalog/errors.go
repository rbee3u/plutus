package catalog

import "github.com/rbee3u/plutus/internal/shared/apperr"

var (
	// ErrNotFound is returned when the listing or instrument is missing.
	ErrNotFound = apperr.NotFound("catalog entry not found")
	// ErrInvalid is returned when a symbol, filter, or page is malformed.
	ErrInvalid = apperr.Invalid("catalog request is invalid")
	// ErrPrecondition is returned when If-Match does not match the listing version.
	ErrPrecondition = apperr.Precondition("listing version mismatch")
)
