// blog_controller.go
package controllers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"anshumanbiswas.com/blog/models"
	"anshumanbiswas.com/blog/utils"
	"github.com/go-chi/chi/v5"
)

type Blog struct {
	Templates struct {
		Post Template
	}
	BlogService    *models.BlogService
	SessionService *models.SessionService
}

func (b *Blog) GetBlogPost(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("DEBUG Controller: GetBlogPost called\n")

	var data struct {
		LoggedIn        bool
		Email           string
		Username        string
		IsAdmin         bool
		SignupDisabled  bool
		Description     string
		CurrentPage     string
		ReadTime        string
		FullURL         string
		Post            *models.Post
		PrevPost        *models.Post
		NextPost        *models.Post
		UserPermissions models.UserPermissions
	}

	// Extract the slug from the URL
	slug := chi.URLParam(r, "slug")

	fmt.Println("Slug:", slug)
	fmt.Printf("DEBUG Controller: About to call BlogService.GetBlogPostBySlug with slug '%s'\n", slug)
	fmt.Printf("DEBUG Controller: BlogService is: %+v\n", b.BlogService)
	// Fetch the blog post using the BlogService
	post, err := b.BlogService.GetBlogPostBySlug(slug)
	fmt.Printf("DEBUG Controller: GetBlogPostBySlug call completed, err: %v\n", err)
	if err != nil {
		fmt.Printf("DEBUG Controller: BlogService.GetBlogPostBySlug returned error: %v\n", err)
		// Handle error (e.g., render a 404 page)
		http.NotFound(w, r)
		return
	}
	fmt.Printf("DEBUG Controller: BlogService.GetBlogPostBySlug returned post ID %d\n", post.ID)

	fmt.Println("Post:", post)

	// Initialize default data
	data.LoggedIn = false
	data.Post = post
	data.SignupDisabled = true // Default based on environment
	data.Description = fmt.Sprintf("%s - Anshuman Biswas Blog", post.Title)
	data.CurrentPage = "blog"
	// Get base URL from environment, fallback to localhost for development
	baseURL := os.Getenv("APP_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:22222"
	}
	data.FullURL = fmt.Sprintf("%s/blog/%s", baseURL, slug)

	// Set prev/next posts to nil for now (can be implemented later)
	data.PrevPost = nil
	data.NextPost = nil

	// Calculate reading time (simple estimation: ~200 words per minute)
	wordCount := len(strings.Fields(post.Content))
	readingMinutes := (wordCount + 199) / 200 // Round up
	if readingMinutes < 1 {
		readingMinutes = 1
	}
	data.ReadTime = fmt.Sprintf("%d", readingMinutes)

	if post.ID == 0 {
		// Handle case where post is not found
		fmt.Println("Post not found")
		http.Redirect(w, r, "/404", http.StatusFound)
		return
	}

	// Fix featured image URL if it's relative
	if post.FeaturedImageURL != "" && !strings.HasPrefix(post.FeaturedImageURL, "http") {
		// Make it a proper static URL
		if post.FeaturedImageURL == "image.jpg" {
			post.FeaturedImageURL = "/static/placeholder-featured.svg"
		} else if !strings.HasPrefix(post.FeaturedImageURL, "/static/") {
			post.FeaturedImageURL = "/static/" + post.FeaturedImageURL
		}
	}
	// If the file doesn't exist (e.g., old hash path), fall back to first image under /static/uploads/featured/{slug}
	ensureFeatured := func() string {
		if post.Slug == "" {
			return ""
		}
		dir := filepath.Join("static", "uploads", "featured", post.Slug)
		entries, err := os.ReadDir(dir)
		if err != nil {
			return ""
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			low := strings.ToLower(name)
			if strings.HasSuffix(low, ".jpg") || strings.HasSuffix(low, ".jpeg") || strings.HasSuffix(low, ".png") || strings.HasSuffix(low, ".gif") || strings.HasSuffix(low, ".webp") || strings.HasSuffix(low, ".svg") {
				return "/static/uploads/featured/" + post.Slug + "/" + name
			}
		}
		return ""
	}
	if post.FeaturedImageURL == "" {
		if v := ensureFeatured(); v != "" {
			post.FeaturedImageURL = v
		}
	} else if strings.HasPrefix(post.FeaturedImageURL, "/static/") {
		local := strings.TrimPrefix(post.FeaturedImageURL, "/")
		if _, err := os.Stat(local); err != nil {
			if v := ensureFeatured(); v != "" {
				post.FeaturedImageURL = v
			}
		}
	}
	// ContentHTML is already prepared by BlogService (Markdown -> HTML, list/blockquote tweaks)

	user, _ := utils.IsUserLoggedIn(r, b.SessionService)
	fmt.Print(user)
	if user != nil {
		data.LoggedIn = true
		data.Email = user.Email
		data.Username = user.Username
		data.IsAdmin = (user.Role == 2) // Administrator role
		data.UserPermissions = models.GetPermissions(user.Role)
	}
	// Render the blog post template with the retrieved data
	// Example: b.Templates.BlogPost.Execute(w, r, post)
	b.Templates.Post.Execute(w, r, data)
}
