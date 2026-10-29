package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// maxRateLimitBody caps how much of a request body is buffered for keying.
const maxRateLimitBody = 1 << 20

type rateBucket struct {
	count int
	start time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	buckets map[string]*rateBucket
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		limit:   limit,
		window:  window,
		buckets: make(map[string]*rateBucket),
	}
}

// allow reports whether a request from key is within the limit, pruning expired buckets.
func (r *rateLimiter) allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	b, ok := r.buckets[key]
	if !ok || now.Sub(b.start) >= r.window {
		r.buckets[key] = &rateBucket{count: 1, start: now}
		return true
	}

	b.count++
	return b.count <= r.limit
}

// key derives the rate-limit key from the email in the body and the client IP.
func (r *rateLimiter) key(req *http.Request) string {
	body, _ := io.ReadAll(io.LimitReader(req.Body, maxRateLimitBody))
	req.Body = io.NopCloser(bytes.NewReader(body))

	var payload struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		payload.Email = ""
	}

	// RemoteAddr is "ip:port"; the port changes per connection, so it must not
	// participate in the key or every request would get a fresh bucket.
	clientIP := req.RemoteAddr
	if host, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		clientIP = host
	}

	return strings.ToLower(strings.TrimSpace(payload.Email)) + "|" + clientIP
}

func (r *rateLimiter) wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !r.allow(r.key(req)) {
			respondError(w, http.StatusTooManyRequests, "too many requests")
			return
		}
		next(w, req)
	}
}
