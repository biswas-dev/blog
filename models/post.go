package models

import (
	"database/sql"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"anshumanbiswas.com/blog/internal/render"
)

type PostsList struct {
	Posts []Post
}

type Post struct {
	ID               int
	UserID           int // Added UserID field
	Username         string // Username of the author
	CategoryID       int // Legacy field, kept for backward compatibility
	Title            string
	Content          string
	ContentHTML      template.HTML
	Slug             string
	PublicationDate  string
	LastEditDate     string
	IsPublished      bool
	Featured         bool   // Boolean field to mark posts as featured
	FeaturedImageURL string
	CreatedAt        string
	Categories       []Category `json:"categories,omitempty"` // New many-to-many categories
}

type PostService struct {
	DB *sql.DB
}

// Create will create a new session for the user provided. The session token
// will be returned as the Token field on the Session type, but only the hashed
// session token is stored in the database.
// func (pp *PostService) Create() (*Post, error) {

// }

func (pp *PostService) GetTopPosts() (*PostsList, error) {
	list := PostsList{}

	query := `SELECT post_id, user_id, category_id, title, content, slug, publication_date, last_edit_date, is_published, featured_image_url, created_at, featured FROM posts WHERE is_published = true ORDER BY created_at DESC LIMIT 5`
	rows, err := pp.DB.Query(query)
	if err != nil {
		return &list, nil
	}

	for rows.Next() {

		var post Post
		err := rows.Scan(&post.ID, &post.UserID, &post.CategoryID, &post.Title, &post.Content, &post.Slug, &post.PublicationDate, &post.LastEditDate, &post.IsPublished, &post.FeaturedImageURL, &post.CreatedAt, &post.Featured)
		if err != nil {
			panic(err)
		}

		// Parse and format CreatedAt
		t, err := time.Parse(time.RFC3339, post.CreatedAt)
		if err != nil {
			fmt.Println(err)
		}
		post.CreatedAt = t.Format(time.RFC3339)            // Keep original for JavaScript
		post.PublicationDate = t.Format("January 2, 2006") // Readable fallback

		// Parse and format PublicationDate if it's different from CreatedAt
		if post.PublicationDate != "" && post.PublicationDate != post.CreatedAt {
			pubDate, pubErr := time.Parse(time.RFC3339, post.PublicationDate)
			if pubErr == nil {
				post.PublicationDate = pubDate.Format("January 2, 2006")
			}
		}

		// Load categories for this post
		categoriesQuery := `SELECT c.category_id, c.category_name, c.created_at 
						   FROM categories c 
						   JOIN post_categories pc ON c.category_id = pc.category_id 
						   WHERE pc.post_id = $1`
		categoryRows, catErr := pp.DB.Query(categoriesQuery, post.ID)
		if catErr == nil {
			var categories []Category
			for categoryRows.Next() {
				var category Category
				categoryRows.Scan(&category.ID, &category.Name, &category.CreatedAt)
				categories = append(categories, category)
			}
			categoryRows.Close()
			post.Categories = categories
		}

		// Build preview from raw content to preserve Markdown list/numbering, then trim for length
		preview := previewContentRaw(post.Content)
		post.ContentHTML = template.HTML(RenderContent(preview))

		list.Posts = append(list.Posts, post)
	}

	if err != nil {
		return nil, fmt.Errorf("create post: %w", err)
	} else {
		fmt.Printf("Posts fetched successfully! Count: %d\n", len(list.Posts))
	}

	return &list, nil
}

