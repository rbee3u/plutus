package financial

import "github.com/rbee3u/plutus/internal/shared/apperr"

// ErrInvalid is returned when a range, granularity, period kind, filter, or page is malformed.
var ErrInvalid = apperr.Invalid("valuation request is invalid")
