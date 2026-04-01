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

	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type SlidesList struct {
	Slides []Slide
}

type Slide struct {
	ID                int
	UserID            int
	Username          string // Username of the author
	AuthorDisplayName string // COALESCE(full_name, username) of the author
	AuthorAvatarURL   string // profile_picture_url of the author
	Title             string
	Slug              string
	ContentFilePath   string
	ContentHTML       template.HTML
	IsPublished       bool
	PasswordHash      string
	SlideMetadata     string // JSONB stored as string
	Description       string
	SlideCount        int
	CreatedAt         string
	UpdatedAt         string
	RelativeTime      string              // For displaying "10 months ago"
	Categories        []Category `json:"categories,omitempty"`
	Contributors      []User     `json:"-"`
}

type SlideService struct {
	DB *sql.DB
}

// MigrateFileContentToDB migrates slide content from files into the DB column.
// Safe to call on every startup — only updates slides with empty DB content.
func (ss *SlideService) MigrateFileContentToDB() {
	// Ensure the content column exists (idempotent)
	ss.DB.Exec(`ALTER TABLE Slides ADD COLUMN IF NOT EXISTS content TEXT DEFAULT ''`)

	// Backfill from files for any slides with empty DB content
	rows, err := ss.DB.Query(`SELECT slide_id, content_file_path FROM Slides WHERE content IS NULL OR content = ''`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var filePath string
		if err := rows.Scan(&id, &filePath); err != nil {
			continue
		}
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		ss.DB.Exec(`UPDATE Slides SET content = $1 WHERE slide_id = $2`, string(content), id)
	}
}

// Create creates a new slide with the given parameters
func (ss *SlideService) Create(userID int, title, slug, content string, isPublished bool, categoryIDs []int, description, metadata, password string) (*Slide, error) {
	// Generate slug if empty
	if slug == "" {
		slug = generateSlug(title)
	} else {
		slug = sanitizeSlug(slug)
	}

	// Create slide directory and content file (best-effort, content is in DB)
	slideDir := filepath.Join("static", "slides", slug)
	os.MkdirAll(slideDir, 0755)
	contentPath := filepath.Join(slideDir, "content.html")
	os.WriteFile(contentPath, []byte(content), 0644) // best-effort, content is in DB

	// Hash password if provided
	var passwordHash string
	if password != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			os.RemoveAll(slideDir)
			return nil, fmt.Errorf("failed to hash password: %v", err)
		}
		passwordHash = string(hashed)
	}

	if metadata == "" {
		metadata = "{}"
	}

	// Insert slide into database (content stored in DB column)
	var slide Slide
	err := ss.DB.QueryRow(`INSERT INTO Slides (user_id, title, slug, content_file_path, content, is_published, description, slide_metadata, password_hash, created_at, updated_at)
			  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			  RETURNING slide_id, created_at, updated_at`,
		userID, title, slug, contentPath, content, isPublished, description, metadata, passwordHash).Scan(
		&slide.ID, &slide.CreatedAt, &slide.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create slide: %v", err)
	}

	slide.UserID = userID
	slide.Title = title
	slide.Slug = slug
	slide.ContentFilePath = contentPath
	slide.IsPublished = isPublished
	slide.Description = description
	slide.SlideMetadata = metadata
	slide.PasswordHash = passwordHash

	// Update search_content for full-text search indexing
	plainText := stripHTML(content)
	ss.DB.Exec(`UPDATE Slides SET search_content = $1 WHERE slide_id = $2`, plainText, slide.ID)

	// Add categories if provided
	if len(categoryIDs) > 0 {
		if err := ss.AddCategories(slide.ID, categoryIDs); err != nil {
			return nil, fmt.Errorf("failed to add categories: %v", err)
		}
	}

	return &slide, nil
}

