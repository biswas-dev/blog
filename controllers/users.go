package controllers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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

	"html/template"

	"anshumanbiswas.com/blog/models"
	"anshumanbiswas.com/blog/utils"
	"github.com/go-chi/chi/v5"
)

func (u Users) New(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Email            string
		Username         string
		LoggedIn         bool
		IsSignupDisabled bool
		SignupDisabled   bool
		IsAdmin          bool
		Description      string
		CurrentPage      string
		UserPermissions  models.UserPermissions
	}

	data.Email = r.FormValue("email")
	data.LoggedIn = false
	data.IsSignupDisabled = false
	data.SignupDisabled = false
	data.IsAdmin = false
	data.Description = "Sign up for Anshuman Biswas Blog"
	data.CurrentPage = "signup"
	data.UserPermissions = models.GetPermissions(models.RoleCommenter)
	u.Templates.New.Execute(w, r, data)
}

func (u Users) Disabled(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Email            string
		LoggedIn         bool
		IsSignupDisabled bool
		SignupDisabled   bool
		IsAdmin          bool
		Description      string
		CurrentPage      string
		Username         string
		UserPermissions  models.UserPermissions
	}
	data.Email = r.FormValue("email")
	data.LoggedIn = false
	data.IsSignupDisabled = true
	data.SignupDisabled = true
	data.IsAdmin = false
	data.Description = "Sign up disabled - Anshuman Biswas Blog"
	data.CurrentPage = "signup"
	data.Username = ""
	data.UserPermissions = models.GetPermissions(models.RoleCommenter)
	u.Templates.New.Execute(w, r, data)
}

type Users struct {
	Templates struct {
		New        Template
		SignIn     Template
		Home       Template
		LoggedIn   Template
		Profile    Template
		AdminPosts Template
		UserPosts  Template
		APIAccess  Template
		PostEditor Template
	}
	UserService     *models.UserService
	SessionService  *models.SessionService
	PostService     *models.PostService
	APITokenService *models.APITokenService
	CategoryService *models.CategoryService
}