func (pp *PostService) GetTopPostsWithPagination(limit int, offset int) (*PostsList, error) {
	list := PostsList{}

	query := `SELECT post_id, user_id, category_id, title, content, slug, publication_date, last_edit_date, is_published, featured_image_url, created_at, featured FROM posts WHERE is_published = true ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := pp.DB.Query(query, limit, offset)
	if err != nil {
		return &list, nil
	}

	for rows.Next() {
		var post Post
		err := rows.Scan(&post.ID, &post.UserID, &post.CategoryID, &post.Title, &post.Content, &post.Slug, &post.PublicationDate, &post.LastEditDate, &post.IsPublished, &post.FeaturedImageURL, &post.CreatedAt, &post.Featured)
		if err != nil {
			panic(err)
		}

		// Parse and format CreatedAt
		t, err := time.Parse(time.RFC3339, post.CreatedAt)
		if err != nil {
			fmt.Println(err)
		}
		post.CreatedAt = t.Format(time.RFC3339)            // Keep original for JavaScript
		post.PublicationDate = t.Format("January 2, 2006") // Readable fallback

		// Parse and format PublicationDate if it's different from CreatedAt
		if post.PublicationDate != "" && post.PublicationDate != post.CreatedAt {
			pubDate, pubErr := time.Parse(time.RFC3339, post.PublicationDate)
			if pubErr == nil {
				post.PublicationDate = pubDate.Format("January 2, 2006")
			}
		}

		// Build preview from raw content to preserve Markdown list/numbering, then trim for length
		preview := previewContentRaw(post.Content)
		post.ContentHTML = template.HTML(RenderContent(preview))

		list.Posts = append(list.Posts, post)
	}

	if err != nil {
		return nil, fmt.Errorf("get paginated posts: %w", err)
	} else {
		fmt.Printf("Paginated posts fetched successfully! Limit: %d, Offset: %d\n", limit, offset)
	}

	return &list, nil
}

func (pp *PostService) GetAllPosts() (*PostsList, error) {
	list := PostsList{}

	query := `SELECT p.post_id, p.user_id, u.username, p.category_id, p.title, p.content, p.slug, p.publication_date, p.last_edit_date, p.is_published, p.featured_image_url, p.created_at, p.featured 
			  FROM posts p 
			  JOIN users u ON p.user_id = u.user_id 
			  ORDER BY p.created_at DESC`
	rows, err := pp.DB.Query(query)
	if err != nil {
		return &list, err
	}
	defer rows.Close()

	for rows.Next() {
		var post Post
		err := rows.Scan(&post.ID, &post.UserID, &post.Username, &post.CategoryID, &post.Title, &post.Content, &post.Slug, &post.PublicationDate, &post.LastEditDate, &post.IsPublished, &post.FeaturedImageURL, &post.CreatedAt, &post.Featured)
		if err != nil {
			return nil, err
		}

		// Parse and format CreatedAt
		t, err := time.Parse(time.RFC3339, post.CreatedAt)
		if err != nil {
			fmt.Println(err)
		}
		post.CreatedAt = t.Format(time.RFC3339)
		post.PublicationDate = t.Format("January 2, 2006")

		// Parse and format PublicationDate if it's different from CreatedAt
		if post.PublicationDate != "" && post.PublicationDate != post.CreatedAt {
			pubDate, pubErr := time.Parse(time.RFC3339, post.PublicationDate)
			if pubErr == nil {
				post.PublicationDate = pubDate.Format("January 2, 2006")
			}
		}

		post.Content = trimContent(post.Content)
		list.Posts = append(list.Posts, post)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("get all posts: %w", err)
	}

	return &list, nil
}

func (pp *PostService) GetPostsByUser(userID int) (*PostsList, error) {
	list := PostsList{}

	query := `SELECT post_id, user_id, category_id, title, content, slug, publication_date, last_edit_date, is_published, featured_image_url, created_at, featured FROM posts WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := pp.DB.Query(query, userID)
	if err != nil {
		return &list, err
	}
	defer rows.Close()

	for rows.Next() {
		var post Post
		err := rows.Scan(&post.ID, &post.UserID, &post.CategoryID, &post.Title, &post.Content, &post.Slug, &post.PublicationDate, &post.LastEditDate, &post.IsPublished, &post.FeaturedImageURL, &post.CreatedAt, &post.Featured)
		if err != nil {
			return nil, err
		}

		// Parse and format CreatedAt
		t, err := time.Parse(time.RFC3339, post.CreatedAt)
		if err != nil {
			fmt.Println(err)
		}
		post.CreatedAt = t.Format(time.RFC3339)
		post.PublicationDate = t.Format("January 2, 2006")

		// Parse and format PublicationDate if it's different from CreatedAt
		if post.PublicationDate != "" && post.PublicationDate != post.CreatedAt {
			pubDate, pubErr := time.Parse(time.RFC3339, post.PublicationDate)
			if pubErr == nil {
				post.PublicationDate = pubDate.Format("January 2, 2006")
			}
		}

		post.Content = trimContent(post.Content)
		list.Posts = append(list.Posts, post)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("get posts by user: %w", err)
	}

	return &list, nil
}

