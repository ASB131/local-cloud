package auth

import (
	"net"
	"time"
)

const (
	loginIPLimit       = 20
	loginIPWindow      = 15 * time.Minute
	limiterCleanupEvery = 5 * time.Minute
)

type ipAttempt struct {
	times []time.Time
}

type LoginIPLimiter struct {
	inner *IPLimiter
}

func NewLoginIPLimiter() *LoginIPLimiter {
	return &LoginIPLimiter{inner: NewIPLimiter(loginIPLimit, loginIPWindow)}
}

func (l *LoginIPLimiter) Allow(ip string) bool {
	return l.inner.Allow(ip)
}

func normalizeIP(ip string) string {
	host, _, err := net.SplitHostPort(ip)
	if err != nil {
		return ip
	}
	return host
}
