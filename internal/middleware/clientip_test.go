package middleware

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestClientIP(t *testing.T) {
	loopback := []netip.Prefix{
		netip.MustParsePrefix("127.0.0.0/8"),
		netip.MustParsePrefix("::1/128"),
	}
	all := []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/0"),
		netip.MustParsePrefix("::/0"),
	}
	proxies := []netip.Prefix{
		netip.MustParsePrefix("127.0.0.1/32"),
		netip.MustParsePrefix("10.0.0.0/8"),
	}

	tests := []struct {
		name    string
		trusted []netip.Prefix
		remote  string
		realIP  string
		xff     string
		want    string
	}{
		{"no trust ignores X-Real-IP", nil, "203.0.113.9:4000", "198.51.100.1", "", "203.0.113.9"},
		{"untrusted peer ignores X-Forwarded-For", loopback, "203.0.113.9:4000", "", "198.51.100.1", "203.0.113.9"},
		{"trusted peer uses X-Real-IP", loopback, "127.0.0.1:4000", "198.51.100.1", "192.0.2.1", "198.51.100.1"},
		{"trusted peer uses last X-Forwarded-For hop", loopback, "127.0.0.1:4000", "", "1.1.1.1, 198.51.100.1", "198.51.100.1"},
		{"skips trusted hops in X-Forwarded-For", proxies, "127.0.0.1:4000", "", "1.1.1.1, 198.51.100.1, 10.1.2.3", "198.51.100.1"},
		{"trust all uses leftmost hop", all, "172.17.0.2:4000", "", "198.51.100.1, 172.17.0.3", "198.51.100.1"},
		{"invalid X-Real-IP falls through", loopback, "127.0.0.1:4000", "nonsense", "198.51.100.1", "198.51.100.1"},
		{"invalid hop stops the walk", loopback, "127.0.0.1:4000", "", "nonsense, 198.51.100.1", "198.51.100.1"},
		{"trusted peer without headers", loopback, "127.0.0.1:4000", "", "", "127.0.0.1"},
		{"IPv6 peer", loopback, "[::1]:4000", "", "2001:db8::1", "2001:db8::1"},
		{"IPv4-mapped peer is unmapped", loopback, "[::ffff:127.0.0.1]:4000", "", "198.51.100.1", "198.51.100.1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/contact", nil)
			r.RemoteAddr = tc.remote
			if tc.realIP != "" {
				r.Header.Set("X-Real-IP", tc.realIP)
			}
			if tc.xff != "" {
				r.Header.Set("X-Forwarded-For", tc.xff)
			}
			got := NewIPResolver(tc.trusted).ClientIP(r)
			if got != tc.want {
				t.Errorf("ClientIP() = %q, want %q", got, tc.want)
			}
		})
	}
}
