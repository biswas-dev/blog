package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"anshumanbiswas.com/blog/models"
	"anshumanbiswas.com/blog/utils"
	"github.com/go-chi/chi/v5"
)


const (
	headerContentType  = "Content-Type"
	mimeJSON           = "application/json"
	errInvalidSystemID = "Invalid system ID"
	errSystemNotFound  = "System not found: %v"
	errInvalidBody     = "Invalid request body"
)

type System struct {
	SystemService         *models.SystemService
	DatabaseBackupService *models.DatabaseBackupService
	SessionService        *models.SessionService
	ExternalSystemService *models.ExternalSystemService
	SyncClient            *models.SyncClient
	CloudinaryService     *models.CloudinaryService
	BrevoService          *models.BrevoService
	Templates             struct {
		Dashboard Template
	}
	// Rate limiting for database exports
	exportLimiter sync.Map // map[userID]time.Time
}

// requireAdmin checks auth and admin role, returning the user or writing an error
func (s *System) requireAdmin(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	user, err := utils.IsUserLoggedIn(r, s.SessionService)
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

// Dashboard renders the system information page (admin-only)
func (s *System) Dashboard(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, s.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	// Check if user is admin
	if !models.IsAdmin(user.Role) {
		http.Error(w, errForbiddenAdmin, http.StatusForbidden)
		return
	}

	// Get system information
	systemInfo, err := s.SystemService.GetSystemInfo()
	if err != nil {
		log.Printf("Error getting system info: %v", err)
		http.Error(w, "Failed to load system information", http.StatusInternalServerError)
		return
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
		SystemInfo      *models.SystemInfo
		UserPermissions models.UserPermissions
	}{
		Email:           user.Email,
		LoggedIn:        true,
		Username:        user.Username,
		IsAdmin:         true,
		SignupDisabled:  true,
		Description:     "System Information - Anshuman Biswas Blog",
		CurrentPage:     "admin-system",
		User:            user,
		SystemInfo:      systemInfo,
		UserPermissions: models.GetPermissions(user.Role),
	}

	s.Templates.Dashboard.Execute(w, r, data)
}

// GetSystemInfoJSON returns system information as JSON (admin-only)
func (s *System) GetSystemInfoJSON(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, s.SessionService)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check if user is admin
	if !models.IsAdmin(user.Role) {
		http.Error(w, errForbiddenAdmin, http.StatusForbidden)
		return
	}

	// Get system information
	systemInfo, err := s.SystemService.GetSystemInfo()
	if err != nil {
		log.Printf("Error getting system info: %v", err)
		http.Error(w, "Failed to get system information", http.StatusInternalServerError)
		return
	}

	w.Header().Set(headerContentType, mimeJSON)
	json.NewEncoder(w).Encode(systemInfo)
}

// ExportDatabase streams a SQL dump of the database (admin-only, rate-limited)
func (s *System) ExportDatabase(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, s.SessionService)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check if user is admin
	if !models.IsAdmin(user.Role) {
		http.Error(w, errForbiddenAdmin, http.StatusForbidden)
		return
	}

	// Rate limiting: 1 export per user per 5 minutes
	userKey := fmt.Sprintf("user_%d", user.UserID)
	if lastExport, ok := s.exportLimiter.Load(userKey); ok {
		if time.Since(lastExport.(time.Time)) < 5*time.Minute {
			http.Error(w, "Rate limit exceeded. Please wait 5 minutes between exports.", http.StatusTooManyRequests)
			return
		}
	}

	// Update rate limit
	s.exportLimiter.Store(userKey, time.Now())

	// Set headers for file download
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("blog-backup-%s.sql", timestamp)
	w.Header().Set("Content-Type", "application/sql")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	// Stream the database dump
	log.Printf("User %s (%s) initiated database export", user.Username, user.Email)
	err = s.DatabaseBackupService.Export(w)
	if err != nil {
		log.Printf("Error exporting database: %v", err)
		// Can't write HTTP error here as we've already started streaming
		// The error will be visible in server logs
		return
	}

	log.Printf("Database export completed successfully for user %s", user.Username)
}

