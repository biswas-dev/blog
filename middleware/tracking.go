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
// If crawlerRules is non-nil, blocked crawlers receive 429 before the request is served.
func TrackingMiddleware(analyticsService *models.AnalyticsService, sessionService *models.SessionService, crawlerRules ...*models.CrawlerRuleService) func(http.Handler) http.Handler {
	var rules *models.CrawlerRuleService
	if len(crawlerRules) > 0 {
		rules = crawlerRules[0]
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check crawler blocking BEFORE serving (must reject early)
			if rules != nil && r.Method == http.MethodGet {
				crawlerType := models.ClassifyCrawler(r.UserAgent())
				if crawlerType != "" && rules.IsBlocked(crawlerType) {
					http.Error(w, "Crawler temporarily restricted", http.StatusTooManyRequests)
					return
				}
			}

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

			// Skip blog sub-resource endpoints (comments, annotations) — not page views
			if strings.HasSuffix(path, "/comments") || strings.HasSuffix(path, "/annotations") {
				return
			}

			// Skip paths with unresolved JS template literals (bot noise, e.g. /blog/${escapeHTML(post.Slug)})
			if strings.Contains(path, "${") {
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
				CrawlerType: models.ClassifyCrawler(r.UserAgent()),
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
