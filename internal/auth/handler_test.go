package auth

import (
	"crm/internal/activity"
	"crm/internal/ctxutil"
	"crm/internal/testdb"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type handlerEnvelope struct {
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func doJSON(t *testing.T, h http.HandlerFunc, method, path, body string, mutate func(*http.Request) *http.Request) (*httptest.ResponseRecorder, handlerEnvelope) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if mutate != nil {
		req = mutate(req)
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	res := rec.Result()
	defer res.Body.Close()
	var env handlerEnvelope
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return rec, env
}

func TestHandlerLoginSuccess(t *testing.T) {
	db := testdb.New(t)
	seedUser(t, db, "alice@example.com", "correct-horse")
	act := activity.NewService(db)
	h := NewHandler(NewService(db, authTestConfig()), 15*time.Minute, 7*24*time.Hour, true, act)

	rec, env := doJSON(t, h.Login, http.MethodPost, "/api/auth/login",
		`{"email":"alice@example.com","password":"correct-horse"}`, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	var data struct {
		AccessToken      string `json:"access_token"`
		ExpiresAt        int64  `json:"expires_at"`
		MustChangePasswd bool   `json:"must_change_password"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if data.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if data.ExpiresAt <= time.Now().Unix() {
		t.Error("expected access token expiry in the future")
	}
	if data.MustChangePasswd {
		t.Error("expected must_change_password=false for a normal login")
	}

	cookies := rec.Result().Cookies()
	byName := map[string]*http.Cookie{}
	for _, c := range cookies {
		byName[c.Name] = c
	}
	refresh, ok := byName[RefreshCookieName]
	if !ok {
		t.Fatalf("expected %s cookie, got %v", RefreshCookieName, cookies)
	}
	if !refresh.HttpOnly {
		t.Error("refresh cookie must be HttpOnly")
	}
	if !refresh.Secure {
		t.Error("refresh cookie must be Secure when secure cookies are enabled")
	}
	if _, ok := byName[CSRFCookieName]; !ok {
		t.Error("expected CSRF cookie")
	}

	var loginCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM audit_logs WHERE action = 'login' AND user_id = (SELECT id FROM users WHERE email = 'alice@example.com')`,
	).Scan(&loginCount); err != nil {
		t.Fatalf("count login audit: %v", err)
	}
	if loginCount != 1 {
		t.Errorf("expected 1 login audit entry, got %d", loginCount)
	}
}

func TestHandlerLoginInvalidJSON(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db, authTestConfig()), 15*time.Minute, 7*24*time.Hour, false, nil)

	rec, env := doJSON(t, h.Login, http.MethodPost, "/api/auth/login", `{not-json`, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if env.Error == nil || env.Error.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST error, got %+v", env.Error)
	}
}

func TestHandlerLoginMissingFields(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db, authTestConfig()), 15*time.Minute, 7*24*time.Hour, false, nil)

	rec, env := doJSON(t, h.Login, http.MethodPost, "/api/auth/login", `{"email":"a@b.c"}`, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if env.Error == nil || env.Error.Message != "Email and password are required" {
		t.Errorf("expected missing-fields error, got %+v", env.Error)
	}
}

func TestHandlerLoginInvalidCredentials(t *testing.T) {
	db := testdb.New(t)
	seedUser(t, db, "alice@example.com", "correct-horse")
	h := NewHandler(NewService(db, authTestConfig()), 15*time.Minute, 7*24*time.Hour, false, nil)

	rec, env := doJSON(t, h.Login, http.MethodPost, "/api/auth/login",
		`{"email":"alice@example.com","password":"wrong-password"}`, nil)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if env.Error == nil || env.Error.Code != "INVALID_CREDENTIALS" {
		t.Errorf("expected INVALID_CREDENTIALS error, got %+v", env.Error)
	}
}