// ImportDatabase imports a SQL dump into the database (admin-only)
func (s *System) ImportDatabase(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, s.SessionService)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check if user is admin
	if !models.IsAdmin(user.Role) {
		http.Error(w, errForbiddenAdmin, http.StatusForbidden)
		return
	}

	// Parse multipart form (32MB max)
	err = r.ParseMultipartForm(32 << 20)
	if err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Get the uploaded file
	file, header, err := r.FormFile("backup_file")
	if err != nil {
		http.Error(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	log.Printf("User %s (%s) initiated database import: %s (%.2f MB)",
		user.Username, user.Email, header.Filename, float64(header.Size)/(1024*1024))

	// Validate the file (read into buffer for validation)
	var buf bytes.Buffer
	_, err = buf.ReadFrom(file)
	if err != nil {
		log.Printf("Error reading upload file: %v", err)
		http.Error(w, "Failed to read uploaded file", http.StatusInternalServerError)
		return
	}

	// Validate SQL file
	validationReader := bytes.NewReader(buf.Bytes())
	err = models.ValidateImportFile(validationReader)
	if err != nil {
		log.Printf("File validation failed: %v", err)
		http.Error(w, fmt.Sprintf("Invalid SQL file: %v", err), http.StatusBadRequest)
		return
	}

	// Import the database
	importReader := bytes.NewReader(buf.Bytes())
	err = s.DatabaseBackupService.Import(importReader)
	if err != nil {
		log.Printf("Error importing database: %v", err)
		http.Error(w, fmt.Sprintf("Database import failed: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Database import completed successfully for user %s", user.Username)

	// Return success response
	w.Header().Set(headerContentType, mimeJSON)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Database imported successfully",
	})
}

// --- External Systems Handlers ---

// ListExternalSystems returns all registered external systems
func (s *System) ListExternalSystems(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	systems, err := s.ExternalSystemService.GetAll()
	if err != nil {
		log.Printf("Error listing external systems: %v", err)
		http.Error(w, "Failed to list external systems", http.StatusInternalServerError)
		return
	}

	w.Header().Set(headerContentType, mimeJSON)
	json.NewEncoder(w).Encode(systems)
}

// GetExternalSystem returns a single external system with headers (API key masked)
func (s *System) GetExternalSystem(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, errInvalidSystemID, http.StatusBadRequest)
		return
	}

	system, err := s.ExternalSystemService.GetByID(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(errSystemNotFound, err), http.StatusNotFound)
		return
	}

	// Mask the API key - only indicate if one is set
	if system.APIKey != "" {
		system.APIKey = ""
	}

	w.Header().Set(headerContentType, mimeJSON)
	json.NewEncoder(w).Encode(system)
}

// CreateExternalSystem registers a new external blog instance
func (s *System) CreateExternalSystem(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}

	var input struct {
		Name          string                `json:"name"`
		BaseURL       string                `json:"base_url"`
		APIKey        string                `json:"api_key"`
		CustomHeaders []models.CustomHeader `json:"custom_headers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, errInvalidBody, http.StatusBadRequest)
		return
	}

	if input.Name == "" || input.BaseURL == "" {
		http.Error(w, "Name and base_url are required", http.StatusBadRequest)
		return
	}

	// Sanitize header keys - strip trailing colons and whitespace
	for i := range input.CustomHeaders {
		input.CustomHeaders[i].Key = strings.TrimRight(strings.TrimSpace(input.CustomHeaders[i].Key), ":")
		input.CustomHeaders[i].Value = strings.TrimSpace(input.CustomHeaders[i].Value)
	}

	system, err := s.ExternalSystemService.Create(input.Name, input.BaseURL, input.APIKey, input.CustomHeaders, user.UserID)
	if err != nil {
		log.Printf("Error creating external system: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create external system: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set(headerContentType, mimeJSON)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(system)
}

// UpdateExternalSystem updates an existing external system
func (s *System) UpdateExternalSystem(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, errInvalidSystemID, http.StatusBadRequest)
		return
	}

	var input struct {
		Name          string                `json:"name"`
		BaseURL       string                `json:"base_url"`
		APIKey        string                `json:"api_key"`
		CustomHeaders []models.CustomHeader `json:"custom_headers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, errInvalidBody, http.StatusBadRequest)
		return
	}

	if input.Name == "" || input.BaseURL == "" {
		http.Error(w, "Name and base_url are required", http.StatusBadRequest)
		return
	}

	// Sanitize header keys - strip trailing colons and whitespace
	for i := range input.CustomHeaders {
		input.CustomHeaders[i].Key = strings.TrimRight(strings.TrimSpace(input.CustomHeaders[i].Key), ":")
		input.CustomHeaders[i].Value = strings.TrimSpace(input.CustomHeaders[i].Value)
	}

	system, err := s.ExternalSystemService.Update(id, input.Name, input.BaseURL, input.APIKey, input.CustomHeaders)
	if err != nil {
		log.Printf("Error updating external system: %v", err)
		http.Error(w, fmt.Sprintf("Failed to update: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set(headerContentType, mimeJSON)
	json.NewEncoder(w).Encode(system)
}

// DeleteExternalSystem removes an external system
func (s *System) DeleteExternalSystem(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, errInvalidSystemID, http.StatusBadRequest)
		return
	}

	if err := s.ExternalSystemService.Delete(id); err != nil {
		log.Printf("Error deleting external system: %v", err)
		http.Error(w, fmt.Sprintf("Failed to delete: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// TestExternalConnection tests connectivity to a remote blog instance
func (s *System) TestExternalConnection(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, errInvalidSystemID, http.StatusBadRequest)
		return
	}

	system, err := s.ExternalSystemService.GetByID(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(errSystemNotFound, err), http.StatusNotFound)
		return
	}

	if err := s.SyncClient.TestConnection(system); err != nil {
		w.Header().Set(headerContentType, mimeJSON)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	w.Header().Set(headerContentType, mimeJSON)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Connection successful",
	})
}

