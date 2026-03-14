package models

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type WikiPage struct {
	ID        int       `json:"page_id"`
	UserID    int       `json:"user_id"`
	Title     string    `json:"title"`
	Slug      string    `json:"slug"`
	Content   string    `json:"content,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WikiPageVersion struct {
	ID            int       `json:"version_id"`
	PageID        int       `json:"page_id"`
	VersionNumber int       `json:"version_number"`
	Content       string    `json:"content,omitempty"`
	ContentHash   string    `json:"content_hash"`
	CreatedBy     int       `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
}

type WikiPageService struct {
	DB *sql.DB
}

var wikiSlugRe = regexp.MustCompile(`[^a-z0-9]+`)

func wikiSlug(title string) string {
	slug := strings.ToLower(title)
	slug = wikiSlugRe.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 100 {
		slug = slug[:100]
		slug = strings.TrimRight(slug, "-")
	}
	if slug == "" {
		slug = fmt.Sprintf("page-%d", time.Now().Unix())
	}
	return slug
}

func (ws *WikiPageService) ensureUniqueSlug(slug string, excludeID int) (string, error) {
	base := slug
	for i := 0; i < 100; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", base, i)
		}
		var exists bool
		err := ws.DB.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM wiki_pages WHERE slug = $1 AND page_id != $2)`,
			candidate, excludeID,
		).Scan(&exists)
		if err != nil {
			return "", fmt.Errorf("check slug uniqueness: %w", err)
		}
		if !exists {
			return candidate, nil
		}
	}
	return fmt.Sprintf("%s-%d", base, time.Now().UnixNano()), nil
}

// Create creates a new wiki page.
func (ws *WikiPageService) Create(userID int, title, content string) (*WikiPage, error) {
	slug := wikiSlug(title)
	slug, err := ws.ensureUniqueSlug(slug, 0)
	if err != nil {
		return nil, err
	}

	var page WikiPage
	err = ws.DB.QueryRow(`
		INSERT INTO wiki_pages (user_id, title, slug, content, created_at, updated_at)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING page_id, user_id, title, slug, content, created_at, updated_at
	`, userID, title, slug, content).Scan(
		&page.ID, &page.UserID, &page.Title, &page.Slug, &page.Content,
		&page.CreatedAt, &page.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create wiki page: %w", err)
	}

	// Create initial version
	_ = ws.MaybeCreateVersion(page.ID, userID, content, true)

	return &page, nil
}

// GetByID returns a wiki page by ID including content.
func (ws *WikiPageService) GetByID(id int) (*WikiPage, error) {
	var page WikiPage
	err := ws.DB.QueryRow(`
		SELECT page_id, user_id, title, slug, content, created_at, updated_at
		FROM wiki_pages WHERE page_id = $1
	`, id).Scan(&page.ID, &page.UserID, &page.Title, &page.Slug, &page.Content,
		&page.CreatedAt, &page.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("wiki page not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get wiki page: %w", err)
	}
	return &page, nil
}

// GetAll returns all wiki pages without content, ordered by updated_at DESC.
func (ws *WikiPageService) GetAll() ([]WikiPage, error) {
	rows, err := ws.DB.Query(`
		SELECT page_id, user_id, title, slug, created_at, updated_at
		FROM wiki_pages ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list wiki pages: %w", err)
	}
	defer rows.Close()

	var pages []WikiPage
	for rows.Next() {
		var p WikiPage
		if err := rows.Scan(&p.ID, &p.UserID, &p.Title, &p.Slug, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan wiki page: %w", err)
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}

// Update updates title and/or content. Regenerates slug if title changed.
func (ws *WikiPageService) Update(id int, title, content string) error {
	existing, err := ws.GetByID(id)
	if err != nil {
		return err
	}

	newTitle := title
	if newTitle == "" {
		newTitle = existing.Title
	}
	newContent := content
	if newContent == "" {
		newContent = existing.Content
	}

	slug := existing.Slug
	if newTitle != existing.Title {
		slug = wikiSlug(newTitle)
		slug, err = ws.ensureUniqueSlug(slug, id)
		if err != nil {
			return err
		}
	}

	_, err = ws.DB.Exec(`
		UPDATE wiki_pages SET title = $1, slug = $2, content = $3, updated_at = CURRENT_TIMESTAMP
		WHERE page_id = $4
	`, newTitle, slug, newContent, id)
	if err != nil {
		return fmt.Errorf("update wiki page: %w", err)
	}
	return nil
}

// UpdateContent updates only the content of a wiki page.
func (ws *WikiPageService) UpdateContent(id int, content string) error {
	_, err := ws.DB.Exec(`
		UPDATE wiki_pages SET content = $1, updated_at = CURRENT_TIMESTAMP
		WHERE page_id = $2
	`, content, id)
	if err != nil {
		return fmt.Errorf("update wiki page content: %w", err)
	}
	return nil
}

// Delete deletes a wiki page (versions cascade).
func (ws *WikiPageService) Delete(id int) error {
	result, err := ws.DB.Exec(`DELETE FROM wiki_pages WHERE page_id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete wiki page: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("wiki page not found")
	}
	return nil
}

// Search searches pages by title or content using ILIKE.
func (ws *WikiPageService) Search(query string, limit int) ([]WikiPage, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	pattern := "%" + query + "%"
	rows, err := ws.DB.Query(`
		SELECT page_id, user_id, title, slug, created_at, updated_at
		FROM wiki_pages
		WHERE title ILIKE $1 OR content ILIKE $1
		ORDER BY updated_at DESC
		LIMIT $2
	`, pattern, limit)
	if err != nil {
		return nil, fmt.Errorf("search wiki pages: %w", err)
	}
	defer rows.Close()

	var pages []WikiPage
	for rows.Next() {
		var p WikiPage
		if err := rows.Scan(&p.ID, &p.UserID, &p.Title, &p.Slug, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan wiki search result: %w", err)
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}

// Autocomplete returns pages matching title prefix (for quick lookup).
func (ws *WikiPageService) Autocomplete(query string, limit int) ([]WikiPage, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	pattern := "%" + query + "%"
	rows, err := ws.DB.Query(`
		SELECT page_id, title, slug
		FROM wiki_pages
		WHERE title ILIKE $1
		ORDER BY updated_at DESC
		LIMIT $2
	`, pattern, limit)
	if err != nil {
		return nil, fmt.Errorf("autocomplete wiki pages: %w", err)
	}
	defer rows.Close()

	var pages []WikiPage
	for rows.Next() {
		var p WikiPage
		if err := rows.Scan(&p.ID, &p.Title, &p.Slug); err != nil {
			return nil, fmt.Errorf("scan autocomplete result: %w", err)
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}

// GetContent returns only the content of a wiki page.
func (ws *WikiPageService) GetContent(id int) (string, error) {
	var content string
	err := ws.DB.QueryRow(`SELECT content FROM wiki_pages WHERE page_id = $1`, id).Scan(&content)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("wiki page not found")
	}
	if err != nil {
		return "", fmt.Errorf("get wiki page content: %w", err)
	}
	return content, nil
}

// MaybeCreateVersion creates a version snapshot if content changed significantly.
// If manualSave is true, always creates a version (ignoring significance threshold).
func (ws *WikiPageService) MaybeCreateVersion(pageID, userID int, content string, manualSave bool) error {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))

	var lastHash string
	var lastLen int
	var lastVersionNumber int
	err := ws.DB.QueryRow(`
		SELECT content_hash, LENGTH(content), version_number
		FROM wiki_page_versions
		WHERE page_id = $1
		ORDER BY version_number DESC
		LIMIT 1
	`, pageID).Scan(&lastHash, &lastLen, &lastVersionNumber)

	if err == sql.ErrNoRows {
		// First version
		_, err = ws.DB.Exec(`
			INSERT INTO wiki_page_versions (page_id, version_number, content, content_hash, created_by, created_at)
			VALUES ($1, 1, $2, $3, $4, NOW())
			ON CONFLICT (page_id, version_number) DO NOTHING
		`, pageID, content, hash, userID)
		return err
	}
	if err != nil {
		return fmt.Errorf("query last wiki version: %w", err)
	}

	if lastHash == hash {
		return nil
	}

	if !manualSave && !isSignificantChange(lastLen, len(content)) {
		return nil
	}

	_, err = ws.DB.Exec(`
		INSERT INTO wiki_page_versions (page_id, version_number, content, content_hash, created_by, created_at)
		VALUES ($1, (SELECT COALESCE(MAX(version_number), 0) + 1 FROM wiki_page_versions WHERE page_id = $1), $2, $3, $4, NOW())
		ON CONFLICT (page_id, version_number) DO NOTHING
	`, pageID, content, hash, userID)
	return err
}

// DeleteVersion deletes a specific version of a wiki page. Returns error if not found.
func (ws *WikiPageService) DeleteVersion(pageID, versionNumber int) error {
	result, err := ws.DB.Exec(`DELETE FROM wiki_page_versions WHERE page_id = $1 AND version_number = $2`, pageID, versionNumber)
	if err != nil {
		return fmt.Errorf("delete wiki version: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("wiki version not found")
	}
	return nil
}

// GetVersions returns version list for a page (no content).
func (ws *WikiPageService) GetVersions(pageID int) ([]WikiPageVersion, error) {
	rows, err := ws.DB.Query(`
		SELECT version_id, page_id, version_number, content_hash, created_by, created_at
		FROM wiki_page_versions
		WHERE page_id = $1
		ORDER BY version_number DESC
	`, pageID)
	if err != nil {
		return nil, fmt.Errorf("list wiki versions: %w", err)
	}
	defer rows.Close()

	var versions []WikiPageVersion
	for rows.Next() {
		var v WikiPageVersion
		if err := rows.Scan(&v.ID, &v.PageID, &v.VersionNumber, &v.ContentHash, &v.CreatedBy, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan wiki version: %w", err)
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// GetVersion returns a single version with content.
func (ws *WikiPageService) GetVersion(pageID, versionNumber int) (*WikiPageVersion, error) {
	var v WikiPageVersion
	err := ws.DB.QueryRow(`
		SELECT version_id, page_id, version_number, content, content_hash, created_by, created_at
		FROM wiki_page_versions
		WHERE page_id = $1 AND version_number = $2
	`, pageID, versionNumber).Scan(
		&v.ID, &v.PageID, &v.VersionNumber, &v.Content, &v.ContentHash, &v.CreatedBy, &v.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("wiki version not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get wiki version: %w", err)
	}
	return &v, nil
}

// RestoreVersion restores a page to a previous version's content.
func (ws *WikiPageService) RestoreVersion(pageID, versionNumber, userID int) error {
	version, err := ws.GetVersion(pageID, versionNumber)
	if err != nil {
		return err
	}

	if err := ws.UpdateContent(pageID, version.Content); err != nil {
		return err
	}

	// Create a new version snapshot for the restore
	return ws.MaybeCreateVersion(pageID, userID, version.Content, true)
}
