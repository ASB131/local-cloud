package netx

import "net"

// IsLocalOrPrivate reports whether ip is suitable for LAN-only access (RFC1918, loopback, link-local, ULA).
func IsLocalOrPrivate(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		switch {
		case ip4[0] == 10:
			return true
		case ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31:
			return true
		case ip4[0] == 192 && ip4[1] == 168:
			return true
		case ip4[0] == 169 && ip4[1] == 254:
			return true
		default:
			return false
		}
	}
	// Unique local IPv6 fc00::/7
	return len(ip) == net.IPv6len && ip[0]&0xfe == 0xfc
}
