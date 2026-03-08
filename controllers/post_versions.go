package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"anshumanbiswas.com/blog/models"
	"anshumanbiswas.com/blog/utils"
	"github.com/go-chi/chi/v5"
)

type PostVersions struct {
	PostVersionService *models.PostVersionService
	SessionService     *models.SessionService
	PostService        *models.PostService
}

func (pv *PostVersions) requireEditor(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	user, err := utils.IsUserLoggedIn(r, pv.SessionService)
	if err != nil || user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	if !models.CanEditPosts(user.Role) && !models.IsAdmin(user.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return nil, false
	}
	return user, true
}

// HandleListVersions GET /api/posts/{postID}/versions
func (pv *PostVersions) HandleListVersions(w http.ResponseWriter, r *http.Request) {
	if _, ok := pv.requireEditor(w, r); !ok {
		return
	}

	postID, err := strconv.Atoi(chi.URLParam(r, "postID"))
	if err != nil {
		http.Error(w, "Invalid post ID", http.StatusBadRequest)
		return
	}

	versions, err := pv.PostVersionService.GetVersions(postID)
	if err != nil {
		http.Error(w, "Failed to fetch versions", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(versions)
}

// HandleGetVersion GET /api/posts/{postID}/versions/{versionNum}
func (pv *PostVersions) HandleGetVersion(w http.ResponseWriter, r *http.Request) {
	if _, ok := pv.requireEditor(w, r); !ok {
		return
	}

	postID, err := strconv.Atoi(chi.URLParam(r, "postID"))
	if err != nil {
		http.Error(w, "Invalid post ID", http.StatusBadRequest)
		return
	}
	versionNum, err := strconv.Atoi(chi.URLParam(r, "versionNum"))
	if err != nil {
		http.Error(w, "Invalid version number", http.StatusBadRequest)
		return
	}

	version, err := pv.PostVersionService.GetVersion(postID, versionNum)
	if err != nil {
		http.Error(w, "Version not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(version)
}

// HandleRestoreVersion POST /api/posts/{postID}/versions/{versionNum}/restore
func (pv *PostVersions) HandleRestoreVersion(w http.ResponseWriter, r *http.Request) {
	user, ok := pv.requireEditor(w, r)
	if !ok {
		return
	}

	postID, err := strconv.Atoi(chi.URLParam(r, "postID"))
	if err != nil {
		http.Error(w, "Invalid post ID", http.StatusBadRequest)
		return
	}
	versionNum, err := strconv.Atoi(chi.URLParam(r, "versionNum"))
	if err != nil {
		http.Error(w, "Invalid version number", http.StatusBadRequest)
		return
	}

	version, err := pv.PostVersionService.GetVersion(postID, versionNum)
	if err != nil {
		http.Error(w, "Version not found", http.StatusNotFound)
		return
	}

	// Load current post to preserve other fields
	post, err := pv.PostService.GetByID(postID)
	if err != nil {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	// Update with restored content (preserve slug, published state, featured, categories)
	if err := pv.PostService.Update(postID, post.CategoryID, version.Title, version.Content,
		post.IsPublished, post.Featured, post.FeaturedImageURL, post.Slug); err != nil {
		http.Error(w, "Failed to restore version", http.StatusInternalServerError)
		return
	}

	// Save restore as a new version
	_ = pv.PostVersionService.MaybeCreateVersion(postID, user.UserID, version.Title, version.Content)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "restored"})
}
