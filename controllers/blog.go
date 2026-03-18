// blog_controller.go
package controllers

import (
	"database/sql"
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
	DB *sql.DB
	Templates struct {
		Post Template
	}
	BlogService        *models.BlogService
	SessionService     *models.SessionService
	PostVersionService *models.PostVersionService
}

func (b *Blog) GetBlogPost(w http.ResponseWriter, r *http.Request) {
	var data struct {
		LoggedIn        bool
		Email           string
		Username        string
		IsAdmin         bool
		CanAnnotate     bool
		UserID          int
		SignupDisabled  bool
		Description     string
		CurrentPage     string
		ReadTime        string
		FullURL         string
		OGImage         string
		OGDescription   string
		Post            *models.Post
		PrevPost        *models.Post
		NextPost        *models.Post
		UserPermissions models.UserPermissions
		Contributors    []models.User
	}

	// Extract the slug from the URL
	slug := chi.URLParam(r, "slug")

	post, err := b.BlogService.GetBlogPostBySlug(slug)
	if err != nil {
		if b.DB != nil {
			go trackSlug404(b.DB, slug)
		}
		http.NotFound(w, r)
		return
	}

	// If the post is not published, only allow users with CanViewUnpublished permission
	if !post.IsPublished {
		user, _ := utils.IsUserLoggedIn(r, b.SessionService)
		if user == nil || !models.CanViewUnpublished(user.Role) {
			http.NotFound(w, r)
			return
		}
	}

	// Initialize default data
	data.LoggedIn = false
	data.Post = post
	data.SignupDisabled = true // Default based on environment
	data.Description = fmt.Sprintf("%s - Anshuman Biswas Blog", post.Title)
	data.CurrentPage = "blog"

	// Compute base URL from request headers (works behind reverse proxy/Cloudflare)
	scheme := "https"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if r.TLS == nil {
		scheme = "http"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, r.Host)
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

	// Compute OG meta fields for social sharing (LinkedIn, Twitter/X)
	if post.FeaturedImageURL != "" {
		if strings.HasPrefix(post.FeaturedImageURL, "http") {
			data.OGImage = post.FeaturedImageURL
		} else {
			data.OGImage = baseURL + post.FeaturedImageURL
		}
	}
	data.OGDescription = ogExcerpt(post.Content, 160)

	if b.PostVersionService != nil {
		contributors, _ := b.PostVersionService.GetContributors(post.ID)
		// Filter out the original author from contributors list
		for _, c := range contributors {
			if c.UserID != post.UserID {
				data.Contributors = append(data.Contributors, c)
			}
		}
	}

	user, _ := utils.IsUserLoggedIn(r, b.SessionService)
	if user != nil {
		data.LoggedIn = true
		data.Email = user.Email
		data.Username = user.Username
		data.UserID = user.UserID
		data.IsAdmin = (user.Role == 2) // Administrator role
		data.UserPermissions = models.GetPermissions(user.Role)
		data.CanAnnotate = data.UserPermissions.CanComment
		w.Header().Set("Cache-Control", "private, no-store")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=300, stale-while-revalidate=3600")
	}
	b.Templates.Post.Execute(w, r, data)
}

// stripMarkdownLinks replaces markdown link syntax [text](url) with just the
// link text, iterating until no more links remain.
func stripMarkdownLinks(s string) string {
	for {
		start := strings.Index(s, "[")
		if start == -1 {
			break
		}
		mid := strings.Index(s[start:], "](")
		if mid == -1 {
			break
		}
		end := strings.Index(s[start+mid:], ")")
		if end == -1 {
			break
		}
		linkText := s[start+1 : start+mid]
		s = s[:start] + linkText + s[start+mid+end+1:]
	}
	return s
}

// trackSlug404 upserts a hit count for an unknown slug in slug_404s.
// Called in a goroutine; errors are silently ignored.
func trackSlug404(db *sql.DB, slug string) {
	_, _ = db.Exec(`
		INSERT INTO slug_404s (slug, hit_count, first_seen, last_seen)
		VALUES ($1, 1, NOW(), NOW())
		ON CONFLICT (slug) DO UPDATE SET
			hit_count = slug_404s.hit_count + 1,
			last_seen = NOW()
	`, slug)
}

// ogExcerpt extracts a plain-text excerpt from markdown content for OG description.
func ogExcerpt(content string, maxLen int) string {
	lines := strings.Split(content, "\n")
	var parts []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "![") ||
			strings.HasPrefix(trimmed, "---") || strings.HasPrefix(trimmed, "```") ||
			strings.HasPrefix(trimmed, "|") || strings.HasPrefix(trimmed, "<more") ||
			strings.HasPrefix(trimmed, "<!--") {
			continue
		}
		// Strip inline markdown
		trimmed = strings.NewReplacer("**", "", "__", "", "`", "").Replace(trimmed)
		// Strip markdown links [text](url) -> text
		trimmed = stripMarkdownLinks(trimmed)
		parts = append(parts, trimmed)
		if len(strings.Join(parts, " ")) > maxLen {
			break
		}
	}
	text := strings.Join(parts, " ")
	if len(text) > maxLen {
		text = text[:maxLen]
		if i := strings.LastIndex(text, " "); i > maxLen/2 {
			text = text[:i]
		}
		text += "..."
	}
	return text
}
