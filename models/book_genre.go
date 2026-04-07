package models

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type BookGenre struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Group     string    `json:"group"` // tech, non-tech, fiction, non-fiction
	CreatedAt time.Time `json:"created_at"`
}

type BookGenreService struct {
	DB *sql.DB
}

// Create creates a new book genre.
func (bgs *BookGenreService) Create(name, group string) (*BookGenre, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("genre name cannot be empty")
	}
	if len(name) > 255 {
		return nil, fmt.Errorf("genre name too long (max 255 characters)")
	}

	genre := &BookGenre{}
	query := `
		INSERT INTO Book_Genres (genre_name, genre_group)
		VALUES ($1, $2)
		RETURNING genre_id, genre_name, genre_group, created_at`

	err := bgs.DB.QueryRow(query, name, group).Scan(
		&genre.ID,
		&genre.Name,
		&genre.Group,
		&genre.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create genre: %w", err)
	}

	return genre, nil
}

// GetByID retrieves a book genre by ID.
func (bgs *BookGenreService) GetByID(id int) (*BookGenre, error) {
	genre := &BookGenre{}
	query := `
		SELECT genre_id, genre_name, genre_group, created_at
		FROM Book_Genres
		WHERE genre_id = $1`

	err := bgs.DB.QueryRow(query, id).Scan(
		&genre.ID,
		&genre.Name,
		&genre.Group,
		&genre.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("genre not found")
		}
		return nil, fmt.Errorf("failed to get genre: %w", err)
	}

	return genre, nil
}

// GetAll retrieves all book genres ordered by group and name.
func (bgs *BookGenreService) GetAll() ([]BookGenre, error) {
	var genres []BookGenre
	query := `
		SELECT genre_id, genre_name, genre_group, created_at
		FROM Book_Genres
		ORDER BY genre_group, genre_name`

	rows, err := bgs.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get genres: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var genre BookGenre
		err := rows.Scan(
			&genre.ID,
			&genre.Name,
			&genre.Group,
			&genre.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan genre: %w", err)
		}
		genres = append(genres, genre)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate genres: %w", err)
	}

	return genres, nil
}

// GetByGroup retrieves all book genres in a specific group.
func (bgs *BookGenreService) GetByGroup(group string) ([]BookGenre, error) {
	var genres []BookGenre
	query := `
		SELECT genre_id, genre_name, genre_group, created_at
		FROM Book_Genres
		WHERE genre_group = $1
		ORDER BY genre_name`

	rows, err := bgs.DB.Query(query, group)
	if err != nil {
		return nil, fmt.Errorf("failed to get genres by group: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var genre BookGenre
		err := rows.Scan(
			&genre.ID,
			&genre.Name,
			&genre.Group,
			&genre.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan genre: %w", err)
		}
		genres = append(genres, genre)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate genres: %w", err)
	}

	return genres, nil
}

// GetByName retrieves a book genre by its name (case-insensitive).
func (bgs *BookGenreService) GetByName(name string) (*BookGenre, error) {
	genre := &BookGenre{}
	query := `
		SELECT genre_id, genre_name, genre_group, created_at
		FROM Book_Genres
		WHERE LOWER(genre_name) = LOWER($1)`
	err := bgs.DB.QueryRow(query, name).Scan(&genre.ID, &genre.Name, &genre.Group, &genre.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("genre not found")
		}
		return nil, fmt.Errorf("failed to get genre: %w", err)
	}
	return genre, nil
}

// Update updates a book genre's name and group.
func (bgs *BookGenreService) Update(id int, name, group string) (*BookGenre, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("genre name cannot be empty")
	}
	if len(name) > 255 {
		return nil, fmt.Errorf("genre name too long (max 255 characters)")
	}

	genre := &BookGenre{}
	query := `
		UPDATE Book_Genres
		SET genre_name = $2, genre_group = $3
		WHERE genre_id = $1
		RETURNING genre_id, genre_name, genre_group, created_at`

	err := bgs.DB.QueryRow(query, id, name, group).Scan(
		&genre.ID,
		&genre.Name,
		&genre.Group,
		&genre.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("genre not found")
		}
		return nil, fmt.Errorf("failed to update genre: %w", err)
	}

	return genre, nil
}

// Delete removes a book genre. Returns an error if it is in use.
func (bgs *BookGenreService) Delete(id int) error {
	var bookCount int
	checkQuery := `SELECT COUNT(*) FROM Book_Genre_Map WHERE genre_id = $1`
	err := bgs.DB.QueryRow(checkQuery, id).Scan(&bookCount)
	if err != nil {
		return fmt.Errorf("failed to check genre usage: %w", err)
	}

	if bookCount > 0 {
		return fmt.Errorf("genre in use by %d books", bookCount)
	}

	query := `DELETE FROM Book_Genres WHERE genre_id = $1`
	result, err := bgs.DB.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete genre: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check delete result: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("genre not found")
	}

	return nil
}
