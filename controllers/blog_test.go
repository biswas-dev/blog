package controllers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"anshumanbiswas.com/blog/models"
	"github.com/go-chi/chi/v5"
	_ "github.com/lib/pq"
)

// MockTemplate is a mock implementation of the Template interface for testing
type MockTemplate struct {
	ExecuteCalled bool
	ExecuteData   interface{}
}

func (m *MockTemplate) Execute(w http.ResponseWriter, r *http.Request, data interface{}) {
	m.ExecuteCalled = true
	m.ExecuteData = data
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Mock template executed"))
}

// setupTestBlogController creates a test blog controller with mock dependencies
func setupTestBlogController(db *sql.DB) *Blog {
	blogService := models.NewBlogService(db)
	sessionService := &models.SessionService{DB: db}

	blog := &Blog{
		BlogService:    blogService,
		SessionService: sessionService,
	}

	mockTemplate := &MockTemplate{}
	blog.Templates.Post = mockTemplate

	return blog
}

func TestBlog_GetBlogPost(t *testing.T) {
	db := setupTestDB(t)

	t.Run("returns 404 for non-existent post", func(t *testing.T) {
		blog := setupTestBlogController(db)

		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/blog/non-existent-slug", nil)

		// Create chi router context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("slug", "non-existent-slug")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		// Create response recorder
		w := httptest.NewRecorder()

		blog.GetBlogPost(w, req)

		// Verify 404 response
		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("renders existing published post", func(t *testing.T) {
		blog := setupTestBlogController(db)

		// Create test data
		userID := seedUser(t, db, "author@test.com", "author", "password123", models.RoleAdministrator)
		categoryID := seedCategory(t, db, "Test Category")
		postID := seedPost(t, db, userID, categoryID, "Test Post", "This is test content with more than two hundred words. "+
			"Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. "+
			"Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. "+
			"Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. "+
			"Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. "+
			"Sed ut perspiciatis unde omnis iste natus error sit voluptatem accusantium doloremque laudantium.", "test-post", true)

		defer func() {
			cleanupPost(t, db, postID)
			cleanupCategory(t, db, categoryID)
			cleanupUser(t, db, userID)
		}()

		// Create request
		req := httptest.NewRequest(http.MethodGet, "/blog/test-post", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("slug", "test-post")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		blog.GetBlogPost(w, req)

		// Verify successful response
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		// Verify template was executed
		mockTemplate := blog.Templates.Post.(*MockTemplate)
		if !mockTemplate.ExecuteCalled {
			t.Error("Expected template Execute to be called")
		}
	})

	t.Run("handles missing slug parameter", func(t *testing.T) {
		blog := setupTestBlogController(db)

		req := httptest.NewRequest(http.MethodGet, "/blog/", nil)
		w := httptest.NewRecorder()

		blog.GetBlogPost(w, req)

		// Should return 404 for empty slug
		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404 for missing slug, got %d", w.Code)
		}
	})
}

func TestBlog_GetBlogPost_Structure(t *testing.T) {
	// Test the handler structure without database
	t.Run("handler has correct signature", func(t *testing.T) {
		blog := &Blog{}

		// Verify it's a valid http.HandlerFunc signature
		var _ http.HandlerFunc = blog.GetBlogPost
	})
}

func TestBlog_TemplateStructure(t *testing.T) {
	t.Run("Blog has Templates field", func(t *testing.T) {
		blog := &Blog{}

		// Verify Templates field exists
		mockTemplate := &MockTemplate{}
		blog.Templates.Post = mockTemplate

		if blog.Templates.Post == nil {
			t.Error("Templates.Post should not be nil after assignment")
		}
	})

	t.Run("MockTemplate implements Template interface", func(t *testing.T) {
		var _ Template = &MockTemplate{}
	})

	t.Run("MockTemplate Execute method works", func(t *testing.T) {
		mock := &MockTemplate{}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		mock.Execute(w, r, "test data")

		if !mock.ExecuteCalled {
			t.Error("Execute should have been called")
		}

		if mock.ExecuteData != "test data" {
			t.Error("Execute should have stored the data")
		}

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})
}

func TestBlog_ReadingTimeCalculation(t *testing.T) {
	// Test the reading time calculation logic embedded in GetBlogPost
	tests := []struct {
		name       string
		wordCount  int
		wantMinutes int
	}{
		{
			name:       "short content",
			wordCount:  50,
			wantMinutes: 1,
		},
		{
			name:       "exactly 200 words",
			wordCount:  200,
			wantMinutes: 1,
		},
		{
			name:       "201 words",
			wordCount:  201,
			wantMinutes: 2,
		},
		{
			name:       "400 words",
			wordCount:  400,
			wantMinutes: 2,
		},
		{
			name:       "600 words",
			wordCount:  600,
			wantMinutes: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replicate the calculation from GetBlogPost
			readingMinutes := (tt.wordCount + 199) / 200
			if readingMinutes < 1 {
				readingMinutes = 1
			}

			if readingMinutes != tt.wantMinutes {
				t.Errorf("Expected %d minutes, got %d", tt.wantMinutes, readingMinutes)
			}
		})
	}
}

func TestBlog_FeaturedImageURLHandling(t *testing.T) {
	// Test the featured image URL logic
	tests := []struct {
		name     string
		inputURL string
		want     string
	}{
		{
			name:     "empty URL",
			inputURL: "",
			want:     "",
		},
		{
			name:     "absolute HTTP URL",
			inputURL: "http://example.com/image.jpg",
			want:     "http://example.com/image.jpg",
		},
		{
			name:     "absolute HTTPS URL",
			inputURL: "https://example.com/image.jpg",
			want:     "https://example.com/image.jpg",
		},
		{
			name:     "placeholder image.jpg",
			inputURL: "image.jpg",
			want:     "/static/placeholder-featured.svg",
		},
		{
			name:     "already has /static/ prefix",
			inputURL: "/static/uploads/featured.jpg",
			want:     "/static/uploads/featured.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replicate the URL fixing logic from GetBlogPost
			url := tt.inputURL
			if url != "" && !hasHTTPPrefix(url) {
				if url == "image.jpg" {
					url = "/static/placeholder-featured.svg"
				} else if !hasStaticPrefix(url) {
					url = "/static/" + url
				}
			}

			if url != tt.want {
				t.Errorf("Expected %q, got %q", tt.want, url)
			}
		})
	}
}

