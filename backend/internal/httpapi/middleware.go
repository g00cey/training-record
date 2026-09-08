package httpapi

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// authMiddleware enforces `Authorization: Bearer <API_KEY>` on every route
// except GET /api/health.
func authMiddleware(apiKey string, next http.Handler) http.Handler {
	want := []byte(apiKey)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			next.ServeHTTP(w, r)
			return
		}
		const prefix = "Bearer "
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, prefix) ||
			subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(h, prefix)), want) != 1 {
			writeErr(w, unauthorized("missing or invalid API key"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
