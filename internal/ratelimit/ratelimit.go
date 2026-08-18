package ratelimit

import (
	"crm/internal/ctxutil"
	"crm/internal/respond"
	"net"
	"net/http"
	"sync"
	"time"
)

type entry struct {
	count       int
	windowStart time.Time
}

// Limiter is a process-local fixed-window limiter keyed by client IP. The app
// is single-instance, so no external store is needed.
type Limiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	requests map[string]*entry
}

func New(limit int, window time.Duration) *Limiter {
	return &Limiter{limit: limit, window: window, requests: map[string]*entry{}}
}

// keyOf derives the per-IP bucket key from the client IP resolved by the
// middleware.ClientIP middleware (trusted proxies only). When no IP was
// resolved, it falls back to the socket peer so the limiter never becomes a
// no-op.
func keyOf(r *http.Request) string {
	if ip := ctxutil.GetClientIP(r); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (l *Limiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.requests) > 10_000 {
		l.pruneLocked(now)
	}
	e, ok := l.requests[key]
	if !ok || now.Sub(e.windowStart) >= l.window {
		l.requests[key] = &entry{count: 1, windowStart: now}
		return true
	}
	e.count++
	return e.count <= l.limit
}

// pruneLocked drops entries whose window elapsed; callers must hold the mutex.
func (l *Limiter) pruneLocked(now time.Time) {
	for key, e := range l.requests {
		if now.Sub(e.windowStart) >= l.window {
			delete(l.requests, key)
		}
	}
}

func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.Allow(keyOf(r)) {
			deny(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// UserMiddleware limits requests per authenticated user, keyed on the user ID
// resolved by the auth middleware. When no user is present it falls back to
// the client IP so the limiter never becomes a no-op. This throttles
// per-account attacks (e.g. current-password guessing) that a per-IP limiter
// would not stop.
func (l *Limiter) UserMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := ctxutil.GetUserID(r)
		if key == "" {
			key = keyOf(r)
		}
		if !l.Allow(key) {
			deny(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func deny(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "60")
	respond.JSON(
		w,
		http.StatusTooManyRequests,
		nil,
		&respond.Error{Code: "RATE_LIMITED", Message: "Too many requests, try again in a minute"},
		nil,
	)
}
