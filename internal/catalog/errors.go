package catalog

import "errors"

var (
	// ErrInvalidPage is returned when offset pagination is out of range.
	ErrInvalidPage = errors.New("page is invalid")
	// ErrNotMapped is returned when a listing has no confirmed mapping.
	ErrNotMapped = errors.New("asset listing is not confirmed")
	// ErrNotFound is returned when a listing, instrument, mapping, or pin is missing.
	ErrNotFound = errors.New("resource not found")
	// ErrPrecondition is returned when If-Match does not match.
	ErrPrecondition = errors.New("version mismatch")
	// ErrInvalid is returned for an illegal catalog payload or filter.
	ErrInvalid = errors.New("catalog decision is invalid")
	// ErrExists is returned when creating a mapping or pin that already exists.
	ErrExists = errors.New("catalog decision already exists")
	// ErrInvalidSubject is returned when a pin subject kind is not listing or instrument.
	ErrInvalidSubject = errors.New("catalog pin subject kind is invalid")
)
