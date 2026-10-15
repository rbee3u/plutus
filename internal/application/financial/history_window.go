package financial

import (
	"fmt"
	"time"

	"github.com/rbee3u/plutus/internal/shared/apperr"
)

// ResolveHistoryWindow resolves one preset or two wall times in the portfolio timezone.
// Relative windows are anchored once by the caller, never during pagination.
func ResolveHistoryWindow(timezone, preset, fromLocal, toLocal string, now time.Time) (time.Time, time.Time, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: invalid portfolio timezone", ErrInvalid)
	}
	if fromLocal != "" || toLocal != "" {
		if preset != "" || fromLocal == "" || toLocal == "" {
			return time.Time{}, time.Time{}, fmt.Errorf("%w: supply both local endpoints without a preset", ErrInvalid)
		}
		from, err := parseHistoryLocal(fromLocal, loc)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		to, err := parseHistoryLocal(toLocal, loc)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		if !to.After(from) {
			return time.Time{}, time.Time{}, apperr.Invalid("End time must be after start time.")
		}
		return from, to, nil
	}
	to := now.UTC().Truncate(time.Second)
	var from time.Time
	switch preset {
	case "1d", "1D":
		from = to.Add(-24 * time.Hour)
	case "3d", "3D":
		from = to.Add(-3 * 24 * time.Hour)
	case "", "1w", "1W":
		from = to.Add(-7 * 24 * time.Hour)
	case "2w", "2W":
		from = to.Add(-14 * 24 * time.Hour)
	case "1Month", "1m", "1M":
		from = subtractHistoryMonths(to.In(loc), 1)
	case "3m", "3M":
		from = subtractHistoryMonths(to.In(loc), 3)
	case "6m", "6M":
		from = subtractHistoryMonths(to.In(loc), 6)
	case "1Year", "1y", "1Y":
		from = subtractHistoryMonths(to.In(loc), 12)
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("%w: unsupported history preset", ErrInvalid)
	}
	return from.UTC(), to, nil
}

func subtractHistoryMonths(t time.Time, months int) time.Time {
	target := time.Date(t.Year(), t.Month()-time.Month(months), 1, 12, 0, 0, 0, t.Location())
	last := time.Date(target.Year(), target.Month()+1, 0, 12, 0, 0, 0, t.Location()).Day()
	day := min(t.Day(), last)
	return time.Date(target.Year(), target.Month(), day, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}

// Test both surrounding offsets: time.ParseInLocation alone silently resolves gaps and folds.
func parseHistoryLocal(value string, loc *time.Location) (time.Time, error) {
	wall, err := time.Parse("2006-01-02T15:04", value)
	if err != nil {
		wall, err = time.Parse("2006-01-02T15:04:05", value)
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: enter a valid portfolio-local date and time", ErrInvalid)
	}
	offsets := map[int]bool{}
	for h := -48; h <= 48; h++ {
		_, offset := wall.Add(time.Duration(h) * time.Hour).In(loc).Zone()
		offsets[offset] = true
	}
	matches := []time.Time{}
	for offset := range offsets {
		candidate := wall.Add(-time.Duration(offset) * time.Second)
		local := candidate.In(loc)
		if local.Year() == wall.Year() && local.Month() == wall.Month() && local.Day() == wall.Day() && local.Hour() == wall.Hour() && local.Minute() == wall.Minute() && local.Second() == wall.Second() && local.Nanosecond() == wall.Nanosecond() {
			matches = append(matches, candidate)
		}
	}
	switch len(matches) {
	case 0:
		return time.Time{}, fmt.Errorf("%w: this local time does not exist in %s; choose another time", ErrInvalid, loc.String())
	case 1:
		return matches[0].UTC(), nil
	default:
		return time.Time{}, fmt.Errorf("%w: this local time is ambiguous in %s; choose a time outside the clock transition", ErrInvalid, loc.String())
	}
}