// UploadImage handles image uploads (cover or inline). Returns JSON {url}
func (u Users) UploadImage(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !models.CanEditPosts(user.Role) && !models.IsAdmin(user.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseMultipartForm(20 << 20); err != nil { // 20MB
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "File required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate type
	buff := make([]byte, 512)
	n, _ := file.Read(buff)
	filetype := http.DetectContentType(buff[:n])
	allowed := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif", "image/webp": ".webp", "image/svg+xml": ".svg"}
	ext, ok := allowed[filetype]
	if !ok {
		// fallback to extension from filename if content-type sniff fails
		ext = strings.ToLower(filepath.Ext(header.Filename))
		ok = ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp" || ext == ".svg"
		if !ok {
			http.Error(w, "Unsupported file type", http.StatusBadRequest)
			return
		}
		if ext == ".jpeg" {
			ext = ".jpg"
		}
	}
	// rewind
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		http.Error(w, "Unable to read file", http.StatusInternalServerError)
		return
	}

	// Optional type and per-slug folder
	uploadType := strings.ToLower(r.URL.Query().Get("type")) // e.g., "featured"
	slug := r.URL.Query().Get("slug")
	slug = strings.ToLower(slug)
	slug = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(slug, "-")

	// Random filename to avoid collisions
	rb := make([]byte, 16)
	if _, err := rand.Read(rb); err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	name := hex.EncodeToString(rb) + ext

	// Ensure upload directory exists
	base := filepath.Join("static", "uploads")
	if uploadType == "featured" {
		// Store featured images under featured/{slug}/ when slug is available
		base = filepath.Join(base, "featured")
		if slug != "" {
			base = filepath.Join(base, slug)
		}
	} else if slug != "" {
		// For post-specific uploads, use /uploads/post/{slug}/
		base = filepath.Join(base, "post", slug)
	}
	_ = os.MkdirAll(base, 0o755)
	fpath := filepath.Join(base, name)
	out, err := os.Create(fpath)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	urlBase := "/static/uploads"
	if uploadType == "featured" {
		urlBase += "/featured"
		if slug != "" {
			urlBase += "/" + slug
		}
	} else if slug != "" {
		urlBase += "/post/" + slug
	}
	url := urlBase + "/" + name
	resp := map[string]string{"url": url}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// UploadMultipleImages handles multiple image uploads. Returns JSON {uploads: [{url, filename, size}]}
func (u Users) UploadMultipleImages(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !models.CanEditPosts(user.Role) && !models.IsAdmin(user.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseMultipartForm(100 << 20); err != nil { // 100MB total
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "No files provided", http.StatusBadRequest)
		return
	}

	// Optional type and per-slug folder
	uploadType := strings.ToLower(r.URL.Query().Get("type")) // e.g., "featured"
	slug := r.URL.Query().Get("slug")
	slug = strings.ToLower(slug)
	slug = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(slug, "-")

	var uploads []map[string]interface{}
	var errors []string

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to open %s: %v", fileHeader.Filename, err))
			continue
		}
		defer file.Close()

		// Validate type
		buff := make([]byte, 512)
		n, _ := file.Read(buff)
		filetype := http.DetectContentType(buff[:n])
		allowed := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif", "image/webp": ".webp", "image/svg+xml": ".svg"}
		ext, ok := allowed[filetype]
		if !ok {
			// fallback to extension from filename if content-type sniff fails
			ext = strings.ToLower(filepath.Ext(fileHeader.Filename))
			ok = ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp" || ext == ".svg"
			if !ok {
				errors = append(errors, fmt.Sprintf("Unsupported file type for %s", fileHeader.Filename))
				continue
			}
			if ext == ".jpeg" {
				ext = ".jpg"
			}
		}

		// rewind
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			errors = append(errors, fmt.Sprintf("Unable to read file %s", fileHeader.Filename))
			continue
		}

		// Random filename to avoid collisions
		rb := make([]byte, 16)
		if _, err := rand.Read(rb); err != nil {
			errors = append(errors, fmt.Sprintf("Internal error for %s", fileHeader.Filename))
			continue
		}
		name := hex.EncodeToString(rb) + ext

		// Ensure upload directory exists
		base := filepath.Join("static", "uploads")
		if uploadType == "featured" {
			base = filepath.Join(base, "featured")
		} else if slug != "" {
			// For post-specific uploads, use /uploads/post/{slug}/
			base = filepath.Join(base, "post", slug)
		}
		_ = os.MkdirAll(base, 0o755)
		fpath := filepath.Join(base, name)
		out, err := os.Create(fpath)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to save %s", fileHeader.Filename))
			continue
		}
		defer out.Close()
		if _, err := io.Copy(out, file); err != nil {
			errors = append(errors, fmt.Sprintf("Failed to save %s", fileHeader.Filename))
			continue
		}

		urlBase := "/static/uploads"
		if uploadType == "featured" {
			urlBase += "/featured"
		} else if slug != "" {
			urlBase += "/post/" + slug
		}
		url := urlBase + "/" + name

		uploads = append(uploads, map[string]interface{}{
			"url":      url,
			"filename": fileHeader.Filename,
			"size":     fileHeader.Size,
		})
	}

	resp := map[string]interface{}{
		"uploads": uploads,
		"success": len(uploads),
		"total":   len(files),
	}
	if len(errors) > 0 {
		resp["errors"] = errors
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// PreviewRender returns rendered HTML for editor preview using server pipeline
func (u Users) PreviewRender(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil || (!models.CanEditPosts(user.Role) && !models.IsAdmin(user.Role)) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}
	content := r.FormValue("content")
	html := models.RenderContent(content)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"html": html})
}

