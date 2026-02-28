package models

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

func setupSearchTestDB(t *testing.T) *sql.DB {
	t.Helper()

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
		dbPassword = "testpass"
	}
	dbName := os.Getenv("PG_DB")
	if dbName == "" {
		dbName = "blog"
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	if err = db.Ping(); err != nil {
		db.Close()
		t.Fatalf("failed to ping database: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

func TestSearchService_EmptyQuery(t *testing.T) {
	db := setupSearchTestDB(t)
	ss := &SearchService{DB: db}

	resp, err := ss.Search(context.Background(), "", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Query != "" {
		t.Errorf("expected empty query, got %q", resp.Query)
	}
	if len(resp.Posts) != 0 {
		t.Errorf("expected 0 posts, got %d", len(resp.Posts))
	}
	if len(resp.Slides) != 0 {
		t.Errorf("expected 0 slides, got %d", len(resp.Slides))
	}
}

func TestSearchService_SearchByTitle(t *testing.T) {
	db := setupSearchTestDB(t)
	ss := &SearchService{DB: db}

	// Ensure roles exist
	db.Exec(`INSERT INTO roles (role_id, role_name) VALUES (2, 'Administrator') ON CONFLICT DO NOTHING`)

	// Create test user
	var userID int
	err := db.QueryRow(`INSERT INTO users (email, username, password, role_id) VALUES ($1, $2, $3, $4) RETURNING user_id`,
		"search-test@test.com", "searchtest", "hash123", 2).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	defer db.Exec("DELETE FROM users WHERE user_id = $1", userID)

	// Create a published post with a unique title
	var postID int
	err = db.QueryRow(`INSERT INTO posts (user_id, category_id, title, content, slug, publication_date, last_edit_date, is_published, featured_image_url, created_at, featured)
		VALUES ($1, 0, $2, $3, $4, NOW(), NOW(), true, '', NOW(), false) RETURNING post_id`,
		userID, "Xylophone Kubernetes Tutorial", "Learn about deploying with Kubernetes", "xylophone-k8s-test").Scan(&postID)
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}
	defer db.Exec("DELETE FROM posts WHERE post_id = $1", postID)

	resp, err := ss.Search(context.Background(), "Xylophone Kubernetes", 10)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}

	if len(resp.Posts) == 0 {
		t.Fatal("expected at least 1 post result")
	}

	if resp.Posts[0].Slug != "xylophone-k8s-test" {
		t.Errorf("expected slug 'xylophone-k8s-test', got %q", resp.Posts[0].Slug)
	}
}

func TestSearchService_DraftsNotReturned(t *testing.T) {
	db := setupSearchTestDB(t)
	ss := &SearchService{DB: db}

	// Ensure roles exist
	db.Exec(`INSERT INTO roles (role_id, role_name) VALUES (2, 'Administrator') ON CONFLICT DO NOTHING`)

	// Create test user
	var userID int
	err := db.QueryRow(`INSERT INTO users (email, username, password, role_id) VALUES ($1, $2, $3, $4) RETURNING user_id`,
		"search-draft@test.com", "searchdraft", "hash123", 2).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	defer db.Exec("DELETE FROM users WHERE user_id = $1", userID)

	// Create an unpublished (draft) post
	var postID int
	err = db.QueryRow(`INSERT INTO posts (user_id, category_id, title, content, slug, publication_date, last_edit_date, is_published, featured_image_url, created_at, featured)
		VALUES ($1, 0, $2, $3, $4, NOW(), NOW(), false, '', NOW(), false) RETURNING post_id`,
		userID, "Zephyr Draft Unicorn Post", "This draft should not appear in search", "zephyr-draft-unicorn").Scan(&postID)
	if err != nil {
		t.Fatalf("failed to create draft post: %v", err)
	}
	defer db.Exec("DELETE FROM posts WHERE post_id = $1", postID)

	resp, err := ss.Search(context.Background(), "Zephyr Draft Unicorn", 10)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}

	if len(resp.Posts) != 0 {
		t.Errorf("expected 0 posts for draft, got %d", len(resp.Posts))
	}
}

func TestSearchService_SearchByContent(t *testing.T) {
	db := setupSearchTestDB(t)
	ss := &SearchService{DB: db}

	db.Exec(`INSERT INTO roles (role_id, role_name) VALUES (2, 'Administrator') ON CONFLICT DO NOTHING`)

	var userID int
	err := db.QueryRow(`INSERT INTO users (email, username, password, role_id) VALUES ($1, $2, $3, $4) RETURNING user_id`,
		"search-content@test.com", "searchcontent", "hash123", 2).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	defer db.Exec("DELETE FROM users WHERE user_id = $1", userID)

	var postID int
	err = db.QueryRow(`INSERT INTO posts (user_id, category_id, title, content, slug, publication_date, last_edit_date, is_published, featured_image_url, created_at, featured)
		VALUES ($1, 0, $2, $3, $4, NOW(), NOW(), true, '', NOW(), false) RETURNING post_id`,
		userID, "Generic Title Here", "This post contains the unique word qwertyuiop for content search testing", "content-search-test").Scan(&postID)
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}
	defer db.Exec("DELETE FROM posts WHERE post_id = $1", postID)

	resp, err := ss.Search(context.Background(), "qwertyuiop", 10)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}

	if len(resp.Posts) == 0 {
		t.Fatal("expected at least 1 post when searching by content")
	}

	if resp.Posts[0].Slug != "content-search-test" {
		t.Errorf("expected slug 'content-search-test', got %q", resp.Posts[0].Slug)
	}
}

