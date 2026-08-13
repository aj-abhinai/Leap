package middleware

import (
	"crm/internal/ctxutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIPNoTrustedProxiesUsesSocketPeer(t *testing.T) {
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if got := ctxutil.GetClientIP(r); got != "1.2.3.4" {
			t.Errorf("client IP = %q, want 1.2.3.4", got)
		}
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	rr := httptest.NewRecorder()

	ClientIP(nil)(handler).ServeHTTP(rr, req)
	if !called {
		t.Error("handler should be called")
	}
}

func TestClientIPSpoofedHeaderIgnoredFromUntrustedPeer(t *testing.T) {
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if got := ctxutil.GetClientIP(r); got != "5.6.7.8" {
			t.Errorf("client IP = %q, want socket peer 5.6.7.8", got)
		}
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "5.6.7.8:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	rr := httptest.NewRecorder()

	ClientIP([]string{"10.0.0.0/8"})(handler).ServeHTTP(rr, req)
	if !called {
		t.Error("handler should be called")
	}
}

func TestClientIPTrustedProxyXFFResolved(t *testing.T) {
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if got := ctxutil.GetClientIP(r); got != "203.0.113.9" {
			t.Errorf("client IP = %q, want 203.0.113.9", got)
		}
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.2:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.1")
	rr := httptest.NewRecorder()

	ClientIP([]string{"10.0.0.0/8"})(handler).ServeHTTP(rr, req)
	if !called {
		t.Error("handler should be called")
	}
}

func TestClientIPTrustedProxyGarbageChainFallsBackToSocket(t *testing.T) {
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if got := ctxutil.GetClientIP(r); got != "10.0.0.2" {
			t.Errorf("client IP = %q, want socket peer 10.0.0.2", got)
		}
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.2:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.9, not-an-ip")
	rr := httptest.NewRecorder()

	ClientIP([]string{"10.0.0.0/8"})(handler).ServeHTTP(rr, req)
	if !called {
		t.Error("handler should be called")
	}
}