// CreatePostFromFile creates a blog post from a file (API endpoint)
func (u Users) CreatePostFromFile(w http.ResponseWriter, r *http.Request) {
	// This endpoint is used via API middleware, so we don't need to check user login
	// The API middleware handles authentication

	if err := r.ParseMultipartForm(50 << 20); err != nil { // 50MB
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "File required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// Get form parameters
	title := r.FormValue("title")
	if title == "" {
		title = strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	}

	userIDStr := r.FormValue("user_id")
	userID, _ := strconv.Atoi(userIDStr)
	if userID == 0 {
		userID = 2 // Default to admin user
	}

	categoryIDStr := r.FormValue("category_id")
	categoryID, _ := strconv.Atoi(categoryIDStr)
	if categoryID == 0 {
		categoryID = 1 // Default category
	}

	isPublished := r.FormValue("is_published") == "true"
	featured := r.FormValue("featured") == "true"
	featuredImageURL := r.FormValue("featured_image_url")

	// Generate slug from title
	slug := strings.ToLower(title)
	slug = regexp.MustCompile(`[^a-z0-9\s-]`).ReplaceAllString(slug, "")
	slug = regexp.MustCompile(`\s+`).ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	// Create post
	post, err := u.PostService.Create(userID, categoryID, title, string(content), isPublished, featured, featuredImageURL, slug)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create post: %v", err), http.StatusInternalServerError)
		return
	}

	resp := map[string]interface{}{
		"id":      post.ID,
		"title":   post.Title,
		"slug":    post.Slug,
		"url":     fmt.Sprintf("/blog/%s", post.Slug),
		"success": true,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// ListUploadedImages returns previously uploaded images for selection
func (u Users) ListUploadedImages(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !models.CanEditPosts(user.Role) && !models.IsAdmin(user.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Get optional slug filter and type
	slug := r.URL.Query().Get("slug")
	uploadType := strings.ToLower(r.URL.Query().Get("type")) // e.g., "featured"

	// Build directory path
	baseDir := "static/uploads"
	if uploadType == "featured" {
		baseDir = filepath.Join(baseDir, "featured")
	} else if slug != "" {
		// For post-specific uploads, use /uploads/post/{slug}/
		baseDir = filepath.Join(baseDir, "post", slug)
	}

	var images []map[string]interface{}

	// Walk through upload directory
	if _, err := os.Stat(baseDir); err == nil {
		_ = filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}
			if info.IsDir() {
				return nil
			}

			ext := strings.ToLower(filepath.Ext(info.Name()))
			if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp" {
				// Convert local path to URL served under /static
				relPath := strings.TrimPrefix(path, "static/")
				url := "/static/" + strings.ReplaceAll(relPath, "\\", "/")

				images = append(images, map[string]interface{}{
					"url":      url,
					"filename": info.Name(),
					"size":     info.Size(),
					"modified": info.ModTime().Unix(),
				})
			}
			return nil
		})
	}

	// Sort by modification time (newest first)
	sort.Slice(images, func(i, j int) bool {
		return images[i]["modified"].(int64) > images[j]["modified"].(int64)
	})

	// Only limit to 50 images when viewing all images (not post-specific)
	// For post-specific views, show all images so users can manage them
	if slug == "" && len(images) > 50 {
		images = images[:50]
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"images": images,
		"total":  len(images),
	})
}

func (u Users) GetTopPosts() (*models.PostsList, error) {
	return u.PostService.GetTopPosts()
}

func (u Users) Home(w http.ResponseWriter, r *http.Request) {

	var data struct {
		Email           string
		LoggedIn        bool
		Posts           *models.PostsList
		Username        string
		IsAdmin         bool
		SignupDisabled  bool
		Description     string
		CurrentPage     string
		UserPermissions models.UserPermissions
	}

	posts, _ := u.GetTopPosts()

	// Get signup disabled setting from environment
	isSignupDisabled, _ := strconv.ParseBool(os.Getenv("APP_DISABLE_SIGNUP"))

	user, err := u.isUserLoggedIn(r)
	if err != nil {
		fmt.Printf("DEBUG HOME: User not logged in, error: %v\n", err)
		fmt.Printf("DEBUG HOME: Session token from cookie: %v\n", func() string {
			if cookie, err := r.Cookie("session"); err == nil {
				return cookie.Value
			}
			return "NO_COOKIE"
		}())
		data.LoggedIn = false
		data.Posts = posts
		data.SignupDisabled = isSignupDisabled
		data.Description = "Engineering Insights - Anshuman Biswas Blog"
		data.CurrentPage = "home"
		data.Username = ""
		data.IsAdmin = false
		data.Email = ""
		data.UserPermissions = models.GetPermissions(models.RoleCommenter)
		u.Templates.Home.Execute(w, r, data)
		return
	}

	data.Email = user.Email
	data.Username = user.Username
	data.LoggedIn = true
	data.Posts = posts
	data.IsAdmin = models.IsAdmin(user.Role)
	fmt.Printf("DEBUG HOME: User logged in: %s, Email: %s, Role: %d, IsAdmin: %v\n", user.Username, user.Email, user.Role, data.IsAdmin)
	data.SignupDisabled = isSignupDisabled
	data.Description = "Engineering Insights - Anshuman Biswas Blog"
	data.CurrentPage = "home"
	data.UserPermissions = models.GetPermissions(user.Role)
	u.Templates.Home.Execute(w, r, data)
}