// GetBySlug retrieves a slide by its slug
func (ss *SlideService) GetBySlug(slug string) (*Slide, error) {
	query := `SELECT s.slide_id, s.user_id, u.username, COALESCE(NULLIF(u.full_name, ''), u.username),
	                 COALESCE(u.profile_picture_url, ''),
	                 s.title, s.slug, s.content_file_path, s.is_published,
	                 COALESCE(s.password_hash, ''), COALESCE(s.slide_metadata::text, '{}'),
	                 COALESCE(s.description, ''), COALESCE(s.slide_count, 0),
	                 s.created_at, s.updated_at
			  FROM Slides s
			  JOIN Users u ON s.user_id = u.user_id
			  WHERE s.slug = $1`

	var slide Slide
	err := ss.DB.QueryRow(query, slug).Scan(
		&slide.ID, &slide.UserID, &slide.Username, &slide.AuthorDisplayName,
		&slide.AuthorAvatarURL,
		&slide.Title, &slide.Slug, &slide.ContentFilePath, &slide.IsPublished,
		&slide.PasswordHash, &slide.SlideMetadata,
		&slide.Description, &slide.SlideCount,
		&slide.CreatedAt, &slide.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("slide not found")
		}
		return nil, fmt.Errorf("failed to get slide: %v", err)
	}

	// Load content from file
	if err := ss.loadSlideContent(&slide); err != nil {
		return nil, err
	}

	// Load categories
	if err := ss.loadSlideCategories(&slide); err != nil {
		return nil, err
	}

	return &slide, nil
}

// GetByID retrieves a slide by its ID
func (ss *SlideService) GetByID(id int) (*Slide, error) {
	query := `SELECT s.slide_id, s.user_id, u.username, COALESCE(NULLIF(u.full_name, ''), u.username),
	                 COALESCE(u.profile_picture_url, ''),
	                 s.title, s.slug, s.content_file_path, s.is_published,
	                 COALESCE(s.password_hash, ''), COALESCE(s.slide_metadata::text, '{}'),
	                 COALESCE(s.description, ''), COALESCE(s.slide_count, 0),
	                 s.created_at, s.updated_at
			  FROM Slides s
			  JOIN Users u ON s.user_id = u.user_id
			  WHERE s.slide_id = $1`

	var slide Slide
	err := ss.DB.QueryRow(query, id).Scan(
		&slide.ID, &slide.UserID, &slide.Username, &slide.AuthorDisplayName,
		&slide.AuthorAvatarURL,
		&slide.Title, &slide.Slug, &slide.ContentFilePath, &slide.IsPublished,
		&slide.PasswordHash, &slide.SlideMetadata,
		&slide.Description, &slide.SlideCount,
		&slide.CreatedAt, &slide.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("slide not found")
		}
		return nil, fmt.Errorf("failed to get slide: %v", err)
	}

	// Load content from file
	if err := ss.loadSlideContent(&slide); err != nil {
		return nil, err
	}

	// Load categories
	if err := ss.loadSlideCategories(&slide); err != nil {
		return nil, err
	}

	return &slide, nil
}

