package valuation

import "github.com/rbee3u/plutus/internal/shared/apperr"

var (
	// ErrNotFound is returned when the point, account, or portfolio is missing.
	ErrNotFound = apperr.NotFound("valuation data not found")
	// ErrInvalid is returned when a range, granularity, period kind, filter, or page is malformed.
	ErrInvalid = apperr.Invalid("valuation request is invalid")
	// ErrNoPoint is returned when a Portfolio has no Valuation Point yet.
	ErrNoPoint = apperr.NotFound("no valuation point exists yet")
)
