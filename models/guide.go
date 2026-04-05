package models

import (
	"database/sql"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/lib/pq"
)

type GuidesList struct {
	Guides []Guide
}

type Guide struct {
	ID                int
	UserID            int
	Username          string
	AuthorDisplayName string
	AuthorAvatarURL   string
	AuthorBio         string
	Title             string
	Content           string
	ContentHTML       template.HTML
	Slug              string
	Description       string
	FeaturedImageURL  string
	IsPublished       bool
	PublicationDate   string
	LastEditDate      string
	CreatedAt         string
	UpdatedAt         string
	ReadingTime       int        `json:"reading_time,omitempty"`
	Categories        []Category `json:"categories,omitempty"`
}

type GuideService struct {
	DB *sql.DB
}

// Create creates a new guide and returns it.
func (gs *GuideService) Create(userID int, title, slug, content, description, featuredImageURL string, isPublished bool, categoryIDs []int) (*Guide, error) {
	if slug == "" {
		slug = generateSlug(title)
	} else {
		slug = sanitizeSlug(slug)
	}

	now := time.Now()
	var guide Guide
	err := gs.DB.QueryRow(`
		INSERT INTO Guides (user_id, title, content, slug, description, featured_image_url, is_published, publication_date, last_edit_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING guide_id, created_at, updated_at`,
		userID, title, content, slug, description, featuredImageURL, isPublished, now, now, now, now,
	).Scan(&guide.ID, &guide.CreatedAt, &guide.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create guide: %w", err)
	}

	guide.UserID = userID
	guide.Title = title
	guide.Content = content
	guide.Slug = slug
	guide.Description = description
	guide.FeaturedImageURL = featuredImageURL
	guide.IsPublished = isPublished
	guide.PublicationDate = now.Format(friendlyDateFormat)
	guide.LastEditDate = now.Format(friendlyDateFormat)

	if len(categoryIDs) > 0 {
		if err := gs.AddCategories(guide.ID, categoryIDs); err != nil {
			return &guide, nil // guide created, categories failed — non-fatal
		}
	}

	return &guide, nil
}

// Update updates an existing guide.
func (gs *GuideService) Update(guideID int, title, slug, content, description, featuredImageURL string, isPublished bool, categoryIDs []int) error {
	if slug == "" {
		slug = generateSlug(title)
	} else {
		slug = sanitizeSlug(slug)
	}

	_, err := gs.DB.Exec(`
		UPDATE Guides SET title=$1, slug=$2, content=$3, description=$4, featured_image_url=$5, is_published=$6, last_edit_date=$7, updated_at=$8
		WHERE guide_id=$9`,
		title, slug, content, description, featuredImageURL, isPublished, time.Now(), time.Now(), guideID)
	if err != nil {
		return fmt.Errorf("update guide: %w", err)
	}
	if len(categoryIDs) > 0 {
		_ = gs.UpdateCategories(guideID, categoryIDs)
	}
	return nil
}

// Delete removes a guide by ID.
func (gs *GuideService) Delete(guideID int) error {
	_, err := gs.DB.Exec(`DELETE FROM Guides WHERE guide_id = $1`, guideID)
	if err != nil {
		return fmt.Errorf("delete guide: %w", err)
	}
	return nil
}

