package serve

import "net"

// LANIP returns the machine's LAN-facing IPv4 address for printing in the QR
// URL, or "" if none can be determined (the caller should fall back to
// instructing the user to find their IP manually).
func LANIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	return pickLANIP(addrs)
}

// pickLANIP chooses the best address to reach this machine over the LAN:
// a private IPv4 if one exists, otherwise the first non-loopback IPv4. It
// ignores loopback and IPv6. Pure, so the selection logic is testable.
func pickLANIP(addrs []net.Addr) string {
	var fallback string
	for _, a := range addrs {
		var ip net.IP
		switch v := a.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		ip4 := ip.To4()
		if ip4 == nil || ip4.IsLoopback() {
			continue
		}
		if ip4.IsPrivate() {
			return ip4.String()
		}
		if fallback == "" {
			fallback = ip4.String()
		}
	}
	return fallback
}
