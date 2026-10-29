package httpapi

import (
	"net"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/asb131/local-cloud/internal/netx"
)

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.", nil)
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		claims, err := s.tokens.ParseAccess(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.", nil)
			return
		}
		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.", nil)
			return
		}
		ctx := withUser(r.Context(), userID, claims.Admin)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok, admin := userFromContext(r.Context())
		if !ok || !admin {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "Admin access required.", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request, trustedProxies []*net.IPNet) net.IP {
	direct := parseHostIP(r.RemoteAddr)
	if len(trustedProxies) > 0 && netx.IPInNets(direct, trustedProxies) {
		if fwd := strings.TrimSpace(r.Header.Get("X-Real-IP")); fwd != "" {
			if ip := net.ParseIP(fwd); ip != nil {
				return ip
			}
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if ip := net.ParseIP(strings.TrimSpace(parts[0])); ip != nil {
				return ip
			}
		}
	}
	return direct
}

func parseHostIP(remoteAddr string) net.IP {
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil {
		return net.IPv4zero
	}
	return ip
}
