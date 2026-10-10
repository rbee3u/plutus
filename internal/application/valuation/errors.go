package valuation

import "github.com/rbee3u/plutus/internal/shared/apperr"

var (
	// ErrNotFound is returned when the point, account, or portfolio is missing.
	ErrNotFound      = apperr.NotFound("valuation data not found")
	ErrDetailExpired = apperr.NotFound("the point summary remains, but its detailed holdings have expired")
	// ErrNoPoint is returned when a Portfolio has no Valuation Point yet.
	ErrNoPoint = apperr.NotFound("no valuation point exists yet")
)
