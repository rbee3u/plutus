package observation

import "errors"

var (
	// ErrNotFound is returned when the observation or related fact is missing.
	ErrNotFound = errors.New("resource not found")
	// ErrPrecondition is returned when If-Match does not match.
	ErrPrecondition = errors.New("version mismatch")
	// ErrInvalid is returned for an illegal correction or fact payload.
	ErrInvalid = errors.New("observation is invalid")
	// ErrInvalidPage is returned when offset pagination is out of range.
	ErrInvalidPage = errors.New("page is invalid")
	// ErrCoverageRequired is returned when Reported Equity has no covered Identity or Account.
	ErrCoverageRequired = errors.New("reported equity coverage is required")
)
