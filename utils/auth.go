// utils/auth.go
package utils

import (
	"net/http"
	"strings"

	"anshumanbiswas.com/blog/models"
)

func IsUserLoggedIn(r *http.Request, sessionService *models.SessionService) (*models.User, error) {
	token, _ := ReadCookie(r, CookieSession)
	email, err := ReadCookie(r, CookieUserEmail)

	if err != nil {
		return nil, err
	}
	return sessionService.User(token, email)
}

// GetClientIP extracts the real client IP from the request, checking
// X-Forwarded-For and X-Real-IP headers before falling back to RemoteAddr.
func GetClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	// Strip port from RemoteAddr if present
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