func TestHandlerLoginMustChangePassword(t *testing.T) {
	db := testdb.New(t)
	seedUserWithFlag(t, db, "bob@example.com", "correct-horse", true)
	h := NewHandler(NewService(db, authTestConfig()), 15*time.Minute, 7*24*time.Hour, false, nil)

	rec, env := doJSON(t, h.Login, http.MethodPost, "/api/auth/login",
		`{"email":"bob@example.com","password":"correct-horse"}`, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var data struct {
		MustChangePasswd bool `json:"must_change_password"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if !data.MustChangePasswd {
		t.Error("expected must_change_password=true for a flagged user")
	}
}

func TestHandlerRefreshRotatesToken(t *testing.T) {
	db := testdb.New(t)
	seedUser(t, db, "alice@example.com", "correct-horse")
	h := NewHandler(NewService(db, authTestConfig()), 15*time.Minute, 7*24*time.Hour, false, nil)

	loginRec, _ := doJSON(t, h.Login, http.MethodPost, "/api/auth/login",
		`{"email":"alice@example.com","password":"correct-horse"}`, nil)
	var refreshCookie *http.Cookie
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == RefreshCookieName {
			refreshCookie = c
		}
	}
	if refreshCookie == nil {
		t.Fatal("login did not set a refresh cookie")
	}

	rec, env := doJSON(t, h.Refresh, http.MethodPost, "/api/auth/refresh", "",
		func(r *http.Request) *http.Request { r.AddCookie(refreshCookie); return r })

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	var data struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if data.AccessToken == "" {
		t.Error("expected non-empty access token")
	}

	var newRefresh string
	for _, c := range rec.Result().Cookies() {
		if c.Name == RefreshCookieName {
			newRefresh = c.Value
		}
	}
	if newRefresh == "" || newRefresh == refreshCookie.Value {
		t.Error("refresh handler should set a rotated refresh cookie")
	}
}

func TestHandlerRefreshMissingCookie(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db, authTestConfig()), 15*time.Minute, 7*24*time.Hour, false, nil)

	rec, env := doJSON(t, h.Refresh, http.MethodPost, "/api/auth/refresh", "", nil)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if env.Error == nil || env.Error.Code != "UNAUTHORIZED" {
		t.Errorf("expected UNAUTHORIZED error, got %+v", env.Error)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == RefreshCookieName && c.MaxAge != -1 {
			t.Error("missing refresh token should clear the refresh cookie")
		}
	}
}

func TestHandlerRefreshInvalidToken(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db, authTestConfig()), 15*time.Minute, 7*24*time.Hour, false, nil)

	rec, env := doJSON(t, h.Refresh, http.MethodPost, "/api/auth/refresh", "",
		func(r *http.Request) *http.Request {
			r.AddCookie(&http.Cookie{Name: RefreshCookieName, Value: "never-issued-token"})
			return r
		})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if env.Error == nil || env.Error.Code != "INVALID_TOKEN" {
		t.Errorf("expected INVALID_TOKEN error, got %+v", env.Error)
	}
}

func TestHandlerRefreshReusedTokenRejected(t *testing.T) {
	db := testdb.New(t)
	seedUser(t, db, "alice@example.com", "correct-horse")
	h := NewHandler(NewService(db, authTestConfig()), 15*time.Minute, 7*24*time.Hour, false, nil)

	loginRec, _ := doJSON(t, h.Login, http.MethodPost, "/api/auth/login",
		`{"email":"alice@example.com","password":"correct-horse"}`, nil)
	var refreshCookie *http.Cookie
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == RefreshCookieName {
			refreshCookie = c
		}
	}

	first, _ := doJSON(t, h.Refresh, http.MethodPost, "/api/auth/refresh", "",
		func(r *http.Request) *http.Request { r.AddCookie(refreshCookie); return r })
	if first.Code != http.StatusOK {
		t.Fatalf("first refresh status = %d, want %d", first.Code, http.StatusOK)
	}

	second, env := doJSON(t, h.Refresh, http.MethodPost, "/api/auth/refresh", "",
		func(r *http.Request) *http.Request { r.AddCookie(refreshCookie); return r })
	if second.Code != http.StatusUnauthorized {
		t.Fatalf("second refresh status = %d, want %d", second.Code, http.StatusUnauthorized)
	}
	if env.Error == nil || env.Error.Code != "TOKEN_REVOKED" {
		t.Errorf("expected TOKEN_REVOKED error, got %+v", env.Error)
	}
}

func TestHandlerLogoutRevokesSession(t *testing.T) {
	db := testdb.New(t)
	seedUser(t, db, "alice@example.com", "correct-horse")
	act := activity.NewService(db)
	h := NewHandler(NewService(db, authTestConfig()), 15*time.Minute, 7*24*time.Hour, false, act)

	loginRec, _ := doJSON(t, h.Login, http.MethodPost, "/api/auth/login",
		`{"email":"alice@example.com","password":"correct-horse"}`, nil)
	var refreshCookie *http.Cookie
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == RefreshCookieName {
			refreshCookie = c
		}
	}
	if refreshCookie == nil {
		t.Fatal("login did not set a refresh cookie")
	}

	rec, env := doJSON(t, h.Logout, http.MethodPost, "/api/auth/logout", "",
		func(r *http.Request) *http.Request { r.AddCookie(refreshCookie); return r })

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var data map[string]string
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if data["message"] != "Logged out" {
		t.Errorf("expected logged-out message, got %+v", data)
	}
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge != -1 {
			t.Errorf("cookie %s should be cleared, got MaxAge=%d", c.Name, c.MaxAge)
		}
	}

	var logoutCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM audit_logs WHERE action = 'logout'`,
	).Scan(&logoutCount); err != nil {
		t.Fatalf("count logout audit: %v", err)
	}
	if logoutCount != 1 {
		t.Errorf("expected 1 logout audit entry, got %d", logoutCount)
	}
}

func TestHandlerLogoutWithoutCookie(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db, authTestConfig()), 15*time.Minute, 7*24*time.Hour, false, nil)

	rec, env := doJSON(t, h.Logout, http.MethodPost, "/api/auth/logout", "", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if env.Data == nil {
		t.Error("expected a data payload")
	}
}

