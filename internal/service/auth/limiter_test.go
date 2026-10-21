package auth

import (
	"testing"
	"time"
)

func TestLimiterBurstRefillAndKeyIsolation(t *testing.T) {
	t.Parallel()

	lim := newLimiter()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// Each step continues the same history; denied attempts must not lose refill progress.
	for _, step := range []struct {
		name     string
		key      string
		after    time.Duration
		attempts int
		allowed  bool
	}{
		{"initial burst", "alice", 0, 5, true},
		{"burst exhausted", "alice", 0, 1, false},
		{"independent key", "bob", 0, 1, true},
		{"partial refill", "alice", 6 * time.Second, 1, false},
		{"one token restored", "alice", 12 * time.Second, 1, true},
		{"restored token consumed", "alice", 12 * time.Second, 1, false},
		{"refill repeats", "alice", 24 * time.Second, 1, true},
		{"no extra token", "alice", 24 * time.Second, 1, false},
		{"long idle restores capacity", "alice", time.Hour, 5, true},
		{"refill capped at capacity", "alice", time.Hour, 1, false},
	} {
		for attempt := 0; attempt < step.attempts; attempt++ {
			if got := lim.allow("user:"+step.key, start.Add(step.after), 5.0/60.0, 5); got != step.allowed {
				t.Fatalf("%s: attempt %d allowed = %v, want %v", step.name, attempt+1, got, step.allowed)
			}
		}
	}
}

func TestCSRFValid(t *testing.T) {
	t.Parallel()

	if CSRFValid("", "token") || CSRFValid("token", "") || CSRFValid("a", "b") {
		t.Fatal("expected mismatch")
	}
	if !CSRFValid("same-token", "same-token") {
		t.Fatal("expected match")
	}
}
