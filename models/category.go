package models

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Category struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type CategoryService struct {
	DB *sql.DB
}

// Create creates a new category
func (cs *CategoryService) Create(name string) (*Category, error) {
	// Sanitize and validate the category name
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("category name cannot be empty")
	}
	if len(name) > 255 {
		return nil, fmt.Errorf("category name too long (max 255 characters)")
	}

	category := &Category{}
	query := `
		INSERT INTO Categories (category_name)
		VALUES ($1)
		RETURNING category_id, category_name, created_at`
	
	err := cs.DB.QueryRow(query, name).Scan(
		&category.ID,
		&category.Name,
		&category.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create category: %w", err)
	}

	return category, nil
}

// GetByID retrieves a category by ID
func (cs *CategoryService) GetByID(id int) (*Category, error) {
	category := &Category{}
	query := `
		SELECT category_id, category_name, created_at
		FROM Categories
		WHERE category_id = $1`
	
	err := cs.DB.QueryRow(query, id).Scan(
		&category.ID,
		&category.Name,
		&category.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("category not found")
		}
		return nil, fmt.Errorf("failed to get category: %w", err)
	}

	return category, nil
}

// GetAll retrieves all categories
func (cs *CategoryService) GetAll() ([]Category, error) {
	var categories []Category
	query := `
		SELECT category_id, category_name, created_at
		FROM Categories
		ORDER BY category_name ASC`
	
	rows, err := cs.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get categories: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var category Category
		err := rows.Scan(
			&category.ID,
			&category.Name,
			&category.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}
		categories = append(categories, category)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate categories: %w", err)
	}

	return categories, nil
}

// Update updates a category's name
func (cs *CategoryService) Update(id int, name string) (*Category, error) {
	// Sanitize and validate the category name
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("category name cannot be empty")
	}
	if len(name) > 255 {
		return nil, fmt.Errorf("category name too long (max 255 characters)")
	}

	category := &Category{}
	query := `
		UPDATE Categories 
		SET category_name = $1
		WHERE category_id = $2
		RETURNING category_id, category_name, created_at`
	
	err := cs.DB.QueryRow(query, name, id).Scan(
		&category.ID,
		&category.Name,
		&category.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("category not found")
		}
		return nil, fmt.Errorf("failed to update category: %w", err)
	}

	return category, nil
}

// Delete removes a category
func (cs *CategoryService) Delete(id int) error {
	// Check if category is in use
	var postCount int
	checkQuery := `SELECT COUNT(*) FROM Post_Categories WHERE category_id = $1`
	err := cs.DB.QueryRow(checkQuery, id).Scan(&postCount)
	if err != nil {
		return fmt.Errorf("failed to check category usage: %w", err)
	}

	if postCount > 0 {
		return fmt.Errorf("cannot delete category: it is assigned to %d post(s)", postCount)
	}

	query := `DELETE FROM Categories WHERE category_id = $1`
	result, err := cs.DB.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check delete result: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("category not found")
	}

	return nil
}

// GetByName retrieves a category by its name (case-insensitive)
func (cs *CategoryService) GetByName(name string) (*Category, error) {
	category := &Category{}
	query := `
		SELECT category_id, category_name, created_at
		FROM Categories
		WHERE LOWER(category_name) = LOWER($1)`
	err := cs.DB.QueryRow(query, name).Scan(&category.ID, &category.Name, &category.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("category not found")
		}
		return nil, fmt.Errorf("failed to get category: %w", err)
	}
	return category, nil
}

// GetCategoriesByPostID retrieves all categories for a specific post
func (cs *CategoryService) GetCategoriesByPostID(postID int) ([]Category, error) {
	var categories []Category
	query := `
		SELECT c.category_id, c.category_name, c.created_at
		FROM Categories c
		INNER JOIN Post_Categories pc ON c.category_id = pc.category_id
		WHERE pc.post_id = $1
		ORDER BY c.category_name ASC`
	
	rows, err := cs.DB.Query(query, postID)
	if err != nil {
		return nil, fmt.Errorf("failed to get post categories: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var category Category
		err := rows.Scan(
			&category.ID,
			&category.Name,
			&category.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}
		categories = append(categories, category)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate categories: %w", err)
	}

	return categories, nil
}

// AssignCategoriesToPost assigns categories to a post
func (cs *CategoryService) AssignCategoriesToPost(postID int, categoryIDs []int) error {
	if len(categoryIDs) == 0 {
		return fmt.Errorf("at least one category must be assigned to a post")
	}

	// Start a transaction
	tx, err := cs.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Remove existing categories for the post
	deleteQuery := `DELETE FROM Post_Categories WHERE post_id = $1`
	_, err = tx.Exec(deleteQuery, postID)
	if err != nil {
		return fmt.Errorf("failed to remove existing categories: %w", err)
	}

	// Insert new categories
	insertQuery := `INSERT INTO Post_Categories (post_id, category_id) VALUES ($1, $2)`
	for _, categoryID := range categoryIDs {
		_, err = tx.Exec(insertQuery, postID, categoryID)
		if err != nil {
			return fmt.Errorf("failed to assign category %d: %w", categoryID, err)
		}
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetPostCountByCategory returns the number of posts in each category
func (cs *CategoryService) GetPostCountByCategory() (map[int]int, error) {
	counts := make(map[int]int)
	query := `
		SELECT c.category_id, COUNT(pc.post_id) as post_count
		FROM Categories c
		LEFT JOIN Post_Categories pc ON c.category_id = pc.category_id
		GROUP BY c.category_id`
	
	rows, err := cs.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get category post counts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var categoryID, postCount int
		err := rows.Scan(&categoryID, &postCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan category count: %w", err)
		}
		counts[categoryID] = postCount
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate category counts: %w", err)
	}

	return counts, nil
}