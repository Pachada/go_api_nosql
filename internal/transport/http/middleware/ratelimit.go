package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter is a per-IP token-bucket rate limiter with automatic stale-entry cleanup.
type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*ipLimiter
	r        rate.Limit
	burst    int
}

// NewRateLimiter creates a per-IP limiter: r requests/second, burst up to burst requests.
// The provided context controls the lifetime of the background cleanup goroutine;
// cancel it (e.g. on server shutdown) to stop the goroutine and avoid leaks.
func NewRateLimiter(ctx context.Context, r rate.Limit, burst int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*ipLimiter),
		r:        r,
		burst:    burst,
	}
	go rl.cleanup(ctx)
	return rl
}

func (rl *RateLimiter) get(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if v, ok := rl.limiters[ip]; ok {
		v.lastSeen = time.Now()
		return v.limiter
	}
	l := rate.NewLimiter(rl.r, rl.burst)
	rl.limiters[ip] = &ipLimiter{limiter: l, lastSeen: time.Now()}
	return l
}

// cleanup removes stale entries every 5 minutes until ctx is cancelled.
func (rl *RateLimiter) cleanup(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Minute):
			rl.mu.Lock()
			for ip, v := range rl.limiters {
				if time.Since(v.lastSeen) > 10*time.Minute {
					delete(rl.limiters, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}

// Limit is the middleware handler that enforces the rate limit per client IP.
// It reads the real client IP from X-Forwarded-For (set by API Gateway / proxies)
// and falls back to RemoteAddr for direct connections.
//
// NOTE: for Lambda + API Gateway deployments this in-process limiter is a
// secondary defence only — its state is lost on cold starts. Configure
// API Gateway throttling and/or WAF rate-based rules as the primary layer.
func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := realIP(r)
		if !rl.get(ip).Allow() {
			http.Error(w, `{"error":"too many requests"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// realIP extracts the originating client IP from X-Forwarded-For (first entry),
// X-Real-Ip, or falls back to the TCP remote address.
//
// SECURITY NOTE: X-Forwarded-For can be spoofed by clients if the API is
// reached directly without a trusted proxy. This limiter should be treated as
// a secondary defence. Configure rate limits at the API Gateway / WAF level
// as the primary layer so that untrusted headers never reach this code.
func realIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can be a comma-separated list: client, proxy1, proxy2
		// The leftmost entry is the original client IP.
		if ip := strings.TrimSpace(strings.SplitN(xff, ",", 2)[0]); ip != "" {
			return ip
		}
	}
	if xri := r.Header.Get("X-Real-Ip"); xri != "" {
		return xri
	}
	// Strip port from RemoteAddr for consistency.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
