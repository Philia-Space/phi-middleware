package middleware

import (
	"net/http"
	"sync"
	"time"
)

// RateLimiter implements a simple token bucket rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	clients  map[string]*clientBucket
	limit    int
	window   time.Duration
}

type clientBucket struct {
	tokens    int
	lastReset time.Time
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		clients: make(map[string]*clientBucket),
		limit:   limit,
		window:  window,
	}
}

// RateLimit returns a rate limiting middleware.
func RateLimit(limit int, window ...time.Duration) func(http.Handler) http.Handler {
	w := time.Minute
	if len(window) > 0 {
		w = window[0]
	}

	limiter := NewRateLimiter(limit, w)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := r.RemoteAddr

			if !limiter.allow(clientIP) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"success":false,"error":{"code":"RATE_LIMITED","message":"too many requests"}}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (rl *RateLimiter) allow(clientIP string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.clients[clientIP]
	now := time.Now()

	if !exists || now.Sub(bucket.lastReset) > rl.window {
		rl.clients[clientIP] = &clientBucket{
			tokens:    rl.limit,
			lastReset: now,
		}
		return true
	}

	if bucket.tokens <= 0 {
		return false
	}

	bucket.tokens--
	return true
}
