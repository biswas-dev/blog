package controllers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"

	"anshumanbiswas.com/blog/models"
	"anshumanbiswas.com/blog/utils"
	"github.com/go-chi/chi/v5"
)

type SlideVersions struct {
	SlideVersionService *models.SlideVersionService
	SessionService      *models.SessionService
	SlideService        *models.SlideService
}

func (sv *SlideVersions) requireEditor(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	user, err := utils.IsUserLoggedIn(r, sv.SessionService)
	if err != nil || user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	if !models.CanEditSlides(user.Role) && !models.IsAdmin(user.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return nil, false
	}
	return user, true
}

// HandleListVersions GET /api/slides/{slideID}/versions
func (sv *SlideVersions) HandleListVersions(w http.ResponseWriter, r *http.Request) {
	if _, ok := sv.requireEditor(w, r); !ok {
		return
	}

	slideID, err := strconv.Atoi(chi.URLParam(r, "slideID"))
	if err != nil {
		http.Error(w, "Invalid slide ID", http.StatusBadRequest)
		return
	}

	versions, err := sv.SlideVersionService.GetVersions(slideID)
	if err != nil {
		http.Error(w, "Failed to fetch versions", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(versions); err != nil {
		log.Printf("list slide versions encode: %v", err)
	}
}

// HandleGetVersion GET /api/slides/{slideID}/versions/{versionNum}
func (sv *SlideVersions) HandleGetVersion(w http.ResponseWriter, r *http.Request) {
	if _, ok := sv.requireEditor(w, r); !ok {
		return
	}

	slideID, err := strconv.Atoi(chi.URLParam(r, "slideID"))
	if err != nil {
		http.Error(w, "Invalid slide ID", http.StatusBadRequest)
		return
	}
	versionNum, err := strconv.Atoi(chi.URLParam(r, "versionNum"))
	if err != nil {
		http.Error(w, "Invalid version number", http.StatusBadRequest)
		return
	}

	version, err := sv.SlideVersionService.GetVersion(slideID, versionNum)
	if err != nil {
		http.Error(w, "Version not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(version); err != nil {
		log.Printf("get slide version encode: %v", err)
	}
}

// HandleDeleteVersion DELETE /api/slides/{slideID}/versions/{versionNum}
func (sv *SlideVersions) HandleDeleteVersion(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, sv.SessionService)
	if err != nil || user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !models.IsAdmin(user.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	slideID, err := strconv.Atoi(chi.URLParam(r, "slideID"))
	if err != nil {
		http.Error(w, "Invalid slide ID", http.StatusBadRequest)
		return
	}
	versionNum, err := strconv.Atoi(chi.URLParam(r, "versionNum"))
	if err != nil {
		http.Error(w, "Invalid version number", http.StatusBadRequest)
		return
	}

	if err := sv.SlideVersionService.DeleteVersion(slideID, versionNum); err != nil {
		http.Error(w, "Version not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "deleted"}); err != nil {
		log.Printf("delete slide version encode: %v", err)
	}
}

// HandleRestoreVersion POST /api/slides/{slideID}/versions/{versionNum}/restore
func (sv *SlideVersions) HandleRestoreVersion(w http.ResponseWriter, r *http.Request) {
	user, ok := sv.requireEditor(w, r)
	if !ok {
		return
	}

	slideID, err := strconv.Atoi(chi.URLParam(r, "slideID"))
	if err != nil {
		http.Error(w, "Invalid slide ID", http.StatusBadRequest)
		return
	}
	versionNum, err := strconv.Atoi(chi.URLParam(r, "versionNum"))
	if err != nil {
		http.Error(w, "Invalid version number", http.StatusBadRequest)
		return
	}

	version, err := sv.SlideVersionService.GetVersion(slideID, versionNum)
	if err != nil {
		http.Error(w, "Version not found", http.StatusNotFound)
		return
	}

	// Load current slide to preserve other fields and snapshot pre-restore state
	slide, err := sv.SlideService.GetByID(slideID)
	if err != nil {
		http.Error(w, "Slide not found", http.StatusNotFound)
		return
	}

	// Snapshot current content BEFORE overwriting it
	_ = sv.SlideVersionService.MaybeCreateVersion(slideID, user.UserID, slide.Title, string(slide.ContentHTML))

	// Restore: overwrite content file with old version content
	if err := os.WriteFile(slide.ContentFilePath, []byte(version.Content), 0644); err != nil {
		http.Error(w, "Failed to restore version", http.StatusInternalServerError)
		return
	}

	// Update title in DB
	if err := sv.SlideService.Update(slideID, version.Title, slide.Slug, version.Content,
		slide.IsPublished, nil, slide.Description, slide.SlideMetadata, ""); err != nil {
		http.Error(w, "Failed to restore version", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "restored"}); err != nil {
		log.Printf("restore slide version encode: %v", err)
	}
}
