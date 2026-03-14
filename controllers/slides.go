package controllers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"anshumanbiswas.com/blog/models"
	"anshumanbiswas.com/blog/utils"
	"anshumanbiswas.com/blog/views"
	"github.com/go-chi/chi/v5"
)

type Slides struct {
	Templates struct {
		AdminSlides       views.Template
		SlideEditor       views.Template
		SlidesList        views.Template
		SlidePresentation views.Template
	}
	SlideService    *models.SlideService
	SessionService  *models.SessionService
	CategoryService *models.CategoryService
}

// AdminSlides displays the admin slides management page
func (s Slides) AdminSlides(w http.ResponseWriter, r *http.Request) {
	user, err := s.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	if !models.CanEditSlides(user.Role) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	slides, err := s.SlideService.GetAllSlides()
	if err != nil {
		log.Printf("Error fetching slides: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	categories, err := s.CategoryService.GetAll()
	if err != nil {
		log.Printf("Error fetching categories: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := struct {
		LoggedIn        bool
		Username        string
		IsAdmin         bool
		SignupDisabled  bool
		Description     string
		CurrentPage     string
		Slides          *models.SlidesList
		Categories      []models.Category
		UserPermissions models.UserPermissions
	}{
		LoggedIn:        true,
		Username:        user.Username,
		IsAdmin:         models.IsAdmin(user.Role),
		SignupDisabled:  true,
		Description:     "Manage Slides - Anshuman Biswas Blog",
		CurrentPage:     "admin-slides",
		Slides:          slides,
		Categories:      categories,
		UserPermissions: models.GetPermissions(user.Role),
	}

	s.Templates.AdminSlides.Execute(w, r, data)
}

// NewSlide displays the slide creation page
func (s Slides) NewSlide(w http.ResponseWriter, r *http.Request) {
	user, err := s.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	if !models.CanEditSlides(user.Role) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	categories, err := s.CategoryService.GetAll()
	if err != nil {
		log.Printf("Error fetching categories: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := struct {
		LoggedIn        bool
		Username        string
		IsAdmin         bool
		SignupDisabled  bool
		Description     string
		CurrentPage     string
		Categories      []models.Category
		Slide           *models.Slide
		IsEdit          bool
		UserPermissions models.UserPermissions
	}{
		LoggedIn:        true,
		Username:        user.Username,
		IsAdmin:         models.IsAdmin(user.Role),
		SignupDisabled:  true,
		Description:     "Create New Slide - Anshuman Biswas Blog",
		CurrentPage:     "admin-slides",
		Categories:      categories,
		Slide:           &models.Slide{},
		IsEdit:          false,
		UserPermissions: models.GetPermissions(user.Role),
	}

	s.Templates.SlideEditor.Execute(w, r, data)
}

// CreateSlide handles slide creation
func (s Slides) CreateSlide(w http.ResponseWriter, r *http.Request) {
	user, err := s.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	if !models.CanEditSlides(user.Role) {
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
	isPublishedStr := r.FormValue("is_published")
	categoriesStr := r.FormValue("categories")

	if title == "" || content == "" {
		http.Error(w, "Title and content are required", http.StatusBadRequest)
		return
	}

	isPublished := isPublishedStr == "on" || isPublishedStr == "true"

	// Parse category IDs
	var categoryIDs []int
	if categoriesStr != "" {
		categoryStrs := strings.Split(categoriesStr, ",")
		for _, catStr := range categoryStrs {
			if catID, err := strconv.Atoi(strings.TrimSpace(catStr)); err == nil {
				categoryIDs = append(categoryIDs, catID)
			}
		}
	}

	slide, err := s.SlideService.Create(user.UserID, title, slug, content, isPublished, categoryIDs)
	if err != nil {
		log.Printf("Error creating slide: %v", err)
		http.Error(w, "Failed to create slide", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/slides/%d/edit", slide.ID), http.StatusFound)
}

// EditSlide displays the slide editing page
func (s Slides) EditSlide(w http.ResponseWriter, r *http.Request) {
	user, err := s.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	if !models.CanEditSlides(user.Role) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	slideIDStr := chi.URLParam(r, "slideID")
	slideID, err := strconv.Atoi(slideIDStr)
	if err != nil {
		http.Error(w, "Invalid slide ID", http.StatusBadRequest)
		return
	}

	slide, err := s.SlideService.GetByID(slideID)
	if err != nil {
		log.Printf("Error fetching slide: %v", err)
		http.Error(w, "Slide not found", http.StatusNotFound)
		return
	}

	categories, err := s.CategoryService.GetAll()
	if err != nil {
		log.Printf("Error fetching categories: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := struct {
		LoggedIn        bool
		Username        string
		IsAdmin         bool
		SignupDisabled  bool
		Description     string
		CurrentPage     string
		Categories      []models.Category
		Slide           *models.Slide
		IsEdit          bool
		UserPermissions models.UserPermissions
	}{
		LoggedIn:        true,
		Username:        user.Username,
		IsAdmin:         models.IsAdmin(user.Role),
		SignupDisabled:  true,
		Description:     "Edit Slide - Anshuman Biswas Blog",
		CurrentPage:     "admin-slides",
		Categories:      categories,
		Slide:           slide,
		IsEdit:          true,
		UserPermissions: models.GetPermissions(user.Role),
	}

	s.Templates.SlideEditor.Execute(w, r, data)
}

// UpdateSlide handles slide updates
func (s Slides) UpdateSlide(w http.ResponseWriter, r *http.Request) {
	user, err := s.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	if !models.CanEditSlides(user.Role) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	slideIDStr := chi.URLParam(r, "slideID")
	slideID, err := strconv.Atoi(slideIDStr)
	if err != nil {
		http.Error(w, "Invalid slide ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	slug := r.FormValue("slug")
	content := r.FormValue("content")
	isPublishedStr := r.FormValue("is_published")
	categoriesStr := r.FormValue("categories")

	if title == "" || content == "" {
		http.Error(w, "Title and content are required", http.StatusBadRequest)
		return
	}

	isPublished := isPublishedStr == "on" || isPublishedStr == "true"

	// Parse category IDs
	var categoryIDs []int
	if categoriesStr != "" {
		categoryStrs := strings.Split(categoriesStr, ",")
		for _, catStr := range categoryStrs {
			if catID, err := strconv.Atoi(strings.TrimSpace(catStr)); err == nil {
				categoryIDs = append(categoryIDs, catID)
			}
		}
	}

	err = s.SlideService.Update(slideID, title, slug, content, isPublished, categoryIDs)
	if err != nil {
		log.Printf("Error updating slide: %v", err)
		http.Error(w, "Failed to update slide", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/slides/%d/edit", slideID), http.StatusFound)
}

// DeleteSlide handles slide deletion (admin-only)
func (s Slides) DeleteSlide(w http.ResponseWriter, r *http.Request) {
	user, err := s.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	if !models.IsAdmin(user.Role) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	slideIDStr := chi.URLParam(r, "slideID")
	slideID, err := strconv.Atoi(slideIDStr)
	if err != nil {
		http.Error(w, "Invalid slide ID", http.StatusBadRequest)
		return
	}

	err = s.SlideService.Delete(slideID)
	if err != nil {
		log.Printf("Error deleting slide: %v", err)
		http.Error(w, "Failed to delete slide", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/slides", http.StatusFound)
}

// PublicSlidesList displays the public slides listing page
func (s Slides) PublicSlidesList(w http.ResponseWriter, r *http.Request) {
	user, _ := s.isUserLoggedIn(r)

	slides, err := s.SlideService.GetPublishedSlides()
	if err != nil {
		log.Printf("Error fetching slides: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Format relative times for slides
	for i := range slides.Slides {
		slides.Slides[i].RelativeTime = utils.FormatRelativeTime(slides.Slides[i].CreatedAt)
	}

	userPerms := models.GetPermissions(models.RoleCommenter)
	if user != nil {
		userPerms = models.GetPermissions(user.Role)
	}

	data := struct {
		LoggedIn        bool
		Username        string
		IsAdmin         bool
		SignupDisabled  bool
		Description     string
		CurrentPage     string
		Slides          *models.SlidesList
		UserPermissions models.UserPermissions
	}{
		LoggedIn:        user != nil,
		Username:        getUsername(user),
		IsAdmin:         user != nil && models.IsAdmin(user.Role),
		SignupDisabled:  true, // Default for public pages
		Description:     "Interactive presentations and talks - Anshuman Biswas Blog",
		CurrentPage:     "slides",
		Slides:          slides,
		UserPermissions: userPerms,
	}

	s.Templates.SlidesList.Execute(w, r, data)
}

// ViewSlide displays a single slide presentation
func (s Slides) ViewSlide(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	user, _ := s.isUserLoggedIn(r)

	slide, err := s.SlideService.GetBySlug(slug)
	if err != nil {
		log.Printf("Error fetching slide: %v", err)
		http.Error(w, "Slide not found", http.StatusNotFound)
		return
	}

	// Check if slide is published (unless user can edit slides)
	if !slide.IsPublished && (user == nil || !models.CanEditSlides(user.Role)) {
		http.Error(w, "Slide not found", http.StatusNotFound)
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
		SignupDisabled  bool
		Description     string
		CurrentPage     string
		Slide           *models.Slide
		UserPermissions models.UserPermissions
	}{
		LoggedIn:        user != nil,
		Username:        getUsername(user),
		IsAdmin:         user != nil && models.IsAdmin(user.Role),
		SignupDisabled:  true, // Default for public pages
		Description:     slide.Title + " - Interactive presentation - Anshuman Biswas Blog",
		CurrentPage:     "slides",
		Slide:           slide,
		UserPermissions: userPerms,
	}

	s.Templates.SlidePresentation.Execute(w, r, data)
}

// PreviewSlide handles slide preview (for admin)
func (s Slides) PreviewSlide(w http.ResponseWriter, r *http.Request) {
	user, err := s.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	if !models.CanEditSlides(user.Role) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	content := r.FormValue("content")
	if content == "" {
		http.Error(w, "Content is required", http.StatusBadRequest)
		return
	}

	// Wrap content in proper Reveal.js HTML structure for preview
	previewHTML := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Slide Preview</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/reveal.js@4.3.1/dist/reveal.css">
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/reveal.js@4.3.1/dist/theme/white.css">
    <style>
        .reveal { font-size: 24px; }
        .reveal h1, .reveal h2, .reveal h3 { text-transform: none; }
    </style>
</head>
<body>
    <div class="reveal">
        <div class="slides">
            %s
        </div>
    </div>
    
    <script src="https://cdn.jsdelivr.net/npm/reveal.js@4.3.1/dist/reveal.js"></script>
    <script>
        Reveal.initialize({
            hash: true,
            controls: true,
            progress: true,
            center: true,
            transition: 'slide',
            width: '100%%',
            height: '100%%'
        });
    </script>
</body>
</html>`, content)

	// Return the rendered HTML content
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(previewHTML))
}

// Helper functions
func (s Slides) isUserLoggedIn(r *http.Request) (*models.User, error) {
	return utils.IsUserLoggedIn(r, s.SessionService)
}

func getUsername(user *models.User) string {
	if user == nil {
		return ""
	}
	return user.Username
}