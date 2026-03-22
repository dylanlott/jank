package app

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type rateLimitEntry struct {
	Count       int
	WindowStart time.Time
}

type RateLimiter struct {
	mu      sync.Mutex
	entries map[string]rateLimitEntry
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		entries: make(map[string]rateLimitEntry),
	}
}

func (rl *RateLimiter) Allow(ip string, maxCount int, window time.Duration) bool {
	now := time.Now()

	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, ok := rl.entries[ip]
	if !ok || now.Sub(entry.WindowStart) >= window {
		rl.entries[ip] = rateLimitEntry{
			Count:       1,
			WindowStart: now,
		}
		return true
	}

	if entry.Count >= maxCount {
		return false
	}

	entry.Count++
	rl.entries[ip] = entry
	return true
}

var authLimiter = NewRateLimiter()

func authRateLimitMiddleware(maxCount int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if !authLimiter.Allow(ip, maxCount, window) {
				w.Header().Set("Retry-After", "60")
				http.Error(w, "Too many requests, try again later", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			first := strings.TrimSpace(parts[0])
			if first != "" {
				return first
			}
		}
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
