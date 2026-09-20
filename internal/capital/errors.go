package capital

import "errors"

var (
	// ErrNotFound is returned when the baseline or flow is missing.
	ErrNotFound = errors.New("resource not found")
	// ErrPrecondition is returned when If-Match does not match.
	ErrPrecondition = errors.New("version mismatch")
	// ErrInvalid is returned for an illegal capital payload.
	ErrInvalid = errors.New("capital fact is invalid")
	// ErrInvalidPage is returned when pagination is out of range.
	ErrInvalidPage = errors.New("page is invalid")
	// ErrInvalidCursor is returned when a revision cursor is malformed.
	ErrInvalidCursor = errors.New("capital cursor is invalid")
	// ErrExists is returned when creating a baseline that already exists without a matching ETag.
	ErrExists = errors.New("capital baseline already exists")
)
