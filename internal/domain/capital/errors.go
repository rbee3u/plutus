package capital

import "github.com/rbee3u/plutus/internal/shared/apperr"

var (
	// ErrNotFound is returned when the flow is missing from the Portfolio.
	ErrNotFound = apperr.NotFound("capital flow not found")
	// ErrInvalid is returned when direction, symbol, quantity, time, or note is malformed.
	ErrInvalid = apperr.Invalid("capital flow is invalid")
	// ErrPrecondition is returned when If-Match does not match the current version.
	ErrPrecondition = apperr.Precondition("capital flow version mismatch")
)
