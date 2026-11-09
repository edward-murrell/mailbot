package middleware

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// IPResolver works out the client IP for a request.
// It only reads X-Real-IP and X-Forwarded-For when the direct peer is a
// trusted proxy. Otherwise a client could set those headers to dodge the
// rate limiter.
type IPResolver struct {
	trusted []netip.Prefix
}

// NewIPResolver returns an IPResolver that trusts proxy headers from peers
// inside any of the given prefixes.
func NewIPResolver(trusted []netip.Prefix) *IPResolver {
	return &IPResolver{trusted: trusted}
}

// ClientIP returns the client IP for r.
//
// If the direct peer is untrusted, it returns the peer address.
// If the peer is trusted, it checks in order:
//  1. X-Real-IP, a single address set by the proxy.
//  2. X-Forwarded-For, read right to left. The first untrusted hop is the
//     client. If every hop is trusted, the leftmost hop is used.
//  3. The peer address.
func (res *IPResolver) ClientIP(r *http.Request) string {
	peer := remoteHost(r.RemoteAddr)
	peerAddr, err := netip.ParseAddr(peer)
	if err != nil {
		return peer
	}
	peer = peerAddr.Unmap().String()
	if !res.isTrusted(peerAddr) {
		return peer
	}

	if v := strings.TrimSpace(r.Header.Get("X-Real-IP")); v != "" {
		if addr, err := netip.ParseAddr(v); err == nil {
			return addr.Unmap().String()
		}
	}

	if xff := r.Header.Values("X-Forwarded-For"); len(xff) > 0 {
		hops := strings.Split(strings.Join(xff, ","), ",")
		client := ""
		for i := len(hops) - 1; i >= 0; i-- {
			addr, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
			if err != nil {
				break
			}
			client = addr.Unmap().String()
			if !res.isTrusted(addr) {
				return client
			}
		}
		if client != "" {
			return client
		}
	}

	return peer
}

func (res *IPResolver) isTrusted(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, p := range res.trusted {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

func remoteHost(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
