package controllers

import (
	"encoding/json"
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

// Helper function to detect AJAX requests
func isAjaxRequest(r *http.Request) bool {
	// Check for explicit AJAX headers
	if r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
		return true
	}
	
	// Check Accept header for JSON
	accept := strings.ToLower(r.Header.Get("Accept"))
	if strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/html") {
		return true
	}
	
	// Check Content-Type for JSON
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.Contains(contentType, "application/json") {
		return true
	}
	
	// Default to false (treat as regular form submission)
	return false
}

type Categories struct {
	CategoryService *models.CategoryService
	PostService     *models.PostService
	SlideService    *models.SlideService
	SessionService  *models.SessionService
	Templates       struct {
		Manage  views.Template
		TagPage views.Template
	}
}

// TagPage displays all posts and slides for a given tag/category name.
func (c *Categories) TagPage(w http.ResponseWriter, r *http.Request) {
	tagName := chi.URLParam(r, "name")
	if tagName == "" {
		http.NotFound(w, r)
		return
	}

	category, err := c.CategoryService.GetByName(tagName)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	posts, err := c.PostService.GetPublishedPostsByCategory(category.ID)
	if err != nil {
		log.Printf("Failed to get posts for tag %q: %v", tagName, err)
		posts = nil
	}

	slides, err := c.SlideService.GetPublishedSlidesByCategory(category.ID)
	if err != nil {
		log.Printf("Failed to get slides for tag %q: %v", tagName, err)
		slides = nil
	}

	user, _ := utils.IsUserLoggedIn(r, c.SessionService)

	var data struct {
		TagName         string
		Posts           []models.Post
		Slides          []models.Slide
		PostCount       int
		SlideCount      int
		TotalCount      int
		LoggedIn        bool
		IsAdmin         bool
		Username        string
		Description     string
		CurrentPage     string
		UserPermissions models.UserPermissions
	}
	data.TagName = category.Name
	data.Posts = posts
	data.Slides = slides
	data.PostCount = len(posts)
	data.SlideCount = len(slides)
	data.TotalCount = len(posts) + len(slides)
	data.Description = fmt.Sprintf("Posts and presentations tagged with %q", category.Name)
	data.CurrentPage = "tags"

	if user != nil {
		data.LoggedIn = true
		data.Username = user.Username
		data.IsAdmin = models.IsAdmin(user.Role)
		data.UserPermissions = models.GetPermissions(user.Role)
	}

	c.Templates.TagPage.Execute(w, r, data)
}

// Admin Category Management Page
func (c *Categories) Manage(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, c.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	// Check if user can view admin panel (admin or editor)
	if !models.CanViewAdminPanel(user.Role) {
		http.Error(w, errForbiddenAdmin, http.StatusForbidden)
		return
	}

	// Get all categories
	categories, err := c.CategoryService.GetAll()
	if err != nil {
		log.Printf("Error getting categories: %v", err)
		http.Error(w, "Failed to load categories", http.StatusInternalServerError)
		return
	}

	// Get post counts for each category
	postCounts, err := c.CategoryService.GetPostCountByCategory()
	if err != nil {
		log.Printf("Error getting post counts: %v", err)
		postCounts = make(map[int]int) // Fallback to empty map
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
		Categories      []models.Category
		PostCounts      map[int]int
		Flash           string
		UserPermissions models.UserPermissions
	}{
		Email:           user.Email,
		LoggedIn:        true,
		Username:        user.Username,
		IsAdmin:         models.IsAdmin(user.Role),
		SignupDisabled:  true, // Default for admin pages
		Description:     "Manage Categories - Anshuman Biswas Blog",
		CurrentPage:     "admin-categories",
		User:            user,
		Categories:      categories,
		PostCounts:      postCounts,
		Flash:           "",
		UserPermissions: models.GetPermissions(user.Role),
	}

	// Check for flash messages
	if msg := r.URL.Query().Get("message"); msg != "" {
		data.Flash = msg
	}

	c.Templates.Manage.Execute(w, r, data)
}

// REST API Endpoints

