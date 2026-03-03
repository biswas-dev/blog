package middleware

import (
	"net/http"
	"strings"
	"time"

	"anshumanbiswas.com/blog/models"
	"anshumanbiswas.com/blog/utils"
)

// TrackingMiddleware records page views for every GET request.
// It calls next.ServeHTTP first (zero added latency) then records asynchronously.
func TrackingMiddleware(analyticsService *models.AnalyticsService, sessionService *models.SessionService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Serve the request first — no latency added
			next.ServeHTTP(w, r)

			// Only track GET requests
			if r.Method != http.MethodGet {
				return
			}

			path := r.URL.Path

			// Skip static assets and API endpoints
			if strings.HasPrefix(path, "/static/") ||
				strings.HasPrefix(path, "/css/") ||
				strings.HasPrefix(path, "/api/") ||
				path == "/favicon.ico" {
				return
			}

			// Extract IP address
			ip := ExtractIP(r)

			// Best-effort user identification via session cookie
			var userID *int
			user, err := utils.IsUserLoggedIn(r, sessionService)
			if err == nil && user != nil {
				userID = &user.UserID
			}

			pv := models.PageView{
				ViewedAt:    time.Now(),
				IPAddress:   ip,
				Path:        path,
				UserAgent:   r.UserAgent(),
				Referrer:    r.Referer(),
				UserID:      userID,
				ContentType: models.ContentTypeForPath(path),
			}

			analyticsService.Record(pv)
		})
	}
}

// ExtractIP tries several headers before falling back to RemoteAddr.
// Exported so firewall.go (same package) and tests can call it directly.
func ExtractIP(r *http.Request) string {
	// Cloudflare
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	// Nginx / reverse proxy
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	// Standard forwarded header (take first)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	// Fall back to RemoteAddr (strip port)
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
