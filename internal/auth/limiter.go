package auth

import (
	"sync"
	"time"
)

type limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens   float64
	last     time.Time
	rate     float64
	capacity float64
}

func newLimiter() *limiter {
	return &limiter{buckets: make(map[string]*bucket)}
}

func (l *limiter) allow(key string, now time.Time, ratePerSec, capacity float64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.buckets[key]
	if b == nil {
		b = &bucket{tokens: capacity, last: now, rate: ratePerSec, capacity: capacity}
		l.buckets[key] = b
	}
	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * b.rate
		if b.tokens > b.capacity {
			b.tokens = b.capacity
		}
		b.last = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
