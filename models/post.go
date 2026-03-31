package models

import (
	"database/sql"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	gowiki "github.com/anchoo2kewl/go-wiki"
	"github.com/lib/pq"
)

// renderCache caches the rendered HTML output of RenderContent keyed by content hash.
// This avoids re-running the markdown pipeline on every request.
// Entries are never evicted (blog posts are stable after publish); the cache
// stays small — one entry per unique post body.
var renderCache sync.Map // map[string]string — content → rendered HTML

const friendlyDateFormat = "January 2, 2006"

type PostsList struct {
	Posts []Post
}

type Post struct {
	ID                  int
	UserID              int // Added UserID field
	Username            string // Username of the author
	AuthorDisplayName   string // COALESCE(full_name, username) of the author
	AuthorAvatarURL     string // profile_picture_url of the author
	AuthorBio           string // bio of the author
	CategoryID          int // Legacy field, kept for backward compatibility
	Title               string
	Content             string
	ContentHTML         template.HTML
	Slug                string
	PublicationDate     string
	LastEditDate        string
	IsPublished         bool
	Featured            bool   // Boolean field to mark posts as featured
	FeaturedImageURL    string
	CreatedAt           string
	ReadingTime         int        `json:"reading_time,omitempty"` // Estimated minutes to read
	Categories          []Category `json:"categories,omitempty"`   // New many-to-many categories
	Contributors        []User     `json:"-"`                      // All users who have edited this post
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

	query := `SELECT p.post_id, p.user_id, u.username, COALESCE(NULLIF(u.full_name, ''), u.username), p.category_id, p.title, p.content, p.slug, p.publication_date, p.last_edit_date, p.is_published, p.featured_image_url, p.created_at, p.featured FROM posts p JOIN users u ON p.user_id = u.user_id WHERE p.is_published = true ORDER BY p.created_at DESC LIMIT 5`
	rows, err := pp.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query top posts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var post Post
		err := rows.Scan(&post.ID, &post.UserID, &post.Username, &post.AuthorDisplayName, &post.CategoryID, &post.Title, &post.Content, &post.Slug, &post.PublicationDate, &post.LastEditDate, &post.IsPublished, &post.FeaturedImageURL, &post.CreatedAt, &post.Featured)
		if err != nil {
			return nil, fmt.Errorf("scan top posts: %w", err)
		}

		formatPostDates(&post)
		list.Posts = append(list.Posts, post)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get top posts: %w", err)
	}

	// Batch-load categories for all posts
	if err := pp.loadCategoriesForPosts(list.Posts); err != nil {
		return nil, fmt.Errorf("load categories: %w", err)
	}

	// Calculate reading time and render previews
	for i := range list.Posts {
		wordCount := len(strings.Fields(list.Posts[i].Content))
		list.Posts[i].ReadingTime = (wordCount + 199) / 200
		if list.Posts[i].ReadingTime < 1 {
			list.Posts[i].ReadingTime = 1
		}
		preview := previewContentRaw(list.Posts[i].Content)
		list.Posts[i].ContentHTML = template.HTML(RenderContent(preview))
	}

	return &list, nil
}

func (pp *PostService) GetTopPostsWithPagination(limit int, offset int) (*PostsList, error) {
	list := PostsList{}

	query := `SELECT p.post_id, p.user_id, u.username, COALESCE(NULLIF(u.full_name, ''), u.username), p.category_id, p.title, p.content, p.slug, p.publication_date, p.last_edit_date, p.is_published, p.featured_image_url, p.created_at, p.featured FROM posts p JOIN users u ON p.user_id = u.user_id WHERE p.is_published = true ORDER BY p.created_at DESC LIMIT $1 OFFSET $2`
	rows, err := pp.DB.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query paginated posts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var post Post
		err := rows.Scan(&post.ID, &post.UserID, &post.Username, &post.AuthorDisplayName, &post.CategoryID, &post.Title, &post.Content, &post.Slug, &post.PublicationDate, &post.LastEditDate, &post.IsPublished, &post.FeaturedImageURL, &post.CreatedAt, &post.Featured)
		if err != nil {
			return nil, fmt.Errorf("scan paginated posts: %w", err)
		}

		formatPostDates(&post)
		list.Posts = append(list.Posts, post)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get paginated posts: %w", err)
	}

	// Batch-load categories for all posts
	if err := pp.loadCategoriesForPosts(list.Posts); err != nil {
		return nil, fmt.Errorf("load categories: %w", err)
	}

	// Calculate reading time and render previews
	for i := range list.Posts {
		wordCount := len(strings.Fields(list.Posts[i].Content))
		list.Posts[i].ReadingTime = (wordCount + 199) / 200
		if list.Posts[i].ReadingTime < 1 {
			list.Posts[i].ReadingTime = 1
		}
		preview := previewContentRaw(list.Posts[i].Content)
		list.Posts[i].ContentHTML = template.HTML(RenderContent(preview))
	}

	return &list, nil
}

