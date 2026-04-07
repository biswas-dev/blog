package controllers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	_ "image/gif"

	"golang.org/x/image/draw"
	"strconv"
	"strings"
	"time"

	"html/template"

	"anshumanbiswas.com/blog/models"
	"anshumanbiswas.com/blog/utils"
	gowiki "github.com/anchoo2kewl/go-wiki"
	"github.com/go-chi/chi/v5"
)

const (
	staticPrefix       = "/static/"
	errImageMetaNotAvail = "image metadata not available"
)

// Pre-compiled regexes for slug sanitisation.
var (
	slugSanitizeRe    = regexp.MustCompile(`[^a-z0-9-]`)
	slugNonAlnumRe    = regexp.MustCompile(`[^a-z0-9\s-]`)
	slugWhitespaceRe  = regexp.MustCompile(`\s+`)
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
		New         Template
		SignIn      Template
		Home        Template
		LoggedIn    Template
		Profile     Template
		AdminPosts  Template
		UserPosts   Template
		APIAccess   Template
		PostEditor  Template
		UserProfile Template
	}
	DB                   *sql.DB
	UserService          *models.UserService
	SessionService       *models.SessionService
	PostService          *models.PostService
	PostVersionService   *models.PostVersionService
	APITokenService      *models.APITokenService
	CategoryService      *models.CategoryService
	CloudinaryService    *models.CloudinaryService
	ImageMetadataService *models.ImageMetadataService
	UserActivityService  *models.UserActivityService
	SlideService         *models.SlideService
	GuideService         *models.GuideService
	BlogWiki             *gowiki.Wiki
}

// extensionForContent resolves the file extension from the detected content-type
// or falls back to the filename extension. Returns (ext, ok).
func extensionForContent(filetype, filename string) (string, bool) {
	allowed := map[string]string{
		"image/jpeg":    ".jpg",
		"image/png":     ".png",
		"image/gif":     ".gif",
		"image/webp":    ".webp",
		"image/svg+xml": ".svg",
	}
	if ext, ok := allowed[filetype]; ok {
		return ext, true
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg":
		if ext == ".jpeg" {
			ext = ".jpg"
		}
		return ext, true
	}
	return "", false
}

// saveUploadedFile validates, generates a random name, and persists a single
// uploaded image to disk. It returns the public URL of the saved file.
func saveUploadedFile(file io.ReadSeeker, filename, uploadType, slug string) (string, error) {
	// Validate content type
	buff := make([]byte, 512)
	n, _ := file.Read(buff)
	filetype := http.DetectContentType(buff[:n])
	ext, ok := extensionForContent(filetype, filename)
	if !ok {
		return "", fmt.Errorf("unsupported file type")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("unable to read file: %w", err)
	}

	// Random filename to avoid collisions
	rb := make([]byte, 16)
	if _, err := rand.Read(rb); err != nil {
		return "", fmt.Errorf("internal error: %w", err)
	}
	name := hex.EncodeToString(rb) + ext

	// Build directory path
	base := filepath.Join("static", "uploads")
	if uploadType == "featured" {
		base = filepath.Join(base, "featured")
		if slug != "" {
			base = filepath.Join(base, slug)
		}
	} else if uploadType == "avatar" {
		base = filepath.Join(base, "avatars")
	} else if uploadType == "slide" {
		base = filepath.Join(base, "slide")
		if slug != "" {
			base = filepath.Join(base, slug)
		}
	} else if slug != "" {
		base = filepath.Join(base, "post", slug)
	}
	_ = os.MkdirAll(base, 0o755)

	fpath := filepath.Join(base, name)
	out, err := os.Create(fpath)
	if err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	// Build public URL
	urlBase := "/static/uploads"
	if uploadType == "featured" {
		urlBase += "/featured"
		if slug != "" {
			urlBase += "/" + slug
		}
	} else if uploadType == "avatar" {
		urlBase += "/avatars"
	} else if uploadType == "slide" {
		urlBase += "/slide"
		if slug != "" {
			urlBase += "/" + slug
		}
	} else if slug != "" {
		urlBase += "/post/" + slug
	}
	url := urlBase + "/" + name

	// Generate thumbnail for avatar uploads
	if uploadType == "avatar" {
		go createThumbnail(fpath, 96)
	}

	// Auto-compress featured/cover images (book covers, post covers)
	// to max 600px wide at JPEG quality 80 — keeps files under ~100 KB
	if uploadType == "featured" {
		go compressImage(fpath, 600, 80)
	}

	return url, nil
}