func TestHandlerMe(t *testing.T) {
	db := testdb.New(t)
	id := seedUser(t, db, "alice@example.com", "correct-horse")
	h := NewHandler(NewService(db, authTestConfig()), 15*time.Minute, 7*24*time.Hour, false, nil)

	rec, env := doJSON(t, h.Me, http.MethodGet, "/api/auth/me", "",
		func(r *http.Request) *http.Request {
			return r.WithContext(ctxutil.WithUserID(r.Context(), id))
		})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var u User
	if err := json.Unmarshal(env.Data, &u); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if u.Email != "alice@example.com" {
		t.Errorf("expected email alice@example.com, got %q", u.Email)
	}
}

func TestHandlerUpdateProfile(t *testing.T) {
	db := testdb.New(t)
	id := seedUser(t, db, "alice@example.com", "correct-horse")
	h := NewHandler(NewService(db, authTestConfig()), 15*time.Minute, 7*24*time.Hour, false, nil)

	rec, env := doJSON(t, h.UpdateProfile, http.MethodPatch, "/api/auth/me",
		`{"name":"Alice Renamed","phone":"1234567890"}`,
		func(r *http.Request) *http.Request {
			return r.WithContext(ctxutil.WithUserID(r.Context(), id))
		})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var u User
	if err := json.Unmarshal(env.Data, &u); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if u.Name != "Alice Renamed" || u.Phone != "1234567890" {
		t.Errorf("expected updated profile, got name=%q phone=%q", u.Name, u.Phone)
	}
}

