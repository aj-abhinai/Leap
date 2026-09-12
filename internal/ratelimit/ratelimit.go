package ratelimit

import (
	"crm/internal/ctxutil"
	"crm/internal/respond"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"time"
)

// maxEntries bounds the tracked keys: address rotation cannot grow the map
// without limit. At capacity, elapsed windows are pruned first, then entries
// are evicted in map order until the map is ~90% full, so a unique-key flood
// cannot turn every request into a full-map scan. The bound is the security
// property; eviction order is deliberately not LRU.
const maxEntries = 50_000

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
// no-op. IPv6 addresses are bucketed by their /64 allocation, so cycling
// through addresses inside one allocation cannot mint unlimited keys.
func keyOf(r *http.Request) string {
	ip := ctxutil.GetClientIP(r)
	if ip == "" {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return r.RemoteAddr
		}
		ip = host
	}
	if addr, err := netip.ParseAddr(ip); err == nil {
		// Unmap so an IPv4-mapped IPv6 address buckets as its IPv4 self, not as
		// the single ::/64 allocation every mapped address shares.
		addr = addr.Unmap()
		if addr.Is6() {
			if prefix, err := addr.Prefix(64); err == nil {
				return prefix.String()
			}
		}
	}
	return ip
}

func (l *Limiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.requests[key]
	if !ok && len(l.requests) >= maxEntries {
		l.evictLocked(now, key)
	}
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

// evictLocked makes room for a new key: it drops elapsed windows, then evicts
// active entries down to ~90% of the cap in one pass so a unique-key flood
// cannot turn every request into a full-map scan. The current requester's key
// is kept. Callers must hold the mutex.
func (l *Limiter) evictLocked(now time.Time, keep string) {
	l.pruneLocked(now)
	target := maxEntries - maxEntries/10
	for key := range l.requests {
		if len(l.requests) <= target {
			break
		}
		if key != keep {
			delete(l.requests, key)
		}
	}
}

func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.Allow(keyOf(r)) {
			l.deny(w)
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
			l.deny(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// deny writes the 429 with the actual window length in Retry-After, never
// below one second.
func (l *Limiter) deny(w http.ResponseWriter) {
	secs := int(l.window.Seconds())
	if secs < 1 {
		secs = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(secs))
	respond.JSON(
		w,
		http.StatusTooManyRequests,
		nil,
		&respond.Error{Code: "RATE_LIMITED", Message: "Too many requests, try again in a minute"},
		nil,
	)
}