// Function to trim content up to the <more--> tag
func trimContent(content string) string {
	// Prefer everything before read-more marker; support escaped version too
	if idx := strings.Index(content, "<more-->"); idx != -1 {
		content = content[:idx]
	} else if idx := strings.Index(content, "&lt;more--&gt;"); idx != -1 {
		content = content[:idx]
	}
	// Remove fenced code blocks ```...```
	fence := regexp.MustCompile("(?s)```.*?```")
	content = fence.ReplaceAllString(content, " ")
	// Remove stray backticks
	content = strings.ReplaceAll(content, "```", " ")
	content = strings.ReplaceAll(content, "`", "")
	// Strip HTML tags
	content = stripHTML(content)
	// Collapse whitespace
	words := strings.Fields(content)
	if len(words) == 0 {
		return ""
	}
	// If there was no read-more, fall back to first N words
	N := 40
	if len(words) > N {
		words = words[:N]
	}
	return strings.Join(words, " ")
}

func stripHTML(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch r {
		case '<':
			in = true
		case '>':
			in = false
		default:
			if !in {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// previewContentRaw returns a trimmed slice of the raw content that preserves
// Markdown structure (numbers, bullets, headings) by cutting on paragraph/line
// boundaries rather than stripping formatting markers first.
func previewContentRaw(content string) string {
	// Check for read-more marker first
	if previewBeforeMore := findContentBeforeMoreTag(content); previewBeforeMore != "" {
		return previewBeforeMore
	}

	// No more tag found, extract preview by length
	return extractPreviewByLength(content)
}

// findContentBeforeMoreTag finds and returns content before the <more--> tag
func findContentBeforeMoreTag(content string) string {
	markers := []string{"<more-->", "<more -->", "&lt;more--&gt;", "&lt;more --&gt;"}

	moreIdx := -1
	for _, marker := range markers {
		if idx := strings.Index(content, marker); idx != -1 && (moreIdx == -1 || idx < moreIdx) {
			moreIdx = idx
		}
	}

	if moreIdx != -1 {
		return strings.TrimSpace(content[:moreIdx])
	}
	return ""
}

// extractPreviewByLength extracts a preview of the content limited to maxChars
func extractPreviewByLength(content string) string {
	const maxChars = 150

	rendered := RenderContent(content)
	plainText := stripHTML(rendered)

	if len(plainText) <= maxChars {
		return strings.TrimSpace(content)
	}

	// Build preview from lines
	return buildPreviewFromLines(content, maxChars)
}

// buildPreviewFromLines builds a preview by iterating through lines
func buildPreviewFromLines(content string, maxChars int) string {
	lines := strings.Split(content, "\n")
	var result strings.Builder
	var currentLength int

	for _, line := range lines {
		if line = strings.TrimSpace(line); line == "" {
			continue
		}

		lineText := stripHTML(RenderContent(line))

		// Handle first line that's too long
		if result.Len() == 0 && len(lineText) > maxChars {
			return truncateFirstLine(line, maxChars)
		}

		if currentLength+len(lineText) > maxChars {
			break
		}

		if result.Len() > 0 {
			result.WriteString("\n\n")
		}
		result.WriteString(line)
		currentLength += len(lineText)
	}

	return strings.TrimSpace(result.String())
}

// truncateFirstLine truncates a long first line at word boundary
func truncateFirstLine(line string, maxChars int) string {
	words := strings.Fields(line)
	var truncated strings.Builder
	var wordLen int

	for _, word := range words {
		wordPlain := stripHTML(RenderContent(word))
		if wordLen+len(wordPlain)+1 > maxChars {
			break
		}
		if truncated.Len() > 0 {
			truncated.WriteString(" ")
		}
		truncated.WriteString(word)
		wordLen += len(wordPlain) + 1
	}

	return strings.TrimSpace(truncated.String()) + "..."
}

func (pp *PostService) Create(userID int, categoryID int, title, content string, isPublished bool, featured bool, featuredImageURL string, slug string) (*Post, error) {
	timefmt := time.Now()
	query := `
		INSERT INTO posts (user_id, category_id, title, content, slug, publication_date, last_edit_date, is_published, featured, featured_image_url, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING post_id
	`
	var postID int
	println(userID, categoryID, title, content, isPublished, featured, featuredImageURL)
	err := pp.DB.QueryRow(query, userID, categoryID, title, content, slug, timefmt,
		timefmt, isPublished, featured, featuredImageURL, timefmt).Scan(&postID)
	if err != nil {
		fmt.Printf("Error: %v", err)
		return nil, fmt.Errorf("create post: %w", err)
	}
	fmt.Println("Post created successfully!")
	fmt.Println(postID)

	return &Post{
		ID:               postID,
		UserID:           userID,
		CategoryID:       categoryID,
		Title:            title,
		Content:          content,
		Slug:             slug,
		PublicationDate:  timefmt.Format("January 2, 2006"),
		LastEditDate:     timefmt.Format("January 2, 2006"),
		IsPublished:      isPublished,
		Featured:         featured,
		FeaturedImageURL: featuredImageURL,
		CreatedAt:        timefmt.Format("January 2, 2006"),
	}, nil
}

func (pp *PostService) GetByID(id int) (*Post, error) {
	var post Post
	row := pp.DB.QueryRow(`SELECT post_id, user_id, category_id, title, content, slug, publication_date, last_edit_date, is_published, featured_image_url, created_at, featured FROM posts WHERE post_id=$1`, id)
	if err := row.Scan(&post.ID, &post.UserID, &post.CategoryID, &post.Title, &post.Content, &post.Slug, &post.PublicationDate, &post.LastEditDate, &post.IsPublished, &post.FeaturedImageURL, &post.CreatedAt, &post.Featured); err != nil {
		return nil, err
	}
	return &post, nil
}

func (pp *PostService) Update(id int, categoryID int, title, content string, isPublished bool, featured bool, featuredImageURL, slug string) error {
	// Fetch existing post to detect slug change
	existing, err := pp.GetByID(id)
	if err != nil {
		return err
	}

	oldSlug := strings.TrimSpace(existing.Slug)
	newSlug := strings.TrimSpace(slug)

	// If slug changed, rename upload dir and rewrite URLs in content and featured image
	if oldSlug != "" && newSlug != "" && oldSlug != newSlug {
		oldDir := filepath.Join("static", "uploads", oldSlug)
		newDir := filepath.Join("static", "uploads", newSlug)
		if _, statErr := os.Stat(oldDir); statErr == nil {
			// Ensure parent exists, then rename dir if possible
			_ = os.MkdirAll(filepath.Dir(newDir), 0o755)
			_ = os.Rename(oldDir, newDir)
		}
		// Update URLs inside content and featured URL
		oldPrefix := "/static/uploads/" + oldSlug + "/"
		newPrefix := "/static/uploads/" + newSlug + "/"
		content = strings.ReplaceAll(content, oldPrefix, newPrefix)
		featuredImageURL = strings.ReplaceAll(featuredImageURL, oldPrefix, newPrefix)
	}

	_, err = pp.DB.Exec(`UPDATE posts SET category_id=$1, title=$2, content=$3, slug=$4, last_edit_date=$5, is_published=$6, featured=$7, featured_image_url=$8 WHERE post_id=$9`,
		categoryID, title, content, newSlug, time.Now(), isPublished, featured, featuredImageURL, id)
	return err
}

// Delete removes a post by ID and cleans up its uploaded images
func (ps PostService) Delete(postID int) error {
	// Get the post slug first for image cleanup
	var slug string
	err := ps.DB.QueryRow("SELECT slug FROM posts WHERE post_id = $1", postID).Scan(&slug)
	if err != nil {
		return fmt.Errorf("post not found: %w", err)
	}

	// Delete the post from database (cascades to post_categories)
	_, err = ps.DB.Exec("DELETE FROM posts WHERE post_id = $1", postID)
	if err != nil {
		return fmt.Errorf("delete post: %w", err)
	}

	// Clean up uploaded images
	if slug != "" {
		for _, subdir := range []string{"featured", "post"} {
			dir := filepath.Join("static", "uploads", subdir, slug)
			os.RemoveAll(dir)
		}
	}

	return nil
}

// RenderContent converts markdown content to HTML using the default renderer
func RenderContent(content string) string {
	renderer := render.NewRenderer(render.DefaultOptions())
	return renderer.Render(content)
}
