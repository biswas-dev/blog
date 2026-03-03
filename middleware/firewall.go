package middleware

import (
	"net/http"

	"anshumanbiswas.com/blog/models"
)

// BanMiddleware checks the in-memory cache BEFORE calling next.
// Returns 403 with no body for banned IPs (minimal bandwidth wasted).
// Must be registered BEFORE TrackingMiddleware so banned IPs are never recorded.
func BanMiddleware(cache *models.IPBanCache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cache.IsBanned(ExtractIP(r)) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
