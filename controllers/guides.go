package controllers

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"anshumanbiswas.com/blog/models"
	"anshumanbiswas.com/blog/utils"
	"anshumanbiswas.com/blog/views"
	"github.com/go-chi/chi/v5"
)

// Guides handles guide CRUD and public display.
type Guides struct {
	Templates struct {
		GuidesList  views.Template
		GuidePage   views.Template
		AdminGuides views.Template
		GuideEditor views.Template
	}
	GuideService    *models.GuideService
	SessionService  *models.SessionService
	CategoryService *models.CategoryService
}

// PublicGuidesList displays the public guides listing page.
// GET /guides
func (g Guides) PublicGuidesList(w http.ResponseWriter, r *http.Request) {
	user, _ := utils.IsUserLoggedIn(r, g.SessionService)

	guides, err := g.GuideService.GetPublishedGuides()
	if err != nil {
		log.Printf("Error fetching guides: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	userPerms := models.GetPermissions(models.RoleCommenter)
	if user != nil {
		userPerms = models.GetPermissions(user.Role)
	}

	data := struct {
		LoggedIn        bool
		Username        string
		IsAdmin         bool
		Description     string
		CurrentPage     string
		Guides          *models.GuidesList
		UserPermissions models.UserPermissions
	}{
		LoggedIn:        user != nil,
		Username:        getUsername(user),
		IsAdmin:         user != nil && models.IsAdmin(user.Role),
		Description:     "In-depth guides and tutorials - Anshuman Biswas Blog",
		CurrentPage:     "guides",
		Guides:          guides,
		UserPermissions: userPerms,
	}

	g.Templates.GuidesList.Execute(w, r, data)
}

// ViewGuide displays a single guide with content.
// GET /guides/{slug}
func (g Guides) ViewGuide(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	user, _ := utils.IsUserLoggedIn(r, g.SessionService)

	guide, err := g.GuideService.GetBySlug(slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Check if guide is published (unless user can view unpublished)
	if !guide.IsPublished && (user == nil || !models.CanViewUnpublished(user.Role)) {
		http.NotFound(w, r)
		return
	}

	userPerms := models.GetPermissions(models.RoleCommenter)
	if user != nil {
		userPerms = models.GetPermissions(user.Role)
	}

	// Compute full URL for share links
	scheme := "https"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if r.TLS == nil {
		scheme = "http"
	}
	fullURL := fmt.Sprintf("%s://%s/guides/%s", scheme, r.Host, slug)

	data := struct {
		LoggedIn        bool
		Username        string
		IsAdmin         bool
		UserID          int
		Description     string
		CurrentPage     string
		Guide           *models.Guide
		FullURL         string
		UserPermissions models.UserPermissions
	}{
		LoggedIn:        user != nil,
		Username:        getUsername(user),
		IsAdmin:         user != nil && models.IsAdmin(user.Role),
		Description:     guide.Title + " - Anshuman Biswas",
		CurrentPage:     "guides",
		Guide:           guide,
		FullURL:         fullURL,
		UserPermissions: userPerms,
	}

	if user != nil {
		data.UserID = user.UserID
	}

	g.Templates.GuidePage.Execute(w, r, data)
}

// AdminGuides displays the admin guides management page.
// GET /admin/guides
func (g Guides) AdminGuides(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, g.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	if !models.CanViewAdminPanel(user.Role) {
		http.Error(w, errForbiddenAdmin, http.StatusForbidden)
		return
	}

	guides, err := g.GuideService.GetAllGuides()
	if err != nil {
		log.Printf("Error fetching guides: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := struct {
		LoggedIn        bool
		Username        string
		IsAdmin         bool
		Description     string
		CurrentPage     string
		Guides          *models.GuidesList
		UserPermissions models.UserPermissions
	}{
		LoggedIn:        true,
		Username:        user.Username,
		IsAdmin:         models.IsAdmin(user.Role),
		Description:     "Manage Guides - Anshuman Biswas Blog",
		CurrentPage:     "admin-guides",
		Guides:          guides,
		UserPermissions: models.GetPermissions(user.Role),
	}

	g.Templates.AdminGuides.Execute(w, r, data)
}

// NewGuide displays the guide creation editor.
// GET /admin/guides/new
func (g Guides) NewGuide(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, g.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	perms := models.GetPermissions(user.Role)
	if !perms.CanEditPosts {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	categories, err := g.CategoryService.GetAll()
	if err != nil {
		log.Printf("Error fetching categories: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := struct {
		LoggedIn        bool
		Username        string
		IsAdmin         bool
		Description     string
		CurrentPage     string
		Categories      []models.Category
		Guide           *models.Guide
		IsEdit          bool
		UserPermissions models.UserPermissions
	}{
		LoggedIn:        true,
		Username:        user.Username,
		IsAdmin:         models.IsAdmin(user.Role),
		Description:     "Create New Guide - Anshuman Biswas Blog",
		CurrentPage:     "admin-guides",
		Categories:      categories,
		Guide:           &models.Guide{},
		IsEdit:          false,
		UserPermissions: perms,
	}

	g.Templates.GuideEditor.Execute(w, r, data)
}

// CreateGuide handles guide creation.
// POST /admin/guides
func (g Guides) CreateGuide(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, g.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	perms := models.GetPermissions(user.Role)
	if !perms.CanEditPosts {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	slug := r.FormValue("slug")
	content := r.FormValue("content")
	description := r.FormValue("description")
	featuredImageURL := r.FormValue("featured_image_url")
	isPublished := r.FormValue("is_published") == "on" || r.FormValue("is_published") == "true"
	categoriesStr := r.FormValue("categories")

	if title == "" || content == "" {
		http.Error(w, "Title and content are required", http.StatusBadRequest)
		return
	}

	var categoryIDs []int
	if categoriesStr != "" {
		for _, catStr := range strings.Split(categoriesStr, ",") {
			if catID, err := strconv.Atoi(strings.TrimSpace(catStr)); err == nil {
				categoryIDs = append(categoryIDs, catID)
			}
		}
	}

	guide, err := g.GuideService.Create(user.UserID, title, slug, content, description, featuredImageURL, isPublished, categoryIDs)
	if err != nil {
		log.Printf("Error creating guide: %v", err)
		http.Error(w, "Failed to create guide", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/guides/%d/edit", guide.ID), http.StatusFound)
}

// EditGuide displays the guide editing page.
// GET /admin/guides/{guideID}/edit
func (g Guides) EditGuide(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, g.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	perms := models.GetPermissions(user.Role)
	if !perms.CanEditPosts {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	guideIDStr := chi.URLParam(r, "guideID")
	guideID, err := strconv.Atoi(guideIDStr)
	if err != nil {
		http.Error(w, "Invalid guide ID", http.StatusBadRequest)
		return
	}

	guide, err := g.GuideService.GetByID(guideID)
	if err != nil {
		log.Printf("Error fetching guide: %v", err)
		http.Error(w, "Guide not found", http.StatusNotFound)
		return
	}

	categories, err := g.CategoryService.GetAll()
	if err != nil {
		log.Printf("Error fetching categories: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := struct {
		LoggedIn        bool
		Username        string
		IsAdmin         bool
		Description     string
		CurrentPage     string
		Categories      []models.Category
		Guide           *models.Guide
		IsEdit          bool
		UserPermissions models.UserPermissions
	}{
		LoggedIn:        true,
		Username:        user.Username,
		IsAdmin:         models.IsAdmin(user.Role),
		Description:     "Edit Guide - Anshuman Biswas Blog",
		CurrentPage:     "admin-guides",
		Categories:      categories,
		Guide:           guide,
		IsEdit:          true,
		UserPermissions: perms,
	}

	g.Templates.GuideEditor.Execute(w, r, data)
}

// UpdateGuide handles guide updates.
// POST /admin/guides/{guideID}
func (g Guides) UpdateGuide(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, g.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	perms := models.GetPermissions(user.Role)
	if !perms.CanEditPosts {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	guideIDStr := chi.URLParam(r, "guideID")
	guideID, err := strconv.Atoi(guideIDStr)
	if err != nil {
		http.Error(w, "Invalid guide ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	slug := r.FormValue("slug")
	content := r.FormValue("content")
	description := r.FormValue("description")
	featuredImageURL := r.FormValue("featured_image_url")
	isPublished := r.FormValue("is_published") == "on" || r.FormValue("is_published") == "true"
	categoriesStr := r.FormValue("categories")

	if title == "" || content == "" {
		http.Error(w, "Title and content are required", http.StatusBadRequest)
		return
	}

	var categoryIDs []int
	if categoriesStr != "" {
		for _, catStr := range strings.Split(categoriesStr, ",") {
			if catID, err := strconv.Atoi(strings.TrimSpace(catStr)); err == nil {
				categoryIDs = append(categoryIDs, catID)
			}
		}
	}

	err = g.GuideService.Update(guideID, title, slug, content, description, featuredImageURL, isPublished, categoryIDs)
	if err != nil {
		log.Printf("Error updating guide: %v", err)
		http.Error(w, "Failed to update guide", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/guides/%d/edit", guideID), http.StatusFound)
}

// DeleteGuide handles guide deletion.
// POST /admin/guides/{guideID}/delete
func (g Guides) DeleteGuide(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, g.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	if !models.IsAdmin(user.Role) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	guideIDStr := chi.URLParam(r, "guideID")
	guideID, err := strconv.Atoi(guideIDStr)
	if err != nil {
		http.Error(w, "Invalid guide ID", http.StatusBadRequest)
		return
	}

	err = g.GuideService.Delete(guideID)
	if err != nil {
		log.Printf("Error deleting guide: %v", err)
		http.Error(w, "Failed to delete guide", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/guides", http.StatusFound)
}

// PreviewGuide handles markdown preview for guides.
// POST /admin/guides/preview
func (g Guides) PreviewGuide(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, g.SessionService)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	perms := models.GetPermissions(user.Role)
	if !perms.CanEditPosts {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	content := r.FormValue("content")
	rendered := models.RenderContent(content)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, template.HTML(rendered))
}
