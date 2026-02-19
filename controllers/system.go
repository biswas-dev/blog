package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"anshumanbiswas.com/blog/models"
	"anshumanbiswas.com/blog/utils"
)

type System struct {
	SystemService         *models.SystemService
	DatabaseBackupService *models.DatabaseBackupService
	SessionService        *models.SessionService
	Templates             struct {
		Dashboard Template
	}
	// Rate limiting for database exports
	exportLimiter sync.Map // map[userID]time.Time
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
		http.Error(w, "Forbidden: Admin access required", http.StatusForbidden)
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
		http.Error(w, "Forbidden: Admin access required", http.StatusForbidden)
		return
	}

	// Get system information
	systemInfo, err := s.SystemService.GetSystemInfo()
	if err != nil {
		log.Printf("Error getting system info: %v", err)
		http.Error(w, "Failed to get system information", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
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
		http.Error(w, "Forbidden: Admin access required", http.StatusForbidden)
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
		http.Error(w, "Forbidden: Admin access required", http.StatusForbidden)
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
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Database imported successfully",
	})
}