// PreviewSync previews what a sync operation would do
func (s *System) PreviewSync(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, errInvalidSystemID, http.StatusBadRequest)
		return
	}

	var input struct {
		Direction string `json:"direction"` // "pull" or "push"
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, errInvalidBody, http.StatusBadRequest)
		return
	}

	system, err := s.ExternalSystemService.GetByID(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(errSystemNotFound, err), http.StatusNotFound)
		return
	}

	var preview *models.SyncPreview
	switch input.Direction {
	case "pull":
		preview, err = s.SyncClient.PreviewPull(system)
	case "push":
		preview, err = s.SyncClient.PreviewPush(system)
	default:
		http.Error(w, "Direction must be 'pull' or 'push'", http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Printf("Error previewing sync: %v", err)
		http.Error(w, fmt.Sprintf("Preview failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set(headerContentType, mimeJSON)
	json.NewEncoder(w).Encode(preview)
}

// ExecuteSync runs a sync operation
func (s *System) ExecuteSync(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, errInvalidSystemID, http.StatusBadRequest)
		return
	}

	var input struct {
		Direction string `json:"direction"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, errInvalidBody, http.StatusBadRequest)
		return
	}

	system, err := s.ExternalSystemService.GetByID(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(errSystemNotFound, err), http.StatusNotFound)
		return
	}

	// Create sync log
	logID, err := s.ExternalSystemService.CreateSyncLog(id, input.Direction, "posts", user.UserID)
	if err != nil {
		log.Printf("Error creating sync log: %v", err)
	}

	var result *models.SyncResult
	switch input.Direction {
	case "pull":
		result, err = s.SyncClient.ExecutePull(system, user.UserID)
	case "push":
		result, err = s.SyncClient.ExecutePush(system)
	default:
		http.Error(w, "Direction must be 'pull' or 'push'", http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Printf("Error executing sync: %v", err)
		if logID > 0 {
			s.ExternalSystemService.UpdateSyncLog(logID, "failed", 0, 0, 0, err.Error())
		}
		s.ExternalSystemService.UpdateSyncStatus(id, "failed", err.Error())
		http.Error(w, fmt.Sprintf("Sync failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Update sync log and status
	status := "success"
	if result.ItemsFailed > 0 {
		status = "partial"
	}
	statusMsg := fmt.Sprintf("%s: %d synced, %d skipped, %d failed", input.Direction, result.ItemsSynced, result.ItemsSkipped, result.ItemsFailed)

	if logID > 0 {
		s.ExternalSystemService.UpdateSyncLog(logID, status, result.ItemsSynced, result.ItemsSkipped, result.ItemsFailed, result.ErrorMessage)
	}
	s.ExternalSystemService.UpdateSyncStatus(id, status, statusMsg)

	w.Header().Set(headerContentType, mimeJSON)
	json.NewEncoder(w).Encode(result)
}

// GetSyncLogs returns sync history for a system
func (s *System) GetSyncLogs(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, errInvalidSystemID, http.StatusBadRequest)
		return
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	logs, err := s.ExternalSystemService.GetSyncLogs(id, limit)
	if err != nil {
		log.Printf("Error getting sync logs: %v", err)
		http.Error(w, "Failed to get sync logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set(headerContentType, mimeJSON)
	json.NewEncoder(w).Encode(logs)
}

// --- Cloudinary Settings Handlers ---

// GetCloudinarySettings returns current Cloudinary settings (secret omitted)
func (s *System) GetCloudinarySettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	settings, err := s.CloudinaryService.Get()
	if err != nil {
		log.Printf("Error getting cloudinary settings: %v", err)
		http.Error(w, "Failed to get cloudinary settings", http.StatusInternalServerError)
		return
	}

	if settings == nil {
		w.Header().Set(headerContentType, mimeJSON)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"configured": false,
		})
		return
	}

	// Never expose the API secret
	settings.APISecret = ""

	w.Header().Set(headerContentType, mimeJSON)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"configured": true,
		"settings":   settings,
	})
}

// SaveCloudinarySettings saves or updates Cloudinary credentials
func (s *System) SaveCloudinarySettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	var input struct {
		CloudName string `json:"cloud_name"`
		APIKey    string `json:"api_key"`
		APISecret string `json:"api_secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, errInvalidBody, http.StatusBadRequest)
		return
	}

	if input.CloudName == "" || input.APIKey == "" {
		http.Error(w, "cloud_name and api_key are required", http.StatusBadRequest)
		return
	}

	if err := s.CloudinaryService.Save(input.CloudName, input.APIKey, input.APISecret); err != nil {
		log.Printf("Error saving cloudinary settings: %v", err)
		http.Error(w, fmt.Sprintf("Failed to save: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set(headerContentType, mimeJSON)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Cloudinary settings saved",
	})
}

// DeleteCloudinarySettings removes Cloudinary credentials
func (s *System) DeleteCloudinarySettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	if err := s.CloudinaryService.Delete(); err != nil {
		log.Printf("Error deleting cloudinary settings: %v", err)
		http.Error(w, "Failed to delete cloudinary settings", http.StatusInternalServerError)
		return
	}

	w.Header().Set(headerContentType, mimeJSON)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Cloudinary settings removed",
	})
}

