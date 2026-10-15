package portfolio

import "github.com/rbee3u/plutus/internal/shared/apperr"

var (
	// ErrNotFound is returned when the portfolio is not owned or missing.
	ErrNotFound = apperr.NotFound("Portfolio not found.")
	// ErrConflict is returned when the portfolio name is already used.
	ErrConflict = apperr.Conflict("Portfolio name already exists.")
	// ErrPrecondition is returned when If-Match does not match the current version.
	ErrPrecondition = apperr.Precondition("Portfolio version mismatch.")
	// ErrUndeletable is returned when deleting the portfolio created with the user.
	ErrUndeletable = apperr.Undeletable("Portfolio is undeletable.")
	// ErrInvalidTimezone is returned when the timezone is not a valid IANA name.
	ErrInvalidTimezone = apperr.Invalid("Timezone is invalid.")
	// ErrInvalidInterval is returned when the collection interval is outside 900..86400 seconds.
	ErrInvalidInterval = apperr.Invalid("Collection interval must be between 900 and 86400 seconds.")
	// ErrInvalidName is returned when the portfolio name is empty or too long.
	ErrInvalidName = apperr.Invalid("Portfolio name is invalid.")
)
