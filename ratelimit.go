package middleware

import (
	"net"
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
	stopCh   chan struct{}
}

type clientBucket struct {
	tokens    int
	lastReset time.Time
}

// NewRateLimiter creates a new rate limiter.
// It starts a background goroutine that periodically cleans up stale client
// buckets (older than 2× window) to prevent unbounded memory growth.
// Call Stop() when the limiter is no longer needed.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		clients: make(map[string]*clientBucket),
		limit:   limit,
		window:  window,
		stopCh:  make(chan struct{}),
	}

	// Cleanup goroutine: evict entries whose lastReset is older than 2× window.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rl.cleanup()
			case <-rl.stopCh:
				return
			}
		}
	}()

	return rl
}

// Stop terminates the background cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// cleanup removes stale client buckets. Caller must NOT hold rl.mu.
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := time.Now().Add(-2 * rl.window)
	for ip, bucket := range rl.clients {
		if bucket.lastReset.Before(cutoff) {
			delete(rl.clients, ip)
		}
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
			clientIP, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				clientIP = r.RemoteAddr
			}

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