// GetPublishedPostsByCategory returns published posts that belong to a specific category.
func (pp *PostService) GetPublishedPostsByCategory(categoryID int) ([]Post, error) {
	query := `SELECT p.post_id, p.user_id, u.username, COALESCE(NULLIF(u.full_name, ''), u.username),
	                 COALESCE(u.profile_picture_url, ''),
	                 p.category_id, p.title, p.content, p.slug,
	                 p.publication_date, p.last_edit_date, p.is_published,
	                 p.featured_image_url, p.created_at, p.featured
			  FROM posts p
			  JOIN users u ON p.user_id = u.user_id
			  JOIN post_categories pc ON p.post_id = pc.post_id
			  WHERE pc.category_id = $1 AND p.is_published = true
			  ORDER BY p.created_at DESC`
	rows, err := pp.DB.Query(query, categoryID)
	if err != nil {
		return nil, fmt.Errorf("query posts by category: %w", err)
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var post Post
		err := rows.Scan(&post.ID, &post.UserID, &post.Username, &post.AuthorDisplayName,
			&post.AuthorAvatarURL,
			&post.CategoryID, &post.Title, &post.Content, &post.Slug,
			&post.PublicationDate, &post.LastEditDate, &post.IsPublished,
			&post.FeaturedImageURL, &post.CreatedAt, &post.Featured)
		if err != nil {
			return nil, fmt.Errorf("scan post by category: %w", err)
		}
		formatPostDates(&post)
		wordCount := len(strings.Fields(post.Content))
		post.ReadingTime = (wordCount + 199) / 200
		if post.ReadingTime < 1 {
			post.ReadingTime = 1
		}
		preview := previewContentRaw(post.Content)
		post.ContentHTML = template.HTML(RenderContent(preview))
		posts = append(posts, post)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate posts by category: %w", err)
	}
	if err := pp.loadCategoriesForPosts(posts); err != nil {
		return nil, fmt.Errorf("load categories: %w", err)
	}
	return posts, nil
}

func (pp *PostService) GetAllPosts() (*PostsList, error) {
	list := PostsList{}

	query := `SELECT p.post_id, p.user_id, u.username, COALESCE(NULLIF(u.full_name, ''), u.username), p.category_id, p.title, p.content, p.slug, p.publication_date, p.last_edit_date, p.is_published, p.featured_image_url, p.created_at, p.featured
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
		err := rows.Scan(&post.ID, &post.UserID, &post.Username, &post.AuthorDisplayName, &post.CategoryID, &post.Title, &post.Content, &post.Slug, &post.PublicationDate, &post.LastEditDate, &post.IsPublished, &post.FeaturedImageURL, &post.CreatedAt, &post.Featured)
		if err != nil {
			return nil, err
		}

		formatPostDates(&post)
		post.Content = trimContent(post.Content)
		list.Posts = append(list.Posts, post)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("get all posts: %w", err)
	}

	if err := pp.loadCategoriesForPosts(list.Posts); err != nil {
		return nil, fmt.Errorf("load categories: %w", err)
	}

	if err := pp.loadContributorsForPosts(list.Posts); err != nil {
		return nil, fmt.Errorf("load contributors: %w", err)
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

		formatPostDates(&post)
		post.Content = trimContent(post.Content)
		list.Posts = append(list.Posts, post)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("get posts by user: %w", err)
	}

	return &list, nil
}

func (pp *PostService) GetPublishedPostsByUser(userID int) ([]Post, error) {
	query := `SELECT post_id, user_id, category_id, title, content, slug, publication_date, last_edit_date, is_published, featured_image_url, created_at, featured FROM posts WHERE user_id = $1 AND is_published = true ORDER BY created_at DESC`
	rows, err := pp.DB.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var post Post
		err := rows.Scan(&post.ID, &post.UserID, &post.CategoryID, &post.Title, &post.Content, &post.Slug, &post.PublicationDate, &post.LastEditDate, &post.IsPublished, &post.FeaturedImageURL, &post.CreatedAt, &post.Featured)
		if err != nil {
			return nil, err
		}
		formatPostDates(&post)
		post.Content = trimContent(post.Content)
		posts = append(posts, post)
	}
	return posts, rows.Err()
}

// fenceRe matches fenced code blocks for stripping during content trimming.
var fenceRe = regexp.MustCompile("(?s)```.*?```")

// Function to trim content up to the <more--> tag
func trimContent(content string) string {
	// Prefer everything before read-more marker; support escaped version too
	if idx := strings.Index(content, "<more-->"); idx != -1 {
		content = content[:idx]
	} else if idx := strings.Index(content, "&lt;more--&gt;"); idx != -1 {
		content = content[:idx]
	}
	// Remove fenced code blocks ```...```
	content = fenceRe.ReplaceAllString(content, " ")
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
	inTag := false
	for _, r := range s {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// formatPostDates normalises date fields for list pages:
//   - CreatedAt stays RFC3339 (consumed by JavaScript)
//   - PublicationDate and LastEditDate become human-friendly strings
func formatPostDates(post *Post) {
	if t, err := time.Parse(time.RFC3339, post.CreatedAt); err == nil {
		post.CreatedAt = t.Format(time.RFC3339)
		post.PublicationDate = t.Format(friendlyDateFormat)
	}
	if post.PublicationDate != "" && post.PublicationDate != post.CreatedAt {
		if t, err := time.Parse(time.RFC3339, post.PublicationDate); err == nil {
			post.PublicationDate = t.Format(friendlyDateFormat)
		}
	}
	if post.LastEditDate != "" {
		if t, err := time.Parse(time.RFC3339, post.LastEditDate); err == nil {
			post.LastEditDate = t.Format(friendlyDateFormat)
		}
	}
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
	err := pp.DB.QueryRow(query, userID, categoryID, title, content, slug, timefmt,
		timefmt, isPublished, featured, featuredImageURL, timefmt).Scan(&postID)
	if err != nil {
		return nil, fmt.Errorf("create post: %w", err)
	}

	return &Post{
		ID:               postID,
		UserID:           userID,
		CategoryID:       categoryID,
		Title:            title,
		Content:          content,
		Slug:             slug,
		PublicationDate:  timefmt.Format(friendlyDateFormat),
		LastEditDate:     timefmt.Format(friendlyDateFormat),
		IsPublished:      isPublished,
		Featured:         featured,
		FeaturedImageURL: featuredImageURL,
		CreatedAt:        timefmt.Format(friendlyDateFormat),
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

// defaultWiki is a singleton to avoid re-allocating on every render call.
var defaultWiki = gowiki.New(gowiki.WithDrawBasePath("/draw"))

// drawEditSrcRe matches data-src attributes ending in /edit inside godraw-embed divs.
var drawEditSrcRe = regexp.MustCompile(`(data-src="[^"]+)/edit"`)

