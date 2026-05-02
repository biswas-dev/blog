package models

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"time"
)

type SlideVersion struct {
	ID            int
	SlideID       int
	VersionNumber int
	Title         string
	Content       string
	ContentHash   string
	CreatedBy     int
	CreatedByName string
	CreatedAt     time.Time
}

type SlideVersionService struct {
	DB *sql.DB
}

// MaybeCreateVersion saves a new version snapshot if content has changed significantly.
// It always upserts the contributor record.
func (svs *SlideVersionService) MaybeCreateVersion(slideID, userID int, title, content string) error {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))

	// Always upsert contributor
	_, err := svs.DB.Exec(`
		INSERT INTO slide_contributors (slide_id, user_id, first_contributed_at, last_contributed_at)
		VALUES ($1, $2, NOW(), NOW())
		ON CONFLICT (slide_id, user_id) DO UPDATE SET last_contributed_at = NOW()
	`, slideID, userID)
	if err != nil {
		return fmt.Errorf("upsert contributor: %w", err)
	}

	// Get last version for this slide
	var lastHash string
	var lastLen int
	var lastVersionNumber int
	err = svs.DB.QueryRow(`
		SELECT content_hash, LENGTH(content), version_number
		FROM slide_versions
		WHERE slide_id = $1
		ORDER BY version_number DESC
		LIMIT 1
	`, slideID).Scan(&lastHash, &lastLen, &lastVersionNumber)

	if err == sql.ErrNoRows {
		// No prior version — create version 1
		_, err = svs.DB.Exec(`
			INSERT INTO slide_versions (slide_id, version_number, title, content, content_hash, created_by, created_at)
			VALUES ($1, 1, $2, $3, $4, $5, NOW())
			ON CONFLICT (slide_id, version_number) DO NOTHING
		`, slideID, title, content, hash, userID)
		return err
	}
	if err != nil {
		return fmt.Errorf("query last version: %w", err)
	}

	// Skip if same content
	if lastHash == hash {
		return nil
	}

	// Skip if change is not significant enough
	if !isSignificantChange(lastLen, len(content)) {
		return nil
	}

	// Use a subquery to compute the next version number atomically, avoiding TOCTOU races.
	// ON CONFLICT DO NOTHING handles the case where a concurrent save wins the same slot.
	_, err = svs.DB.Exec(`
		INSERT INTO slide_versions (slide_id, version_number, title, content, content_hash, created_by, created_at)
		VALUES ($1, (SELECT COALESCE(MAX(version_number), 0) + 1 FROM slide_versions WHERE slide_id = $1), $2, $3, $4, $5, NOW())
		ON CONFLICT (slide_id, version_number) DO NOTHING
	`, slideID, title, content, hash, userID)
	return err
}

// GetVersions returns version list for a slide (no content — too large for list view).
func (svs *SlideVersionService) GetVersions(slideID int) ([]SlideVersion, error) {
	rows, err := svs.DB.Query(`
		SELECT v.id, v.slide_id, v.version_number, v.title, v.content_hash,
		       v.created_by, COALESCE(NULLIF(u.full_name, ''), u.username), v.created_at
		FROM slide_versions v
		JOIN Users u ON u.user_id = v.created_by
		WHERE v.slide_id = $1
		ORDER BY v.version_number DESC
	`, slideID)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()

	var versions []SlideVersion
	for rows.Next() {
		var v SlideVersion
		if err := rows.Scan(&v.ID, &v.SlideID, &v.VersionNumber, &v.Title, &v.ContentHash,
			&v.CreatedBy, &v.CreatedByName, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// GetVersion returns a single version including full content.
func (svs *SlideVersionService) GetVersion(slideID, versionNumber int) (*SlideVersion, error) {
	var v SlideVersion
	err := svs.DB.QueryRow(`
		SELECT v.id, v.slide_id, v.version_number, v.title, v.content, v.content_hash,
		       v.created_by, COALESCE(NULLIF(u.full_name, ''), u.username), v.created_at
		FROM slide_versions v
		JOIN Users u ON u.user_id = v.created_by
		WHERE v.slide_id = $1 AND v.version_number = $2
	`, slideID, versionNumber).Scan(
		&v.ID, &v.SlideID, &v.VersionNumber, &v.Title, &v.Content, &v.ContentHash,
		&v.CreatedBy, &v.CreatedByName, &v.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("version not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get version: %w", err)
	}
	return &v, nil
}

// DeleteVersion deletes a specific version of a slide. Returns error if not found.
func (svs *SlideVersionService) DeleteVersion(slideID, versionNumber int) error {
	result, err := svs.DB.Exec(`DELETE FROM slide_versions WHERE slide_id = $1 AND version_number = $2`, slideID, versionNumber)
	if err != nil {
		return fmt.Errorf("delete slide version: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("version not found")
	}
	return nil
}

// GetContributors returns co-author users recorded in slide_contributors,
// excluding the slide's primary author by user_id AND by username/full_name
// (covers the legacy case where the same person has two user rows that
// would otherwise both render as "Anshuman Biswas · Anshuman Biswas").
func (svs *SlideVersionService) GetContributors(slideID int) ([]User, error) {
	rows, err := svs.DB.Query(`
		SELECT u.user_id, u.username, COALESCE(u.full_name, ''), COALESCE(u.profile_picture_url, '')
		FROM Users u
		JOIN slide_contributors sc ON sc.user_id = u.user_id
		JOIN Slides s ON s.slide_id = sc.slide_id
		JOIN Users author ON author.user_id = s.user_id
		WHERE sc.slide_id = $1
		  AND sc.user_id != s.user_id
		  AND lower(u.username) != lower(author.username)
		  AND lower(COALESCE(NULLIF(u.full_name, ''), u.username)) != lower(COALESCE(NULLIF(author.full_name, ''), author.username))
		ORDER BY sc.first_contributed_at ASC
	`, slideID)
	if err != nil {
		return nil, fmt.Errorf("get contributors: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.UserID, &u.Username, &u.FullName, &u.AvatarURL); err != nil {
			return nil, fmt.Errorf("scan contributor: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// GetContributedSlides returns published slides the user has edited but did not author.
func (svs *SlideVersionService) GetContributedSlides(userID int) ([]Slide, error) {
	rows, err := svs.DB.Query(`
		SELECT s.slide_id, s.title, s.slug, s.created_at
		FROM Slides s
		JOIN slide_contributors sc ON sc.slide_id = s.slide_id
		WHERE sc.user_id = $1 AND s.user_id <> $1 AND s.is_published = true
		ORDER BY sc.last_contributed_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("get contributed slides: %w", err)
	}
	defer rows.Close()

	var slides []Slide
	for rows.Next() {
		var s Slide
		if err := rows.Scan(&s.ID, &s.Title, &s.Slug, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan contributed slide: %w", err)
		}
		slides = append(slides, s)
	}
	return slides, rows.Err()
}
