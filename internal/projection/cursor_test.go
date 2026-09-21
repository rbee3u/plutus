package projection

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDecodeProjectionCursorRejectsForeignPortfolio(t *testing.T) {
	t.Parallel()

	portfolioID := uuid.MustParse("01a0bdfc-1fa4-7a54-9fa1-24f4721a71af")
	other := uuid.MustParse("01a0bdfc-1fa4-7af4-bedf-65d968b9122b")
	encoded, err := encodeCursor(valuationCursorKey, listCursor{PortfolioID: portfolioID, At: 1, ID: portfolioID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeCursor(valuationCursorKey, encoded, other); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("foreign cursor %v", err)
	}
	if _, err := decodeCursor(valueChangeCursorKey, encoded, portfolioID); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("wrong key %v", err)
	}
}

func TestSamplePointsBucketsAndMissingIntervals(t *testing.T) {
	t.Parallel()

	loc := time.UTC
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, loc)
	points := []samplePoint{
		{At: start, ID: uuid.MustParse("01a0bdfc-1fa4-7001-8000-000000000001"), Index: 0},
		{At: start.Add(2 * time.Hour), ID: uuid.MustParse("01a0bdfc-1fa4-7001-8000-000000000002"), Index: 1},
		{At: start.Add(26 * time.Hour), ID: uuid.MustParse("01a0bdfc-1fa4-7001-8000-000000000003"), Index: 2},
	}
	sampled, grain := samplePoints(points, loc, "day", 200)
	if grain != "day" || len(sampled) != 2 {
		t.Fatalf("sampled %s %+v", grain, sampled)
	}
	missing := missingFromPoints(sampled, grain)
	if len(missing) != 0 {
		t.Fatalf("unexpected missing %+v", missing)
	}
	gapped := []time.Time{start, start.Add(5 * 24 * time.Hour)}
	if got := missingIntervals(gapped, "day"); len(got) != 1 {
		t.Fatalf("missing %+v", got)
	}
}

func TestInferGrainUsesTypicalSpacing(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	points := make([]samplePoint, 0, 46)
	for i := range 45 {
		points = append(points, samplePoint{At: start.AddDate(0, 0, i).Add(16 * time.Hour)})
	}
	points = append(points, samplePoint{At: start.AddDate(0, 0, 45).Add(2 * time.Hour)})
	if got := inferGrain(points); got != "day" {
		t.Fatalf("typical daily series inferred %s", got)
	}

	hourly := []samplePoint{
		{At: start},
		{At: start.Add(time.Hour)},
		{At: start.Add(2 * time.Hour)},
		{At: start.Add(3 * time.Hour)},
	}
	if got := inferGrain(hourly); got != "hour" {
		t.Fatalf("hourly series inferred %s", got)
	}
}
