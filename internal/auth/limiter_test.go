package auth

import (
	"testing"
	"time"
)

func TestLimiterAllowsBurstThenBlocks(t *testing.T) {
	t.Parallel()

	lim := newLimiter()
	now := time.Now()
	for i := 0; i < 5; i++ {
		if !lim.allow("user:alice", now, 5.0/60.0, 5) {
			t.Fatalf("attempt %d blocked", i)
		}
	}
	if lim.allow("user:alice", now, 5.0/60.0, 5) {
		t.Fatal("expected sixth attempt to be limited")
	}
	if !lim.allow("user:bob", now, 5.0/60.0, 5) {
		t.Fatal("different key should have its own burst")
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
