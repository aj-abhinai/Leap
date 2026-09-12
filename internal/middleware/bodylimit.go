package middleware

import "net/http"

// MaxBodyBytes caps request bodies so oversized payloads fail fast
// before handlers decode them.
const MaxBodyBytes = 1 << 20 // 1 MiB

// MaxBulkBodyBytes is the larger cap for the contact import route, whose
// payload is a batch by design (≤2 MiB per the import contract).
const MaxBulkBodyBytes = 2 << 20 // 2 MiB

// bulkImportPath is the one route allowed the larger payload.
const bulkImportPath = "/api/contacts/bulk"

// BodyLimit wraps r.Body with a MaxBytesReader so a request larger than the
// route's cap is rejected by the first read. The bulk import route carries a
// larger cap than every other endpoint.
func BodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := int64(MaxBodyBytes)
		if r.URL.Path == bulkImportPath {
			limit = MaxBulkBodyBytes
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		next.ServeHTTP(w, r)
	})
}
