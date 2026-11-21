package controllers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
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
		AdminSlides   views.Template
		SlideEditor   views.Template
		SlidesList    views.Template
		SlidePassword views.Template
		SlideDeck     views.Template
	}
	SlideService        *models.SlideService
	SlideVersionService *models.SlideVersionService
	SessionService      *models.SessionService
	CategoryService     *models.CategoryService
	APITokenService     *models.APITokenService
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

	// Multipart when the new-deck form carries a .pptx; ParseMultipartForm also parses plain forms.
	if err := r.ParseMultipartForm(32 << 20); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	slug := r.FormValue("slug")
	isPublishedStr := r.FormValue("is_published")
	categoriesStr := r.FormValue("categories")

	// Every slide is a DeckCraft deck: its slides live in its file, optionally a .pptx uploaded here.
	if title == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}
	deckName, deckData, err := uploadedDeck(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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

	slide, err := s.SlideService.Create(user.UserID, title, slug, "", isPublished, categoryIDs, description, metadata, password, featuredImageURL)
	if err != nil {
		log.Printf("Error creating slide: %v", err)
		http.Error(w, "Failed to create slide", http.StatusInternalServerError)
		return
	}

	if err := s.SlideService.SaveDeck(slide.ID, deckName, deckData); err != nil {
		log.Printf("Error marking slide %d as a deck: %v", slide.ID, err)
		http.Error(w, "Failed to create deck", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/slides/%d/edit", slide.ID), http.StatusFound)
}

// EditSlide displays the DeckCraft editor and the deck's settings.
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
	isPublishedStr := r.FormValue("is_published")
	categoriesStr := r.FormValue("categories")

	existing, err := s.SlideService.GetByID(slideID)
	if err != nil {
		http.Error(w, "Slide not found", http.StatusNotFound)
		return
	}
	if title == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}
	// The form edits settings only. A converted deck keeps its old HTML, which site search indexes.
	content := string(existing.ContentHTML)

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

	// Published/editor and password checks.
	visible, unlocked := s.slideGate(r, slide, user)
	if !visible {
		http.Error(w, "Slide not found", http.StatusNotFound)
		return
	}
	if !unlocked {
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

	s.Templates.SlideDeck.Execute(w, r, data)
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

// authSlideEditor returns a user with edit-slide permission, accepting
// either a session cookie or a Bearer token. Two Bearer paths, mirroring
// the fallback in users.UploadImage:
//  1. API_TOKEN env var (legacy / deploy automation)
//  2. A user-issued token in the api_tokens table, validated via
//     APITokenService — returns the actual issuing user so version
//     history credits the right person.
func (s Slides) authSlideEditor(r *http.Request) *models.User {
	if user, err := s.isUserLoggedIn(r); err == nil && models.CanEditSlides(user.Role) {
		return user
	}
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return nil
	}
	token := strings.TrimPrefix(h, "Bearer ")
	if env := os.Getenv("API_TOKEN"); env != "" && token == env {
		return &models.User{Role: models.RoleAdministrator}
	}
	if s.APITokenService != nil {
		if u, err := s.APITokenService.ValidateToken(token); err == nil && u != nil && models.CanEditSlides(u.Role) {
			return u
		}
	}
	return nil
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
