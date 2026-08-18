package ratelimit

import (
	"crm/internal/ctxutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAllowWithinLimit(t *testing.T) {
	l := New(10, time.Minute)
	for i := 0; i < 10; i++ {
		if !l.Allow("1.2.3.4") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if l.Allow("1.2.3.4") {
		t.Error("11th request should be denied")
	}
}

func TestWindowResets(t *testing.T) {
	l := New(1, 10*time.Millisecond)
	if !l.Allow("1.2.3.4") {
		t.Fatal("first request should be allowed")
	}
	if l.Allow("1.2.3.4") {
		t.Fatal("second request within window should be denied")
	}
	time.Sleep(15 * time.Millisecond)
	if !l.Allow("1.2.3.4") {
		t.Error("request after window expiry should be allowed")
	}
}

func TestMiddlewareTooMany(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := New(10, time.Minute).Middleware(handler)

	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rr := httptest.NewRecorder()
		mw.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, rr.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rr.Code)
	}
	if rr.Header().Get("Retry-After") != "60" {
		t.Errorf("Retry-After = %q, want 60", rr.Header().Get("Retry-After"))
	}
}

func TestPerIPIsolation(t *testing.T) {
	l := New(1, time.Minute)
	if !l.Allow("1.2.3.4") {
		t.Fatal("first IP should be allowed")
	}
	if l.Allow("1.2.3.4") {
		t.Fatal("same IP should be denied at limit")
	}
	if !l.Allow("5.6.7.8") {
		t.Error("different IP should have its own bucket")
	}
}

func TestKeyOfUsesResolvedClientIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = req.WithContext(ctxutil.WithClientIP(req.Context(), "203.0.113.9"))
	req.RemoteAddr = "10.0.0.2:1234"

	if got := keyOf(req); got != "203.0.113.9" {
		t.Errorf("keyOf = %q, want resolved client IP 203.0.113.9", got)
	}
}

func TestKeyOfFallsBackToSocketPeer(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "10.0.0.2:1234"

	if got := keyOf(req); got != "10.0.0.2" {
		t.Errorf("keyOf = %q, want socket peer 10.0.0.2", got)
	}
}

func TestUserMiddlewareKeysPerUser(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := New(1, time.Minute).UserMiddleware(handler)

	req := httptest.NewRequest(http.MethodPatch, "/", nil)
	req = req.WithContext(ctxutil.WithUserID(req.Context(), "user-a"))
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("user-a first request: expected 200, got %d", rr.Code)
	}

	// A different user has its own bucket and is not throttled.
	req = httptest.NewRequest(http.MethodPatch, "/", nil)
	req = req.WithContext(ctxutil.WithUserID(req.Context(), "user-b"))
	rr = httptest.NewRecorder()
	mw.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("user-b should have its own bucket: expected 200, got %d", rr.Code)
	}

	// The same user is now over the limit.
	req = httptest.NewRequest(http.MethodPatch, "/", nil)
	req = req.WithContext(ctxutil.WithUserID(req.Context(), "user-a"))
	rr = httptest.NewRecorder()
	mw.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("user-a second request: expected 429, got %d", rr.Code)
	}
	if rr.Header().Get("Retry-After") != "60" {
		t.Errorf("Retry-After = %q, want 60", rr.Header().Get("Retry-After"))
	}
}

func TestUserMiddlewareFallsBackToIPWhenUnauthenticated(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := New(1, time.Minute).UserMiddleware(handler)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPatch, "/", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rr := httptest.NewRecorder()
		mw.ServeHTTP(rr, req)
		if i == 0 && rr.Code != http.StatusOK {
			t.Fatalf("first request: expected 200, got %d", rr.Code)
		}
		if i == 1 && rr.Code != http.StatusTooManyRequests {
			t.Fatalf("second request from same IP: expected 429, got %d", rr.Code)
		}
	}
}

func TestSpoofedHeadersDoNotResetBucket(t *testing.T) {
	// A client rotating forwarded headers from an untrusted peer must keep
	// hitting the same bucket keyed on the socket peer.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := New(1, time.Minute).Middleware(handler)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.2")
	rr = httptest.NewRecorder()
	mw.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("spoofed header must not reset the bucket: expected 429, got %d", rr.Code)
	}
}
