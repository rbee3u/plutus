package retention

import (
	"testing"
	"time"
)

func TestHistoryCutoffCalendarYears(t *testing.T) {
	for _, tc := range []struct{ now, zone, want string }{
		{"2036-02-29T12:00:00Z", "UTC", "2026-02-28T00:00:00Z"},
		{"2036-10-10T20:00:00Z", "Asia/Shanghai", "2026-10-10T16:00:00Z"},
		{"2036-10-11T12:00:00Z", "America/New_York", "2026-10-11T04:00:00Z"},
	} {
		t.Run(tc.zone, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339, tc.now)
			if err != nil {
				t.Fatal(err)
			}
			loc, err := time.LoadLocation(tc.zone)
			if err != nil {
				t.Fatal(err)
			}
			if got := historyCutoff(now, loc).Format(time.RFC3339); got != tc.want {
				t.Fatalf("cutoff=%s want %s", got, tc.want)
			}
		})
	}
}
