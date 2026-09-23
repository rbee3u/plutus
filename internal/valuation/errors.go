package valuation

import "errors"

var (
	// ErrNotFound is returned when the point, account, or portfolio is missing.
	ErrNotFound = errors.New("valuation data not found")
	// ErrInvalid is returned when a range, granularity, period kind, filter, or page is malformed.
	ErrInvalid = errors.New("valuation request is invalid")
	// ErrNoPoint is returned when a Portfolio has no Valuation Point yet.
	ErrNoPoint = errors.New("no valuation point exists yet")
)
