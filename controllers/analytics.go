package controllers

import (
	"encoding/json"
	"log"
	"net/http"

	"anshumanbiswas.com/blog/models"
	"anshumanbiswas.com/blog/utils"
)

// Analytics handles the analytics admin dashboard
type Analytics struct {
	AnalyticsService *models.AnalyticsService
	SessionService   *models.SessionService
	Templates        struct {
		Dashboard Template
	}
}

// Dashboard renders the analytics dashboard page (admin-only)
func (a *Analytics) Dashboard(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, a.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	if !models.IsAdmin(user.Role) {
		http.Error(w, "Forbidden: Admin access required", http.StatusForbidden)
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
	user, err := utils.IsUserLoggedIn(r, a.SessionService)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !models.IsAdmin(user.Role) {
		http.Error(w, "Forbidden: Admin access required", http.StatusForbidden)
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