// ThumbURL derives the thumbnail URL from an original image URL.
// "/static/uploads/avatars/abc123.jpg" → "/static/uploads/avatars/abc123_thumb.jpg"
func ThumbURL(originalURL string) string {
	if originalURL == "" {
		return ""
	}
	ext := filepath.Ext(originalURL)
	return originalURL[:len(originalURL)-len(ext)] + "_thumb" + ext
}

// createThumbnail generates a square thumbnail of the given size from the source image.
// It writes to the same directory with _thumb inserted before the extension.
// Runs in a goroutine — errors are logged but never block the caller.
func createThumbnail(srcPath string, size int) {
	ext := strings.ToLower(filepath.Ext(srcPath))
	thumbPath := srcPath[:len(srcPath)-len(ext)] + "_thumb" + ext

	src, err := os.Open(srcPath)
	if err != nil {
		log.Printf("thumbnail: open %s: %v", srcPath, err)
		return
	}
	defer src.Close()

	img, _, err := image.Decode(src)
	if err != nil {
		log.Printf("thumbnail: decode %s: %v", srcPath, err)
		return
	}

	// Crop to square from center, then scale down
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	cropSize := w
	if h < cropSize {
		cropSize = h
	}
	x0 := (w - cropSize) / 2
	y0 := (h - cropSize) / 2

	// Create the thumbnail
	thumb := image.NewRGBA(image.Rect(0, 0, size, size))

	// Use SubImage for cropping, then scale
	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}
	cropped := img
	if si, ok := img.(subImager); ok {
		cropped = si.SubImage(image.Rect(x0+bounds.Min.X, y0+bounds.Min.Y, x0+bounds.Min.X+cropSize, y0+bounds.Min.Y+cropSize))
	}

	draw.CatmullRom.Scale(thumb, thumb.Bounds(), cropped, cropped.Bounds(), draw.Over, nil)

	out, err := os.Create(thumbPath)
	if err != nil {
		log.Printf("thumbnail: create %s: %v", thumbPath, err)
		return
	}
	defer out.Close()

	switch ext {
	case ".png":
		err = png.Encode(out, thumb)
	default: // .jpg, .jpeg, .gif, .webp → save as JPEG
		err = jpeg.Encode(out, thumb, &jpeg.Options{Quality: 85})
	}
	if err != nil {
		log.Printf("thumbnail: encode %s: %v", thumbPath, err)
	}
}

// compressImage resizes an image to maxWidth (preserving aspect ratio) and
// re-encodes as JPEG at the given quality. The file is replaced in-place.
// PNGs are converted to JPEG (the extension stays the same in the URL, but
// content-type detection will serve the correct type).
// This keeps uploaded featured images and book covers under ~100 KB.
func compressImage(fpath string, maxWidth, quality int) {
	src, err := os.Open(fpath)
	if err != nil {
		log.Printf("compress: open %s: %v", fpath, err)
		return
	}
	img, _, err := image.Decode(src)
	src.Close()
	if err != nil {
		log.Printf("compress: decode %s: %v", fpath, err)
		return
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Only resize if wider than maxWidth
	newW, newH := w, h
	if w > maxWidth {
		newW = maxWidth
		newH = int(float64(h) * float64(maxWidth) / float64(w))
	}

	var dst *image.RGBA
	if newW != w {
		dst = image.NewRGBA(image.Rect(0, 0, newW, newH))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)
	} else {
		// No resize needed — just re-encode at lower quality
		dst = image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Copy(dst, image.Point{}, img, bounds, draw.Over, nil)
	}

	// Always write as JPEG for maximum compression
	out, err := os.Create(fpath)
	if err != nil {
		log.Printf("compress: create %s: %v", fpath, err)
		return
	}
	defer out.Close()
	if err := jpeg.Encode(out, dst, &jpeg.Options{Quality: quality}); err != nil {
		log.Printf("compress: encode %s: %v", fpath, err)
		return
	}

	// Log the result
	if info, err := os.Stat(fpath); err == nil {
		log.Printf("compress: %s → %dx%d %d KB (was %dx%d)", filepath.Base(fpath), newW, newH, info.Size()/1024, w, h)
	}
}

