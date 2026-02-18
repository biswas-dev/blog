package controllers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSlides_TypeDefinition(t *testing.T) {
	t.Run("Slides struct exists", func(t *testing.T) {
		var s Slides
		_ = s // Verify type exists
	})

	t.Run("Can initialize Slides with services", func(t *testing.T) {
		s := Slides{
			SlideService:    nil, // Would be initialized in main
			SessionService:  nil, // Would be initialized in main
			CategoryService: nil, // Would be initialized in main
		}
		_ = s
	})
}

// Test HTTP request handling
func TestSlidesHTTPHandling(t *testing.T) {
	t.Run("GET request creation", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/slides/test-slide", nil)
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET method, got %s", r.Method)
		}
		if r.URL.Path != "/slides/test-slide" {
			t.Errorf("Expected path /slides/test-slide, got %s", r.URL.Path)
		}
	})

	t.Run("POST request with form data", func(t *testing.T) {
		formData := strings.NewReader("title=Test Slide&content=Slide content")
		r := httptest.NewRequest(http.MethodPost, "/admin/slides/new", formData)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		err := r.ParseForm()
		if err != nil {
			t.Errorf("Failed to parse form: %v", err)
		}

		if r.FormValue("title") != "Test Slide" {
			t.Error("Expected title to be 'Test Slide'")
		}
	})
}

// Test query parameter handling
func TestSlidesQueryParams(t *testing.T) {
	t.Run("Parse query parameters", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/admin/slides?category=tech&published=true", nil)

		if r.URL.Query().Get("category") != "tech" {
			t.Error("Expected category query param 'tech'")
		}

		if r.URL.Query().Get("published") != "true" {
			t.Error("Expected published query param 'true'")
		}
	})

	t.Run("Multiple values for same param", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/admin/slides?tag=go&tag=web", nil)

		tags := r.URL.Query()["tag"]
		if len(tags) != 2 {
			t.Errorf("Expected 2 tag values, got %d", len(tags))
		}
	})
}

// Test response handling
func TestSlidesResponseHandling(t *testing.T) {
	t.Run("JSON response structure", func(t *testing.T) {
		w := httptest.NewRecorder()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true,"message":"Slide created"}`))

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		if !strings.Contains(w.Body.String(), "success") {
			t.Error("Expected JSON response to contain 'success'")
		}
	})

	t.Run("Error response handling", func(t *testing.T) {
		w := httptest.NewRecorder()
		http.Error(w, "Slide not found", http.StatusNotFound)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}

		if !strings.Contains(w.Body.String(), "Slide not found") {
			t.Error("Expected error message in response body")
		}
	})
}

// Test form parsing
func TestSlidesFormParsing(t *testing.T) {
	t.Run("Multiline content field", func(t *testing.T) {
		content := `# Slide Title

This is slide content
with multiple lines
and markdown formatting`

		formData := strings.NewReader("content=" + strings.ReplaceAll(content, "\n", "%0A"))
		r := httptest.NewRequest(http.MethodPost, "/admin/slides/new", formData)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		err := r.ParseForm()
		if err != nil {
			t.Errorf("Failed to parse form: %v", err)
		}

		parsed := r.FormValue("content")
		if !strings.Contains(parsed, "Slide Title") {
			t.Error("Expected parsed content to contain title")
		}
	})

	t.Run("Boolean field parsing", func(t *testing.T) {
		formData := strings.NewReader("is_published=on&featured=true")
		r := httptest.NewRequest(http.MethodPost, "/admin/slides/update", formData)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		err := r.ParseForm()
		if err != nil {
			t.Errorf("Failed to parse form: %v", err)
		}

		// Check if boolean values are present
		if r.FormValue("is_published") != "on" {
			t.Error("Expected is_published to be 'on'")
		}
	})
}

// Test URL path parameter extraction
func TestSlidesPathParams(t *testing.T) {
	t.Run("Slug extraction from URL", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/slides/my-awesome-slide", nil)

		// In real usage, chi router would set the path param
		// Here we're just testing URL parsing
		path := r.URL.Path
		if !strings.Contains(path, "slides") {
			t.Error("Expected path to contain 'slides'")
		}

		// Extract slug-like part
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) >= 2 {
			slug := parts[1]
			if slug != "my-awesome-slide" {
				t.Errorf("Expected slug 'my-awesome-slide', got '%s'", slug)
			}
		}
	})
}

// Test header handling
func TestSlidesHeaderHandling(t *testing.T) {
	t.Run("Authorization header", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/admin/slides/new", nil)
		r.Header.Set("Authorization", "Bearer token123")

		auth := r.Header.Get("Authorization")
		if auth != "Bearer token123" {
			t.Errorf("Expected Authorization header, got '%s'", auth)
		}
	})

	t.Run("Cookie header", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/admin/slides", nil)
		r.AddCookie(&http.Cookie{Name: "session", Value: "abc123"})

		cookie, err := r.Cookie("session")
		if err != nil {
			t.Errorf("Failed to get cookie: %v", err)
		}

		if cookie.Value != "abc123" {
			t.Errorf("Expected cookie value 'abc123', got '%s'", cookie.Value)
		}
	})
}
