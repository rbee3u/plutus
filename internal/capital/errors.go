package capital

import "errors"

var (
	// ErrNotFound is returned when the flow is missing from the Portfolio.
	ErrNotFound = errors.New("capital flow not found")
	// ErrInvalid is returned when direction, symbol, quantity, time, or note is malformed.
	ErrInvalid = errors.New("capital flow is invalid")
	// ErrPrecondition is returned when If-Match does not match the current version.
	ErrPrecondition = errors.New("capital flow version mismatch")
)