// stripDrawEditMode removes /edit from go-draw embed URLs so that
// published content always renders a read-only canvas viewer.
func stripDrawEditMode(html string) string {
	return drawEditSrcRe.ReplaceAllString(html, `$1"`)
}

// RenderContent converts markdown content to HTML using the default renderer.
// Draw embeds are forced to view-only mode for published content.
// Results are memoized in renderCache to avoid re-running the markdown
// pipeline on repeated requests for the same content.
func RenderContent(content string) string {
	if v, ok := renderCache.Load(content); ok {
		return v.(string)
	}
	result := stripDrawEditMode(defaultWiki.RenderContent(content))
	renderCache.Store(content, result)
	return result
}

// loadCategoriesForPosts batch-loads categories for all posts in a single query,
// replacing the N+1 pattern of querying categories per-post inside a loop.
func (pp *PostService) loadCategoriesForPosts(posts []Post) error {
	if len(posts) == 0 {
		return nil
	}
	ids := make([]int, len(posts))
	idIdx := make(map[int][]int) // post_id -> indices in posts slice
	for i, p := range posts {
		ids[i] = p.ID
		idIdx[p.ID] = append(idIdx[p.ID], i)
	}

	query := `SELECT pc.post_id, c.category_id, c.category_name, c.created_at
			  FROM categories c
			  JOIN post_categories pc ON c.category_id = pc.category_id
			  WHERE pc.post_id = ANY($1)`
	rows, err := pp.DB.Query(query, pq.Array(ids))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var postID int
		var cat Category
		if err := rows.Scan(&postID, &cat.ID, &cat.Name, &cat.CreatedAt); err != nil {
			return err
		}
		for _, idx := range idIdx[postID] {
			posts[idx].Categories = append(posts[idx].Categories, cat)
		}
	}
	return rows.Err()
}

// loadContributorsForPosts batch-loads contributors for all posts in a single query.
func (pp *PostService) loadContributorsForPosts(posts []Post) error {
	if len(posts) == 0 {
		return nil
	}
	ids := make([]int, len(posts))
	idIdx := make(map[int][]int)
	for i, p := range posts {
		ids[i] = p.ID
		idIdx[p.ID] = append(idIdx[p.ID], i)
	}

	rows, err := pp.DB.Query(`
		SELECT pc.post_id, u.user_id, u.username, COALESCE(NULLIF(u.full_name, ''), u.username)
		FROM post_contributors pc
		JOIN Users u ON u.user_id = pc.user_id
		WHERE pc.post_id = ANY($1)
		ORDER BY pc.first_contributed_at ASC
	`, pq.Array(ids))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var postID int
		var u User
		if err := rows.Scan(&postID, &u.UserID, &u.Username, &u.FullName); err != nil {
			return err
		}
		for _, idx := range idIdx[postID] {
			posts[idx].Contributors = append(posts[idx].Contributors, u)
		}
	}
	return rows.Err()
}