// GetPublishedSlides retrieves all published slides
func (ss *SlideService) GetPublishedSlides() (*SlidesList, error) {
	list := SlidesList{}

	query := `SELECT s.slide_id, s.user_id, u.username, COALESCE(NULLIF(u.full_name, ''), u.username),
	                 COALESCE(u.profile_picture_url, ''),
	                 s.title, s.slug, s.content_file_path, s.is_published,
	                 COALESCE(s.password_hash, ''), COALESCE(s.description, ''), COALESCE(s.slide_count, 0),
	                 s.created_at, s.updated_at
			  FROM Slides s
			  JOIN Users u ON s.user_id = u.user_id
			  WHERE s.is_published = true ORDER BY s.created_at DESC`

	rows, err := ss.DB.Query(query)
	if err != nil {
		return &list, fmt.Errorf("failed to query slides: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var slide Slide
		err := rows.Scan(&slide.ID, &slide.UserID, &slide.Username, &slide.AuthorDisplayName,
			&slide.AuthorAvatarURL,
			&slide.Title, &slide.Slug, &slide.ContentFilePath, &slide.IsPublished,
			&slide.PasswordHash, &slide.Description, &slide.SlideCount,
			&slide.CreatedAt, &slide.UpdatedAt)
		if err != nil {
			return &list, fmt.Errorf("failed to scan slide: %v", err)
		}
		list.Slides = append(list.Slides, slide)
	}

	// Batch-load categories and contributors for all slides
	if err := ss.loadCategoriesForSlides(list.Slides); err != nil {
		return &list, err
	}
	if err := ss.loadContributorsForSlides(list.Slides); err != nil {
		return &list, err
	}

	return &list, nil
}

// GetPublishedSlidesByCategory returns published slides that belong to a specific category.
func (ss *SlideService) GetPublishedSlidesByCategory(categoryID int) ([]Slide, error) {
	query := `SELECT s.slide_id, s.user_id, u.username, COALESCE(NULLIF(u.full_name, ''), u.username),
	                 COALESCE(u.profile_picture_url, ''),
	                 s.title, s.slug, s.content_file_path, s.is_published,
	                 COALESCE(s.password_hash, ''), COALESCE(s.description, ''), COALESCE(s.slide_count, 0),
	                 s.created_at, s.updated_at
			  FROM Slides s
			  JOIN Users u ON s.user_id = u.user_id
			  JOIN Slide_Categories sc ON s.slide_id = sc.slide_id
			  WHERE sc.category_id = $1 AND s.is_published = true
			  ORDER BY s.created_at DESC`
	rows, err := ss.DB.Query(query, categoryID)
	if err != nil {
		return nil, fmt.Errorf("query slides by category: %w", err)
	}
	defer rows.Close()

	var slides []Slide
	for rows.Next() {
		var slide Slide
		err := rows.Scan(&slide.ID, &slide.UserID, &slide.Username, &slide.AuthorDisplayName,
			&slide.AuthorAvatarURL,
			&slide.Title, &slide.Slug, &slide.ContentFilePath, &slide.IsPublished,
			&slide.PasswordHash, &slide.Description, &slide.SlideCount,
			&slide.CreatedAt, &slide.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan slide by category: %w", err)
		}
		slides = append(slides, slide)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate slides by category: %w", err)
	}
	if err := ss.loadCategoriesForSlides(slides); err != nil {
		return nil, err
	}
	return slides, nil
}

// GetPublishedSlidesByUser returns published slides authored by a specific user.
func (ss *SlideService) GetPublishedSlidesByUser(userID int) ([]Slide, error) {
	rows, err := ss.DB.Query(`
		SELECT s.slide_id, s.title, s.slug, COALESCE(s.description, ''), COALESCE(s.slide_count, 0), s.created_at
		FROM Slides s
		WHERE s.user_id = $1 AND s.is_published = true
		ORDER BY s.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var slides []Slide
	for rows.Next() {
		var s Slide
		if err := rows.Scan(&s.ID, &s.Title, &s.Slug, &s.Description, &s.SlideCount, &s.CreatedAt); err != nil {
			return nil, err
		}
		slides = append(slides, s)
	}
	return slides, rows.Err()
}

// GetAllSlides retrieves all slides (for admin)
func (ss *SlideService) GetAllSlides() (*SlidesList, error) {
	list := SlidesList{}

	query := `SELECT s.slide_id, s.user_id, u.username, COALESCE(NULLIF(u.full_name, ''), u.username),
	                 COALESCE(u.profile_picture_url, ''),
	                 s.title, s.slug, s.content_file_path, s.is_published,
	                 COALESCE(s.password_hash, ''), COALESCE(s.description, ''), COALESCE(s.slide_count, 0),
	                 s.created_at, s.updated_at
			  FROM Slides s
			  JOIN Users u ON s.user_id = u.user_id
			  ORDER BY s.created_at DESC`

	rows, err := ss.DB.Query(query)
	if err != nil {
		return &list, fmt.Errorf("failed to query slides: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var slide Slide
		err := rows.Scan(&slide.ID, &slide.UserID, &slide.Username, &slide.AuthorDisplayName,
			&slide.AuthorAvatarURL,
			&slide.Title, &slide.Slug, &slide.ContentFilePath, &slide.IsPublished,
			&slide.PasswordHash, &slide.Description, &slide.SlideCount,
			&slide.CreatedAt, &slide.UpdatedAt)
		if err != nil {
			return &list, fmt.Errorf("failed to scan slide: %v", err)
		}
		list.Slides = append(list.Slides, slide)
	}

	// Batch-load categories and contributors for all slides
	if err := ss.loadCategoriesForSlides(list.Slides); err != nil {
		return &list, err
	}
	if err := ss.loadContributorsForSlides(list.Slides); err != nil {
		return &list, err
	}

	return &list, nil
}

// Update updates an existing slide
func (ss *SlideService) Update(slideID int, title, slug, content string, isPublished bool, categoryIDs []int, description, metadata, password string) error {
	// Get current slide to access file path
	currentSlide, err := ss.GetByID(slideID)
	if err != nil {
		return err
	}

	// Sanitize slug
	if slug == "" {
		slug = generateSlug(title)
	} else {
		slug = sanitizeSlug(slug)
	}

	// Write content file as backup (best-effort, may fail in containers)
	if currentSlide.ContentFilePath != "" {
		os.MkdirAll(filepath.Dir(currentSlide.ContentFilePath), 0755)
		os.WriteFile(currentSlide.ContentFilePath, []byte(content), 0644)
	}

	// Handle password: empty string means remove, non-empty means set new
	passwordHash := currentSlide.PasswordHash
	if password == "__remove__" {
		passwordHash = ""
	} else if password != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash password: %v", err)
		}
		passwordHash = string(hashed)
	}

	if metadata == "" {
		metadata = currentSlide.SlideMetadata
	}

	// Update database record (content stored in DB column)
	_, err = ss.DB.Exec(`UPDATE Slides SET title = $1, slug = $2, content = $3, is_published = $4, description = $5,
	          slide_metadata = $6, password_hash = $7, updated_at = CURRENT_TIMESTAMP
			  WHERE slide_id = $8`, title, slug, content, isPublished, description, metadata, passwordHash, slideID)
	if err != nil {
		return fmt.Errorf("failed to update slide: %v", err)
	}

	// Update search_content for full-text search indexing
	plainText := stripHTML(content)
	ss.DB.Exec(`UPDATE Slides SET search_content = $1 WHERE slide_id = $2`, plainText, slideID)

	// Update categories
	if err := ss.UpdateCategories(slideID, categoryIDs); err != nil {
		return fmt.Errorf("failed to update categories: %v", err)
	}

	return nil
}

// Delete deletes a slide and its associated files
func (ss *SlideService) Delete(slideID int) error {
	// Get slide to access file path
	slide, err := ss.GetByID(slideID)
	if err != nil {
		return err
	}

	// Delete from database first
	query := `DELETE FROM Slides WHERE slide_id = $1`
	_, err = ss.DB.Exec(query, slideID)
	if err != nil {
		return fmt.Errorf("failed to delete slide from database: %v", err)
	}

	// Remove slide directory
	slideDir := filepath.Dir(slide.ContentFilePath)
	if err := os.RemoveAll(slideDir); err != nil {
		return fmt.Errorf("failed to remove slide directory: %v", err)
	}

	return nil
}

// AddCategories adds categories to a slide using a single multi-row insert.
func (ss *SlideService) AddCategories(slideID int, categoryIDs []int) error {
	if len(categoryIDs) == 0 {
		return nil
	}
	query := `INSERT INTO Slide_Categories (slide_id, category_id)
			  SELECT $1, unnest($2::int[]) ON CONFLICT DO NOTHING`
	_, err := ss.DB.Exec(query, slideID, pq.Array(categoryIDs))
	if err != nil {
		return fmt.Errorf("failed to add categories: %v", err)
	}
	return nil
}

// UpdateCategories updates the categories associated with a slide
func (ss *SlideService) UpdateCategories(slideID int, categoryIDs []int) error {
	// Remove existing categories
	deleteQuery := `DELETE FROM Slide_Categories WHERE slide_id = $1`
	_, err := ss.DB.Exec(deleteQuery, slideID)
	if err != nil {
		return fmt.Errorf("failed to remove existing categories: %v", err)
	}

	// Add new categories
	if len(categoryIDs) > 0 {
		return ss.AddCategories(slideID, categoryIDs)
	}

	return nil
}

// loadSlideContent loads HTML content from DB. Falls back to file for migration.
func (ss *SlideService) loadSlideContent(slide *Slide) error {
	var dbContent string
	err := ss.DB.QueryRow(`SELECT COALESCE(content, '') FROM Slides WHERE slide_id = $1`, slide.ID).Scan(&dbContent)
	if err == nil && dbContent != "" {
		slide.ContentHTML = template.HTML(dbContent)
		return nil
	}

	// Fallback: read from file and migrate to DB
	content, err := os.ReadFile(slide.ContentFilePath)
	if err != nil {
		slide.ContentHTML = ""
		return nil
	}
	slide.ContentHTML = template.HTML(content)

	// Migrate file content into DB
	ss.DB.Exec(`UPDATE Slides SET content = $1 WHERE slide_id = $2 AND (content IS NULL OR content = '')`,
		string(content), slide.ID)

	return nil
}

// loadSlideCategories loads the categories for a slide
func (ss *SlideService) loadSlideCategories(slide *Slide) error {
	query := `SELECT c.category_id, c.category_name, c.created_at 
			  FROM Categories c 
			  JOIN Slide_Categories sc ON c.category_id = sc.category_id 
			  WHERE sc.slide_id = $1`
	
	rows, err := ss.DB.Query(query, slide.ID)
	if err != nil {
		return fmt.Errorf("failed to load categories: %v", err)
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var category Category
		err := rows.Scan(&category.ID, &category.Name, &category.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to scan category: %v", err)
		}
		categories = append(categories, category)
	}

	slide.Categories = categories
	return nil
}

// loadCategoriesForSlides batch-loads categories for all slides in a single query.
func (ss *SlideService) loadCategoriesForSlides(slides []Slide) error {
	if len(slides) == 0 {
		return nil
	}
	ids := make([]int, len(slides))
	idIdx := make(map[int][]int)
	for i, s := range slides {
		ids[i] = s.ID
		idIdx[s.ID] = append(idIdx[s.ID], i)
	}

	query := `SELECT sc.slide_id, c.category_id, c.category_name, c.created_at
			  FROM Categories c
			  JOIN Slide_Categories sc ON c.category_id = sc.category_id
			  WHERE sc.slide_id = ANY($1)`
	rows, err := ss.DB.Query(query, pq.Array(ids))
	if err != nil {
		return fmt.Errorf("failed to batch-load slide categories: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var slideID int
		var cat Category
		if err := rows.Scan(&slideID, &cat.ID, &cat.Name, &cat.CreatedAt); err != nil {
			return err
		}
		for _, idx := range idIdx[slideID] {
			slides[idx].Categories = append(slides[idx].Categories, cat)
		}
	}
	return rows.Err()
}

// SetPassword hashes and sets the password on a slide.
func (s *Slide) SetPassword(plaintext string) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	s.PasswordHash = string(hashed)
	return nil
}

// CheckPassword checks a plaintext password against the slide's hash.
func (s *Slide) CheckPassword(plaintext string) bool {
	if s.PasswordHash == "" {
		return true
	}
	return bcrypt.CompareHashAndPassword([]byte(s.PasswordHash), []byte(plaintext)) == nil
}

// loadContributorsForSlides batch-loads contributors for all slides in a single query.
func (ss *SlideService) loadContributorsForSlides(slides []Slide) error {
	if len(slides) == 0 {
		return nil
	}
	ids := make([]int, len(slides))
	idIdx := make(map[int][]int)
	for i, s := range slides {
		ids[i] = s.ID
		idIdx[s.ID] = append(idIdx[s.ID], i)
	}

	rows, err := ss.DB.Query(`
		SELECT sc.slide_id, u.user_id, u.username, COALESCE(NULLIF(u.full_name, ''), u.username),
		       COALESCE(u.profile_picture_url, '')
		FROM slide_contributors sc
		JOIN Users u ON u.user_id = sc.user_id
		JOIN Slides s ON s.slide_id = sc.slide_id
		WHERE sc.slide_id = ANY($1)
		  AND sc.user_id != s.user_id
		ORDER BY sc.first_contributed_at ASC
	`, pq.Array(ids))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var slideID int
		var u User
		if err := rows.Scan(&slideID, &u.UserID, &u.Username, &u.FullName, &u.AvatarURL); err != nil {
			return err
		}
		for _, idx := range idIdx[slideID] {
			slides[idx].Contributors = append(slides[idx].Contributors, u)
		}
	}
	return rows.Err()
}

// UpdateSlideCount updates the slide_count column for a slide.
func (ss *SlideService) UpdateSlideCount(slideID, count int) error {
	_, err := ss.DB.Exec(`UPDATE Slides SET slide_count = $1 WHERE slide_id = $2`, count, slideID)
	return err
}

// Slug sanitisation regexes, compiled once.
var (
	slugNonAlnumRe = regexp.MustCompile(`[^a-z0-9]+`)
	slugNonAlnumDashRe = regexp.MustCompile(`[^a-z0-9-]`)
)

// Helper functions
func generateSlug(title string) string {
	// Convert to lowercase and replace spaces with hyphens
	slug := strings.ToLower(title)
	slug = slugNonAlnumRe.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	
	// Ensure slug is not empty
	if slug == "" {
		slug = fmt.Sprintf("slide-%d", time.Now().Unix())
	}
	
	return slug
}

func sanitizeSlug(slug string) string {
	// Convert to lowercase and sanitize
	slug = strings.ToLower(slug)
	slug = slugNonAlnumDashRe.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	
	// Ensure slug is not empty
	if slug == "" {
		slug = fmt.Sprintf("slide-%d", time.Now().Unix())
	}
	
	return slug
}