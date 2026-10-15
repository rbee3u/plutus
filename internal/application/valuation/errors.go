package valuation

import "github.com/rbee3u/plutus/internal/shared/apperr"

var (
	// ErrNotFound is returned when the point, account, or portfolio is missing.
	ErrNotFound      = apperr.NotFound("Valuation data not found.")
	ErrDetailExpired = apperr.NotFound("The point summary remains, but its detailed holdings have expired.")
	// ErrNoPoint is returned when a portfolio has no valuation point yet.
	ErrNoPoint = apperr.NotFound("No valuation point exists yet.")
)
