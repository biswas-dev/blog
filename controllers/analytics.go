package controllers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"anshumanbiswas.com/blog/models"
	"anshumanbiswas.com/blog/utils"
	"github.com/go-chi/chi/v5"
)

const errForbiddenAdmin = "Forbidden: Admin access required"

// Analytics handles the analytics admin dashboard
type Analytics struct {
	DB               *sql.DB
	AnalyticsService *models.AnalyticsService
	SessionService   *models.SessionService
	Templates        struct {
		Dashboard Template
	}
}

// requireAdmin checks session and admin role; returns false and writes error if not admin.
func (a *Analytics) requireAdmin(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	user, err := utils.IsUserLoggedIn(r, a.SessionService)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	if !models.IsAdmin(user.Role) {
		http.Error(w, errForbiddenAdmin, http.StatusForbidden)
		return nil, false
	}
	return user, true
}

// Dashboard renders the analytics dashboard page (admin-only)
func (a *Analytics) Dashboard(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, a.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}
	if !models.IsAdmin(user.Role) {
		http.Error(w, errForbiddenAdmin, http.StatusForbidden)
		return
	}

	// Get today's live stats
	liveStats, err := a.AnalyticsService.GetTodayLiveCount()
	if err != nil {
		log.Printf("Error getting live stats: %v", err)
		liveStats = &models.LiveStats{}
	}

	data := struct {
		Email           string
		LoggedIn        bool
		Username        string
		IsAdmin         bool
		SignupDisabled  bool
		Description     string
		CurrentPage     string
		User            *models.User
		UserPermissions models.UserPermissions
		LiveStats       *models.LiveStats
	}{
		Email:           user.Email,
		LoggedIn:        true,
		Username:        user.Username,
		IsAdmin:         true,
		SignupDisabled:  true,
		Description:     "Analytics - Anshuman Biswas Blog",
		CurrentPage:     "admin-analytics",
		User:            user,
		UserPermissions: models.GetPermissions(user.Role),
		LiveStats:       liveStats,
	}

	a.Templates.Dashboard.Execute(w, r, data)
}

