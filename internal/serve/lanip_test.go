package serve

import (
	"net"
	"testing"
)

func ipnet(cidr string) *net.IPNet {
	ip, n, _ := net.ParseCIDR(cidr)
	n.IP = ip
	return n
}

func TestPickLANIP(t *testing.T) {
	tests := []struct {
		name  string
		addrs []net.Addr
		want  string
	}{
		{
			name:  "prefers private IPv4 over loopback",
			addrs: []net.Addr{ipnet("127.0.0.1/8"), ipnet("192.168.1.5/24")},
			want:  "192.168.1.5",
		},
		{
			name:  "skips IPv6 and loopback",
			addrs: []net.Addr{ipnet("::1/128"), ipnet("127.0.0.1/8")},
			want:  "",
		},
		{
			name:  "falls back to a non-loopback public IPv4 when no private",
			addrs: []net.Addr{ipnet("127.0.0.1/8"), ipnet("203.0.113.7/24")},
			want:  "203.0.113.7",
		},
		{
			name:  "private wins even when listed after a public address",
			addrs: []net.Addr{ipnet("203.0.113.7/24"), ipnet("10.0.0.9/8")},
			want:  "10.0.0.9",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pickLANIP(tt.addrs); got != tt.want {
				t.Errorf("pickLANIP = %q, want %q", got, tt.want)
			}
		})
	}
}
