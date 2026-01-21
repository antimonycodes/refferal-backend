package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/cirvee/referral-backend/internal/cache"
)

// AuthRateLimiter provides stricter rate limiting for authentication endpoints
type AuthRateLimiter struct {
	cache    *cache.Cache
	requests int           // Max requests per window (e.g., 5)
	window   time.Duration // Time window (e.g., 1 minute)
}

// NewAuthRateLimiter creates a new auth-specific rate limiter
// Recommended: 5 requests per minute for login/register/password-reset
func NewAuthRateLimiter(cache *cache.Cache, requests int, window time.Duration) *AuthRateLimiter {
	return &AuthRateLimiter{
		cache:    cache,
		requests: requests,
		window:   window,
	}
}

// Limit applies stricter rate limiting for auth endpoints
func (rl *AuthRateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get client IP
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = forwarded
		}

		// Use a different key prefix for auth rate limiting
		key := fmt.Sprintf("auth_rate_limit:%s:%s", r.URL.Path, ip)
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
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error": "too many authentication attempts, please try again later"}`))
			return
		}

		// Add rate limit headers
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.requests))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", rl.requests-int(count)))

		next.ServeHTTP(w, r)
	})
}
