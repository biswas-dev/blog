package models

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"time"
)

type BookVersion struct {
	ID            int
	BookID        int
	VersionNumber int
	Title         string
	Content       string
	ContentHash   string
	CreatedBy     int
	CreatedByName string
	CreatedAt     time.Time
}

type BookVersionService struct {
	DB *sql.DB
}

// MaybeCreateVersion saves a new version snapshot if content has changed significantly.
func (bvs *BookVersionService) MaybeCreateVersion(bookID, userID int, title, content string) error {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))

	// Get last version for this book
	var lastHash string
	var lastLen int
	var lastVersionNumber int
	err := bvs.DB.QueryRow(`
		SELECT content_hash, LENGTH(content), version_number
		FROM book_versions
		WHERE book_id = $1
		ORDER BY version_number DESC
		LIMIT 1
	`, bookID).Scan(&lastHash, &lastLen, &lastVersionNumber)

	if err == sql.ErrNoRows {
		// No prior version — create version 1
		_, err = bvs.DB.Exec(`
			INSERT INTO book_versions (book_id, version_number, title, content, content_hash, created_by, created_at)
			VALUES ($1, 1, $2, $3, $4, $5, NOW())
			ON CONFLICT (book_id, version_number) DO NOTHING
		`, bookID, title, content, hash, userID)
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
	_, err = bvs.DB.Exec(`
		INSERT INTO book_versions (book_id, version_number, title, content, content_hash, created_by, created_at)
		VALUES ($1, (SELECT COALESCE(MAX(version_number), 0) + 1 FROM book_versions WHERE book_id = $1), $2, $3, $4, $5, NOW())
		ON CONFLICT (book_id, version_number) DO NOTHING
	`, bookID, title, content, hash, userID)
	return err
}

// GetVersions returns version list for a book (no content — too large for list view).
func (bvs *BookVersionService) GetVersions(bookID int) ([]BookVersion, error) {
	rows, err := bvs.DB.Query(`
		SELECT v.id, v.book_id, v.version_number, v.title, v.content_hash,
		       v.created_by, COALESCE(NULLIF(u.full_name, ''), u.username), v.created_at
		FROM book_versions v
		JOIN Users u ON u.user_id = v.created_by
		WHERE v.book_id = $1
		ORDER BY v.version_number DESC
	`, bookID)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()

	var versions []BookVersion
	for rows.Next() {
		var v BookVersion
		if err := rows.Scan(&v.ID, &v.BookID, &v.VersionNumber, &v.Title, &v.ContentHash,
			&v.CreatedBy, &v.CreatedByName, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// GetVersion returns a single version including full content.
func (bvs *BookVersionService) GetVersion(bookID, versionNumber int) (*BookVersion, error) {
	var v BookVersion
	err := bvs.DB.QueryRow(`
		SELECT v.id, v.book_id, v.version_number, v.title, v.content, v.content_hash,
		       v.created_by, COALESCE(NULLIF(u.full_name, ''), u.username), v.created_at
		FROM book_versions v
		JOIN Users u ON u.user_id = v.created_by
		WHERE v.book_id = $1 AND v.version_number = $2
	`, bookID, versionNumber).Scan(
		&v.ID, &v.BookID, &v.VersionNumber, &v.Title, &v.Content, &v.ContentHash,
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

// DeleteVersion deletes a specific version of a book. Returns error if not found.
func (bvs *BookVersionService) DeleteVersion(bookID, versionNumber int) error {
	result, err := bvs.DB.Exec(`DELETE FROM book_versions WHERE book_id = $1 AND version_number = $2`, bookID, versionNumber)
	if err != nil {
		return fmt.Errorf("delete book version: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("version not found")
	}
	return nil
}