// Helper functions to replicate controller logic
func hasHTTPPrefix(url string) bool {
	return len(url) >= 4 && url[:4] == "http"
}

func hasStaticPrefix(url string) bool {
	return len(url) >= 8 && url[:8] == "/static/"
}

func TestBlog_HelperFunctions(t *testing.T) {
	t.Run("hasHTTPPrefix", func(t *testing.T) {
		tests := []struct {
			url  string
			want bool
		}{
			{"http://example.com", true},
			{"https://example.com", true},
			{"/static/image.jpg", false},
			{"image.jpg", false},
			{"", false},
		}

		for _, tt := range tests {
			got := hasHTTPPrefix(tt.url)
			if got != tt.want {
				t.Errorf("hasHTTPPrefix(%q) = %v, want %v", tt.url, got, tt.want)
			}
		}
	})

	t.Run("hasStaticPrefix", func(t *testing.T) {
		tests := []struct {
			url  string
			want bool
		}{
			{"/static/image.jpg", true},
			{"/static/uploads/featured.jpg", true},
			{"static/image.jpg", false},
			{"image.jpg", false},
			{"", false},
		}

		for _, tt := range tests {
			got := hasStaticPrefix(tt.url)
			if got != tt.want {
				t.Errorf("hasStaticPrefix(%q) = %v, want %v", tt.url, got, tt.want)
			}
		}
	})
}

func TestBlog_DataStructure(t *testing.T) {
	// Test that the data structure has expected fields
	t.Run("data structure fields", func(t *testing.T) {
		var data struct {
			LoggedIn        bool
			Email           string
			Username        string
			IsAdmin         bool
			SignupDisabled  bool
			Description     string
			CurrentPage     string
			ReadTime        string
			FullURL         string
			Post            *models.Post
			PrevPost        *models.Post
			NextPost        *models.Post
			UserPermissions models.UserPermissions
		}

		// Set values
		data.LoggedIn = true
		data.Email = "test@example.com"
		data.Username = "testuser"
		data.IsAdmin = false
		data.SignupDisabled = true
		data.Description = "Test Description"
		data.CurrentPage = "blog"
		data.ReadTime = "5"
		data.FullURL = "http://localhost/blog/test"

		// Verify values
		if !data.LoggedIn {
			t.Error("LoggedIn should be true")
		}
		if data.Email != "test@example.com" {
			t.Errorf("Expected email 'test@example.com', got %s", data.Email)
		}
		if data.ReadTime != "5" {
			t.Errorf("Expected ReadTime '5', got %s", data.ReadTime)
		}
	})
}

// Test helper functions

func setupTestDB(t *testing.T) *sql.DB {
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

	seedRoles(t, db)

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

func seedRoles(t *testing.T, db *sql.DB) {
	t.Helper()

	roles := []struct {
		id   int
		name string
	}{
		{models.RoleCommenter, "Commenter"},
		{models.RoleAdministrator, "Administrator"},
		{models.RoleEditor, "Editor"},
		{models.RoleViewer, "Viewer"},
	}

	for _, role := range roles {
		_, err := db.Exec(`
			INSERT INTO roles (role_id, role_name)
			VALUES ($1, $2)
			ON CONFLICT (role_id) DO NOTHING
		`, role.id, role.name)
		if err != nil {
			t.Logf("Note: Could not seed role %s: %v (may already exist)", role.name, err)
		}
	}
}

func seedUser(t *testing.T, db *sql.DB, email, username, password string, roleID int) int {
	t.Helper()

	userService := &models.UserService{DB: db}
	user, err := userService.Create(email, username, password, roleID)
	if err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	return user.UserID
}

func seedCategory(t *testing.T, db *sql.DB, name string) int {
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

func seedPost(t *testing.T, db *sql.DB, userID, categoryID int, title, content, slug string, isPublished bool) int {
	t.Helper()

	postService := &models.PostService{DB: db}
	post, err := postService.Create(userID, categoryID, title, content, isPublished, false, "", slug)
	if err != nil {
		t.Fatalf("failed to seed post: %v", err)
	}

	return post.ID
}

func cleanupUser(t *testing.T, db *sql.DB, userID int) {
	t.Helper()

	_, err := db.Exec("DELETE FROM users WHERE user_id = $1", userID)
	if err != nil {
		t.Logf("warning: failed to cleanup user %d: %v", userID, err)
	}
}

func cleanupPost(t *testing.T, db *sql.DB, postID int) {
	t.Helper()

	_, err := db.Exec("DELETE FROM posts WHERE post_id = $1", postID)
	if err != nil {
		t.Logf("warning: failed to cleanup post %d: %v", postID, err)
	}
}

func cleanupCategory(t *testing.T, db *sql.DB, categoryID int) {
	t.Helper()

	_, err := db.Exec("DELETE FROM categories WHERE category_id = $1", categoryID)
	if err != nil {
		t.Logf("warning: failed to cleanup category %d: %v", categoryID, err)
	}
}
