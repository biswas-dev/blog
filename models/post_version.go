package models

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"time"
)

type PostVersion struct {
	ID            int
	PostID        int
	VersionNumber int
	Title         string
	Content       string
	ContentHash   string
	CreatedBy     int
	CreatedByName string
	CreatedAt     time.Time
}

type PostVersionService struct {
	DB *sql.DB
}

func isSignificantChange(oldLen, newLen int) bool {
	diff := oldLen - newLen
	if diff < 0 {
		diff = -diff
	}
	if diff > 500 {
		return true
	}
	if oldLen == 0 {
		return newLen > 0
	}
	ratio := float64(diff) / float64(oldLen)
	return ratio > 0.15
}

// MaybeCreateVersion saves a new version snapshot if content has changed significantly.
// It always upserts the contributor record.
func (pvs *PostVersionService) MaybeCreateVersion(postID, userID int, title, content string) error {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))

	// Always upsert contributor
	_, err := pvs.DB.Exec(`
		INSERT INTO post_contributors (post_id, user_id, first_contributed_at, last_contributed_at)
		VALUES ($1, $2, NOW(), NOW())
		ON CONFLICT (post_id, user_id) DO UPDATE SET last_contributed_at = NOW()
	`, postID, userID)
	if err != nil {
		return fmt.Errorf("upsert contributor: %w", err)
	}

	// Get last version for this post
	var lastHash string
	var lastLen int
	var lastVersionNumber int
	err = pvs.DB.QueryRow(`
		SELECT content_hash, LENGTH(content), version_number
		FROM post_versions
		WHERE post_id = $1
		ORDER BY version_number DESC
		LIMIT 1
	`, postID).Scan(&lastHash, &lastLen, &lastVersionNumber)

	if err == sql.ErrNoRows {
		// No prior version — create version 1
		_, err = pvs.DB.Exec(`
			INSERT INTO post_versions (post_id, version_number, title, content, content_hash, created_by, created_at)
			VALUES ($1, 1, $2, $3, $4, $5, NOW())
			ON CONFLICT (post_id, version_number) DO NOTHING
		`, postID, title, content, hash, userID)
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
	_, err = pvs.DB.Exec(`
		INSERT INTO post_versions (post_id, version_number, title, content, content_hash, created_by, created_at)
		VALUES ($1, (SELECT COALESCE(MAX(version_number), 0) + 1 FROM post_versions WHERE post_id = $1), $2, $3, $4, $5, NOW())
		ON CONFLICT (post_id, version_number) DO NOTHING
	`, postID, title, content, hash, userID)
	return err
}

// GetVersions returns version list for a post (no content — too large for list view).
func (pvs *PostVersionService) GetVersions(postID int) ([]PostVersion, error) {
	rows, err := pvs.DB.Query(`
		SELECT v.id, v.post_id, v.version_number, v.title, v.content_hash,
		       v.created_by, COALESCE(NULLIF(u.full_name, ''), u.username), v.created_at
		FROM post_versions v
		JOIN Users u ON u.user_id = v.created_by
		WHERE v.post_id = $1
		ORDER BY v.version_number DESC
	`, postID)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()

	var versions []PostVersion
	for rows.Next() {
		var v PostVersion
		if err := rows.Scan(&v.ID, &v.PostID, &v.VersionNumber, &v.Title, &v.ContentHash,
			&v.CreatedBy, &v.CreatedByName, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// GetVersion returns a single version including full content.
func (pvs *PostVersionService) GetVersion(postID, versionNumber int) (*PostVersion, error) {
	var v PostVersion
	err := pvs.DB.QueryRow(`
		SELECT v.id, v.post_id, v.version_number, v.title, v.content, v.content_hash,
		       v.created_by, COALESCE(NULLIF(u.full_name, ''), u.username), v.created_at
		FROM post_versions v
		JOIN Users u ON u.user_id = v.created_by
		WHERE v.post_id = $1 AND v.version_number = $2
	`, postID, versionNumber).Scan(
		&v.ID, &v.PostID, &v.VersionNumber, &v.Title, &v.Content, &v.ContentHash,
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

// DeleteVersion deletes a specific version of a post. Returns error if not found.
func (pvs *PostVersionService) DeleteVersion(postID, versionNumber int) error {
	result, err := pvs.DB.Exec(`DELETE FROM post_versions WHERE post_id = $1 AND version_number = $2`, postID, versionNumber)
	if err != nil {
		return fmt.Errorf("delete post version: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("version not found")
	}
	return nil
}

// GetContributors returns all users recorded in post_contributors for the given post.
// Callers are responsible for filtering out the original author if desired.
func (pvs *PostVersionService) GetContributors(postID int) ([]User, error) {
	rows, err := pvs.DB.Query(`
		SELECT u.user_id, u.username, COALESCE(u.full_name, ''), COALESCE(u.profile_picture_url, ''), COALESCE(u.bio, '')
		FROM Users u
		JOIN post_contributors pc ON pc.user_id = u.user_id
		WHERE pc.post_id = $1
		ORDER BY pc.first_contributed_at ASC
	`, postID)
	if err != nil {
		return nil, fmt.Errorf("get contributors: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.UserID, &u.Username, &u.FullName, &u.AvatarURL, &u.Bio); err != nil {
			return nil, fmt.Errorf("scan contributor: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// GetContributedPosts returns published posts the user has edited but did not author.
func (pvs *PostVersionService) GetContributedPosts(userID int) ([]Post, error) {
	rows, err := pvs.DB.Query(`
		SELECT p.post_id, p.title, p.slug, p.created_at
		FROM Posts p
		JOIN post_contributors pc ON pc.post_id = p.post_id
		WHERE pc.user_id = $1 AND p.user_id <> $1 AND p.is_published = true
		ORDER BY pc.last_contributed_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("get contributed posts: %w", err)
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan contributed post: %w", err)
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}
