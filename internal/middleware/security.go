package middleware

import "net/http"

// SecurityHeaders sets defensive response headers on every request. The
// CSP allows the self-hosted Vite SPA (self-only scripts and fonts, inline
// styles for Vue bindings). The sha256 hash allows the inline dark-mode
// bootstrap script in frontend/index.html; it must match that script's exact
// content — the Vite build fails if the script and this hash drift apart.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'sha256-LKh/DSvln4WNdab5WhVzQKPz/H9Q5TcG4NvqSUq1Ec8='; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'self'")
		next.ServeHTTP(w, r)
	})
}