// GetByID retrieves a guide by its ID.
func (gs *GuideService) GetByID(id int) (*Guide, error) {
	var guide Guide
	err := gs.DB.QueryRow(`
		SELECT g.guide_id, g.user_id, u.username, COALESCE(NULLIF(u.full_name, ''), u.username),
		       COALESCE(u.profile_picture_url, ''), COALESCE(u.bio, ''),
		       g.title, g.content, g.slug, COALESCE(g.description, ''), COALESCE(g.featured_image_url, ''),
		       g.is_published, g.publication_date, g.last_edit_date, g.created_at, g.updated_at
		FROM Guides g
		JOIN Users u ON g.user_id = u.user_id
		WHERE g.guide_id = $1`, id).Scan(
		&guide.ID, &guide.UserID, &guide.Username, &guide.AuthorDisplayName,
		&guide.AuthorAvatarURL, &guide.AuthorBio,
		&guide.Title, &guide.Content, &guide.Slug, &guide.Description, &guide.FeaturedImageURL,
		&guide.IsPublished, &guide.PublicationDate, &guide.LastEditDate, &guide.CreatedAt, &guide.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("guide not found")
		}
		return nil, fmt.Errorf("get guide by id: %w", err)
	}

	formatGuideDates(&guide)

	// Load categories
	if err := gs.loadCategoriesForGuides([]Guide{guide}); err != nil {
		return nil, fmt.Errorf("load categories: %w", err)
	}

	return &guide, nil
}

// GetBySlug retrieves a guide by its slug and renders content to HTML.
func (gs *GuideService) GetBySlug(slug string) (*Guide, error) {
	var guide Guide
	err := gs.DB.QueryRow(`
		SELECT g.guide_id, g.user_id, u.username, COALESCE(NULLIF(u.full_name, ''), u.username),
		       COALESCE(u.profile_picture_url, ''), COALESCE(u.bio, ''),
		       g.title, g.content, g.slug, COALESCE(g.description, ''), COALESCE(g.featured_image_url, ''),
		       g.is_published, g.publication_date, g.last_edit_date, g.created_at, g.updated_at
		FROM Guides g
		JOIN Users u ON g.user_id = u.user_id
		WHERE g.slug = $1`, slug).Scan(
		&guide.ID, &guide.UserID, &guide.Username, &guide.AuthorDisplayName,
		&guide.AuthorAvatarURL, &guide.AuthorBio,
		&guide.Title, &guide.Content, &guide.Slug, &guide.Description, &guide.FeaturedImageURL,
		&guide.IsPublished, &guide.PublicationDate, &guide.LastEditDate, &guide.CreatedAt, &guide.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("guide not found")
		}
		return nil, fmt.Errorf("get guide by slug: %w", err)
	}

	formatGuideDates(&guide)

	// Render markdown content to HTML
	guide.ContentHTML = template.HTML(RenderContent(guide.Content))

	// Calculate reading time
	wordCount := len(strings.Fields(guide.Content))
	guide.ReadingTime = (wordCount + 199) / 200
	if guide.ReadingTime < 1 {
		guide.ReadingTime = 1
	}

	// Load categories
	guides := []Guide{guide}
	if err := gs.loadCategoriesForGuides(guides); err != nil {
		return nil, fmt.Errorf("load categories: %w", err)
	}
	guide.Categories = guides[0].Categories

	return &guide, nil
}