func TestSearchService_TitleRanksHigher(t *testing.T) {
	db := setupSearchTestDB(t)
	ss := &SearchService{DB: db}

	db.Exec(`INSERT INTO roles (role_id, role_name) VALUES (2, 'Administrator') ON CONFLICT DO NOTHING`)

	var userID int
	err := db.QueryRow(`INSERT INTO users (email, username, password, role_id) VALUES ($1, $2, $3, $4) RETURNING user_id`,
		"search-rank@test.com", "searchrank", "hash123", 2).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	defer db.Exec("DELETE FROM users WHERE user_id = $1", userID)

	// Post with "flamingo" in title
	var postIDTitle int
	err = db.QueryRow(`INSERT INTO posts (user_id, category_id, title, content, slug, publication_date, last_edit_date, is_published, featured_image_url, created_at, featured)
		VALUES ($1, 0, $2, $3, $4, NOW(), NOW(), true, '', NOW(), false) RETURNING post_id`,
		userID, "Flamingo Migration Guide", "How to handle data migration effectively", "flamingo-title-rank").Scan(&postIDTitle)
	if err != nil {
		t.Fatalf("failed to create post: %v", err)
	}
	defer db.Exec("DELETE FROM posts WHERE post_id = $1", postIDTitle)

	// Post with "flamingo" in content only
	var postIDContent int
	err = db.QueryRow(`INSERT INTO posts (user_id, category_id, title, content, slug, publication_date, last_edit_date, is_published, featured_image_url, created_at, featured)
		VALUES ($1, 0, $2, $3, $4, NOW(), NOW(), true, '', NOW(), false) RETURNING post_id`,
		userID, "General Bird Facts", "The flamingo is a beautiful pink bird found in many regions", "flamingo-content-rank").Scan(&postIDContent)
	if err != nil {
		t.Fatalf("failed to create post: %v", err)
	}
	defer db.Exec("DELETE FROM posts WHERE post_id = $1", postIDContent)

	resp, err := ss.Search(context.Background(), "flamingo", 10)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}

	if len(resp.Posts) < 2 {
		t.Fatalf("expected at least 2 posts, got %d", len(resp.Posts))
	}

	// Title match should rank higher
	if resp.Posts[0].Slug != "flamingo-title-rank" {
		t.Errorf("expected title match 'flamingo-title-rank' to rank first, got %q", resp.Posts[0].Slug)
	}
}

func TestSearchResponse_Structure(t *testing.T) {
	resp := &SearchResponse{
		Query:      "test",
		TotalCount: 2,
		Posts:      []SearchResult{{Type: "post", Title: "Test Post", Slug: "test-post"}},
		Slides:     []SearchResult{{Type: "slide", Title: "Test Slide", Slug: "test-slide"}},
	}

	if resp.TotalCount != 2 {
		t.Errorf("expected TotalCount 2, got %d", resp.TotalCount)
	}
	if resp.Posts[0].Type != "post" {
		t.Errorf("expected type 'post', got %q", resp.Posts[0].Type)
	}
	if resp.Slides[0].Type != "slide" {
		t.Errorf("expected type 'slide', got %q", resp.Slides[0].Type)
	}
}
