package controllers

import (
	"encoding/json"
	"net/http"

	"anshumanbiswas.com/blog/models"
	"anshumanbiswas.com/blog/utils"
)

// Security handles the IP ban/allow management admin UI and API.
type Security struct {
	IPRulesService *models.IPRulesService
	SessionService *models.SessionService
	Templates      struct {
		Dashboard Template
	}
}

// securityDashboardData is the template data for admin-security.gohtml.
type securityDashboardData struct {
	Email          string
	LoggedIn       bool
	Username       string
	IsAdmin        bool
	SignupDisabled bool
	Description    string
	CurrentPage    string
	User           *models.User
	BannedRules    []models.IPRule
	AllowedRules   []models.IPRule
}

// Dashboard renders the security admin page (admin-only).
func (s *Security) Dashboard(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, s.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}
	if !models.IsAdmin(user.Role) {
		http.Error(w, errForbiddenAdmin, http.StatusForbidden)
		return
	}

	rules, err := s.IPRulesService.ListRules()
	if err != nil {
		http.Error(w, "Failed to load rules", http.StatusInternalServerError)
		return
	}

	var banned, allowed []models.IPRule
	for _, r := range rules {
		switch r.Action {
		case "ban":
			banned = append(banned, r)
		case "allow":
			allowed = append(allowed, r)
		}
	}

	data := securityDashboardData{
		Email:         user.Email,
		LoggedIn:      true,
		Username:      user.Username,
		IsAdmin:       true,
		SignupDisabled: true,
		Description:   "Security - Anshuman Biswas Blog",
		CurrentPage:   "admin-security",
		User:          user,
		BannedRules:   banned,
		AllowedRules:  allowed,
	}

	s.Templates.Dashboard.Execute(w, r, data)
}

// ListRulesJSON returns all IP rules as JSON (admin-only).
func (s *Security) ListRulesJSON(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, s.SessionService)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !models.IsAdmin(user.Role) {
		http.Error(w, errForbiddenAdmin, http.StatusForbidden)
		return
	}

	rules, err := s.IPRulesService.ListRules()
	if err != nil {
		http.Error(w, "Failed to load rules", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rules)
}

type ipActionRequest struct {
	IP     string `json:"ip"`
	Reason string `json:"reason"`
}

// BanIP bans an IP address (admin-only).
func (s *Security) BanIP(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, s.SessionService)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !models.IsAdmin(user.Role) {
		http.Error(w, errForbiddenAdmin, http.StatusForbidden)
		return
	}

	var req ipActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.IP == "" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Reason == "" {
		req.Reason = "manually banned via admin UI"
	}

	if err := s.IPRulesService.BanIP(req.IP, req.Reason); err != nil {
		http.Error(w, "Failed to ban IP", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "banned", "ip": req.IP})
}

// AllowIP adds an IP to the allow list (admin-only).
func (s *Security) AllowIP(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, s.SessionService)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !models.IsAdmin(user.Role) {
		http.Error(w, errForbiddenAdmin, http.StatusForbidden)
		return
	}

	var req ipActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.IP == "" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Reason == "" {
		req.Reason = "manually allowed via admin UI"
	}

	if err := s.IPRulesService.AllowIP(req.IP, req.Reason); err != nil {
		http.Error(w, "Failed to allow IP", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "allowed", "ip": req.IP})
}

type ipRemoveRequest struct {
	IP string `json:"ip"`
}

// RemoveRule removes an IP rule (admin-only). Returns 204 No Content on success.
func (s *Security) RemoveRule(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, s.SessionService)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !models.IsAdmin(user.Role) {
		http.Error(w, errForbiddenAdmin, http.StatusForbidden)
		return
	}

	var req ipRemoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.IP == "" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.IPRulesService.RemoveRule(req.IP); err != nil {
		http.Error(w, "Failed to remove rule", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
