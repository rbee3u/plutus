package listingexclusion

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHistoryUsesEligibleVersionAndStrictTime(t *testing.T) {
	id := uuid.New()
	at := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	// Versions, rather than effective timestamps, decide precedence.
	h := History{listings: map[uuid.UUID][]policyEvent{id: {
		{at: at.Add(-time.Hour), ignored: false},
		{at: at, ignored: true},
		{at: at.Add(-2 * time.Hour), ignored: true},
	}}}
	if h.At(at.Add(-2 * time.Hour))[id] {
		t.Fatal("event at exactly the cycle time became effective")
	}
	if !h.At(at.Add(-time.Hour))[id] {
		t.Fatal("equal-time newer version replaced the earlier eligible event")
	}
	if h.At(at)[id] || h.At(at.Add(time.Hour))[id] {
		t.Fatal("highest eligible version did not win")
	}
}
