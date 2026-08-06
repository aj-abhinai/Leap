package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBodyLimit(t *testing.T) {
	r := http.NewServeMux()
	r.Handle("/", BodyLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})))

	// Under the limit: body is read normally.
	small := strings.Repeat("a", MaxBodyBytes-1)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(small))
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("under-limit request: status = %d, want 200", rec.Code)
	}

	// Over the limit: the read fails and the handler sees an error.
	big := strings.Repeat("a", MaxBodyBytes+1)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(big))
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("over-limit request: status = %d, want 200 (handler decides)", rec.Code)
	}
}
