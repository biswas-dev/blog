package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"anshumanbiswas.com/blog/models"
)

type EngagementController struct {
	Service *models.EngagementService
}

// Ingest accepts a beacon POST from the browser.
// Called via navigator.sendBeacon (no response required).
func (c EngagementController) Ingest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// sendBeacon sends Content-Type: text/plain by default; accept anything.
	defer r.Body.Close()

	var beacon models.EngagementBeacon
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&beacon); err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Reject noise: no session or bad path
	if beacon.SessionID == "" || beacon.Path == "" || !strings.HasPrefix(beacon.Path, "/") {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	// Skip bot UAs and very short engagements (< 2s active)
	ua := r.UserAgent()
	if isBot(ua) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	ip := extractIP(r)
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	// Note: user_id extraction intentionally omitted — engagement tied to session_id + ip
	_ = c.Service.Upsert(ctx, beacon, ip, ua, nil)
	w.WriteHeader(http.StatusNoContent)
}

func extractIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		if comma := strings.Index(ip, ","); comma > 0 {
			return strings.TrimSpace(ip[:comma])
		}
		return ip
	}
	// RemoteAddr is host:port
	if colon := strings.LastIndex(r.RemoteAddr, ":"); colon > 0 {
		return r.RemoteAddr[:colon]
	}
	return r.RemoteAddr
}

func isBot(ua string) bool {
	ua = strings.ToLower(ua)
	for _, s := range []string{"bot", "crawler", "spider", "headless", "lighthouse"} {
		if strings.Contains(ua, s) {
			return true
		}
	}
	return false
}

// GetEngagementJSON returns engagement summary for admin dashboard.
// GET /api/admin/engagement-stats?path=/blog/xxx&days=30
func (c EngagementController) GetSummary(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	daysStr := r.URL.Query().Get("days")
	days := 30
	if daysStr != "" {
		// simple parse
		for _, ch := range daysStr {
			if ch < '0' || ch > '9' {
				days = 30
				break
			}
		}
		if d := parseIntDefault(daysStr, 30); d > 0 && d <= 365 {
			days = d
		}
	}
	since := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	w.Header().Set("Content-Type", "application/json")

	if path != "" {
		summary, err := c.Service.SummaryForPath(ctx, path, since)
		if err != nil {
			http.Error(w, "failed to load", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(summary)
		return
	}
	// No path: return top engaged paths
	top, err := c.Service.TopEngagedPaths(ctx, since, 20)
	if err != nil {
		http.Error(w, "failed to load", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"top_engaged": top,
		"since":       since.Format(time.RFC3339),
	})
}

func parseIntDefault(s string, def int) int {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return def
		}
		n = n*10 + int(ch-'0')
	}
	return n
}
