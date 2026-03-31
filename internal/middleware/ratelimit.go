package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter applies per-IP rate limiting using a token bucket.
// Each IP is allowed one request per interval.
//
// The limiter map grows with the number of unique IPs seen. For a contact
// form with typical low traffic this is acceptable. A TTL-based eviction
// strategy would be needed if operating at high volume or under a DDoS.
type RateLimiter struct {
	mu       sync.Mutex
	entries  map[string]*rateLimitEntry
	interval time.Duration
}

type rateLimitEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter creates a RateLimiter allowing one request per interval per IP.
func NewRateLimiter(interval time.Duration) *RateLimiter {
	return &RateLimiter{
		entries:  make(map[string]*rateLimitEntry),
		interval: interval,
	}
}

// Middleware returns an http.Handler middleware that enforces the rate limit.
// Exceeded requests receive 429 Too Many Requests with a Retry-After header.
//
// IP extraction honours X-Real-IP and X-Forwarded-For headers set by upstream
// reverse proxies. See clientIP for extraction priority and trust assumptions.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !rl.allow(ip) {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(rl.interval.Seconds())))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"ok":false,"error":"rate limit exceeded"}`)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	e, ok := rl.entries[ip]
	if !ok {
		e = &rateLimitEntry{
			limiter: rate.NewLimiter(rate.Every(rl.interval), 1),
		}
		rl.entries[ip] = e
	}
	e.lastSeen = time.Now()
	return e.limiter.Allow()
}

// clientIP extracts the real client IP from the request.
//
// Priority:
//  1. X-Real-IP — set by nginx/Caddy as a single verified IP.
//  2. First address in X-Forwarded-For — the original client IP prepended
//     by the reverse proxy (assumed trusted in this deployment).
//  3. r.RemoteAddr — the direct TCP connection address.
//
// Note: X-Forwarded-For can be spoofed if the reverse proxy does not strip
// client-supplied values. This is a deployment concern, not a code concern.
func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Real-IP"); v != "" {
		return strings.TrimSpace(v)
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first, _, found := strings.Cut(xff, ",")
		if found {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
