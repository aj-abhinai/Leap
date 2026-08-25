package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBodyLimit(t *testing.T) {
	var readErr error
	readAll := func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		readErr = err
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}
	r := http.NewServeMux()
	r.Handle("/", BodyLimit(http.HandlerFunc(readAll)))

	// Under the limit: the body is read in full.
	small := strings.Repeat("a", MaxBodyBytes-1)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(small))
	r.ServeHTTP(rec, req)
	if readErr != nil {
		t.Fatalf("under-limit read failed: %v", readErr)
	}
	if rec.Body.Len() != len(small) {
		t.Errorf("under-limit body length = %d, want %d", rec.Body.Len(), len(small))
	}

	// Over the limit: the read fails and the handler only ever sees a
	// truncated body.
	big := strings.Repeat("a", MaxBodyBytes*2)
	readErr = nil
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(big))
	r.ServeHTTP(rec, req)
	var mbe *http.MaxBytesError
	if !errors.As(readErr, &mbe) {
		t.Fatalf("over-limit read err = %v, want *http.MaxBytesError", readErr)
	}
	if rec.Body.Len() >= len(big) {
		t.Errorf("over-limit body length = %d, want truncated below %d", rec.Body.Len(), len(big))
	}
}
