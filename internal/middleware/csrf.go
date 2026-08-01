package middleware

import (
	"crm/internal/auth"
	"crm/internal/respond"
	"net/http"
	"strings"
)

// CSRF guards cookie-authenticated mutations with a stateless double-submit
// pattern: the client must echo the readable CSRF cookie in X-CSRF-Token.
// Requests carrying a Bearer Authorization header are token-authenticated and
// skip validation.
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		default:
			next.ServeHTTP(w, r)
			return
		}

		if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(auth.CSRFCookieName)
		if err != nil || cookie.Value == "" || cookie.Value != r.Header.Get("X-CSRF-Token") {
			respond.JSON(
				w,
				http.StatusForbidden,
				nil,
				&respond.Error{Code: "CSRF_MISMATCH", Message: "Invalid or missing CSRF token"},
				nil,
			)
			return
		}
		next.ServeHTTP(w, r)
	})
}
