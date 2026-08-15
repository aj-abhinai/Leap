package middleware

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// themeScriptHash computes the CSP hash of the inline dark-mode bootstrap
// script in frontend/index.html over its exact text content — the same way a
// browser verifies a hash-source. Keeps the header CSP and the script in sync.
func themeScriptHash(t *testing.T) string {
	t.Helper()
	htmlPath := filepath.Join("..", "..", "frontend", "index.html")
	html, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	m := regexp.MustCompile(`(?s)<script>(.*?)</script>`).FindAllSubmatch(html, -1)
	if len(m) != 1 {
		t.Fatalf("expected exactly one inline <script> in frontend/index.html, found %d", len(m))
	}
	sum := sha256.Sum256(m[0][1])
	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}

func TestSecurityHeaders(t *testing.T) {
	r := chi.NewRouter()
	r.Use(SecurityHeaders)
	r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	for _, h := range []string{"X-Content-Type-Options", "Referrer-Policy", "X-Frame-Options", "Permissions-Policy", "Strict-Transport-Security", "Content-Security-Policy"} {
		if rec.Header().Get(h) == "" {
			t.Errorf("missing security header %q", h)
		}
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY", got)
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("Referrer-Policy = %q, want no-referrer", got)
	}
	csp := rec.Header().Get("Content-Security-Policy")
	for _, want := range []string{
		"script-src 'self' " + themeScriptHash(t),
		"font-src 'self' https://fonts.gstatic.com",
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
		"object-src 'none'",
		"frame-ancestors 'none'",
	} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP missing %q: %s", want, csp)
		}
	}
}
