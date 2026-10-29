package netx

import (
	"net"
	"testing"
)

func TestIsLocalOrPrivate(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"192.168.1.10", true},
		{"10.0.0.5", true},
		{"172.16.0.1", true},
		{"127.0.0.1", true},
		{"8.8.8.8", false},
		{"203.0.113.1", false},
		{"::1", true},
		{"fd12::1", true},
		{"2001:db8::1", false},
	}
	for _, tc := range tests {
		ip := net.ParseIP(tc.ip)
		if got := IsLocalOrPrivate(ip); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.ip, got, tc.want)
		}
	}
}
