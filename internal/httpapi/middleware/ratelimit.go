package middleware

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// RateLimiter is the seam behind the AI rate limit (ADR-0010): a fixed-window
// per-key counter. Increment records one hit for key and returns the new count
// within the current window, starting a fresh window on the first hit. A Redis
// implementation backs production; MemoryRateLimiter backs tests and the default
// (no Redis needed for CI), mirroring the asynq fake-queue approach.
type RateLimiter interface {
	Increment(ctx context.Context, key string, window time.Duration) (int64, error)
}

// rateLimitMessage mirrors the Rails controllers' throttled body.
const rateLimitMessage = "Rate limit exceeded. Please try again later."

// RateLimit caps admitted requests at limit per window, per (feature, user).
// It runs inside the RequireAuth group, so CurrentUser is set. It increments on
// admission BEFORE the handler runs — so input-validation failures still count
// and a throttled request never reaches the handler (never calls Claude),
// matching Rails' `rate_limit` before_action. Throttled -> 429 JSend fail with a
// Retry-After header. Fail-open: a limiter error is logged and the request is
// allowed (parity with treating the cache as best-effort).
func RateLimit(limiter RateLimiter, feature string, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := CurrentUser(r.Context())
			if !ok {
				// Should not happen inside RequireAuth; let the handler deal with
				// the missing user rather than counting an unattributable hit.
				next.ServeHTTP(w, r)
				return
			}

			key := "ratelimit:" + feature + ":" + strconv.FormatInt(user.ID, 10)
			count, err := limiter.Increment(r.Context(), key, window)
			if err != nil {
				log.Printf("rate limiter error (allowing request): %v", err)
				next.ServeHTTP(w, r)
				return
			}

			if count > int64(limit) {
				writeRateLimited(w, window)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// writeRateLimited renders the 429 body byte-for-byte like Rails' render_rate_limited
// (JSend fail, no trailing newline, charset set) with Retry-After = window seconds.
func writeRateLimited(w http.ResponseWriter, window time.Duration) {
	w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusTooManyRequests)
	data, _ := json.Marshal(map[string]string{"status": "fail", "message": rateLimitMessage})
	_, _ = w.Write(data)
}

// MemoryRateLimiter is an in-process fixed-window limiter for tests and the
// default when no Redis-backed limiter is wired. Safe for concurrent use.
type MemoryRateLimiter struct {
	mu      sync.Mutex
	windows map[string]*memoryWindow
}

type memoryWindow struct {
	count     int64
	expiresAt time.Time
}

// NewMemoryRateLimiter returns an empty in-memory limiter.
func NewMemoryRateLimiter() *MemoryRateLimiter {
	return &MemoryRateLimiter{windows: make(map[string]*memoryWindow)}
}

// Increment records a hit and returns the count within the current window,
// resetting the window once it has elapsed.
func (m *MemoryRateLimiter) Increment(_ context.Context, key string, window time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	entry, ok := m.windows[key]
	if !ok || now.After(entry.expiresAt) {
		entry = &memoryWindow{expiresAt: now.Add(window)}
		m.windows[key] = entry
	}
	entry.count++
	return entry.count, nil
}
