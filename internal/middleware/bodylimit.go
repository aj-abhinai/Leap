package middleware

import "net/http"

// MaxBodyBytes caps request bodies so oversized payloads fail fast
// before handlers decode them.
const MaxBodyBytes = 1 << 20 // 1 MiB

// BodyLimit wraps r.Body with a MaxBytesReader so a request larger than
// MaxBodyBytes is rejected by the first read.
func BodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
		next.ServeHTTP(w, r)
	})
}
