package ratelimit

import (
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
