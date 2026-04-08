package models

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"time"
)

type PaperVersion struct {
	ID            int
	PaperID       int
	VersionNumber int
	Title         string
	Content       string
	ContentHash   string
	CreatedBy     int
	CreatedByName string
	CreatedAt     time.Time
}

type PaperVersionService struct {
	DB *sql.DB
}

// MaybeCreateVersion saves a new version snapshot if content has changed significantly.
func (pvs *PaperVersionService) MaybeCreateVersion(paperID, userID int, title, content string) error {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))

	// Get last version for this paper
	var lastHash string
	var lastLen int
	var lastVersionNumber int
	err := pvs.DB.QueryRow(`
		SELECT content_hash, LENGTH(content), version_number
		FROM paper_versions
		WHERE paper_id = $1
		ORDER BY version_number DESC
		LIMIT 1
	`, paperID).Scan(&lastHash, &lastLen, &lastVersionNumber)

	if err == sql.ErrNoRows {
		// No prior version — create version 1
		_, err = pvs.DB.Exec(`
			INSERT INTO paper_versions (paper_id, version_number, title, content, content_hash, created_by, created_at)
			VALUES ($1, 1, $2, $3, $4, $5, NOW())
			ON CONFLICT (paper_id, version_number) DO NOTHING
		`, paperID, title, content, hash, userID)
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
		INSERT INTO paper_versions (paper_id, version_number, title, content, content_hash, created_by, created_at)
		VALUES ($1, (SELECT COALESCE(MAX(version_number), 0) + 1 FROM paper_versions WHERE paper_id = $1), $2, $3, $4, $5, NOW())
		ON CONFLICT (paper_id, version_number) DO NOTHING
	`, paperID, title, content, hash, userID)
	return err
}

// GetVersions returns version list for a paper (no content — too large for list view).
func (pvs *PaperVersionService) GetVersions(paperID int) ([]PaperVersion, error) {
	rows, err := pvs.DB.Query(`
		SELECT v.id, v.paper_id, v.version_number, v.title, v.content_hash,
		       v.created_by, COALESCE(NULLIF(u.full_name, ''), u.username), v.created_at
		FROM paper_versions v
		JOIN Users u ON u.user_id = v.created_by
		WHERE v.paper_id = $1
		ORDER BY v.version_number DESC
	`, paperID)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()

	var versions []PaperVersion
	for rows.Next() {
		var v PaperVersion
		if err := rows.Scan(&v.ID, &v.PaperID, &v.VersionNumber, &v.Title, &v.ContentHash,
			&v.CreatedBy, &v.CreatedByName, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// GetVersion returns a single version including full content.
func (pvs *PaperVersionService) GetVersion(paperID, versionNumber int) (*PaperVersion, error) {
	var v PaperVersion
	err := pvs.DB.QueryRow(`
		SELECT v.id, v.paper_id, v.version_number, v.title, v.content, v.content_hash,
		       v.created_by, COALESCE(NULLIF(u.full_name, ''), u.username), v.created_at
		FROM paper_versions v
		JOIN Users u ON u.user_id = v.created_by
		WHERE v.paper_id = $1 AND v.version_number = $2
	`, paperID, versionNumber).Scan(
		&v.ID, &v.PaperID, &v.VersionNumber, &v.Title, &v.Content, &v.ContentHash,
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

// DeleteVersion deletes a specific version of a paper. Returns error if not found.
func (pvs *PaperVersionService) DeleteVersion(paperID, versionNumber int) error {
	result, err := pvs.DB.Exec(`DELETE FROM paper_versions WHERE paper_id = $1 AND version_number = $2`, paperID, versionNumber)
	if err != nil {
		return fmt.Errorf("delete paper version: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("version not found")
	}
	return nil
}