// ListCategories - GET /api/categories
func (c *Categories) ListCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := c.CategoryService.GetAll()
	if err != nil {
		log.Printf("Error getting categories: %v", err)
		http.Error(w, "Failed to get categories", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(categories)
}

// GetCategory - GET /api/categories/{id}
func (c *Categories) GetCategory(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid category ID", http.StatusBadRequest)
		return
	}

	category, err := c.CategoryService.GetByID(id)
	if err != nil {
		log.Printf("Error getting category: %v", err)
		http.Error(w, "Category not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(category)
}

// CreateCategory - POST /api/categories
func (c *Categories) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	category, err := c.CategoryService.Create(req.Name)
	if err != nil {
		log.Printf("Error creating category: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create category: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(category)
}

// UpdateCategory - PUT /api/categories/{id}
func (c *Categories) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid category ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	category, err := c.CategoryService.Update(id, req.Name)
	if err != nil {
		log.Printf("Error updating category: %v", err)
		if err.Error() == "category not found" {
			http.Error(w, "Category not found", http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Failed to update category: %v", err), http.StatusBadRequest)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(category)
}

// DeleteCategory - DELETE /api/categories/{id}
func (c *Categories) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid category ID", http.StatusBadRequest)
		return
	}

	err = c.CategoryService.Delete(id)
	if err != nil {
		log.Printf("Error deleting category: %v", err)
		if err.Error() == "category not found" {
			http.Error(w, "Category not found", http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Failed to delete category: %v", err), http.StatusBadRequest)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// requireAdminOrRedirect checks that the request is authenticated and from an admin
// or editor user. On failure it writes the appropriate HTTP response and returns (nil, false).
func requireAdminOrRedirect(w http.ResponseWriter, r *http.Request, ss *models.SessionService, ajax bool) (*models.User, bool) {
	user, err := utils.IsUserLoggedIn(r, ss)
	if err != nil || user == nil {
		if ajax {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		} else {
			http.Redirect(w, r, "/signin", http.StatusFound)
		}
		return nil, false
	}
	if !models.CanViewAdminPanel(user.Role) {
		http.Error(w, errForbiddenAdmin, http.StatusForbidden)
		return nil, false
	}
	return user, true
}

// Form-based endpoints for web interface

// CreateCategoryForm - POST /admin/categories
func (c *Categories) CreateCategoryForm(w http.ResponseWriter, r *http.Request) {
	ajax := isAjaxRequest(r)
	if _, ok := requireAdminOrRedirect(w, r, c.SessionService, ajax); !ok {
		return
	}

	var name string
	if isAjaxRequest(r) {
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}
		name = strings.TrimSpace(req.Name)
	} else {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}
		name = r.Form.Get("name")
	}

	if name == "" {
		if isAjaxRequest(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Category name is required"})
			return
		}
		http.Redirect(w, r, "/admin/categories?message=Category+name+is+required", http.StatusFound)
		return
	}

	category, err := c.CategoryService.Create(name)
	if err != nil {
		log.Printf("Error creating category: %v", err)
		if isAjaxRequest(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create category"})
			return
		}
		http.Redirect(w, r, "/admin/categories?message=Failed+to+create+category", http.StatusFound)
		return
	}

	if isAjaxRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(category)
		return
	}
	http.Redirect(w, r, "/admin/categories?message=Category+created+successfully", http.StatusFound)
}

// UpdateCategoryForm - POST /admin/categories/{id}
func (c *Categories) UpdateCategoryForm(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	user, err := utils.IsUserLoggedIn(r, c.SessionService)
	if err != nil || user == nil {
		if isAjaxRequest(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}
	// Check if user is admin
	if !models.IsAdmin(user.Role) {
		if isAjaxRequest(r) {
			http.Error(w, errForbiddenAdmin, http.StatusForbidden)
			return
		}
		http.Error(w, errForbiddenAdmin, http.StatusForbidden)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid category ID", http.StatusBadRequest)
		return
	}

	var name string
	if isAjaxRequest(r) {
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}
		name = strings.TrimSpace(req.Name)
	} else {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}
		name = r.Form.Get("name")
	}

	if name == "" {
		if isAjaxRequest(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Category name is required"})
			return
		}
		http.Redirect(w, r, "/admin/categories?message=Category+name+is+required", http.StatusFound)
		return
	}

	category, err := c.CategoryService.Update(id, name)
	if err != nil {
		log.Printf("Error updating category: %v", err)
		if isAjaxRequest(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update category"})
			return
		}
		http.Redirect(w, r, "/admin/categories?message=Failed+to+update+category", http.StatusFound)
		return
	}

	if isAjaxRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(category)
		return
	}
	http.Redirect(w, r, "/admin/categories?message=Category+updated+successfully", http.StatusFound)
}

// DeleteCategoryForm - POST /admin/categories/{id}/delete
func (c *Categories) DeleteCategoryForm(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	user, err := utils.IsUserLoggedIn(r, c.SessionService)
	if err != nil || user == nil {
		if isAjaxRequest(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}
	// Check if user is admin
	if !models.IsAdmin(user.Role) {
		if isAjaxRequest(r) {
			http.Error(w, errForbiddenAdmin, http.StatusForbidden)
			return
		}
		http.Error(w, errForbiddenAdmin, http.StatusForbidden)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid category ID", http.StatusBadRequest)
		return
	}

	err = c.CategoryService.Delete(id)
	if err != nil {
		log.Printf("Error deleting category: %v", err)
		if isAjaxRequest(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to delete category"})
			return
		}
		http.Redirect(w, r, "/admin/categories?message=Failed+to+delete+category", http.StatusFound)
		return
	}

	if isAjaxRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "Category deleted successfully"})
		return
	}
	http.Redirect(w, r, "/admin/categories?message=Category+deleted+successfully", http.StatusFound)
}