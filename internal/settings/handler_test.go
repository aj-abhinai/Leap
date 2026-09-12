package settings

import (
	"crm/internal/testdb"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestDefaultCountryCodeGetDefaultsAndPersists(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	h := NewHandler(svc)

	r := chi.NewRouter()
	r.Get("/api/settings/default-country-code", h.GetDefaultCountryCode)
	r.Put("/api/settings/default-country-code", h.SetDefaultCountryCode)

	// No row → the code default.
	req := httptest.NewRequest(http.MethodGet, "/api/settings/default-country-code", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET default = %d, want 200", rr.Code)
	}
	var env struct {
		Data struct {
			CountryCode string `json:"country_code"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.CountryCode != "+91" {
		t.Errorf("default = %q, want +91", env.Data.CountryCode)
	}

	// PUT persists; GET returns the stored value.
	req = httptest.NewRequest(
		http.MethodPut,
		"/api/settings/default-country-code",
		strings.NewReader(`{"country_code":"+971"}`),
	)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT = %d, want 200", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/settings/default-country-code", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	env = struct {
		Data struct {
			CountryCode string `json:"country_code"`
		} `json:"data"`
	}{}
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.CountryCode != "+971" {
		t.Errorf("stored = %q, want +971", env.Data.CountryCode)
	}
}

// A malformed code is rejected so a bad row can never be stamped onto stored
// phone numbers by the canonicalization paths.
func TestDefaultCountryCodeRejectsMalformed(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	for _, raw := range []string{"", "91", "+", "+9a1", "++91", "+91 234", "+1234"} {
		if _, err := svc.SetDefaultCountryCode(raw); err == nil {
			t.Errorf("SetDefaultCountryCode(%q) accepted, want rejection", raw)
		}
	}

	if _, err := svc.SetDefaultCountryCode("+971"); err != nil {
		t.Fatalf("SetDefaultCountryCode(+971): %v", err)
	}
	cc, err := DefaultCountryCode(db)
	if err != nil {
		t.Fatalf("read stored code: %v", err)
	}
	if cc != "+971" {
		t.Errorf("stored code = %q, want +971", cc)
	}
}

// The handler surfaces a malformed code as a 400 so the UI shows the
// validation message rather than a server error.
func TestDefaultCountryCodeHandlerRejectsMalformed(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db))

	r := chi.NewRouter()
	r.Put("/api/settings/default-country-code", h.SetDefaultCountryCode)
	req := httptest.NewRequest(
		http.MethodPut,
		"/api/settings/default-country-code",
		strings.NewReader(`{"country_code":"9 1"}`),
	)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("PUT malformed = %d, want 400", rr.Code)
	}
}

// An absent or null minutes value is a malformed client, not an instruction to
// store zero; an explicit zero stays valid (no lead time).
func TestSetNudgeLeadMinutesRequiresExplicitValue(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db))

	r := chi.NewRouter()
	r.Put("/api/settings/nudge-lead-minutes", h.SetNudgeLeadMinutes)

	for _, body := range []string{`{}`, `{"minutes":null}`} {
		req := httptest.NewRequest(http.MethodPut, "/api/settings/nudge-lead-minutes", strings.NewReader(body))
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("PUT %s = %d, want 400", body, rr.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPut, "/api/settings/nudge-lead-minutes", strings.NewReader(`{"minutes":0}`))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT zero = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}
	minutes, err := NewService(db).GetNudgeLeadMinutes()
	if err != nil || minutes != 0 {
		t.Errorf("stored minutes = (%d, %v), want (0, nil)", minutes, err)
	}
}