func TestHandlerChangePasswordSuccess(t *testing.T) {
	db := testdb.New(t)
	id := seedUserWithFlag(t, db, "carol@example.com", "original-pw", true)
	h := NewHandler(NewService(db, authTestConfig()), 15*time.Minute, 7*24*time.Hour, false, nil)

	rec, _ := doJSON(t, h.ChangePassword, http.MethodPatch, "/api/auth/me/password",
		`{"current_password":"original-pw","new_password":"new-password-1"}`,
		func(r *http.Request) *http.Request {
			return r.WithContext(ctxutil.WithUserID(r.Context(), id))
		})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestHandlerChangePasswordWrongCurrent(t *testing.T) {
	db := testdb.New(t)
	id := seedUserWithFlag(t, db, "dave@example.com", "real-pw", true)
	h := NewHandler(NewService(db, authTestConfig()), 15*time.Minute, 7*24*time.Hour, false, nil)

	rec, env := doJSON(t, h.ChangePassword, http.MethodPatch, "/api/auth/me/password",
		`{"current_password":"wrong-current","new_password":"new-password-1"}`,
		func(r *http.Request) *http.Request {
			return r.WithContext(ctxutil.WithUserID(r.Context(), id))
		})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if env.Error == nil || env.Error.Code != "INCORRECT_PASSWORD" {
		t.Errorf("expected INCORRECT_PASSWORD error, got %+v", env.Error)
	}
}

func TestHandlerChangePasswordMissingFields(t *testing.T) {
	db := testdb.New(t)
	id := seedUser(t, db, "eve@example.com", "correct-horse")
	h := NewHandler(NewService(db, authTestConfig()), 15*time.Minute, 7*24*time.Hour, false, nil)

	rec, env := doJSON(t, h.ChangePassword, http.MethodPatch, "/api/auth/me/password",
		`{"current_password":"correct-horse"}`,
		func(r *http.Request) *http.Request {
			return r.WithContext(ctxutil.WithUserID(r.Context(), id))
		})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if env.Error == nil || env.Error.Message != "Current and new passwords are required" {
		t.Errorf("expected missing-fields error, got %+v", env.Error)
	}
}

func TestHandlerMeUnknownUser(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db, authTestConfig()), 15*time.Minute, 7*24*time.Hour, false, nil)

	rec, env := doJSON(t, h.Me, http.MethodGet, "/api/auth/me", "",
		func(r *http.Request) *http.Request {
			return r.WithContext(ctxutil.WithUserID(r.Context(), "00000000-0000-0000-0000-000000000000"))
		})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if env.Error == nil || env.Error.Code != "INTERNAL" {
		t.Errorf("expected INTERNAL error, got %+v", env.Error)
	}
}

func TestHandlerUpdateProfileInvalidJSON(t *testing.T) {
	db := testdb.New(t)
	id := seedUser(t, db, "alice@example.com", "correct-horse")
	h := NewHandler(NewService(db, authTestConfig()), 15*time.Minute, 7*24*time.Hour, false, nil)

	rec, env := doJSON(t, h.UpdateProfile, http.MethodPatch, "/api/auth/me", `{bad-json`,
		func(r *http.Request) *http.Request {
			return r.WithContext(ctxutil.WithUserID(r.Context(), id))
		})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if env.Error == nil || env.Error.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST error, got %+v", env.Error)
	}
}

func TestHandlerUpdateProfileUnknownUser(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db, authTestConfig()), 15*time.Minute, 7*24*time.Hour, false, nil)

	rec, env := doJSON(t, h.UpdateProfile, http.MethodPatch, "/api/auth/me",
		`{"name":"Nobody"}`,
		func(r *http.Request) *http.Request {
			return r.WithContext(ctxutil.WithUserID(r.Context(), "00000000-0000-0000-0000-000000000000"))
		})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if env.Error == nil || env.Error.Code != "INTERNAL" {
		t.Errorf("expected INTERNAL error, got %+v", env.Error)
	}
}

func TestHandlerChangePasswordUnknownUser(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db, authTestConfig()), 15*time.Minute, 7*24*time.Hour, false, nil)

	rec, env := doJSON(t, h.ChangePassword, http.MethodPatch, "/api/auth/me/password",
		`{"current_password":"pw","new_password":"new-password-1"}`,
		func(r *http.Request) *http.Request {
			return r.WithContext(ctxutil.WithUserID(r.Context(), "00000000-0000-0000-0000-000000000000"))
		})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if env.Error == nil || env.Error.Code != "INTERNAL" {
		t.Errorf("expected INTERNAL error, got %+v", env.Error)
	}
}