// TestCloudinaryConnection tests connectivity to Cloudinary using Basic auth against the ping endpoint
func (s *System) TestCloudinaryConnection(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	settings, err := s.CloudinaryService.Get()
	if err != nil || settings == nil {
		w.Header().Set(headerContentType, mimeJSON)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Cloudinary not configured",
		})
		return
	}

	// Ping Cloudinary API: GET https://api.cloudinary.com/v1_1/{cloud_name}/ping with Basic auth
	pingURL := fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/ping", settings.CloudName)
	req, err := http.NewRequest("GET", pingURL, nil)
	if err != nil {
		w.Header().Set(headerContentType, mimeJSON)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Failed to create request: %v", err),
		})
		return
	}
	req.SetBasicAuth(settings.APIKey, settings.APISecret)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		s.CloudinaryService.UpdateHealthStatus("error", settings.ConsecutiveFailures+1)
		w.Header().Set(headerContentType, mimeJSON)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Connection failed: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	if resp.StatusCode == http.StatusOK {
		s.CloudinaryService.UpdateHealthStatus("healthy", 0)
		w.Header().Set(headerContentType, mimeJSON)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Connection successful",
		})
	} else {
		s.CloudinaryService.UpdateHealthStatus("error", settings.ConsecutiveFailures+1)
		w.Header().Set(headerContentType, mimeJSON)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Cloudinary returned status %d: %s", resp.StatusCode, string(body)),
		})
	}
}

