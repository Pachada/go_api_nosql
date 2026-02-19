package middleware

import (
	"net/http"
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
func NewRateLimiter(r rate.Limit, burst int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*ipLimiter),
		r:        r,
		burst:    burst,
	}
	go rl.cleanup()
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

// cleanup removes stale entries every 5 minutes.
func (rl *RateLimiter) cleanup() {
	for {
		time.Sleep(5 * time.Minute)
		rl.mu.Lock()
		for ip, v := range rl.limiters {
			if time.Since(v.lastSeen) > 10*time.Minute {
				delete(rl.limiters, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Limit is the middleware handler that enforces the rate limit per remote IP.
func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if !rl.get(ip).Allow() {
			http.Error(w, `{"error":"too many requests"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
