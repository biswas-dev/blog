package controllers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"anshumanbiswas.com/blog/models"
	"anshumanbiswas.com/blog/utils"
	"anshumanbiswas.com/blog/views"
	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

type Slides struct {
	Templates struct {
		AdminSlides       views.Template
		SlideEditor       views.Template
		SlidesList        views.Template
		SlidePresentation views.Template
		SlidePassword     views.Template
	}
	SlideService        *models.SlideService
	SlideVersionService *models.SlideVersionService
	SessionService      *models.SessionService
	CategoryService     *models.CategoryService
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

	description := r.FormValue("description")
	metadata := r.FormValue("metadata")
	password := r.FormValue("password")
	featuredImageURL := r.FormValue("featured_image_url")

	slide, err := s.SlideService.Create(user.UserID, title, slug, content, isPublished, categoryIDs, description, metadata, password, featuredImageURL)
	if err != nil {
		log.Printf("Error creating slide: %v", err)
		http.Error(w, "Failed to create slide", http.StatusInternalServerError)
		return
	}

	// Create initial version
	if s.SlideVersionService != nil {
		_ = s.SlideVersionService.MaybeCreateVersion(slide.ID, user.UserID, title, content)
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

	description := r.FormValue("description")
	metadata := r.FormValue("metadata")
	password := r.FormValue("password")
	featuredImageURL := r.FormValue("featured_image_url")

	// Create version snapshot before updating
	if s.SlideVersionService != nil {
		_ = s.SlideVersionService.MaybeCreateVersion(slideID, user.UserID, title, content)
	}

	err = s.SlideService.Update(slideID, title, slug, content, isPublished, categoryIDs, description, metadata, password, featuredImageURL)
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

	// Load contributors
	if s.SlideVersionService != nil {
		contributors, _ := s.SlideVersionService.GetContributors(slide.ID)
		slide.Contributors = contributors
	}

	// Check if slide is published (unless user can edit slides)
	if !slide.IsPublished && (user == nil || !models.CanEditSlides(user.Role)) {
		http.Error(w, "Slide not found", http.StatusNotFound)
		return
	}

	// Password protection check: editors/admins bypass
	isEditor := user != nil && models.CanEditSlides(user.Role)
	if slide.PasswordHash != "" && !isEditor {
		cookieName := fmt.Sprintf("slide_access_%s", slug)
		cookie, err := r.Cookie(cookieName)
		if err != nil || cookie.Value != "granted" {
			// Render password prompt
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
				Error           string
				UserPermissions models.UserPermissions
			}{
				LoggedIn:        user != nil,
				Username:        getUsername(user),
				IsAdmin:         user != nil && models.IsAdmin(user.Role),
				SignupDisabled:  true,
				Description:     slide.Title + " - Password Required",
				CurrentPage:     "slides",
				Slide:           slide,
				UserPermissions: userPerms,
			}
			s.Templates.SlidePassword.Execute(w, r, data)
			return
		}
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

// VerifySlidePassword handles POST /slides/{slug}/verify
func (s Slides) VerifySlidePassword(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	slide, err := s.SlideService.GetBySlug(slug)
	if err != nil {
		http.Error(w, "Slide not found", http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	password := r.FormValue("password")
	if bcrypt.CompareHashAndPassword([]byte(slide.PasswordHash), []byte(password)) != nil {
		// Wrong password — re-render password prompt with error
		user, _ := s.isUserLoggedIn(r)
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
			Error           string
			UserPermissions models.UserPermissions
		}{
			LoggedIn:        user != nil,
			Username:        getUsername(user),
			IsAdmin:         user != nil && models.IsAdmin(user.Role),
			SignupDisabled:  true,
			Description:     slide.Title + " - Password Required",
			CurrentPage:     "slides",
			Slide:           slide,
			Error:           "Incorrect password. Please try again.",
			UserPermissions: userPerms,
		}
		s.Templates.SlidePassword.Execute(w, r, data)
		return
	}

	// Correct password — set session cookie and redirect
	cookieName := fmt.Sprintf("slide_access_%s", slug)
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "granted",
		Path:     fmt.Sprintf("/slides/%s", slug),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, fmt.Sprintf("/slides/%s", slug), http.StatusFound)
}

// AutoSave handles POST /api/admin/slides/{slideID}/autosave
func (s Slides) AutoSave(w http.ResponseWriter, r *http.Request) {
	user, err := s.isUserLoggedIn(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !models.CanEditSlides(user.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	slideID, err := strconv.Atoi(chi.URLParam(r, "slideID"))
	if err != nil {
		http.Error(w, "Invalid slide ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Content  string `json:"content"`
		Title    string `json:"title"`
		Metadata string `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	slide, err := s.SlideService.GetByID(slideID)
	if err != nil {
		http.Error(w, "Slide not found", http.StatusNotFound)
		return
	}

	// Write content to file
	if req.Content != "" {
		if err := os.WriteFile(slide.ContentFilePath, []byte(req.Content), 0644); err != nil {
			log.Printf("Autosave write error: %v", err)
			http.Error(w, "Failed to save", http.StatusInternalServerError)
			return
		}
	}

	title := req.Title
	if title == "" {
		title = slide.Title
	}
	content := req.Content
	if content == "" {
		content = string(slide.ContentHTML)
	}

	// Create version snapshot
	var versionNum int
	if s.SlideVersionService != nil {
		_ = s.SlideVersionService.MaybeCreateVersion(slideID, user.UserID, title, content)
		versions, err := s.SlideVersionService.GetVersions(slideID)
		if err == nil && len(versions) > 0 {
			versionNum = versions[0].VersionNumber
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "saved",
		"version": versionNum,
	})
}

// UploadSlideImage handles POST /admin/slides/upload-image
func (s Slides) UploadSlideImage(w http.ResponseWriter, r *http.Request) {
	user, err := s.isUserLoggedIn(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !models.CanEditSlides(user.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "No image file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	slug := r.FormValue("slug")
	if slug == "" {
		slug = "general"
	}

	url, err := saveUploadedFile(file, header.Filename, "slide", slug)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": url})
}

// ImportPPTX handles POST /api/admin/slides/import-pptx
func (s Slides) ImportPPTX(w http.ResponseWriter, r *http.Request) {
	user, err := s.isUserLoggedIn(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !models.CanEditSlides(user.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseMultipartForm(50 << 20); err != nil { // 50MB
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("pptx")
	if err != nil {
		http.Error(w, "No PPTX file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read file into memory for zip processing
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, file); err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	zipReader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		http.Error(w, "Invalid PPTX file", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	if title == "" {
		title = "Imported Presentation"
	}
	slug := r.FormValue("slug")

	// Parse slides from PPTX
	sections, mediaFiles := parsePPTX(zipReader)

	// Create the slide first to get ID and slug
	content := strings.Join(sections, "\n")
	slide, err := s.SlideService.Create(user.UserID, title, slug, content, false, nil, "", `{"source":"pptx"}`, "", "")
	if err != nil {
		log.Printf("Error creating imported slide: %v", err)
		http.Error(w, "Failed to create slide", http.StatusInternalServerError)
		return
	}

	// Save extracted media files
	if len(mediaFiles) > 0 {
		mediaDir := filepath.Join("static", "uploads", "slide", slide.Slug)
		os.MkdirAll(mediaDir, 0755)
		for name, data := range mediaFiles {
			destPath := filepath.Join(mediaDir, name)
			os.WriteFile(destPath, data, 0644)
		}
	}

	// Update slide count
	s.SlideService.UpdateSlideCount(slide.ID, len(sections))

	// Create initial version
	if s.SlideVersionService != nil {
		_ = s.SlideVersionService.MaybeCreateVersion(slide.ID, user.UserID, title, content)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"slideID":    slide.ID,
		"slug":       slide.Slug,
		"slideCount": len(sections),
	})
}

// ReimportPPTX handles POST /api/admin/slides/{slideID}/reimport-pptx
// Replaces an existing slide's content with freshly parsed PPTX.
func (s Slides) ReimportPPTX(w http.ResponseWriter, r *http.Request) {
	user, err := s.isUserLoggedIn(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !models.CanEditSlides(user.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	slideIDStr := chi.URLParam(r, "slideID")
	slideID, err := strconv.Atoi(slideIDStr)
	if err != nil {
		http.Error(w, "Invalid slide ID", http.StatusBadRequest)
		return
	}

	// Verify slide exists
	slide, err := s.SlideService.GetByID(slideID)
	if err != nil {
		http.Error(w, "Slide not found", http.StatusNotFound)
		return
	}

	if err := r.ParseMultipartForm(50 << 20); err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("pptx")
	if err != nil {
		http.Error(w, "No PPTX file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, file); err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	zipReader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		http.Error(w, "Invalid PPTX file", http.StatusBadRequest)
		return
	}

	sections, mediaFiles := parsePPTX(zipReader)
	content := strings.Join(sections, "\n")

	// Create version snapshot before updating
	if s.SlideVersionService != nil {
		_ = s.SlideVersionService.MaybeCreateVersion(slideID, user.UserID, slide.Title, content)
	}

	// Ensure content file directory exists (may not exist in fresh container)
	os.MkdirAll(filepath.Dir(slide.ContentFilePath), 0755)

	// Update slide content (keep existing title, slug, published state, etc.)
	// Mark as PPTX source in metadata
	metadata := slide.SlideMetadata
	if metadata == "" || metadata == "{}" {
		metadata = `{"source":"pptx"}`
	} else if !strings.Contains(metadata, `"source"`) {
		metadata = strings.TrimSuffix(metadata, "}") + `,"source":"pptx"}`
	}
	err = s.SlideService.Update(slideID, slide.Title, slide.Slug, content, slide.IsPublished, nil, slide.Description, metadata, "", slide.FeaturedImageURL)
	if err != nil {
		log.Printf("Error reimporting slide: %v", err)
		http.Error(w, "Failed to update slide", http.StatusInternalServerError)
		return
	}

	// Save extracted media files
	if len(mediaFiles) > 0 {
		mediaDir := filepath.Join("static", "uploads", "slide", slide.Slug)
		os.MkdirAll(mediaDir, 0755)
		for name, data := range mediaFiles {
			destPath := filepath.Join(mediaDir, name)
			os.WriteFile(destPath, data, 0644)
		}
	}

	// Update slide count
	s.SlideService.UpdateSlideCount(slideID, len(sections))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"slideID":    slideID,
		"slug":       slide.Slug,
		"slideCount": len(sections),
	})
}

// PPTX slide dimensions in EMU (English Metric Units)
const (
	pptxSlideWidthEMU  = 9144000.0
	pptxSlideHeightEMU = 5143500.0
	pptxBaseFontPt     = 20.0 // reveal.js base font size (30px / 1.5 scale factor for readability)
)

// parsePPTX extracts slide content and media from a PPTX zip archive.
func parsePPTX(zr *zip.Reader) (sections []string, media map[string][]byte) {
	media = make(map[string][]byte)

	// Collect slide XML files and sort by number
	type slideFile struct {
		num  int
		file *zip.File
	}
	var slideFiles []slideFile

	slideNumRe := regexp.MustCompile(`ppt/slides/slide(\d+)\.xml`)

	for _, f := range zr.File {
		if m := slideNumRe.FindStringSubmatch(f.Name); m != nil {
			num, _ := strconv.Atoi(m[1])
			slideFiles = append(slideFiles, slideFile{num: num, file: f})
		}
		// Extract media files
		if strings.HasPrefix(f.Name, "ppt/media/") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}
			baseName := filepath.Base(f.Name)
			media[baseName] = data
		}
	}

	sort.Slice(slideFiles, func(i, j int) bool {
		return slideFiles[i].num < slideFiles[j].num
	})

	for _, sf := range slideFiles {
		sectionAttrs, body := parseSlideXML(sf.file)
		sections = append(sections, fmt.Sprintf("<section%s>\n%s\n</section>", sectionAttrs, body))
	}

	if len(sections) == 0 {
		sections = append(sections, "<section>\n<h1>Imported Presentation</h1>\n<p>No slides could be extracted.</p>\n</section>")
	}

	return sections, media
}

// OOXML types for parsing slide content with full styling support
type pptSlide struct {
	CSld struct {
		Bg *struct {
			BgPr *struct {
				SolidFill *pptSolidFill `xml:"solidFill"`
			} `xml:"bgPr"`
		} `xml:"bg"`
		SpTree struct {
			Shapes []pptShape `xml:"sp"`
		} `xml:"spTree"`
	} `xml:"cSld"`
}

type pptSolidFill struct {
	SrgbClr *struct {
		Val string `xml:"val,attr"`
	} `xml:"srgbClr"`
}

type pptShape struct {
	NvSpPr struct {
		CNvPr struct {
			Name string `xml:"name,attr"`
		} `xml:"cNvPr"`
		NvPr struct {
			Ph *struct {
				Type string `xml:"type,attr"`
				Idx  string `xml:"idx,attr"`
			} `xml:"ph"`
		} `xml:"nvPr"`
	} `xml:"nvSpPr"`
	SpPr *struct {
		Xfrm *struct {
			Off *struct {
				X int64 `xml:"x,attr"`
				Y int64 `xml:"y,attr"`
			} `xml:"off"`
			Ext *struct {
				Cx int64 `xml:"cx,attr"`
				Cy int64 `xml:"cy,attr"`
			} `xml:"ext"`
		} `xml:"xfrm"`
		SolidFill *pptSolidFill `xml:"solidFill"`
		NoFill    *struct{}     `xml:"noFill"`
	} `xml:"spPr"`
	TxBody *struct {
		BodyPr *struct {
			LIns *int64 `xml:"lIns,attr"`
			TIns *int64 `xml:"tIns,attr"`
			RIns *int64 `xml:"rIns,attr"`
			BIns *int64 `xml:"bIns,attr"`
		} `xml:"bodyPr"`
		Paragraphs []pptParagraph `xml:"p"`
	} `xml:"txBody"`
}

type pptParagraph struct {
	PPr *struct {
		Algn    string   `xml:"algn,attr"`
		BuNone  *struct{} `xml:"buNone"`
		BuChar  *struct {
			Char string `xml:"char,attr"`
		} `xml:"buChar"`
		BuAutoNum *struct{} `xml:"buAutoNum"`
		Lvl       int       `xml:"lvl,attr"`
		MarL      int64     `xml:"marL,attr"`
		Indent    int64     `xml:"indent,attr"`
	} `xml:"pPr"`
	Runs      []pptRun `xml:"r"`
	EndParaRPr *pptRunProps `xml:"endParaRPr"`
}

type pptRun struct {
	RPr  *pptRunProps `xml:"rPr"`
	Text string       `xml:"t"`
}

type pptRunProps struct {
	B         int           `xml:"b,attr"`
	I         int           `xml:"i,attr"`
	Sz        int           `xml:"sz,attr"`
	Spc       int           `xml:"spc,attr"`
	SolidFill *pptSolidFill `xml:"solidFill"`
}

// parseSlideXML extracts a single slide's HTML with full styling.
// Returns (section attributes string, inner HTML).
func parseSlideXML(f *zip.File) (string, string) {
	rc, err := f.Open()
	if err != nil {
		return "", "<p>Failed to parse slide</p>"
	}
	defer rc.Close()

	var slide pptSlide
	if err := xml.NewDecoder(rc).Decode(&slide); err != nil {
		return "", "<p>Failed to parse slide</p>"
	}

	// Extract slide background color
	var sectionAttrs string
	if slide.CSld.Bg != nil && slide.CSld.Bg.BgPr != nil &&
		slide.CSld.Bg.BgPr.SolidFill != nil && slide.CSld.Bg.BgPr.SolidFill.SrgbClr != nil {
		sectionAttrs = fmt.Sprintf(` data-background-color="#%s"`, slide.CSld.Bg.BgPr.SolidFill.SrgbClr.Val)
	}

	var parts []string
	hasPositioned := false

	// Collect bounding boxes of all decorative shapes for overlap detection
	type shapeRect struct {
		left, top, width, height float64
	}
	var decoRects []shapeRect
	for _, shape := range slide.CSld.SpTree.Shapes {
		if pptxIsDecorativeShape(shape) && shape.SpPr != nil && shape.SpPr.Xfrm != nil &&
			shape.SpPr.Xfrm.Off != nil && shape.SpPr.Xfrm.Ext != nil {
			r := shapeRect{
				left:   float64(shape.SpPr.Xfrm.Off.X) / pptxSlideWidthEMU * 100,
				top:    float64(shape.SpPr.Xfrm.Off.Y) / pptxSlideHeightEMU * 100,
				width:  float64(shape.SpPr.Xfrm.Ext.Cx) / pptxSlideWidthEMU * 100,
				height: float64(shape.SpPr.Xfrm.Ext.Cy) / pptxSlideHeightEMU * 100,
			}
			// Only consider large decorative shapes (>5% height) as potential occluders
			if r.height > 5 {
				decoRects = append(decoRects, r)
			}
		}
	}

	for _, shape := range slide.CSld.SpTree.Shapes {
		posStyle := pptxShapePosition(shape)
		if posStyle != "" {
			hasPositioned = true
		}

		// Check if this is a decorative (filled, no meaningful text) shape
		if pptxIsDecorativeShape(shape) {
			if html := pptxRenderDecorativeShape(shape, posStyle); html != "" {
				parts = append(parts, html)
			}
			continue
		}

		if shape.TxBody == nil {
			continue
		}

		// For text shapes, check if they overlap with a large decorative shape below.
		// If so, shrink the text shape's height to end at the decorative shape's top edge.
		if shape.SpPr != nil && shape.SpPr.Xfrm != nil &&
			shape.SpPr.Xfrm.Off != nil && shape.SpPr.Xfrm.Ext != nil {
			tLeft := float64(shape.SpPr.Xfrm.Off.X) / pptxSlideWidthEMU * 100
			tTop := float64(shape.SpPr.Xfrm.Off.Y) / pptxSlideHeightEMU * 100
			tWidth := float64(shape.SpPr.Xfrm.Ext.Cx) / pptxSlideWidthEMU * 100
			tHeight := float64(shape.SpPr.Xfrm.Ext.Cy) / pptxSlideHeightEMU * 100
			tRight := tLeft + tWidth
			tBottom := tTop + tHeight

			for _, dr := range decoRects {
				drRight := dr.left + dr.width
				// Check horizontal overlap
				if tLeft < drRight && tRight > dr.left {
					// Decorative shape starts within or below the text shape
					if dr.top > tTop && dr.top < tBottom {
						// Shrink text shape height to end above the decorative shape with a gap
						tHeight = dr.top - tTop - 1.5
						if tHeight < 1 {
							tHeight = 1
						}
						posStyle = fmt.Sprintf("position:absolute;left:%.2f%%;top:%.2f%%;width:%.2f%%;height:%.2f%%",
							tLeft, tTop, tWidth, tHeight)
						break
					}
				}
			}
		}

		// Render text shape
		if html := pptxRenderTextShape(shape, posStyle); html != "" {
			parts = append(parts, html)
		}
	}

	if hasPositioned {
		sectionAttrs += ` class="pptx-slide"`
	}

	if len(parts) == 0 {
		return sectionAttrs, "<p></p>"
	}
	return sectionAttrs, strings.Join(parts, "\n")
}

// pptxShapePosition extracts absolute CSS position from shape's xfrm transform.
func pptxShapePosition(shape pptShape) string {
	if shape.SpPr == nil || shape.SpPr.Xfrm == nil || shape.SpPr.Xfrm.Off == nil || shape.SpPr.Xfrm.Ext == nil {
		return ""
	}
	left := float64(shape.SpPr.Xfrm.Off.X) / pptxSlideWidthEMU * 100
	top := float64(shape.SpPr.Xfrm.Off.Y) / pptxSlideHeightEMU * 100
	width := float64(shape.SpPr.Xfrm.Ext.Cx) / pptxSlideWidthEMU * 100
	height := float64(shape.SpPr.Xfrm.Ext.Cy) / pptxSlideHeightEMU * 100
	return fmt.Sprintf("position:absolute;left:%.2f%%;top:%.2f%%;width:%.2f%%;height:%.2f%%", left, top, width, height)
}

// pptxIsDecorativeShape returns true if the shape is a filled rectangle with no meaningful text.
func pptxIsDecorativeShape(shape pptShape) bool {
	// Must have a solid fill (not noFill)
	if shape.SpPr == nil || shape.SpPr.SolidFill == nil {
		return false
	}
	// Check if there's no text, or only empty text
	if shape.TxBody == nil {
		return true
	}
	for _, p := range shape.TxBody.Paragraphs {
		for _, r := range p.Runs {
			if strings.TrimSpace(r.Text) != "" {
				return false
			}
		}
	}
	return true
}

// pptxRenderDecorativeShape renders a filled decorative shape as a div.
func pptxRenderDecorativeShape(shape pptShape, posStyle string) string {
	if shape.SpPr == nil || shape.SpPr.SolidFill == nil || shape.SpPr.SolidFill.SrgbClr == nil {
		return ""
	}
	color := shape.SpPr.SolidFill.SrgbClr.Val
	if posStyle == "" {
		return fmt.Sprintf(`<div style="background:#%s;width:100%%;height:4px;margin:0.5em 0"></div>`, color)
	}
	return fmt.Sprintf(`<div style="%s;background:#%s"></div>`, posStyle, color)
}

// pptxRenderTextShape renders a text shape with full styling.
func pptxRenderTextShape(shape pptShape, posStyle string) string {
	if shape.TxBody == nil {
		return ""
	}

	var paragraphs []string
	isBulletList := false

	for _, para := range shape.TxBody.Paragraphs {
		text := pptxExtractRunText(para.Runs)
		if text == "" {
			continue
		}

		// Paragraph-level alignment
		var paraStyle string
		if para.PPr != nil && para.PPr.Algn != "" {
			switch para.PPr.Algn {
			case "ctr":
				paraStyle = ` style="text-align:center"`
			case "r":
				paraStyle = ` style="text-align:right"`
			case "just":
				paraStyle = ` style="text-align:justify"`
			}
		}

		// Check for bullet points
		hasBullet := para.PPr != nil && para.PPr.BuChar != nil
		hasAutoNum := para.PPr != nil && para.PPr.BuAutoNum != nil

		if hasBullet || hasAutoNum {
			isBulletList = true
			paragraphs = append(paragraphs, fmt.Sprintf("<li%s>%s</li>", paraStyle, text))
		} else {
			// Determine tag from placeholder type
			phType := ""
			if shape.NvSpPr.NvPr.Ph != nil {
				phType = shape.NvSpPr.NvPr.Ph.Type
			}

			switch phType {
			case "title", "ctrTitle":
				paragraphs = append(paragraphs, fmt.Sprintf("<h1%s>%s</h1>", paraStyle, text))
			case "subTitle":
				paragraphs = append(paragraphs, fmt.Sprintf("<h3%s>%s</h3>", paraStyle, text))
			default:
				paragraphs = append(paragraphs, fmt.Sprintf("<p%s>%s</p>", paraStyle, text))
			}
		}
	}

	if len(paragraphs) == 0 {
		return ""
	}

	var inner string
	if isBulletList {
		inner = "<ul>\n" + strings.Join(paragraphs, "\n") + "\n</ul>"
	} else {
		inner = strings.Join(paragraphs, "\n")
	}

	// Wrap in a positioned div if we have position data
	if posStyle != "" {
		// Compute padding from bodyPr insets (default: 91440 EMU ≈ 0.1 inch)
		padding := pptxBodyPadding(shape)
		return fmt.Sprintf(`<div style="%s;display:flex;align-items:center;overflow:hidden;%s">%s</div>`, posStyle, padding, inner)
	}
	return inner
}

// pptxBodyPadding returns a CSS padding string derived from OOXML bodyPr insets.
// OOXML defaults: lIns=91440, tIns=45720, rIns=91440, bIns=45720 EMU.
// When insets are explicitly 0, we still add a small minimum padding (0.4%)
// to prevent text from touching the edge of overlapping background shapes.
func pptxBodyPadding(shape pptShape) string {
	const (
		defLR   int64 = 91440  // default left/right inset EMU
		defTB   int64 = 45720  // default top/bottom inset EMU
		minPad        = 0.4    // minimum padding percentage
	)

	lIns, tIns, rIns, bIns := defLR, defTB, defLR, defTB
	if shape.TxBody != nil && shape.TxBody.BodyPr != nil {
		bp := shape.TxBody.BodyPr
		if bp.LIns != nil {
			lIns = *bp.LIns
		}
		if bp.TIns != nil {
			tIns = *bp.TIns
		}
		if bp.RIns != nil {
			rIns = *bp.RIns
		}
		if bp.BIns != nil {
			bIns = *bp.BIns
		}
	}

	// Convert EMU to percentage of slide dimensions, with minimum
	lPct := max(float64(lIns)/pptxSlideWidthEMU*100, minPad)
	rPct := max(float64(rIns)/pptxSlideWidthEMU*100, minPad)
	tPct := max(float64(tIns)/pptxSlideHeightEMU*100, minPad)
	bPct := max(float64(bIns)/pptxSlideHeightEMU*100, minPad)

	return fmt.Sprintf("padding:%.2f%% %.2f%% %.2f%% %.2f%%", tPct, rPct, bPct, lPct)
}

// pptxExtractRunText converts text runs to HTML with inline styling.
func pptxExtractRunText(runs []pptRun) string {
	var parts []string
	for _, run := range runs {
		text := run.Text
		if run.RPr == nil {
			parts = append(parts, text)
			continue
		}

		var styles []string
		if run.RPr.SolidFill != nil && run.RPr.SolidFill.SrgbClr != nil {
			styles = append(styles, fmt.Sprintf("color:#%s", run.RPr.SolidFill.SrgbClr.Val))
		}
		if run.RPr.Sz > 0 {
			emSize := float64(run.RPr.Sz) / 100.0 / pptxBaseFontPt
			styles = append(styles, fmt.Sprintf("font-size:%.2fem", emSize))
		}
		if run.RPr.Spc != 0 {
			spacing := float64(run.RPr.Spc) / 100.0
			styles = append(styles, fmt.Sprintf("letter-spacing:%.1fpt", spacing))
		}
		if run.RPr.B == 1 {
			styles = append(styles, "font-weight:bold")
		}
		if run.RPr.I == 1 {
			styles = append(styles, "font-style:italic")
		}

		if len(styles) > 0 {
			text = fmt.Sprintf(`<span style="%s">%s</span>`, strings.Join(styles, ";"), text)
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "")
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