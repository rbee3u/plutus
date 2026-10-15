package financial

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/shared/apperr"
)

func TestHistoryWindowPresets(t *testing.T) {
	now := time.Date(2024, 3, 31, 8, 30, 0, 0, time.UTC)
	for _, tc := range []struct{ preset, want string }{
		{"", "2024-03-24T08:30:00Z"},
		{"1d", "2024-03-30T08:30:00Z"},
		{"1D", "2024-03-30T08:30:00Z"},
		{"3d", "2024-03-28T08:30:00Z"},
		{"3D", "2024-03-28T08:30:00Z"},
		{"1w", "2024-03-24T08:30:00Z"},
		{"1W", "2024-03-24T08:30:00Z"},
		{"2w", "2024-03-17T08:30:00Z"},
		{"2W", "2024-03-17T08:30:00Z"},
		{"1Month", "2024-02-29T08:30:00Z"},
		{"1M", "2024-02-29T08:30:00Z"},
		{"3M", "2023-12-31T08:30:00Z"},
		{"6M", "2023-09-30T08:30:00Z"},
		{"1Year", "2023-03-31T08:30:00Z"},
	} {
		t.Run(tc.preset, func(t *testing.T) {
			from, to, err := ResolveHistoryWindow("Asia/Shanghai", tc.preset, "", "", now)
			if err != nil || from.Format(time.RFC3339) != tc.want || !to.Equal(now) {
				t.Fatalf("from=%s to=%s err=%v", from, to, err)
			}
		})
	}
	leap := time.Date(2024, 2, 29, 1, 0, 0, 0, time.UTC)
	from, _, err := ResolveHistoryWindow("UTC", "1Year", "", "", leap)
	if err != nil || from.Format(time.RFC3339) != "2023-02-28T01:00:00Z" {
		t.Fatalf("leap: %v %v", from, err)
	}
	// A calendar month preserves local clock time across DST; a day remains exactly 24 hours.
	now = time.Date(2026, 4, 4, 16, 0, 0, 0, time.UTC)
	from, _, err = ResolveHistoryWindow("America/New_York", "1Month", "", "", now)
	if err != nil || from.Format(time.RFC3339) != "2026-03-04T17:00:00Z" {
		t.Fatalf("calendar month: %v %v", from, err)
	}
}

func TestHistoryLocalTimeValidation(t *testing.T) {
	for _, tc := range []struct{ zone, from, to, reason string }{
		{"Asia/Shanghai", "2026-10-04T12:00", "2026-10-04T13:00", ""},
		{"America/New_York", "2026-03-08T02:30", "2026-03-08T04:00", "does not exist"},
		{"America/New_York", "2026-11-01T01:30", "2026-11-01T03:00", "ambiguous"},
		{"Australia/Lord_Howe", "2026-04-05T01:45", "2026-04-05T03:00", "ambiguous"},
		{"UTC", "2026-10-04T13:00", "2026-10-04T12:00", "End time must be after start time."},
		{"UTC", "garbage", "2026-10-04T12:00", "valid"},
		{"UTC", "2026-02-30T12:00", "2026-10-04T12:00", "valid"},
		{"UTC", "2026-10-04T12:00", "", "both"},
	} {
		t.Run(tc.zone+tc.from, func(t *testing.T) {
			from, _, err := ResolveHistoryWindow(tc.zone, "", tc.from, tc.to, time.Now())
			if tc.reason != "" {
				if apperr.KindOf(err) != apperr.KindInvalid || !strings.Contains(err.Error(), tc.reason) {
					t.Fatalf("%v", err)
				}
				return
			}
			if err != nil || from.Format(time.RFC3339) != "2026-10-04T04:00:00Z" {
				t.Fatalf("%v %v", from, err)
			}
		})
	}
	if _, _, err := ResolveHistoryWindow("UTC", "1w", "2026-10-01T00:00", "2026-10-04T00:00", time.Now()); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, _, err := ResolveHistoryWindow("UTC", "all", "", "", time.Now()); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}

func TestHistorySamplingDST(t *testing.T) {
	for _, tc := range []struct {
		zone      string
		dates     []string
		hour, day []int
	}{
		{"America/New_York", []string{"2026-11-01T05:15:00Z", "2026-11-01T05:45:00Z", "2026-11-01T06:15:00Z", "2026-11-01T06:45:00Z", "2026-11-01T07:15:00Z"}, []int{3, 4}, []int{4}},
		{"America/New_York", []string{"2026-03-08T06:45:00Z", "2026-03-08T07:15:00Z"}, []int{0, 1}, []int{1}},
		{"Australia/Lord_Howe", []string{"2026-04-04T14:35:00Z", "2026-04-04T15:05:00Z", "2026-04-04T16:05:00Z"}, []int{1, 2}, []int{2}},
	} {
		t.Run(tc.zone+tc.dates[0], func(t *testing.T) {
			loc, err := time.LoadLocation(tc.zone)
			if err != nil {
				t.Fatal(err)
			}
			points := make([]sqlcgen.ListValuationPointHeadersBetweenRow, 0, len(tc.dates))
			for i, date := range tc.dates {
				at, err := time.Parse(time.RFC3339, date)
				if err != nil {
					t.Fatal(err)
				}
				points = append(points, sqlcgen.ListValuationPointHeadersBetweenRow{CycleID: []byte{byte(i + 1)}, EvaluatedAt: at})
			}
			for kind, indices := range map[string][]int{"hour": tc.hour, "day": tc.day} {
				want := make([]sqlcgen.ListValuationPointHeadersBetweenRow, 0, len(indices))
				for _, i := range indices {
					want = append(want, points[i])
				}
				got := sample(points, kind, loc)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("%s: got %+v, want %+v", kind, got, want)
				}
			}
		})
	}
}

func FuzzHistorySamplingKeepsFinalPoints(f *testing.F) {
	f.Add(uint32(0), []byte{})
	f.Add(uint32(3599), []byte{0, 0, 4, 96, 255, 1})
	f.Add(uint32(86399), []byte{0, 1, 1, 1, 96})
	f.Fuzz(func(t *testing.T, start uint32, steps []byte) {
		// Bounded increasing UTC histories give an independent integer-bucket
		// oracle; the explicit DST table owns local-calendar transitions.
		steps = steps[:min(len(steps), 128)]
		seconds := int64(1700000000) + int64(start)
		points := make([]sqlcgen.ListValuationPointHeadersBetweenRow, 0, len(steps))
		for i, step := range steps {
			seconds += int64(step)*900 + 1
			points = append(points, sqlcgen.ListValuationPointHeadersBetweenRow{
				CycleID: []byte{byte(i + 1)}, EvaluatedAt: time.Unix(seconds, 0).UTC(),
			})
		}
		original := slices.Clone(points)
		for kind, width := range map[string]int64{"raw": 0, "hour": 3600, "day": 86400} {
			want := make([]sqlcgen.ListValuationPointHeadersBetweenRow, 0, len(points))
			for i, point := range original {
				if width == 0 || i == len(original)-1 || point.EvaluatedAt.Unix()/width != original[i+1].EvaluatedAt.Unix()/width {
					want = append(want, point)
				}
			}
			got := sample(points, kind, time.UTC)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s: got %+v, want final actual points %+v", kind, got, want)
			}
			if again := sample(got, kind, time.UTC); !reflect.DeepEqual(again, got) {
				t.Fatalf("%s sampling is not idempotent", kind)
			}
			if !reflect.DeepEqual(points, original) {
				t.Fatalf("%s sampling changed its input", kind)
			}
		}
	})
}
