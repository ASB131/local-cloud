package netx

import (
	"net"
	"strings"
)

// ParseTrustedProxyCIDRs parses comma-separated CIDRs (e.g. "127.0.0.1/32,172.16.0.0/12").
func ParseTrustedProxyCIDRs(raw string) ([]*net.IPNet, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]*net.IPNet, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		_, n, err := net.ParseCIDR(p)
		if err != nil {
			// Allow single IP without suffix.
			if ip := net.ParseIP(p); ip != nil {
				if ip4 := ip.To4(); ip4 != nil {
					n = &net.IPNet{IP: ip4, Mask: net.CIDRMask(32, 32)}
				} else {
					n = &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)}
				}
			} else {
				return nil, err
			}
		}
		out = append(out, n)
	}
	return out, nil
}

func IPInNets(ip net.IP, nets []*net.IPNet) bool {
	if ip == nil {
		return false
	}
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
