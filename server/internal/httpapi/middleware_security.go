package httpapi

import (
	"net/http"
	"net/url"

	"github.com/asb131/local-cloud/internal/netx"
)

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// SAMEORIGIN allows in-app preview iframes (PDF / office) on the same host; blocks external embedders.
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if s.cfg.TLSCertFile != "" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		frameSrc := "'self' blob:"
		if s.cfg.OnlyOfficePublicURL != "" {
			if u, err := url.Parse(s.cfg.OnlyOfficePublicURL); err == nil && u.Scheme != "" && u.Host != "" {
				frameSrc += " " + u.Scheme + "://" + u.Host
			}
		}
		scriptSrc := "'self'"
		if s.cfg.OnlyOfficePublicURL != "" {
			if u, err := url.Parse(s.cfg.OnlyOfficePublicURL); err == nil && u.Scheme != "" && u.Host != "" {
				scriptSrc += " " + u.Scheme + "://" + u.Host
			}
		}
		csp := "default-src 'self'; img-src 'self' blob: data:; media-src 'self' blob:; frame-src " + frameSrc +
			"; connect-src 'self' ws: wss:; style-src 'self' 'unsafe-inline'; script-src " + scriptSrc
		w.Header().Set("Content-Security-Policy", csp)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireLANClient(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r, s.trustedProxies)
		if !netx.IsLocalOrPrivate(ip) {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "This service is only available on your local network.", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) rateLimitAPI(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r, s.trustedProxies).String()
		if !s.apiLimiter.Allow(ip) {
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests. Slow down and try again.", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}
