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

	slide, err := s.SlideService.Create(user.UserID, title, slug, content, isPublished, categoryIDs, description, metadata, password)
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

	// Create version snapshot before updating
	if s.SlideVersionService != nil {
		_ = s.SlideVersionService.MaybeCreateVersion(slideID, user.UserID, title, content)
	}

	err = s.SlideService.Update(slideID, title, slug, content, isPublished, categoryIDs, description, metadata, password)
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
	slide, err := s.SlideService.Create(user.UserID, title, slug, content, false, nil, "", "{}", "")
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
		html := parseSlideXML(sf.file)
		sections = append(sections, fmt.Sprintf("<section>\n%s\n</section>", html))
	}

	if len(sections) == 0 {
		sections = append(sections, "<section>\n<h1>Imported Presentation</h1>\n<p>No slides could be extracted.</p>\n</section>")
	}

	return sections, media
}

// OOXML types for parsing slide content
type pptSlide struct {
	CSld struct {
		SpTree struct {
			Shapes []pptShape `xml:"sp"`
		} `xml:"spTree"`
	} `xml:"cSld"`
}

type pptShape struct {
	NvSpPr struct {
		NvPr struct {
			Ph *struct {
				Type string `xml:"type,attr"`
				Idx  string `xml:"idx,attr"`
			} `xml:"ph"`
		} `xml:"nvPr"`
	} `xml:"nvSpPr"`
	TxBody *struct {
		Paragraphs []pptParagraph `xml:"p"`
	} `xml:"txBody"`
}

type pptParagraph struct {
	PPr *struct {
		BuNone *struct{} `xml:"buNone"`
		BuChar *struct {
			Char string `xml:"char,attr"`
		} `xml:"buChar"`
		BuAutoNum *struct{} `xml:"buAutoNum"`
		Lvl       int       `xml:"lvl,attr"`
	} `xml:"pPr"`
	Runs []pptRun `xml:"r"`
}

type pptRun struct {
	RPr *struct {
		B int `xml:"b,attr"`
		I int `xml:"i,attr"`
	} `xml:"rPr"`
	Text string `xml:"t"`
}

func parseSlideXML(f *zip.File) string {
	rc, err := f.Open()
	if err != nil {
		return "<p>Failed to parse slide</p>"
	}
	defer rc.Close()

	var slide pptSlide
	if err := xml.NewDecoder(rc).Decode(&slide); err != nil {
		return "<p>Failed to parse slide</p>"
	}

	var parts []string

	for _, shape := range slide.CSld.SpTree.Shapes {
		if shape.TxBody == nil {
			continue
		}

		// Determine shape type from placeholder
		phType := ""
		if shape.NvSpPr.NvPr.Ph != nil {
			phType = shape.NvSpPr.NvPr.Ph.Type
		}

		var paragraphs []string
		isBulletList := false

		for _, para := range shape.TxBody.Paragraphs {
			text := extractRunText(para.Runs)
			if text == "" {
				continue
			}

			// Check for bullet points
			hasBullet := para.PPr != nil && para.PPr.BuChar != nil
			hasAutoNum := para.PPr != nil && para.PPr.BuAutoNum != nil

			if hasBullet || hasAutoNum {
				isBulletList = true
				paragraphs = append(paragraphs, fmt.Sprintf("<li>%s</li>", text))
			} else if phType == "title" || phType == "ctrTitle" {
				paragraphs = append(paragraphs, fmt.Sprintf("<h1>%s</h1>", text))
			} else if phType == "subTitle" {
				paragraphs = append(paragraphs, fmt.Sprintf("<h3>%s</h3>", text))
			} else {
				paragraphs = append(paragraphs, fmt.Sprintf("<p>%s</p>", text))
			}
		}

		if isBulletList && len(paragraphs) > 0 {
			parts = append(parts, "<ul>\n"+strings.Join(paragraphs, "\n")+"\n</ul>")
		} else {
			parts = append(parts, strings.Join(paragraphs, "\n"))
		}
	}

	if len(parts) == 0 {
		return "<p></p>"
	}
	return strings.Join(parts, "\n")
}

func extractRunText(runs []pptRun) string {
	var parts []string
	for _, run := range runs {
		text := run.Text
		if run.RPr != nil {
			if run.RPr.B == 1 {
				text = "<strong>" + text + "</strong>"
			}
			if run.RPr.I == 1 {
				text = "<em>" + text + "</em>"
			}
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