// GetPublishedGuides returns all published guides ordered by creation date.
func (gs *GuideService) GetPublishedGuides() (*GuidesList, error) {
	list := GuidesList{}

	rows, err := gs.DB.Query(`
		SELECT g.guide_id, g.user_id, u.username, COALESCE(NULLIF(u.full_name, ''), u.username),
		       COALESCE(u.profile_picture_url, ''), COALESCE(u.bio, ''),
		       g.title, g.content, g.slug, COALESCE(g.description, ''), COALESCE(g.featured_image_url, ''),
		       g.is_published, g.publication_date, g.last_edit_date, g.created_at, g.updated_at
		FROM Guides g
		JOIN Users u ON g.user_id = u.user_id
		WHERE g.is_published = true
		ORDER BY g.created_at DESC`)
	if err != nil {
		return &list, fmt.Errorf("query published guides: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var guide Guide
		if err := rows.Scan(
			&guide.ID, &guide.UserID, &guide.Username, &guide.AuthorDisplayName,
			&guide.AuthorAvatarURL, &guide.AuthorBio,
			&guide.Title, &guide.Content, &guide.Slug, &guide.Description, &guide.FeaturedImageURL,
			&guide.IsPublished, &guide.PublicationDate, &guide.LastEditDate, &guide.CreatedAt, &guide.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan published guide: %w", err)
		}
		formatGuideDates(&guide)

		// Calculate reading time
		wordCount := len(strings.Fields(guide.Content))
		guide.ReadingTime = (wordCount + 199) / 200
		if guide.ReadingTime < 1 {
			guide.ReadingTime = 1
		}

		list.Guides = append(list.Guides, guide)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate published guides: %w", err)
	}

	if err := gs.loadCategoriesForGuides(list.Guides); err != nil {
		return nil, fmt.Errorf("load categories: %w", err)
	}

	return &list, nil
}

// GetPublishedGuidesByUser returns published guides by a specific user.
func (gs *GuideService) GetPublishedGuidesByUser(userID int) ([]Guide, error) {
	rows, err := gs.DB.Query(`
		SELECT g.guide_id, g.title, g.slug, COALESCE(g.description, ''),
		       COALESCE(g.featured_image_url, ''), g.created_at
		FROM Guides g
		WHERE g.user_id = $1 AND g.is_published = true
		ORDER BY g.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var guides []Guide
	for rows.Next() {
		var g Guide
		if err := rows.Scan(&g.ID, &g.Title, &g.Slug, &g.Description, &g.FeaturedImageURL, &g.CreatedAt); err != nil {
			return nil, err
		}
		formatGuideDates(&g)
		guides = append(guides, g)
	}
	return guides, rows.Err()
}

// GetAllGuides returns all guides (for admin) ordered by creation date.
func (gs *GuideService) GetAllGuides() (*GuidesList, error) {
	list := GuidesList{}

	rows, err := gs.DB.Query(`
		SELECT g.guide_id, g.user_id, u.username, COALESCE(NULLIF(u.full_name, ''), u.username),
		       COALESCE(u.profile_picture_url, ''), COALESCE(u.bio, ''),
		       g.title, g.content, g.slug, COALESCE(g.description, ''), COALESCE(g.featured_image_url, ''),
		       g.is_published, g.publication_date, g.last_edit_date, g.created_at, g.updated_at
		FROM Guides g
		JOIN Users u ON g.user_id = u.user_id
		ORDER BY g.created_at DESC`)
	if err != nil {
		return &list, fmt.Errorf("query all guides: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var guide Guide
		if err := rows.Scan(
			&guide.ID, &guide.UserID, &guide.Username, &guide.AuthorDisplayName,
			&guide.AuthorAvatarURL, &guide.AuthorBio,
			&guide.Title, &guide.Content, &guide.Slug, &guide.Description, &guide.FeaturedImageURL,
			&guide.IsPublished, &guide.PublicationDate, &guide.LastEditDate, &guide.CreatedAt, &guide.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan guide: %w", err)
		}
		formatGuideDates(&guide)
		list.Guides = append(list.Guides, guide)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate all guides: %w", err)
	}

	if err := gs.loadCategoriesForGuides(list.Guides); err != nil {
		return nil, fmt.Errorf("load categories: %w", err)
	}

	return &list, nil
}

// GetPublishedGuidesByCategory returns published guides that belong to a specific category.
func (gs *GuideService) GetPublishedGuidesByCategory(categoryID int) ([]Guide, error) {
	rows, err := gs.DB.Query(`
		SELECT g.guide_id, g.user_id, u.username, COALESCE(NULLIF(u.full_name, ''), u.username),
		       COALESCE(u.profile_picture_url, ''), COALESCE(u.bio, ''),
		       g.title, g.content, g.slug, COALESCE(g.description, ''), COALESCE(g.featured_image_url, ''),
		       g.is_published, g.publication_date, g.last_edit_date, g.created_at, g.updated_at
		FROM Guides g
		JOIN Users u ON g.user_id = u.user_id
		JOIN Guide_Categories gc ON g.guide_id = gc.guide_id
		WHERE gc.category_id = $1 AND g.is_published = true
		ORDER BY g.created_at DESC`, categoryID)
	if err != nil {
		return nil, fmt.Errorf("query guides by category: %w", err)
	}
	defer rows.Close()

	var guides []Guide
	for rows.Next() {
		var guide Guide
		if err := rows.Scan(
			&guide.ID, &guide.UserID, &guide.Username, &guide.AuthorDisplayName,
			&guide.AuthorAvatarURL, &guide.AuthorBio,
			&guide.Title, &guide.Content, &guide.Slug, &guide.Description, &guide.FeaturedImageURL,
			&guide.IsPublished, &guide.PublicationDate, &guide.LastEditDate, &guide.CreatedAt, &guide.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan guide by category: %w", err)
		}
		formatGuideDates(&guide)

		wordCount := len(strings.Fields(guide.Content))
		guide.ReadingTime = (wordCount + 199) / 200
		if guide.ReadingTime < 1 {
			guide.ReadingTime = 1
		}

		guides = append(guides, guide)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate guides by category: %w", err)
	}

	if err := gs.loadCategoriesForGuides(guides); err != nil {
		return nil, fmt.Errorf("load categories: %w", err)
	}

	return guides, nil
}

// AddCategories adds categories to a guide.
func (gs *GuideService) AddCategories(guideID int, categoryIDs []int) error {
	if len(categoryIDs) == 0 {
		return nil
	}
	_, err := gs.DB.Exec(`
		INSERT INTO Guide_Categories (guide_id, category_id)
		SELECT $1, unnest($2::int[]) ON CONFLICT DO NOTHING`,
		guideID, pq.Array(categoryIDs))
	if err != nil {
		return fmt.Errorf("add guide categories: %w", err)
	}
	return nil
}

// UpdateCategories replaces all categories for a guide.
func (gs *GuideService) UpdateCategories(guideID int, categoryIDs []int) error {
	_, err := gs.DB.Exec(`DELETE FROM Guide_Categories WHERE guide_id = $1`, guideID)
	if err != nil {
		return fmt.Errorf("remove existing guide categories: %w", err)
	}
	if len(categoryIDs) > 0 {
		return gs.AddCategories(guideID, categoryIDs)
	}
	return nil
}

// loadCategoriesForGuides batch-loads categories for all guides in a single query.
func (gs *GuideService) loadCategoriesForGuides(guides []Guide) error {
	if len(guides) == 0 {
		return nil
	}
	ids := make([]int, len(guides))
	idIdx := make(map[int][]int)
	for i, g := range guides {
		ids[i] = g.ID
		idIdx[g.ID] = append(idIdx[g.ID], i)
	}

	rows, err := gs.DB.Query(`
		SELECT gc.guide_id, c.category_id, c.category_name, c.created_at
		FROM Categories c
		JOIN Guide_Categories gc ON c.category_id = gc.category_id
		WHERE gc.guide_id = ANY($1)`, pq.Array(ids))
	if err != nil {
		return fmt.Errorf("batch-load guide categories: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var guideID int
		var cat Category
		if err := rows.Scan(&guideID, &cat.ID, &cat.Name, &cat.CreatedAt); err != nil {
			return err
		}
		for _, idx := range idIdx[guideID] {
			guides[idx].Categories = append(guides[idx].Categories, cat)
		}
	}
	return rows.Err()
}

// formatGuideDates normalises date fields for display.
func formatGuideDates(guide *Guide) {
	if t, err := time.Parse(time.RFC3339, guide.CreatedAt); err == nil {
		guide.CreatedAt = t.Format(time.RFC3339)
		guide.PublicationDate = t.Format(friendlyDateFormat)
	}
	if guide.PublicationDate != "" && guide.PublicationDate != guide.CreatedAt {
		if t, err := time.Parse(time.RFC3339, guide.PublicationDate); err == nil {
			guide.PublicationDate = t.Format(friendlyDateFormat)
		}
	}
	if guide.LastEditDate != "" {
		if t, err := time.Parse(time.RFC3339, guide.LastEditDate); err == nil {
			guide.LastEditDate = t.Format(friendlyDateFormat)
		}
	}
}