func (u Users) LoadMorePosts(w http.ResponseWriter, r *http.Request) {
	// Get offset from query parameters
	offsetStr := r.URL.Query().Get("offset")
	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		offset = 0
	}

	// Get posts with pagination (5 posts at a time)
	posts, err := u.PostService.GetTopPostsWithPagination(5, offset)
	if err != nil {
		http.Error(w, "Failed to load posts", http.StatusInternalServerError)
		return
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(posts)
}

func (u Users) SignIn(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Email           string
		LoggedIn        bool
		SignupDisabled  bool
		IsAdmin         bool
		Description     string
		CurrentPage     string
		Username        string
		UserPermissions models.UserPermissions
	}
	data.Email = r.FormValue("email")
	data.LoggedIn = false
	data.SignupDisabled, _ = strconv.ParseBool(os.Getenv("APP_DISABLE_SIGNUP"))
	data.IsAdmin = false
	data.Description = "Sign in to Anshuman Biswas Blog"
	data.CurrentPage = "signin"
	data.Username = ""
	data.UserPermissions = models.GetPermissions(models.RoleCommenter)
	u.Templates.SignIn.Execute(w, r, data)
}

func (u Users) ProcessSignIn(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Email    string
		Password string
	}
	data.Email = r.FormValue("email")
	data.Password = r.FormValue("password")

	user, err := u.UserService.Authenticate(data.Email, data.Password)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Something went wrong.", http.StatusInternalServerError)
		return
	}
	session, err := u.SessionService.Create(user.UserID)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Something went wrong.", http.StatusInternalServerError)
		return
	}
	setCookie(w, CookieSession, session.Token)
	setCookie(w, CookieUserEmail, data.Email)

	http.Redirect(w, r, "/", http.StatusFound)
}

func (u Users) Create(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	username := r.FormValue("username")
	password := r.FormValue("password")
	fmt.Printf("[Creating user: %s/%s]", email, username)
	user, err := u.UserService.Create(email, username, password, 1)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Something went wrong.", http.StatusInternalServerError)
		return
	}
	session, err := u.SessionService.Create(user.UserID)
	if err != nil {
		fmt.Println(err)
		// TODO: Long term, we should show a warning about not being able to sign the user in.
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}
	setCookie(w, CookieSession, session.Token)
	setCookie(w, CookieUserEmail, email)

	http.Redirect(w, r, "/", http.StatusFound)
}

// DeleteImage handles deletion of uploaded images
func (u Users) DeleteImage(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !models.CanEditPosts(user.Role) && !models.IsAdmin(user.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get image path from query parameters
	imagePath := r.URL.Query().Get("path")
	if imagePath == "" {
		http.Error(w, "Image path required", http.StatusBadRequest)
		return
	}

	// Security check: ensure path is within uploads directory
	if !strings.HasPrefix(imagePath, "/static/uploads/") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// Convert URL path to file system path
	filePath := filepath.Join("static", strings.TrimPrefix(imagePath, "/static/"))

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "Image not found", http.StatusNotFound)
		return
	}

	// Delete the file
	if err := os.Remove(filePath); err != nil {
		http.Error(w, "Failed to delete image", http.StatusInternalServerError)
		return
	}

	resp := map[string]string{"status": "deleted", "path": imagePath}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (u Users) isUserLoggedIn(r *http.Request) (*models.User, error) {
	return utils.IsUserLoggedIn(r, u.SessionService)
}

