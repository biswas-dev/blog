package models

import (
	"database/sql"
	"time"
)

type ImageMetadata struct {
	ID        int       `json:"id"`
	ImageURL  string    `json:"image_url"`
	AltText   string    `json:"alt_text"`
	Title     string    `json:"title"`
	Caption   string    `json:"caption"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ImageMetadataService struct {
	DB *sql.DB
}

// Upsert creates or updates metadata for an image URL.
func (s *ImageMetadataService) Upsert(imageURL, altText, title, caption string) (*ImageMetadata, error) {
	var m ImageMetadata
	err := s.DB.QueryRow(`
		INSERT INTO image_metadata (image_url, alt_text, title, caption, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (image_url)
		DO UPDATE SET alt_text = $2, title = $3, caption = $4, updated_at = $5
		RETURNING id, image_url, alt_text, title, caption, created_at, updated_at
	`, imageURL, altText, title, caption, time.Now()).Scan(
		&m.ID, &m.ImageURL, &m.AltText, &m.Title, &m.Caption, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByURL returns metadata for a single image URL, or nil if not found.
func (s *ImageMetadataService) GetByURL(imageURL string) (*ImageMetadata, error) {
	var m ImageMetadata
	err := s.DB.QueryRow(`
		SELECT id, image_url, alt_text, title, caption, created_at, updated_at
		FROM image_metadata WHERE image_url = $1
	`, imageURL).Scan(&m.ID, &m.ImageURL, &m.AltText, &m.Title, &m.Caption, &m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByURLs returns metadata for multiple image URLs as a map keyed by URL.
func (s *ImageMetadataService) GetByURLs(urls []string) (map[string]*ImageMetadata, error) {
	result := make(map[string]*ImageMetadata)
	if len(urls) == 0 {
		return result, nil
	}

	// Build query with ANY($1)
	rows, err := s.DB.Query(`
		SELECT id, image_url, alt_text, title, caption, created_at, updated_at
		FROM image_metadata WHERE image_url = ANY($1)
	`, pqStringArray(urls))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var m ImageMetadata
		if err := rows.Scan(&m.ID, &m.ImageURL, &m.AltText, &m.Title, &m.Caption, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		result[m.ImageURL] = &m
	}
	return result, rows.Err()
}

// Delete removes metadata for an image URL.
func (s *ImageMetadataService) Delete(imageURL string) error {
	_, err := s.DB.Exec(`DELETE FROM image_metadata WHERE image_url = $1`, imageURL)
	return err
}

// pqStringArray converts a Go string slice to a PostgreSQL array literal.
func pqStringArray(ss []string) string {
	if len(ss) == 0 {
		return "{}"
	}
	out := "{"
	for i, s := range ss {
		if i > 0 {
			out += ","
		}
		// Escape quotes in values
		escaped := ""
		for _, c := range s {
			if c == '"' || c == '\\' {
				escaped += "\\"
			}
			escaped += string(c)
		}
		out += `"` + escaped + `"`
	}
	out += "}"
	return out
}