// GetCloudinarySignature generates a signed upload signature for client-side uploads
func (s *System) GetCloudinarySignature(w http.ResponseWriter, r *http.Request) {
	// Allow editors too (not just admins) since they upload images
	user, err := utils.IsUserLoggedIn(r, s.SessionService)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !models.CanEditPosts(user.Role) && !models.IsAdmin(user.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	settings, err := s.CloudinaryService.Get()
	if err != nil || settings == nil {
		http.Error(w, "Cloudinary not configured", http.StatusBadRequest)
		return
	}

	var input struct {
		Folder    string `json:"folder"`
		Timestamp string `json:"timestamp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, errInvalidBody, http.StatusBadRequest)
		return
	}

	if input.Timestamp == "" {
		input.Timestamp = fmt.Sprintf("%d", time.Now().Unix())
	}

	// Build params to sign
	params := map[string]string{
		"timestamp": input.Timestamp,
	}
	if input.Folder != "" {
		params["folder"] = input.Folder
	}

	signature := models.GenerateSignature(params, settings.APISecret)

	w.Header().Set(headerContentType, mimeJSON)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"signature":  signature,
		"timestamp":  input.Timestamp,
		"api_key":    settings.APIKey,
		"cloud_name": settings.CloudName,
	})
}

// --- Brevo Email Settings Handlers ---

// GetBrevoSettings returns current Brevo settings (API key omitted)
func (s *System) GetBrevoSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	settings, err := s.BrevoService.Get()
	if err != nil {
		log.Printf("Error getting brevo settings: %v", err)
		http.Error(w, "Failed to get brevo settings", http.StatusInternalServerError)
		return
	}

	if settings == nil {
		w.Header().Set(headerContentType, mimeJSON)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"configured": false,
		})
		return
	}

	// Never expose the API key
	settings.APIKey = ""

	w.Header().Set(headerContentType, mimeJSON)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"configured": true,
		"settings":   settings,
	})
}

// SaveBrevoSettings saves or updates Brevo credentials
func (s *System) SaveBrevoSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	var input struct {
		APIKey    string `json:"api_key"`
		FromEmail string `json:"from_email"`
		FromName  string `json:"from_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, errInvalidBody, http.StatusBadRequest)
		return
	}

	if input.FromEmail == "" {
		http.Error(w, "from_email is required", http.StatusBadRequest)
		return
	}
	if input.FromName == "" {
		input.FromName = "Blog"
	}

	if err := s.BrevoService.Save(input.APIKey, input.FromEmail, input.FromName); err != nil {
		log.Printf("Error saving brevo settings: %v", err)
		http.Error(w, fmt.Sprintf("Failed to save: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set(headerContentType, mimeJSON)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Brevo settings saved",
	})
}

// DeleteBrevoSettings removes Brevo credentials
func (s *System) DeleteBrevoSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	if err := s.BrevoService.Delete(); err != nil {
		log.Printf("Error deleting brevo settings: %v", err)
		http.Error(w, "Failed to delete brevo settings", http.StatusInternalServerError)
		return
	}

	w.Header().Set(headerContentType, mimeJSON)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Brevo settings removed",
	})
}

// TestBrevoConnection tests the Brevo API key and optionally sends a test email
func (s *System) TestBrevoConnection(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	var input struct {
		SendTestTo string `json:"send_test_to"`
	}
	// Body is optional — if empty, just test the API key
	json.NewDecoder(r.Body).Decode(&input)

	settings, err := s.BrevoService.Get()
	if err != nil || settings == nil {
		w.Header().Set(headerContentType, mimeJSON)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Brevo not configured",
		})
		return
	}

	// First test API key validity
	ok, msg := s.BrevoService.TestConnection(settings.APIKey)
	if !ok {
		s.BrevoService.UpdateHealthStatus("error", settings.ConsecutiveFailures+1)
		w.Header().Set(headerContentType, mimeJSON)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": msg,
		})
		return
	}

	s.BrevoService.UpdateHealthStatus("healthy", 0)

	// If a test email address was provided, send a test email
	if input.SendTestTo != "" {
		emailOk, emailMsg := s.BrevoService.SendTestEmail(
			settings.APIKey, settings.FromEmail, settings.FromName, input.SendTestTo,
		)
		if !emailOk {
			w.Header().Set(headerContentType, mimeJSON)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": fmt.Sprintf("API key valid (%s), but email send failed: %s", msg, emailMsg),
			})
			return
		}
		msg = emailMsg
	}

	w.Header().Set(headerContentType, mimeJSON)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": msg,
	})
}
