package auth

import (
	"sync"
	"time"
)

// IPLimiter tracks request counts per IP in a sliding window.
type IPLimiter struct {
	mu      sync.Mutex
	max     int
	window  time.Duration
	entries map[string]*ipAttempt
}

func NewIPLimiter(max int, window time.Duration) *IPLimiter {
	l := &IPLimiter{
		max:     max,
		window:  window,
		entries: make(map[string]*ipAttempt),
	}
	go l.cleanupLoop()
	return l
}

func (l *IPLimiter) Allow(ip string) bool {
	key := normalizeIP(ip)
	now := time.Now().UTC()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.entries[key]
	if !ok {
		e = &ipAttempt{}
		l.entries[key] = e
	}
	filtered := e.times[:0]
	for _, t := range e.times {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	e.times = filtered
	if len(e.times) >= l.max {
		return false
	}
	e.times = append(e.times, now)
	return true
}

func (l *IPLimiter) cleanupLoop() {
	ticker := time.NewTicker(limiterCleanupEvery)
	defer ticker.Stop()
	for range ticker.C {
		l.cleanup()
	}
}

func (l *IPLimiter) cleanup() {
	now := time.Now().UTC()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()
	for key, e := range l.entries {
		filtered := e.times[:0]
		for _, t := range e.times {
			if t.After(cutoff) {
				filtered = append(filtered, t)
			}
		}
		if len(filtered) == 0 {
			delete(l.entries, key)
			continue
		}
		e.times = filtered
	}
}