// GetAnalyticsJSON returns analytics data as JSON (admin-only)
func (a *Analytics) GetAnalyticsJSON(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdmin(w, r); !ok {
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}

	summary, err := a.AnalyticsService.GetSummary(period)
	if err != nil {
		log.Printf("Error getting analytics summary: %v", err)
		http.Error(w, "Failed to get analytics data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

// GetVisitorDetail returns detailed page views for a specific IP address (admin-only)
func (a *Analytics) GetVisitorDetail(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdmin(w, r); !ok {
		return
	}

	ip := r.URL.Query().Get("ip")
	if ip == "" {
		http.Error(w, "Missing ip parameter", http.StatusBadRequest)
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}

	details, err := a.AnalyticsService.GetVisitorActivity(ip, period)
	if err != nil {
		log.Printf("Error getting visitor detail for %s: %v", ip, err)
		http.Error(w, "Failed to get visitor details", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(details)
}

// ----- Engagement Management -----

type engagementComment struct {
	CommentID   int    `json:"comment_id"`
	PostTitle   string `json:"post_title"`
	PostSlug    string `json:"post_slug"`
	AuthorName  string `json:"author_name"`
	Content     string `json:"content"`
	CommentDate string `json:"comment_date"`
}

type engagementAnnotation struct {
	ID           int    `json:"id"`
	PostTitle    string `json:"post_title"`
	PostSlug     string `json:"post_slug"`
	AuthorName   string `json:"author_name"`
	SelectedText string `json:"selected_text"`
	Color        string `json:"color"`
	CommentCount int    `json:"comment_count"`
	CreatedAt    string `json:"created_at"`
}

type engagementResponse struct {
	Comments    []engagementComment    `json:"comments"`
	Annotations []engagementAnnotation `json:"annotations"`
}

// GetEngagementJSON returns all comments and annotations across posts (admin-only).
func (a *Analytics) GetEngagementJSON(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdmin(w, r); !ok {
		return
	}

	resp := engagementResponse{
		Comments:    []engagementComment{},
		Annotations: []engagementAnnotation{},
	}

	// Fetch comments
	cRows, err := a.DB.QueryContext(r.Context(), `
		SELECT c.comment_id,
		       p.title, p.slug,
		       COALESCE(NULLIF(u.full_name, ''), u.username),
		       c.content, c.comment_date
		FROM Comments c
		JOIN Users u ON u.user_id = c.user_id
		JOIN Posts p ON p.post_id = c.post_id
		ORDER BY c.comment_date DESC
		LIMIT 200`)
	if err != nil {
		log.Printf("GetEngagementJSON: comments query error: %v", err)
	} else {
		defer cRows.Close()
		for cRows.Next() {
			var ec engagementComment
			var t time.Time
			if err := cRows.Scan(&ec.CommentID, &ec.PostTitle, &ec.PostSlug, &ec.AuthorName, &ec.Content, &t); err == nil {
				ec.CommentDate = t.Format(time.RFC3339)
				resp.Comments = append(resp.Comments, ec)
			}
		}
	}

	// Fetch annotations
	aRows, err := a.DB.QueryContext(r.Context(), `
		SELECT a.id,
		       p.title, p.slug,
		       COALESCE(NULLIF(u.full_name, ''), u.username),
		       a.selected_text, a.color, a.created_at,
		       (SELECT COUNT(*) FROM post_annotation_comments WHERE annotation_id = a.id)
		FROM post_annotations a
		JOIN Users u ON u.user_id = a.author_id
		JOIN Posts p ON p.post_id = a.post_id
		ORDER BY a.created_at DESC
		LIMIT 200`)
	if err != nil {
		log.Printf("GetEngagementJSON: annotations query error: %v", err)
	} else {
		defer aRows.Close()
		for aRows.Next() {
			var ea engagementAnnotation
			var t time.Time
			if err := aRows.Scan(&ea.ID, &ea.PostTitle, &ea.PostSlug, &ea.AuthorName,
				&ea.SelectedText, &ea.Color, &t, &ea.CommentCount); err == nil {
				ea.CreatedAt = t.Format(time.RFC3339)
				resp.Annotations = append(resp.Annotations, ea)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("engagement encode: %v", err)
	}
}

// AdminDeleteComment deletes any comment by ID (admin-only).
// DELETE /api/admin/engagement/comments/{id}
func (a *Analytics) AdminDeleteComment(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdmin(w, r); !ok {
		return
	}
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	if _, err := a.DB.ExecContext(r.Context(), `DELETE FROM Comments WHERE comment_id = $1`, id); err != nil {
		http.Error(w, "Failed to delete comment", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AdminDeleteAnnotation deletes any annotation by ID (admin-only).
// DELETE /api/admin/engagement/annotations/{id}
func (a *Analytics) AdminDeleteAnnotation(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdmin(w, r); !ok {
		return
	}
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	if _, err := a.DB.ExecContext(r.Context(), `DELETE FROM post_annotations WHERE id = $1`, id); err != nil {
		http.Error(w, "Failed to delete annotation", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ----- 404 Slug Tracking -----

type slug404Row struct {
	ID            int    `json:"id"`
	Slug          string `json:"slug"`
	HitCount      int    `json:"hit_count"`
	FirstSeen     string `json:"first_seen"`
	LastSeen      string `json:"last_seen"`
	Whitelisted   bool   `json:"whitelisted"`
	WhitelistNote string `json:"whitelist_note"`
}

// GetSlug404sJSON returns all tracked unknown slugs (admin-only).
func (a *Analytics) GetSlug404sJSON(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdmin(w, r); !ok {
		return
	}

	rows, err := a.DB.QueryContext(r.Context(), `
		SELECT id, slug, hit_count, first_seen, last_seen, whitelisted, COALESCE(whitelist_note, '')
		FROM slug_404s
		ORDER BY whitelisted ASC, hit_count DESC, last_seen DESC
		LIMIT 300`)
	if err != nil {
		http.Error(w, "Failed to fetch 404 slugs", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	result := []slug404Row{}
	for rows.Next() {
		var s slug404Row
		var firstSeen, lastSeen time.Time
		if err := rows.Scan(&s.ID, &s.Slug, &s.HitCount, &firstSeen, &lastSeen, &s.Whitelisted, &s.WhitelistNote); err == nil {
			s.FirstSeen = firstSeen.Format(time.RFC3339)
			s.LastSeen = lastSeen.Format(time.RFC3339)
			result = append(result, s)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("slug-404s encode: %v", err)
	}
}

// WhitelistSlug404 marks a slug as whitelisted (admin-only).
// POST /api/admin/slug-404s/{id}/whitelist
func (a *Analytics) WhitelistSlug404(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdmin(w, r); !ok {
		return
	}
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	var body struct {
		Note string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if _, err := a.DB.ExecContext(r.Context(),
		`UPDATE slug_404s SET whitelisted = true, whitelist_note = $1 WHERE id = $2`,
		body.Note, id,
	); err != nil {
		http.Error(w, "Failed to whitelist slug", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteSlug404 removes a 404 slug record (admin-only).
// DELETE /api/admin/slug-404s/{id}
func (a *Analytics) DeleteSlug404(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdmin(w, r); !ok {
		return
	}
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	if _, err := a.DB.ExecContext(r.Context(), `DELETE FROM slug_404s WHERE id = $1`, id); err != nil {
		http.Error(w, "Failed to delete slug record", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
