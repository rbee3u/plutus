package valuation

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestHistoryWindowPresets(t *testing.T) {
	now := time.Date(2024, 3, 31, 8, 30, 0, 0, time.UTC)
	for _, tc := range []struct{ preset, want string }{
		{"", "2024-03-24T08:30:00Z"},
		{"1d", "2024-03-30T08:30:00Z"},
		{"3d", "2024-03-28T08:30:00Z"},
		{"1w", "2024-03-24T08:30:00Z"},
		{"2w", "2024-03-17T08:30:00Z"},
		{"1Month", "2024-02-29T08:30:00Z"},
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
		{"UTC", "2026-10-04T13:00", "2026-10-04T12:00", "end must"},
		{"UTC", "garbage", "2026-10-04T12:00", "valid"},
		{"UTC", "2026-02-30T12:00", "2026-10-04T12:00", "valid"},
		{"UTC", "2026-10-04T12:00", "", "both"},
	} {
		t.Run(tc.zone+tc.from, func(t *testing.T) {
			from, _, err := ResolveHistoryWindow(tc.zone, "", tc.from, tc.to, time.Now())
			if tc.reason != "" {
				if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), tc.reason) {
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
		zone              string
		dates             []string
		wantHour, wantDay int
	}{
		{"America/New_York", []string{"2026-11-01T05:15:00Z", "2026-11-01T05:45:00Z", "2026-11-01T06:15:00Z", "2026-11-01T06:45:00Z", "2026-11-01T07:15:00Z"}, 2, 1},
		{"America/New_York", []string{"2026-03-08T06:45:00Z", "2026-03-08T07:15:00Z"}, 2, 1},
		{"Australia/Lord_Howe", []string{"2026-04-04T14:35:00Z", "2026-04-04T15:05:00Z", "2026-04-04T16:05:00Z"}, 2, 1},
	} {
		t.Run(tc.zone+tc.dates[0], func(t *testing.T) {
			loc, err := time.LoadLocation(tc.zone)
			if err != nil {
				t.Fatal(err)
			}
			points := make([]Point, 0, len(tc.dates))
			for _, date := range tc.dates {
				at, err := time.Parse(time.RFC3339, date)
				if err != nil {
					t.Fatal(err)
				}
				points = append(points, Point{EvaluatedAt: at})
			}
			for kind, want := range map[string]int{"hour": tc.wantHour, "day": tc.wantDay} {
				got := sample(points, kind, loc)
				if len(got) != want || !got[len(got)-1].EvaluatedAt.Equal(points[len(points)-1].EvaluatedAt) {
					t.Fatalf("%s: %+v", kind, got)
				}
			}
		})
	}
}
