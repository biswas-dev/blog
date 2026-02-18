package controllers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsAjaxRequest(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		expected bool
	}{
		{
			name:     "XMLHttpRequest header",
			headers:  map[string]string{"X-Requested-With": "XMLHttpRequest"},
			expected: true,
		},
		{
			name:     "Accept JSON without HTML",
			headers:  map[string]string{"Accept": "application/json"},
			expected: true,
		},
		{
			name:     "Accept JSON with HTML prefers JSON",
			headers:  map[string]string{"Accept": "application/json, text/html"},
			expected: false, // Has both, so treated as regular request
		},
		{
			name:     "Content-Type JSON",
			headers:  map[string]string{"Content-Type": "application/json"},
			expected: true,
		},
		{
			name:     "No special headers",
			headers:  map[string]string{},
			expected: false,
		},
		{
			name:     "Regular form submission",
			headers:  map[string]string{"Content-Type": "application/x-www-form-urlencoded"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			for key, value := range tt.headers {
				r.Header.Set(key, value)
			}

			result := isAjaxRequest(r)
			if result != tt.expected {
				t.Errorf("isAjaxRequest() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCategories_TypeDefinition(t *testing.T) {
	t.Run("Categories struct exists", func(t *testing.T) {
		var c Categories
		_ = c // Verify type exists
	})

	t.Run("Can initialize Categories with services", func(t *testing.T) {
		c := Categories{
			CategoryService: nil, // Would be initialized in main
			SessionService:  nil, // Would be initialized in main
		}
		_ = c
	})
}

// Test helper functions and type safety
func TestCategoriesHelpers(t *testing.T) {
	t.Run("isAjaxRequest with case insensitive headers", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.Header.Set("Content-Type", "APPLICATION/JSON; charset=utf-8")

		if !isAjaxRequest(r) {
			t.Error("Expected isAjaxRequest to handle uppercase Content-Type")
		}
	})

	t.Run("isAjaxRequest with mixed case Accept", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Accept", "Application/JSON")

		if !isAjaxRequest(r) {
			t.Error("Expected isAjaxRequest to handle mixed case Accept header")
		}
	})
}

// Test request parsing
func TestCategoriesRequestParsing(t *testing.T) {
	t.Run("POST request with form data", func(t *testing.T) {
		formData := strings.NewReader("category_name=Technology&description=Tech posts")
		r := httptest.NewRequest(http.MethodPost, "/admin/categories", formData)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		err := r.ParseForm()
		if err != nil {
			t.Errorf("Failed to parse form: %v", err)
		}

		if r.FormValue("category_name") != "Technology" {
			t.Error("Expected category_name to be Technology")
		}
	})

	t.Run("JSON request detection", func(t *testing.T) {
		jsonData := strings.NewReader(`{"name":"Technology"}`)
		r := httptest.NewRequest(http.MethodPost, "/admin/categories", jsonData)
		r.Header.Set("Content-Type", "application/json")

		if !isAjaxRequest(r) {
			t.Error("JSON request should be detected as AJAX")
		}
	})
}

// Test HTTP method handling
func TestCategoriesHTTPMethods(t *testing.T) {
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
	}

	for _, method := range methods {
		t.Run("Request with "+method, func(t *testing.T) {
			r := httptest.NewRequest(method, "/admin/categories", nil)
			if r.Method != method {
				t.Errorf("Expected method %s, got %s", method, r.Method)
			}
		})
	}
}

// Test response writer behavior
func TestCategoriesResponseWriter(t *testing.T) {
	t.Run("httptest recorder captures status", func(t *testing.T) {
		w := httptest.NewRecorder()
		w.WriteHeader(http.StatusOK)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}
	})

	t.Run("httptest recorder captures headers", func(t *testing.T) {
		w := httptest.NewRecorder()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if w.Header().Get("Content-Type") != "application/json" {
			t.Error("Expected Content-Type header to be set")
		}
	})

	t.Run("httptest recorder captures body", func(t *testing.T) {
		w := httptest.NewRecorder()
		w.Write([]byte("test response"))

		if w.Body.String() != "test response" {
			t.Errorf("Expected body 'test response', got '%s'", w.Body.String())
		}
	})
}