func (u Users) CurrentUser(w http.ResponseWriter, r *http.Request) {

	user, err := u.isUserLoggedIn(r)
	if err != nil {
		fmt.Println(err)
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	var data struct {
		Email           string
		LoggedIn        bool
		Username        string
		IsAdmin         bool
		SignupDisabled  bool
		Description     string
		CurrentPage     string
		Message         string
		UserPermissions models.UserPermissions
	}

	data.Email = user.Email
	data.Username = user.Username
	data.LoggedIn = true
	data.IsAdmin = models.IsAdmin(user.Role)
	data.SignupDisabled, _ = strconv.ParseBool(os.Getenv("APP_DISABLE_SIGNUP"))
	data.Description = "Profile Management - Anshuman Biswas Blog"
	data.CurrentPage = "profile"
	data.Message = r.URL.Query().Get("message")
	data.UserPermissions = models.GetPermissions(user.Role)

	u.Templates.Profile.Execute(w, r, data)
}

func (u Users) Logout(w http.ResponseWriter, r *http.Request) {

	email, err := readCookie(r, CookieUserEmail)
	if err != nil {
		fmt.Println(err)
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	u.SessionService.Logout(email)

	deleteCookie(w, CookieSession, "XXXXXX")
	deleteCookie(w, CookieUserEmail, "XXXXXXX")

	http.Redirect(w, r, "/", http.StatusFound)

}

func (u Users) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := u.UserService.GetAllUsers()
	if err != nil {
		http.Error(w, "Failed to fetch users", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func (u Users) CreateUser(w http.ResponseWriter, r *http.Request) {
	var newUser models.User

	// Parse request body
	err := json.NewDecoder(r.Body).Decode(&newUser)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Create user
	user, err := u.UserService.Create(newUser.Email, newUser.Username, newUser.Password, newUser.Role)
	if err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	// Return created user
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (u Users) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	currentPassword := r.FormValue("current_password")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	if newPassword != confirmPassword {
		http.Redirect(w, r, "/users/me?message=Passwords do not match", http.StatusFound)
		return
	}

	// Verify current password
	_, err = u.UserService.Authenticate(user.Email, currentPassword)
	if err != nil {
		http.Redirect(w, r, "/users/me?message=Current password is incorrect", http.StatusFound)
		return
	}

	// Update password
	err = u.UserService.UpdatePassword(user.UserID, newPassword)
	if err != nil {
		http.Redirect(w, r, "/users/me?message=Failed to update password", http.StatusFound)
		return
	}

	http.Redirect(w, r, "/users/me?message=Password updated successfully", http.StatusFound)
}

func (u Users) UpdateEmail(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	newEmail := r.FormValue("new_email")
	password := r.FormValue("password")

	// Verify password
	_, err = u.UserService.Authenticate(user.Email, password)
	if err != nil {
		http.Redirect(w, r, "/users/me?message=Password is incorrect", http.StatusFound)
		return
	}

	// Update email
	err = u.UserService.UpdateEmail(user.UserID, newEmail)
	if err != nil {
		http.Redirect(w, r, "/users/me?message=Failed to update email", http.StatusFound)
		return
	}

	// Update cookie with new email
	setCookie(w, CookieUserEmail, newEmail)

	http.Redirect(w, r, "/users/me?message=Email updated successfully", http.StatusFound)
}

// AdminPosts shows all posts for admin users
func (u Users) AdminPosts(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	// Check if user is admin
	if !models.IsAdmin(user.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	posts, err := u.PostService.GetAllPosts()
	if err != nil {
		http.Error(w, "Failed to fetch posts", http.StatusInternalServerError)
		return
	}

	var data struct {
		Email           string
		LoggedIn        bool
		Username        string
		IsAdmin         bool
		SignupDisabled  bool
		Description     string
		CurrentPage     string
		Posts           *models.PostsList
		UserPermissions models.UserPermissions
	}

	data.Email = user.Email
	data.Username = user.Username
	data.LoggedIn = true
	data.IsAdmin = true
	data.SignupDisabled, _ = strconv.ParseBool(os.Getenv("APP_DISABLE_SIGNUP"))
	data.Description = "Manage All Posts - Anshuman Biswas Blog"
	data.CurrentPage = "admin-posts"
	data.Posts = posts
	data.UserPermissions = models.GetPermissions(user.Role)

	u.Templates.AdminPosts.Execute(w, r, data)
}

// UserPosts shows posts for the current user
func (u Users) UserPosts(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	posts, err := u.PostService.GetPostsByUser(user.UserID)
	if err != nil {
		http.Error(w, "Failed to fetch posts", http.StatusInternalServerError)
		return
	}

	var data struct {
		Email           string
		LoggedIn        bool
		Username        string
		IsAdmin         bool
		SignupDisabled  bool
		Description     string
		CurrentPage     string
		Posts           *models.PostsList
		UserPermissions models.UserPermissions
	}

	data.Email = user.Email
	data.Username = user.Username
	data.LoggedIn = true
	data.IsAdmin = (user.Role == 2)
	data.SignupDisabled, _ = strconv.ParseBool(os.Getenv("APP_DISABLE_SIGNUP"))
	data.Description = "My Posts - Anshuman Biswas Blog"
	data.CurrentPage = "my-posts"
	data.Posts = posts
	data.UserPermissions = models.GetPermissions(user.Role)

	u.Templates.UserPosts.Execute(w, r, data)
}

// NewPost renders the editor for creating a post
func (u Users) NewPost(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}
	// Permission: editor or admin
	if !models.CanEditPosts(user.Role) && !models.IsAdmin(user.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Load all categories for the form
	categories, err := u.CategoryService.GetAll()
	if err != nil {
		log.Printf("Error loading categories: %v", err)
		categories = []models.Category{} // fallback to empty slice
	}

	var data struct {
		Email              string
		LoggedIn           bool
		Username           string
		IsAdmin            bool
		SignupDisabled     bool
		Description        string
		CurrentPage        string
		UserPermissions    models.UserPermissions
		Mode               string
		Post               *models.Post
		Categories         []models.Category
		SelectedCategories []int
	}
	data.Email = user.Email
	data.Username = user.Username
	data.LoggedIn = true
	data.IsAdmin = models.IsAdmin(user.Role)
	data.SignupDisabled, _ = strconv.ParseBool(os.Getenv("APP_DISABLE_SIGNUP"))
	data.Description = "Create Post - Anshuman Biswas Blog"
	data.CurrentPage = "admin-posts"
	data.UserPermissions = models.GetPermissions(user.Role)
	data.Mode = "new"
	data.Post = &models.Post{}
	data.Categories = categories
	data.SelectedCategories = []int{} // empty for new posts
	u.Templates.PostEditor.Execute(w, r, data)
}

// CreatePost handles post creation
func (u Users) CreatePost(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}
	if !models.CanEditPosts(user.Role) && !models.IsAdmin(user.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	title := r.FormValue("title")
	content := r.FormValue("content")
	featuredImageURL := r.FormValue("featured_image_url")
	featured := r.FormValue("featured") == "on"
	slug := r.FormValue("slug")
	isPublished := r.FormValue("is_published") == "on"

	// Parse multiple categories
	categoryIDStrings := r.Form["categories"] // Get all category values
	var categoryIDs []int
	for _, idStr := range categoryIDStrings {
		if id, err := strconv.Atoi(idStr); err == nil {
			categoryIDs = append(categoryIDs, id)
		}
	}

	// Ensure at least one category is selected
	if len(categoryIDs) == 0 {
		http.Error(w, "At least one category must be selected", http.StatusBadRequest)
		return
	}

	// Legacy category handling (keep for backward compatibility)
	categoryID, _ := strconv.Atoi(r.FormValue("category_id"))
	if categoryID == 0 {
		categoryID = 1
	}

	if slug == "" {
		// Basic slug from title
		slug = strings.ToLower(title)
		slug = strings.ReplaceAll(slug, " ", "-")
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	post, err := u.PostService.Create(user.UserID, categoryID, title, content, isPublished, featured, featuredImageURL, slug)
	if err != nil {
		http.Error(w, "Failed to create post", http.StatusInternalServerError)
		return
	}

	// Assign categories to the post
	if err := u.CategoryService.AssignCategoriesToPost(post.ID, categoryIDs); err != nil {
		log.Printf("Error assigning categories to post: %v", err)
		// Don't fail the entire request, just log the error
	}

	http.Redirect(w, r, "/admin/posts", http.StatusFound)
}

// EditPost renders the editor for an existing post
func (u Users) EditPost(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}
	if !models.CanEditPosts(user.Role) && !models.IsAdmin(user.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Load the post
	idStr := chi.URLParam(r, "postID")
	id, _ := strconv.Atoi(idStr)
	post, err := u.PostService.GetByID(id)
	if err != nil {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}
	// Ensure ContentHTML for prefill
	post.ContentHTML = template.HTML(post.Content)

	// Load all categories for the form
	categories, err := u.CategoryService.GetAll()
	if err != nil {
		log.Printf("Error loading categories: %v", err)
		categories = []models.Category{} // fallback to empty slice
	}

	// Load existing categories for this post
	postCategories, err := u.CategoryService.GetCategoriesByPostID(id)
	if err != nil {
		log.Printf("Error loading post categories: %v", err)
		postCategories = []models.Category{} // fallback to empty slice
	}

	// Convert to selected category IDs
	selectedCategories := make([]int, len(postCategories))
	for i, cat := range postCategories {
		selectedCategories[i] = cat.ID
	}

	var data struct {
		Email              string
		LoggedIn           bool
		Username           string
		IsAdmin            bool
		SignupDisabled     bool
		Description        string
		CurrentPage        string
		UserPermissions    models.UserPermissions
		Mode               string
		Post               *models.Post
		Categories         []models.Category
		SelectedCategories []int
	}
	data.Email = user.Email
	data.Username = user.Username
	data.LoggedIn = true
	data.IsAdmin = models.IsAdmin(user.Role)
	data.SignupDisabled, _ = strconv.ParseBool(os.Getenv("APP_DISABLE_SIGNUP"))
	data.Description = "Edit Post - Anshuman Biswas Blog"
	data.CurrentPage = "admin-posts"
	data.UserPermissions = models.GetPermissions(user.Role)
	data.Mode = "edit"
	data.Post = post
	data.Categories = categories
	data.SelectedCategories = selectedCategories
	u.Templates.PostEditor.Execute(w, r, data)
}

// UpdatePost persists edits to an existing post
func (u Users) UpdatePost(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}
	if !models.CanEditPosts(user.Role) && !models.IsAdmin(user.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	idStr := chi.URLParam(r, "postID")
	id, _ := strconv.Atoi(idStr)
	title := r.FormValue("title")
	content := r.FormValue("content")
	featuredImageURL := r.FormValue("featured_image_url")
	featured := r.FormValue("featured") == "on"
	slug := r.FormValue("slug")
	isPublished := r.FormValue("is_published") == "on"

	// Parse multiple categories
	categoryIDStrings := r.Form["categories"] // Get all category values
	var categoryIDs []int
	for _, idStr := range categoryIDStrings {
		if catID, err := strconv.Atoi(idStr); err == nil {
			categoryIDs = append(categoryIDs, catID)
		}
	}

	// Ensure at least one category is selected
	if len(categoryIDs) == 0 {
		http.Error(w, "At least one category must be selected", http.StatusBadRequest)
		return
	}

	// Legacy category handling (keep for backward compatibility)
	categoryID, _ := strconv.Atoi(r.FormValue("category_id"))
	if categoryID == 0 {
		categoryID = 1
	}

	if slug == "" {
		slug = strings.ToLower(title)
		slug = strings.ReplaceAll(slug, " ", "-")
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	if err := u.PostService.Update(id, categoryID, title, content, isPublished, featured, featuredImageURL, slug); err != nil {
		http.Error(w, "Failed to update post", http.StatusInternalServerError)
		return
	}

	// Update categories for the post
	if err := u.CategoryService.AssignCategoriesToPost(id, categoryIDs); err != nil {
		log.Printf("Error updating categories for post: %v", err)
		// Don't fail the entire request, just log the error
	}

	http.Redirect(w, r, "/blog/"+slug, http.StatusFound)
}

// APIAccess shows the API access management page
func (u Users) APIAccess(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	tokens, err := u.APITokenService.GetByUser(user.UserID)
	if err != nil {
		http.Error(w, "Failed to fetch API tokens", http.StatusInternalServerError)
		return
	}

	var data struct {
		Email           string
		LoggedIn        bool
		Username        string
		IsAdmin         bool
		SignupDisabled  bool
		Description     string
		CurrentPage     string
		Message         string
		Tokens          []*models.APIToken
		UserPermissions models.UserPermissions
		Request         *http.Request
	}

	data.Email = user.Email
	data.Username = user.Username
	data.LoggedIn = true
	data.IsAdmin = models.IsAdmin(user.Role)
	data.SignupDisabled, _ = strconv.ParseBool(os.Getenv("APP_DISABLE_SIGNUP"))
	data.Description = "API Access Management - Anshuman Biswas Blog"
	data.CurrentPage = "api-access"
	data.Message = r.URL.Query().Get("message")
	data.Tokens = tokens
	data.UserPermissions = models.GetPermissions(user.Role)
	data.Request = r

	u.Templates.APIAccess.Execute(w, r, data)
}

// CreateAPIToken creates a new API token for the user
func (u Users) CreateAPIToken(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	tokenName := r.FormValue("name")
	if tokenName == "" {
		http.Redirect(w, r, "/api-access?message=Token name is required", http.StatusFound)
		return
	}

	token, err := u.APITokenService.Create(user.UserID, tokenName, nil)
	if err != nil {
		// Log the actual error for debugging
		log.Printf("Failed to create API token for user %d: %v", user.UserID, err)
		http.Redirect(w, r, "/api-access?message=Failed to create API token", http.StatusFound)
		return
	}

	// For security, we show the token only once after creation
	http.Redirect(w, r, fmt.Sprintf("/api-access?message=Token created successfully: %s&new_token=%s", tokenName, token.Token), http.StatusFound)
}

// RevokeAPIToken revokes an API token
func (u Users) RevokeAPIToken(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	tokenIDStr := r.FormValue("token_id")
	tokenID, err := strconv.Atoi(tokenIDStr)
	if err != nil {
		http.Redirect(w, r, "/api-access?message=Invalid token ID", http.StatusFound)
		return
	}

	err = u.APITokenService.Revoke(tokenID, user.UserID)
	if err != nil {
		http.Redirect(w, r, "/api-access?message=Failed to revoke token", http.StatusFound)
		return
	}

	http.Redirect(w, r, "/api-access?message=Token revoked successfully", http.StatusFound)
}

// DeleteAPIToken deletes an API token
func (u Users) DeleteAPIToken(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	tokenIDStr := r.FormValue("token_id")
	tokenID, err := strconv.Atoi(tokenIDStr)
	if err != nil {
		http.Redirect(w, r, "/api-access?message=Invalid token ID", http.StatusFound)
		return
	}

	err = u.APITokenService.Delete(tokenID, user.UserID)
	if err != nil {
		http.Redirect(w, r, "/api-access?message=Failed to delete token", http.StatusFound)
		return
	}

	http.Redirect(w, r, "/api-access?message=Token deleted successfully", http.StatusFound)
}

// JSON API endpoints for AJAX operations

// GetAPITokensJSON returns user's API tokens as JSON
func (u Users) GetAPITokensJSON(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	tokens, err := u.APITokenService.GetByUser(user.UserID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to fetch API tokens"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tokens": tokens,
	})
}

// CreateAPITokenJSON creates an API token and returns JSON response
func (u Users) CreateAPITokenJSON(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	tokenName := r.FormValue("name")
	if tokenName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Token name is required"})
		return
	}

	token, err := u.APITokenService.Create(user.UserID, tokenName, nil)
	if err != nil {
		log.Printf("Failed to create API token for user %d: %v", user.UserID, err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create API token"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Token created successfully",
		"token":   token,
	})
}

// RevokeAPITokenJSON revokes an API token and returns JSON response
func (u Users) RevokeAPITokenJSON(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	tokenIDStr := r.FormValue("token_id")
	tokenID, err := strconv.Atoi(tokenIDStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid token ID"})
		return
	}

	err = u.APITokenService.Revoke(tokenID, user.UserID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to revoke token"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Token revoked successfully",
	})
}

// DeleteAPITokenJSON deletes an API token and returns JSON response
func (u Users) DeleteAPITokenJSON(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	tokenIDStr := chi.URLParam(r, "token_id")
	tokenID, err := strconv.Atoi(tokenIDStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid token ID"})
		return
	}

	err = u.APITokenService.Delete(tokenID, user.UserID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to delete token"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Token deleted successfully",
	})
}
