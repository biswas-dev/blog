package models

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

// SetupTestDB creates a database connection for testing.
// It reads from environment variables or uses sensible defaults.
// Use t.Cleanup() to ensure proper teardown.
func SetupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	// Read environment variables (same as main app)
	dbHost := os.Getenv("PG_HOST")
	if dbHost == "" {
		dbHost = "127.0.0.1"
	}
	dbPort := os.Getenv("PG_PORT")
	if dbPort == "" {
		dbPort = "5433"
	}
	dbUser := os.Getenv("PG_USER")
	if dbUser == "" {
		dbUser = "blog"
	}
	dbPassword := os.Getenv("PG_PASSWORD")
	if dbPassword == "" {
		dbPassword = "1234"
	}
	dbName := os.Getenv("PG_DB")
	if dbName == "" {
		dbName = "blog"
	}

	// Connect to database
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	// Test database connection
	if err = db.Ping(); err != nil {
		db.Close()
		t.Fatalf("failed to ping database: %v", err)
	}

	// Register cleanup
	t.Cleanup(func() {
		db.Close()
	})

	return db
}

// TxSetup begins a transaction for testing and rolls it back on cleanup.
// This provides test isolation without actually modifying the database.
func TxSetup(t *testing.T, db *sql.DB) *sql.Tx {
	t.Helper()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	// Register cleanup to rollback transaction
	t.Cleanup(func() {
		tx.Rollback()
	})

	return tx
}

// SeedUser creates a test user and returns the user ID.
// Uses a transaction if provided, otherwise uses the database directly.
func SeedUser(t *testing.T, db *sql.DB, email, username, password string, roleID int) int {
	t.Helper()

	userService := &UserService{DB: db}
	user, err := userService.Create(email, username, password, roleID)
	if err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	return user.UserID
}

// SeedCategory creates a test category and returns the category ID.
func SeedCategory(t *testing.T, db *sql.DB, name string) int {
	t.Helper()

	var categoryID int
	err := db.QueryRow(
		"INSERT INTO categories (category_name, created_at) VALUES ($1, NOW()) RETURNING category_id",
		name,
	).Scan(&categoryID)
	if err != nil {
		t.Fatalf("failed to seed category: %v", err)
	}

	return categoryID
}

// SeedPost creates a test post and returns the post ID.
func SeedPost(t *testing.T, db *sql.DB, userID, categoryID int, title, content, slug string, isPublished bool) int {
	t.Helper()

	postService := &PostService{DB: db}
	post, err := postService.Create(userID, categoryID, title, content, isPublished, false, "", slug)
	if err != nil {
		t.Fatalf("failed to seed post: %v", err)
	}

	return post.ID
}

// CleanupUser deletes a test user by ID.
func CleanupUser(t *testing.T, db *sql.DB, userID int) {
	t.Helper()

	_, err := db.Exec("DELETE FROM users WHERE user_id = $1", userID)
	if err != nil {
		t.Logf("warning: failed to cleanup user %d: %v", userID, err)
	}
}

// CleanupPost deletes a test post by ID.
func CleanupPost(t *testing.T, db *sql.DB, postID int) {
	t.Helper()

	_, err := db.Exec("DELETE FROM posts WHERE post_id = $1", postID)
	if err != nil {
		t.Logf("warning: failed to cleanup post %d: %v", postID, err)
	}
}

// CleanupCategory deletes a test category by ID.
func CleanupCategory(t *testing.T, db *sql.DB, categoryID int) {
	t.Helper()

	_, err := db.Exec("DELETE FROM categories WHERE category_id = $1", categoryID)
	if err != nil {
		t.Logf("warning: failed to cleanup category %d: %v", categoryID, err)
	}
}

// UserExists checks if a user with the given ID exists in the database.
func UserExists(t *testing.T, db *sql.DB, userID int) bool {
	t.Helper()

	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE user_id = $1)", userID).Scan(&exists)
	if err != nil {
		t.Fatalf("failed to check if user exists: %v", err)
	}

	return exists
}
