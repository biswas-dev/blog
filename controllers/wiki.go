package controllers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"anshumanbiswas.com/blog/models"
	authmw "anshumanbiswas.com/blog/middleware"
	"github.com/go-chi/chi/v5"
)

type Wiki struct {
	WikiPageService *models.WikiPageService
}

func (wc *Wiki) parsePageID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(chi.URLParam(r, "pageID"))
	if err != nil {
		http.Error(w, "Invalid page ID", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func (wc *Wiki) jsonOK(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("wiki json encode: %v", err)
	}
}

// ListPages GET /api/wiki/pages
func (wc *Wiki) ListPages(w http.ResponseWriter, r *http.Request) {
	pages, err := wc.WikiPageService.GetAll()
	if err != nil {
		http.Error(w, "Failed to list pages", http.StatusInternalServerError)
		return
	}
	if pages == nil {
		pages = []models.WikiPage{}
	}
	wc.jsonOK(w, pages)
}

// CreatePage POST /api/wiki/pages
func (wc *Wiki) CreatePage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID  int    `json:"user_id"`
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}

	page, err := wc.WikiPageService.Create(req.UserID, req.Title, req.Content)
	if err != nil {
		log.Printf("wiki create: %v", err)
		http.Error(w, "Failed to create page", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(page)
}

// GetPage GET /api/wiki/pages/{pageID}
func (wc *Wiki) GetPage(w http.ResponseWriter, r *http.Request) {
	id, ok := wc.parsePageID(w, r)
	if !ok {
		return
	}
	page, err := wc.WikiPageService.GetByID(id)
	if err != nil {
		http.Error(w, "Page not found", http.StatusNotFound)
		return
	}
	wc.jsonOK(w, page)
}

// UpdatePage PUT /api/wiki/pages/{pageID}
func (wc *Wiki) UpdatePage(w http.ResponseWriter, r *http.Request) {
	id, ok := wc.parsePageID(w, r)
	if !ok {
		return
	}

	var req struct {
		Title      string `json:"title"`
		Content    string `json:"content"`
		UserID     int    `json:"user_id"`
		ManualSave bool   `json:"manual_save"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		return
	}

	if err := wc.WikiPageService.Update(id, req.Title, req.Content); err != nil {
		log.Printf("wiki update: %v", err)
		http.Error(w, "Failed to update page", http.StatusInternalServerError)
		return
	}

	// Version snapshot
	if req.Content != "" && req.UserID > 0 {
		_ = wc.WikiPageService.MaybeCreateVersion(id, req.UserID, req.Content, req.ManualSave)
	}

	page, err := wc.WikiPageService.GetByID(id)
	if err != nil {
		http.Error(w, "Failed to fetch updated page", http.StatusInternalServerError)
		return
	}
	wc.jsonOK(w, page)
}

// DeletePage DELETE /api/wiki/pages/{pageID}
func (wc *Wiki) DeletePage(w http.ResponseWriter, r *http.Request) {
	id, ok := wc.parsePageID(w, r)
	if !ok {
		return
	}
	if err := wc.WikiPageService.Delete(id); err != nil {
		http.Error(w, "Failed to delete page", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetPageContent GET /api/wiki/pages/{pageID}/content
func (wc *Wiki) GetPageContent(w http.ResponseWriter, r *http.Request) {
	id, ok := wc.parsePageID(w, r)
	if !ok {
		return
	}
	content, err := wc.WikiPageService.GetContent(id)
	if err != nil {
		http.Error(w, "Page not found", http.StatusNotFound)
		return
	}
	wc.jsonOK(w, map[string]string{"content": content})
}

// UpdatePageContent PUT /api/wiki/pages/{pageID}/content
func (wc *Wiki) UpdatePageContent(w http.ResponseWriter, r *http.Request) {
	id, ok := wc.parsePageID(w, r)
	if !ok {
		return
	}

	var req struct {
		Content    string `json:"content"`
		UserID     int    `json:"user_id"`
		ManualSave bool   `json:"manual_save"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		return
	}

	if err := wc.WikiPageService.UpdateContent(id, req.Content); err != nil {
		http.Error(w, "Failed to update content", http.StatusInternalServerError)
		return
	}

	if req.UserID > 0 {
		_ = wc.WikiPageService.MaybeCreateVersion(id, req.UserID, req.Content, req.ManualSave)
	}

	wc.jsonOK(w, map[string]string{"status": "updated"})
}

// ListVersions GET /api/wiki/pages/{pageID}/versions
func (wc *Wiki) ListVersions(w http.ResponseWriter, r *http.Request) {
	id, ok := wc.parsePageID(w, r)
	if !ok {
		return
	}
	versions, err := wc.WikiPageService.GetVersions(id)
	if err != nil {
		http.Error(w, "Failed to list versions", http.StatusInternalServerError)
		return
	}
	if versions == nil {
		versions = []models.WikiPageVersion{}
	}
	wc.jsonOK(w, versions)
}

// GetVersion GET /api/wiki/pages/{pageID}/versions/{versionNum}
func (wc *Wiki) GetVersion(w http.ResponseWriter, r *http.Request) {
	id, ok := wc.parsePageID(w, r)
	if !ok {
		return
	}
	versionNum, err := strconv.Atoi(chi.URLParam(r, "versionNum"))
	if err != nil {
		http.Error(w, "Invalid version number", http.StatusBadRequest)
		return
	}
	version, err := wc.WikiPageService.GetVersion(id, versionNum)
	if err != nil {
		http.Error(w, "Version not found", http.StatusNotFound)
		return
	}
	wc.jsonOK(w, version)
}

// RestoreVersion POST /api/wiki/pages/{pageID}/versions/{versionNum}/restore
func (wc *Wiki) RestoreVersion(w http.ResponseWriter, r *http.Request) {
	id, ok := wc.parsePageID(w, r)
	if !ok {
		return
	}
	versionNum, err := strconv.Atoi(chi.URLParam(r, "versionNum"))
	if err != nil {
		http.Error(w, "Invalid version number", http.StatusBadRequest)
		return
	}

	var req struct {
		UserID int `json:"user_id"`
	}
	// User ID from body; ignore decode errors (optional)
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := wc.WikiPageService.RestoreVersion(id, versionNum, req.UserID); err != nil {
		log.Printf("wiki restore: %v", err)
		http.Error(w, "Failed to restore version", http.StatusInternalServerError)
		return
	}

	wc.jsonOK(w, map[string]string{"status": "restored"})
}

// DeleteVersionHandler DELETE /api/wiki/pages/{pageID}/versions/{versionNum}
func (wc *Wiki) DeleteVersionHandler(w http.ResponseWriter, r *http.Request) {
	user := authmw.GetUserFromContext(r.Context())
	if user == nil || !models.IsAdmin(user.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, ok := wc.parsePageID(w, r)
	if !ok {
		return
	}
	versionNum, err := strconv.Atoi(chi.URLParam(r, "versionNum"))
	if err != nil {
		http.Error(w, "Invalid version number", http.StatusBadRequest)
		return
	}

	if err := wc.WikiPageService.DeleteVersion(id, versionNum); err != nil {
		http.Error(w, "Version not found", http.StatusNotFound)
		return
	}

	wc.jsonOK(w, map[string]string{"status": "deleted"})
}

// SearchPages GET /api/wiki/search?q=...
func (wc *Wiki) SearchPages(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, "Query parameter 'q' is required", http.StatusBadRequest)
		return
	}
	limit := 20
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}
	pages, err := wc.WikiPageService.Search(q, limit)
	if err != nil {
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}
	if pages == nil {
		pages = []models.WikiPage{}
	}
	wc.jsonOK(w, pages)
}

// AutocompletePages GET /api/wiki/autocomplete?q=...
func (wc *Wiki) AutocompletePages(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, "Query parameter 'q' is required", http.StatusBadRequest)
		return
	}
	limit := 10
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}
	pages, err := wc.WikiPageService.Autocomplete(q, limit)
	if err != nil {
		http.Error(w, "Autocomplete failed", http.StatusInternalServerError)
		return
	}
	if pages == nil {
		pages = []models.WikiPage{}
	}
	wc.jsonOK(w, pages)
}
