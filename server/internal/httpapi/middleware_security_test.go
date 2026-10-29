package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

)

func TestRequireLANClientBlocksPublicIP(t *testing.T) {
	s := &Server{}
	h := s.requireLANClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.50:12345"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d want 403", rec.Code)
	}
}

func TestRequireLANClientAllowsPrivateIP(t *testing.T) {
	s := &Server{}
	h := s.requireLANClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.20:54321"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d want 200", rec.Code)
	}
}

func TestClientIPIgnoresSpoofedXFFWithoutTrustedProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.50:12345"
	req.Header.Set("X-Forwarded-For", "192.168.1.1")
	ip := clientIP(req, nil)
	if ip.String() != "203.0.113.50" {
		t.Fatalf("got %s want public direct IP", ip)
	}
}
