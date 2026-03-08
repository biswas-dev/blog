package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"anshumanbiswas.com/blog/models"
	"anshumanbiswas.com/blog/utils"
	"github.com/go-chi/chi/v5"
)

// AdminUsers handles the admin user management UI and API.
type AdminUsers struct {
	UserActivityService *models.UserActivityService
	SessionService      *models.SessionService
	Templates           struct {
		Dashboard Template
	}
}

type adminUsersDashboardData struct {
	Email          string
	LoggedIn       bool
	Username       string
	IsAdmin        bool
	SignupDisabled  bool
	Description    string
	CurrentPage    string
	User           *models.User
	Users          []models.UserWithStats
	UserPermissions models.UserPermissions
}

func (a *AdminUsers) requireAdmin(r *http.Request) (*models.User, bool) {
	user, err := utils.IsUserLoggedIn(r, a.SessionService)
	if err != nil || !models.IsAdmin(user.Role) {
		return nil, false
	}
	return user, true
}

// Dashboard renders the admin users page.
func (a *AdminUsers) Dashboard(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireAdmin(r)
	if !ok {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	users, err := a.UserActivityService.GetUsersWithStats()
	if err != nil {
		http.Error(w, "Failed to load users", http.StatusInternalServerError)
		return
	}

	data := adminUsersDashboardData{
		Email:           user.Email,
		LoggedIn:        true,
		Username:        user.Username,
		IsAdmin:         true,
		Description:     "User Management",
		CurrentPage:     "admin",
		User:            user,
		Users:           users,
		UserPermissions: models.GetPermissions(user.Role),
	}
	a.Templates.Dashboard.Execute(w, r, data)
}

// GetUsersJSON returns all users with stats as JSON.
func (a *AdminUsers) GetUsersJSON(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdmin(r); !ok {
		http.Error(w, errForbiddenAdmin, http.StatusForbidden)
		return
	}
	users, err := a.UserActivityService.GetUsersWithStats()
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users) //nolint:errcheck
}

// GetUserActivityJSON returns activity log for a specific user.
func (a *AdminUsers) GetUserActivityJSON(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdmin(r); !ok {
		http.Error(w, errForbiddenAdmin, http.StatusForbidden)
		return
	}
	userID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}
	activities, err := a.UserActivityService.GetUserActivity(userID, 100)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(activities) //nolint:errcheck
}

// UpdateUserRole changes the role of a user (cannot change own role).
func (a *AdminUsers) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireAdmin(r)
	if !ok {
		http.Error(w, errForbiddenAdmin, http.StatusForbidden)
		return
	}
	userID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}
	if userID == admin.UserID {
		http.Error(w, "Cannot change your own role", http.StatusBadRequest)
		return
	}
	var body struct {
		Role int `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if body.Role < 1 || body.Role > 4 {
		http.Error(w, "Invalid role (must be 1-4)", http.StatusBadRequest)
		return
	}
	if err := a.UserActivityService.UpdateUserRole(userID, body.Role); err != nil {
		http.Error(w, "Failed to update role", http.StatusInternalServerError)
		return
	}
	a.UserActivityService.Log(userID, "role_change", utils.GetClientIP(r), "admin:"+admin.Username)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"}) //nolint:errcheck
}

// AdminResetPassword sets a new password for any user.
func (a *AdminUsers) AdminResetPassword(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireAdmin(r)
	if !ok {
		http.Error(w, errForbiddenAdmin, http.StatusForbidden)
		return
	}
	userID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	body.Password = strings.TrimSpace(body.Password)
	if len(body.Password) < 8 {
		http.Error(w, "Password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	if err := a.UserActivityService.AdminResetPassword(userID, body.Password); err != nil {
		http.Error(w, "Failed to reset password", http.StatusInternalServerError)
		return
	}
	a.UserActivityService.Log(userID, "password_change", utils.GetClientIP(r), "admin reset by:"+admin.Username)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"}) //nolint:errcheck
}
