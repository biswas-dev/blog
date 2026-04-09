package models

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type PaperResearchArea struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type PaperResearchAreaService struct {
	DB *sql.DB
}

// Create creates a new paper research area.
func (s *PaperResearchAreaService) Create(name string) (*PaperResearchArea, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("research area name cannot be empty")
	}
	if len(name) > 255 {
		return nil, fmt.Errorf("research area name too long (max 255 characters)")
	}

	area := &PaperResearchArea{}
	query := `
		INSERT INTO paper_research_areas (area_name)
		VALUES ($1)
		RETURNING area_id, area_name, created_at`

	err := s.DB.QueryRow(query, name).Scan(
		&area.ID,
		&area.Name,
		&area.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create research area: %w", err)
	}

	return area, nil
}

// GetByID retrieves a paper research area by ID.
func (s *PaperResearchAreaService) GetByID(id int) (*PaperResearchArea, error) {
	area := &PaperResearchArea{}
	query := `
		SELECT area_id, area_name, created_at
		FROM paper_research_areas
		WHERE area_id = $1`

	err := s.DB.QueryRow(query, id).Scan(
		&area.ID,
		&area.Name,
		&area.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("research area not found")
		}
		return nil, fmt.Errorf("failed to get research area: %w", err)
	}

	return area, nil
}

// GetAll retrieves all paper research areas ordered by name.
func (s *PaperResearchAreaService) GetAll() ([]PaperResearchArea, error) {
	var areas []PaperResearchArea
	query := `
		SELECT area_id, area_name, created_at
		FROM paper_research_areas
		ORDER BY area_name`

	rows, err := s.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get research areas: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var area PaperResearchArea
		err := rows.Scan(
			&area.ID,
			&area.Name,
			&area.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan research area: %w", err)
		}
		areas = append(areas, area)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate research areas: %w", err)
	}

	return areas, nil
}

// GetByName retrieves a paper research area by its name (case-insensitive).
func (s *PaperResearchAreaService) GetByName(name string) (*PaperResearchArea, error) {
	area := &PaperResearchArea{}
	query := `
		SELECT area_id, area_name, created_at
		FROM paper_research_areas
		WHERE LOWER(area_name) = LOWER($1)`
	err := s.DB.QueryRow(query, name).Scan(&area.ID, &area.Name, &area.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("research area not found")
		}
		return nil, fmt.Errorf("failed to get research area: %w", err)
	}
	return area, nil
}

// Update updates a paper research area's name.
func (s *PaperResearchAreaService) Update(id int, name string) (*PaperResearchArea, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("research area name cannot be empty")
	}
	if len(name) > 255 {
		return nil, fmt.Errorf("research area name too long (max 255 characters)")
	}

	area := &PaperResearchArea{}
	query := `
		UPDATE paper_research_areas
		SET area_name = $2
		WHERE area_id = $1
		RETURNING area_id, area_name, created_at`

	err := s.DB.QueryRow(query, id, name).Scan(
		&area.ID,
		&area.Name,
		&area.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("research area not found")
		}
		return nil, fmt.Errorf("failed to update research area: %w", err)
	}

	return area, nil
}

// Delete removes a paper research area. Returns an error if it is in use.
func (s *PaperResearchAreaService) Delete(id int) error {
	var paperCount int
	checkQuery := `SELECT COUNT(*) FROM paper_research_area_map WHERE area_id = $1`
	err := s.DB.QueryRow(checkQuery, id).Scan(&paperCount)
	if err != nil {
		return fmt.Errorf("failed to check research area usage: %w", err)
	}

	if paperCount > 0 {
		return fmt.Errorf("research area in use by %d papers", paperCount)
	}

	query := `DELETE FROM paper_research_areas WHERE area_id = $1`
	result, err := s.DB.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete research area: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check delete result: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("research area not found")
	}

	return nil
}
