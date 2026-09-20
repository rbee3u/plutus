package projection

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

const defaultMaxPoints = 200

var (
	periodKinds   = map[string]bool{"day": true, "iso_week": true, "month": true, "quarter": true, "year": true}
	granularities = map[string]bool{
		"": true, "auto": true, "hour": true, "day": true, "iso_week": true, "month": true, "quarter": true, "year": true,
	}
	exposureGroups = map[string]bool{"": true, "asset": true, "platform": true, "identity": true, "account": true}
)

type samplePoint struct {
	At    time.Time
	ID    uuid.UUID
	Index int
}

func normalizeMaxPoints(value int) (int, error) {
	if value == 0 {
		return defaultMaxPoints, nil
	}
	if value < 1 || value > 500 {
		return 0, ErrInvalidPage
	}
	return value, nil
}

func loadLocation(name string) *time.Location {
	if name == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

func samplePoints(points []samplePoint, loc *time.Location, grain string, maxPoints int) ([]samplePoint, string) {
	if grain == "" {
		grain = "auto"
	}
	used := grain
	if grain == "auto" {
		used = chooseGrain(points, loc, maxPoints)
	}
	if used != "auto" && grainRank(used) > 0 {
		points = bucketLast(points, loc, used)
	}
	if len(points) > maxPoints {
		points = stridePoints(points, maxPoints)
	}
	return points, used
}

func chooseGrain(points []samplePoint, loc *time.Location, maxPoints int) string {
	if len(points) <= maxPoints {
		return inferGrain(points)
	}
	for _, grain := range []string{"hour", "day", "iso_week", "month", "quarter", "year"} {
		if len(bucketLast(points, loc, grain)) <= maxPoints {
			return grain
		}
	}
	return "year"
}

func inferGrain(points []samplePoint) string {
	if len(points) < 2 {
		return "auto"
	}
	minGap := points[1].At.Sub(points[0].At)
	for i := 2; i < len(points); i++ {
		gap := points[i].At.Sub(points[i-1].At)
		if gap > 0 && gap < minGap {
			minGap = gap
		}
	}
	switch {
	case minGap <= 2*time.Hour:
		return "hour"
	case minGap <= 36*time.Hour:
		return "day"
	case minGap <= 10*24*time.Hour:
		return "iso_week"
	case minGap <= 40*24*time.Hour:
		return "month"
	case minGap <= 100*24*time.Hour:
		return "quarter"
	default:
		return "year"
	}
}

func grainRank(grain string) int {
	switch grain {
	case "hour":
		return 1
	case "day":
		return 2
	case "iso_week":
		return 3
	case "month":
		return 4
	case "quarter":
		return 5
	case "year":
		return 6
	default:
		return 0
	}
}

func bucketLast(points []samplePoint, loc *time.Location, grain string) []samplePoint {
	best := map[int64]samplePoint{}
	for _, point := range points {
		key := truncateTime(point.At, loc, grain).UnixMicro()
		cur, ok := best[key]
		if !ok || point.At.After(cur.At) || (point.At.Equal(cur.At) && point.ID.String() > cur.ID.String()) {
			best[key] = point
		}
	}
	out := make([]samplePoint, 0, len(best))
	for _, point := range best {
		out = append(out, point)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].At.Equal(out[j].At) {
			return out[i].At.Before(out[j].At)
		}
		return out[i].ID.String() < out[j].ID.String()
	})
	return out
}

func stridePoints(points []samplePoint, maxPoints int) []samplePoint {
	if maxPoints <= 1 || len(points) <= maxPoints {
		if len(points) == 0 {
			return points
		}
		return []samplePoint{points[len(points)-1]}
	}
	out := make([]samplePoint, 0, maxPoints)
	last := -1
	for i := 0; i < maxPoints; i++ {
		idx := i * (len(points) - 1) / (maxPoints - 1)
		if idx == last {
			continue
		}
		out = append(out, points[idx])
		last = idx
	}
	return out
}

func truncateTime(at time.Time, loc *time.Location, grain string) time.Time {
	local := at.In(loc)
	switch grain {
	case "hour":
		return time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), 0, 0, 0, loc)
	case "day":
		return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	case "iso_week":
		weekday := int(local.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		day := local.AddDate(0, 0, -(weekday - 1))
		return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	case "month":
		return time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, loc)
	case "quarter":
		month := time.Month((int(local.Month())-1)/3*3 + 1)
		return time.Date(local.Year(), month, 1, 0, 0, 0, 0, loc)
	case "year":
		return time.Date(local.Year(), 1, 1, 0, 0, 0, 0, loc)
	default:
		return at
	}
}

func missingIntervals(times []time.Time, grain string) []MissingInterval {
	if len(times) < 2 {
		return nil
	}
	step := grainStep(grain)
	if step == 0 {
		step = medianGap(times)
		if step < time.Hour {
			step = time.Hour
		}
	}
	var out []MissingInterval
	for i := 1; i < len(times); i++ {
		gap := times[i].Sub(times[i-1])
		if gap > 2*step {
			out = append(out, MissingInterval{Start: times[i-1].Add(step), End: times[i]})
		}
	}
	return out
}

func grainStep(grain string) time.Duration {
	switch grain {
	case "hour":
		return time.Hour
	case "day":
		return 24 * time.Hour
	case "iso_week":
		return 7 * 24 * time.Hour
	case "month":
		return 32 * 24 * time.Hour
	case "quarter":
		return 93 * 24 * time.Hour
	case "year":
		return 366 * 24 * time.Hour
	default:
		return 0
	}
}

func medianGap(times []time.Time) time.Duration {
	if len(times) < 2 {
		return time.Hour
	}
	gaps := make([]time.Duration, 0, len(times)-1)
	for i := 1; i < len(times); i++ {
		gap := times[i].Sub(times[i-1])
		if gap > 0 {
			gaps = append(gaps, gap)
		}
	}
	if len(gaps) == 0 {
		return time.Hour
	}
	sort.Slice(gaps, func(i, j int) bool { return gaps[i] < gaps[j] })
	return gaps[len(gaps)/2]
}

func periodOverlaps(start, end time.Time, from, to *time.Time) bool {
	if from != nil && end.Before(*from) {
		return false
	}
	if to != nil && start.After(*to) {
		return false
	}
	return true
}

func inWindow(at time.Time, from, to *time.Time) bool {
	if from != nil && at.Before(*from) {
		return false
	}
	if to != nil && at.After(*to) {
		return false
	}
	return true
}