// BackfillAvatarThumbnails generates thumbnails for all existing avatar images
// that don't already have a _thumb version. Intended to run once at startup.
func BackfillAvatarThumbnails() {
	avatarDir := filepath.Join("static", "uploads", "avatars")
	entries, err := os.ReadDir(avatarDir)
	if err != nil {
		return // no avatars dir yet
	}
	for _, e := range entries {
		if e.IsDir() || strings.Contains(e.Name(), "_thumb") {
			continue
		}
		ext := filepath.Ext(e.Name())
		thumbName := e.Name()[:len(e.Name())-len(ext)] + "_thumb" + ext
		thumbPath := filepath.Join(avatarDir, thumbName)
		if _, err := os.Stat(thumbPath); err == nil {
			continue // thumb already exists
		}
		srcPath := filepath.Join(avatarDir, e.Name())
		createThumbnail(srcPath, 96)
		log.Printf("thumbnail: backfilled %s", thumbName)
	}
}

// CompressOversizedImages scans static/uploads/featured/ and compresses any
// image larger than 200 KB to max 600px wide at JPEG quality 80. Intended to
// run once at startup to fix images uploaded before auto-compression was added.
func CompressOversizedImages() {
	const maxBytes = 200 * 1024 // 200 KB threshold
	dirs := []string{
		filepath.Join("static", "uploads", "featured"),
	}
	// Also scan subdirectories (featured/{slug}/)
	if entries, err := os.ReadDir(dirs[0]); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				dirs = append(dirs, filepath.Join(dirs[0], e.Name()))
			}
		}
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || strings.Contains(e.Name(), "_thumb") {
				continue
			}
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
				continue
			}
			info, err := e.Info()
			if err != nil || info.Size() <= maxBytes {
				continue
			}
			fpath := filepath.Join(dir, e.Name())
			log.Printf("compress: oversized %s (%d KB) — compressing", e.Name(), info.Size()/1024)
			compressImage(fpath, 600, 80)
		}
	}
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

	uploadType := strings.ToLower(r.URL.Query().Get("type"))
	slug := r.URL.Query().Get("slug")
	slug = strings.ToLower(slug)
	slug = slugSanitizeRe.ReplaceAllString(slug, "-")

	url, err := saveUploadedFile(file, header.Filename, uploadType, slug)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

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

	uploadType := strings.ToLower(r.URL.Query().Get("type"))
	slug := r.URL.Query().Get("slug")
	slug = strings.ToLower(slug)
	slug = slugSanitizeRe.ReplaceAllString(slug, "-")

	var uploads []map[string]interface{}
	var errors []string

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to open %s: %v", fileHeader.Filename, err))
			continue
		}

		url, err := saveUploadedFile(file, fileHeader.Filename, uploadType, slug)
		file.Close()
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", fileHeader.Filename, err))
			continue
		}

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
	slug = slugNonAlnumRe.ReplaceAllString(slug, "")
	slug = slugWhitespaceRe.ReplaceAllString(slug, "-")
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
				url := staticPrefix + strings.ReplaceAll(relPath, "\\", "/")

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

	// Normalize featured image URLs for display
	if posts != nil {
		for i := range posts.Posts {
			p := &posts.Posts[i]
			if p.FeaturedImageURL != "" && !strings.HasPrefix(p.FeaturedImageURL, "http") && !strings.HasPrefix(p.FeaturedImageURL, staticPrefix) {
				if p.FeaturedImageURL == "image.jpg" {
					p.FeaturedImageURL = ""
				} else {
					p.FeaturedImageURL = staticPrefix + p.FeaturedImageURL
				}
			}
		}
	}

	// Get signup disabled setting from environment
	isSignupDisabled, _ := strconv.ParseBool(os.Getenv("APP_DISABLE_SIGNUP"))

	user, err := u.isUserLoggedIn(r)
	if err != nil {
		data.LoggedIn = false
		data.Posts = posts
		data.SignupDisabled = isSignupDisabled
		data.Description = "Engineering Insights - Anshuman Biswas Blog"
		data.CurrentPage = "home"
		data.Username = ""
		data.IsAdmin = false
		data.Email = ""
		data.UserPermissions = models.GetPermissions(models.RoleCommenter)
		w.Header().Set("Cache-Control", "public, max-age=300, stale-while-revalidate=3600")
		u.Templates.Home.Execute(w, r, data)
		return
	}

	data.Email = user.Email
	data.Username = user.Username
	data.LoggedIn = true
	data.Posts = posts
	data.IsAdmin = models.IsAdmin(user.Role)
	data.SignupDisabled = isSignupDisabled
	data.Description = "Engineering Insights - Anshuman Biswas Blog"
	data.CurrentPage = "home"
	data.UserPermissions = models.GetPermissions(user.Role)
	w.Header().Set("Cache-Control", "private, no-store")
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

	// Normalize featured image URLs for display
	if posts != nil {
		for i := range posts.Posts {
			p := &posts.Posts[i]
			if p.FeaturedImageURL != "" && !strings.HasPrefix(p.FeaturedImageURL, "http") && !strings.HasPrefix(p.FeaturedImageURL, staticPrefix) {
				if p.FeaturedImageURL == "image.jpg" {
					p.FeaturedImageURL = ""
				} else {
					p.FeaturedImageURL = staticPrefix + p.FeaturedImageURL
				}
			}
		}
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
		if u.UserActivityService != nil {
			// Best-effort: look up userID by email to log the failure
			if existing, lookupErr := u.UserService.GetByEmail(data.Email); lookupErr == nil {
				u.UserActivityService.Log(existing.UserID, "failed_login", utils.GetClientIP(r), r.UserAgent())
			}
		}
		http.Error(w, "Something went wrong.", http.StatusInternalServerError)
		return
	}
	if u.UserActivityService != nil {
		u.UserActivityService.Log(user.UserID, "login", utils.GetClientIP(r), r.UserAgent())
	}
	session, err := u.SessionService.Create(user.UserID)
	if err != nil {
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
	user, err := u.UserService.Create(email, username, password, 1)
	if err != nil {
		http.Error(w, "Something went wrong.", http.StatusInternalServerError)
		return
	}
	session, err := u.SessionService.Create(user.UserID)
	if err != nil {
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
	filePath := filepath.Join("static", strings.TrimPrefix(imagePath, staticPrefix))

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

// GetUploadConfig returns upload configuration so the editor JS knows whether to use Cloudinary or local uploads
func (u Users) GetUploadConfig(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !models.CanEditPosts(user.Role) && !models.IsAdmin(user.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	resp := map[string]interface{}{
		"cloudinary_enabled": false,
	}

	if u.CloudinaryService != nil && u.CloudinaryService.IsConfigured() {
		settings, err := u.CloudinaryService.Get()
		if err == nil && settings != nil {
			resp["cloudinary_enabled"] = true
			resp["cloud_name"] = settings.CloudName
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (u Users) isUserLoggedIn(r *http.Request) (*models.User, error) {
	return utils.IsUserLoggedIn(r, u.SessionService)
}

func (u Users) CurrentUser(w http.ResponseWriter, r *http.Request) {

	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	var data struct {
		Email           string
		LoggedIn        bool
		Username        string
		FullName        string
		AvatarURL       string
		Bio             string
		IsAdmin         bool
		SignupDisabled  bool
		Description     string
		CurrentPage     string
		Message         string
		UserPermissions models.UserPermissions
	}

	data.Email = user.Email
	data.Username = user.Username
	data.FullName = user.FullName
	data.AvatarURL = user.AvatarURL
	data.Bio = user.Bio
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

// UpdateName updates the user's display name.
// UploadAvatar updates the user's profile picture via file upload or URL.
func (u Users) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	var avatarURL string

	// Try file upload first.
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20) // 5 MB
	if err := r.ParseMultipartForm(5 << 20); err == nil {
		file, header, ferr := r.FormFile("avatar")
		if ferr == nil {
			defer file.Close()
			url, serr := saveUploadedFile(file, header.Filename, "avatar", fmt.Sprintf("%d", user.UserID))
			if serr != nil {
				http.Redirect(w, r, "/users/me?message=Failed to save image", http.StatusFound)
				return
			}
			avatarURL = url
		}
	}

	// Fall back to URL field.
	if avatarURL == "" {
		raw := strings.TrimSpace(r.FormValue("avatar_url"))
		if raw != "" && (strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://")) {
			avatarURL = raw
		}
	}

	if avatarURL == "" {
		http.Redirect(w, r, "/users/me?message=No image provided", http.StatusFound)
		return
	}

	if err := u.UserService.UpdateAvatarURL(user.UserID, avatarURL); err != nil {
		http.Redirect(w, r, "/users/me?message=Failed to update avatar", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/users/me?message=Profile picture updated", http.StatusFound)
}

func (u Users) UpdateName(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	fullName := strings.TrimSpace(r.FormValue("full_name"))
	if len(fullName) > 100 {
		http.Redirect(w, r, "/users/me?message=Name too long (max 100 characters)", http.StatusFound)
		return
	}
	if err := u.UserService.UpdateName(user.UserID, fullName); err != nil {
		http.Redirect(w, r, "/users/me?message=Failed to update name", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/users/me?message=Name updated successfully", http.StatusFound)
}

func (u Users) UpdateBio(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	bio := strings.TrimSpace(r.FormValue("bio"))
	if len(bio) > 500 {
		http.Redirect(w, r, "/users/me?message=Bio too long (max 500 characters)", http.StatusFound)
		return
	}
	if err := u.UserService.UpdateBio(user.UserID, bio); err != nil {
		http.Redirect(w, r, "/users/me?message=Failed to update bio", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/users/me?message=Bio updated successfully", http.StatusFound)
}

// AdminPosts shows all posts for admin users
func (u Users) AdminPosts(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	// Allow admins and editors
	if !models.CanEditPosts(user.Role) && !models.IsAdmin(user.Role) {
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
	data.IsAdmin = models.IsAdmin(user.Role)
	data.SignupDisabled, _ = strconv.ParseBool(os.Getenv("APP_DISABLE_SIGNUP"))
	data.Description = "Manage All Posts - Anshuman Biswas Blog"
	data.CurrentPage = "admin-posts"
	data.Posts = posts
	data.UserPermissions = models.GetPermissions(user.Role)

	u.Templates.AdminPosts.Execute(w, r, data)
}

// DeletePosts handles deleting one or more posts (JSON API)
func (u Users) DeletePosts(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !models.IsAdmin(user.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var input struct {
		IDs []int `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(input.IDs) == 0 {
		http.Error(w, "No post IDs provided", http.StatusBadRequest)
		return
	}

	deleted := 0
	var errors []string
	for _, id := range input.IDs {
		if err := u.PostService.Delete(id); err != nil {
			errors = append(errors, fmt.Sprintf("post %d: %v", id, err))
		} else {
			deleted++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"deleted": deleted,
		"errors":  errors,
	})
}

// UserComment is a comment made by a user, enriched with post info.
type UserComment struct {
	CommentID   int
	PostTitle   string
	PostSlug    string
	Content     string
	CommentDate time.Time
}

// UserAnnotation is an annotation made by a user, enriched with post info.
type UserAnnotation struct {
	ID           int
	PostTitle    string
	PostSlug     string
	SelectedText string
	Color        string
	CreatedAt    time.Time
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

	// Fetch user's comments with post info.
	var userComments []UserComment
	commentRows, err := u.DB.QueryContext(r.Context(), `
		SELECT c.comment_id, p.title, p.slug, c.content, c.comment_date
		FROM Comments c
		JOIN Posts p ON p.post_id = c.post_id
		WHERE c.user_id = $1
		ORDER BY c.comment_date DESC`, user.UserID)
	if err == nil {
		defer commentRows.Close()
		for commentRows.Next() {
			var uc UserComment
			if err := commentRows.Scan(&uc.CommentID, &uc.PostTitle, &uc.PostSlug, &uc.Content, &uc.CommentDate); err == nil {
				userComments = append(userComments, uc)
			}
		}
	}

	// Fetch user's annotations with post info.
	var userAnnotations []UserAnnotation
	annRows, err := u.DB.QueryContext(r.Context(), `
		SELECT a.id, p.title, p.slug, a.selected_text, a.color, a.created_at
		FROM post_annotations a
		JOIN Posts p ON p.post_id = a.post_id
		WHERE a.author_id = $1
		ORDER BY a.created_at DESC`, user.UserID)
	if err == nil {
		defer annRows.Close()
		for annRows.Next() {
			var ua UserAnnotation
			if err := annRows.Scan(&ua.ID, &ua.PostTitle, &ua.PostSlug, &ua.SelectedText, &ua.Color, &ua.CreatedAt); err == nil {
				userAnnotations = append(userAnnotations, ua)
			}
		}
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
		UserComments    []UserComment
		UserAnnotations []UserAnnotation
	}

	data.Email = user.Email
	data.Username = user.Username
	data.LoggedIn = true
	data.IsAdmin = (user.Role == 2)
	data.SignupDisabled, _ = strconv.ParseBool(os.Getenv("APP_DISABLE_SIGNUP"))
	data.Description = "My Posts & Comments - Anshuman Biswas Blog"
	data.CurrentPage = "my-posts"
	data.Posts = posts
	data.UserPermissions = models.GetPermissions(user.Role)
	data.UserComments = userComments
	data.UserAnnotations = userAnnotations

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

	// Generate editor HTML from go-wiki
	editorHTML, err := u.BlogWiki.EditorHTML("")
	if err != nil {
		log.Printf("Error generating editor HTML: %v", err)
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
		EditorHTML         template.HTML
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
	data.EditorHTML = editorHTML
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

	if u.PostVersionService != nil {
		_ = u.PostVersionService.MaybeCreateVersion(post.ID, user.UserID, title, content)
	}

	// Assign categories to the post
	if err := u.CategoryService.AssignCategoriesToPost(post.ID, categoryIDs); err != nil {
		log.Printf("Error assigning categories to post: %v", err)
		// Don't fail the entire request, just log the error
	}

	// Ping search engines (IndexNow) if the post is published
	if isPublished {
		PingIndexNow("https://" + IndexNowHost + "/blog/" + slug)
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

	// Generate editor HTML from go-wiki
	editorHTML, err := u.BlogWiki.EditorHTML(post.Content)
	if err != nil {
		log.Printf("Error generating editor HTML: %v", err)
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
		EditorHTML         template.HTML
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
	data.EditorHTML = editorHTML
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

	if u.PostVersionService != nil {
		_ = u.PostVersionService.MaybeCreateVersion(id, user.UserID, title, content)
	}

	// Update categories for the post
	if err := u.CategoryService.AssignCategoriesToPost(id, categoryIDs); err != nil {
		log.Printf("Error updating categories for post: %v", err)
		// Don't fail the entire request, just log the error
	}

	// Ping search engines (IndexNow) if the post is published
	if isPublished {
		PingIndexNow("https://" + IndexNowHost + "/blog/" + slug)
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

// ListTrackedImages returns all image metadata records from the database.
// GET /api/admin/images
func (u Users) ListTrackedImages(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !models.CanEditPosts(user.Role) && !models.IsAdmin(user.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if u.ImageMetadataService == nil {
		http.Error(w, errImageMetaNotAvail, http.StatusServiceUnavailable)
		return
	}

	images, err := u.ImageMetadataService.List()
	if err != nil {
		http.Error(w, "failed to list images: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if images == nil {
		images = []models.ImageMetadata{}
	}

	// Transform to the format expected by the JS image browser
	type imageEntry struct {
		URL       string `json:"url"`
		Filename  string `json:"filename"`
		AltText   string `json:"alt_text"`
		Caption   string `json:"caption"`
		Title     string `json:"title"`
		CreatedAt string `json:"created_at"`
	}
	entries := make([]imageEntry, len(images))
	for i, img := range images {
		// Derive filename from URL
		filename := img.ImageURL
		if idx := strings.LastIndex(filename, "/"); idx >= 0 {
			filename = filename[idx+1:]
		}
		entries[i] = imageEntry{
			URL:       img.ImageURL,
			Filename:  filename,
			AltText:   img.AltText,
			Caption:   img.Caption,
			Title:     img.Title,
			CreatedAt: img.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"images": entries,
	})
}

// DeleteImageMetadata handles DELETE /api/admin/image-metadata?url=... — remove metadata for one image.
func (u Users) DeleteImageMetadata(w http.ResponseWriter, r *http.Request) {
	user, err := u.isUserLoggedIn(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !models.CanEditPosts(user.Role) && !models.IsAdmin(user.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if u.ImageMetadataService == nil {
		http.Error(w, errImageMetaNotAvail, http.StatusServiceUnavailable)
		return
	}

	imageURL := r.URL.Query().Get("url")
	if imageURL == "" {
		http.Error(w, "url param is required", http.StatusBadRequest)
		return
	}

	if err := u.ImageMetadataService.Delete(imageURL); err != nil {
		http.Error(w, "failed to delete: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// SaveImageMetadata handles PUT /api/admin/image-metadata — upsert metadata for one image.
func (u Users) SaveImageMetadata(w http.ResponseWriter, r *http.Request) {
	if u.ImageMetadataService == nil {
		http.Error(w, errImageMetaNotAvail, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		ImageURL string `json:"image_url"`
		AltText  string `json:"alt_text"`
		Title    string `json:"title"`
		Caption  string `json:"caption"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.ImageURL == "" {
		http.Error(w, "image_url is required", http.StatusBadRequest)
		return
	}

	meta, err := u.ImageMetadataService.Upsert(req.ImageURL, req.AltText, req.Title, req.Caption)
	if err != nil {
		http.Error(w, "failed to save: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(meta)
}

// GetImageMetadata handles GET /api/admin/image-metadata?url=... — get metadata for one image.
func (u Users) GetImageMetadata(w http.ResponseWriter, r *http.Request) {
	if u.ImageMetadataService == nil {
		http.Error(w, errImageMetaNotAvail, http.StatusServiceUnavailable)
		return
	}

	imageURL := r.URL.Query().Get("url")
	if imageURL == "" {
		http.Error(w, "url param is required", http.StatusBadRequest)
		return
	}

	meta, err := u.ImageMetadataService.GetByURL(imageURL)
	if err != nil {
		http.Error(w, "lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if meta == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("null"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(meta)
}

// GetImageMetadataBulk handles POST /api/admin/image-metadata/bulk — get metadata for multiple image URLs.
func (u Users) GetImageMetadataBulk(w http.ResponseWriter, r *http.Request) {
	if u.ImageMetadataService == nil {
		http.Error(w, errImageMetaNotAvail, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		URLs []string `json:"urls"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	result, err := u.ImageMetadataService.GetByURLs(req.URLs)
	if err != nil {
		http.Error(w, "lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// PublicProfile shows the public profile page for a user.
func (u Users) PublicProfile(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")

	profileUser, err := u.UserService.GetByUsername(username)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	authoredPosts, _ := u.PostService.GetPublishedPostsByUser(profileUser.UserID)

	var contributedPosts []models.Post
	if u.PostVersionService != nil {
		contributedPosts, _ = u.PostVersionService.GetContributedPosts(profileUser.UserID)
	}

	var userComments []UserComment
	commentRows, err := u.DB.QueryContext(r.Context(), `
		SELECT c.comment_id, p.title, p.slug, c.content, c.comment_date
		FROM Comments c
		JOIN Posts p ON p.post_id = c.post_id
		WHERE c.user_id = $1
		ORDER BY c.comment_date DESC`, profileUser.UserID)
	if err == nil {
		defer commentRows.Close()
		for commentRows.Next() {
			var uc UserComment
			if err := commentRows.Scan(&uc.CommentID, &uc.PostTitle, &uc.PostSlug, &uc.Content, &uc.CommentDate); err == nil {
				userComments = append(userComments, uc)
			}
		}
	}

	var authoredSlides []models.Slide
	if u.SlideService != nil {
		authoredSlides, _ = u.SlideService.GetPublishedSlidesByUser(profileUser.UserID)
	}

	var authoredGuides []models.Guide
	if u.GuideService != nil {
		authoredGuides, _ = u.GuideService.GetPublishedGuidesByUser(profileUser.UserID)
	}

	var data struct {
		LoggedIn         bool
		Email            string
		Username         string
		IsAdmin          bool
		SignupDisabled   bool
		Description      string
		CurrentPage      string
		UserPermissions  models.UserPermissions
		ProfileUser      *models.User
		AuthoredPosts    []models.Post
		ContributedPosts []models.Post
		AuthoredSlides   []models.Slide
		AuthoredGuides   []models.Guide
		UserComments     []UserComment
	}

	data.Description = profileUser.DisplayName() + " - Anshuman Biswas Blog"
	data.CurrentPage = "profile"
	data.ProfileUser = profileUser
	data.AuthoredPosts = authoredPosts
	data.ContributedPosts = contributedPosts
	data.AuthoredSlides = authoredSlides
	data.AuthoredGuides = authoredGuides
	data.UserComments = userComments
	data.SignupDisabled, _ = strconv.ParseBool(os.Getenv("APP_DISABLE_SIGNUP"))

	loggedInUser, _ := u.isUserLoggedIn(r)
	if loggedInUser != nil {
		data.LoggedIn = true
		data.Email = loggedInUser.Email
		data.Username = loggedInUser.Username
		data.IsAdmin = models.IsAdmin(loggedInUser.Role)
		data.UserPermissions = models.GetPermissions(loggedInUser.Role)
	}

	u.Templates.UserProfile.Execute(w, r, data)
}
