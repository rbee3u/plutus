package financial

import "github.com/rbee3u/plutus/internal/shared/apperr"

// ErrInvalid is returned when a range, granularity, period kind, filter, or page is malformed.
var ErrInvalid = apperr.Invalid("Valuation request is invalid.")

// ErrSeriesLimit asks the caller to narrow the range, coarsen buckets, or page raw points.
var ErrSeriesLimit = apperr.Unprocessable("The series exceeds maxPoints. Use a coarser granularity, a narrower range, or paginated valuation-points for all raw records.")
