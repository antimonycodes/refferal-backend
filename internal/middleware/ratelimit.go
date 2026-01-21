package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/cirvee/referral-backend/internal/cache"
)

type RateLimiter struct {
	cache    *cache.Cache
	requests int
	window   time.Duration
}

func NewRateLimiter(cache *cache.Cache, requests int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		cache:    cache,
		requests: requests,
		window:   window,
	}
}

func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get client IP
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = forwarded
		}

		key := fmt.Sprintf("rate_limit:%s", ip)
		ctx := r.Context()

		// Increment counter
		count, err := rl.cache.Incr(ctx, key)
		if err != nil {
			// If Redis fails, allow request but log error
			next.ServeHTTP(w, r)
			return
		}

		// Set expiry on first request
		if count == 1 {
			rl.cache.Expire(ctx, key, rl.window)
		}

		// Check limit
		if int(count) > rl.requests {
			w.Header().Set("Retry-After", fmt.Sprintf("%.0f", rl.window.Seconds()))
			http.Error(w, `{"error": "rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		// Add rate limit headers
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.requests))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", rl.requests-int(count)))

		next.ServeHTTP(w, r)
	})
}
