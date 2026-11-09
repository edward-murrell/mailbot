package middleware

import (
	"fmt"
	"net/http"
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
	ips      *IPResolver
}

type rateLimitEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter creates a RateLimiter allowing one request per interval per IP.
// ips decides which address a request counts against.
func NewRateLimiter(interval time.Duration, ips *IPResolver) *RateLimiter {
	return &RateLimiter{
		entries:  make(map[string]*rateLimitEntry),
		interval: interval,
		ips:      ips,
	}
}

// Middleware returns an http.Handler middleware that enforces the rate limit.
// Exceeded requests receive 429 Too Many Requests with a Retry-After header.
//
// See IPResolver.ClientIP for how the client IP is chosen.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := rl.ips.ClientIP(r)
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
