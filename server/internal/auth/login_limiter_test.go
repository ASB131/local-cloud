package auth

import "testing"

func TestLoginIPLimiterAllowsTwentyThenBlocks(t *testing.T) {
	l := &LoginIPLimiter{entries: make(map[string]*ipAttempt)}
	ip := "203.0.113.10"
	for i := 0; i < loginIPLimit; i++ {
		if !l.Allow(ip) {
			t.Fatalf("attempt %d blocked early", i+1)
		}
	}
	if l.Allow(ip) {
		t.Fatal("21st attempt should be blocked")
	}
}
