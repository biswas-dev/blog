package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"anshumanbiswas.com/blog/models"
)

func TestStaticHandler(t *testing.T) {
	t.Run("creates valid handler", func(t *testing.T) {
		mockTemplate := &MockTemplate{}
		mockSessionService := &models.SessionService{} // DB is nil, but that's OK for structure test

		handler := StaticHandler(mockTemplate, mockSessionService)

		if handler == nil {
			t.Fatal("StaticHandler should return a valid handler")
		}

		// Verify it's a valid http.HandlerFunc
		var _ http.HandlerFunc = handler
	})

	t.Run("handler executes template", func(t *testing.T) {
		mockTemplate := &MockTemplate{}
		mockSessionService := &models.SessionService{}

		handler := StaticHandler(mockTemplate, mockSessionService)

		req := httptest.NewRequest(http.MethodGet, "/about", nil)
		w := httptest.NewRecorder()

		handler(w, req)

		if !mockTemplate.ExecuteCalled {
			t.Error("Template Execute should have been called")
		}

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("sets correct page data for /about", func(t *testing.T) {
		mockTemplate := &MockTemplate{}
		mockSessionService := &models.SessionService{}

		handler := StaticHandler(mockTemplate, mockSessionService)

		req := httptest.NewRequest(http.MethodGet, "/about", nil)
		w := httptest.NewRecorder()

		handler(w, req)

		// Check that template was called with data
		if mockTemplate.ExecuteData == nil {
			t.Fatal("Execute should have been called with data")
		}

		// Type assert the data
		data, ok := mockTemplate.ExecuteData.(struct {
			Email           string
			LoggedIn        bool
			SignupDisabled  bool
			IsAdmin         bool
			Description     string
			CurrentPage     string
			Username        string
			UserPermissions models.UserPermissions
		})

		if !ok {
			t.Fatal("Data should have correct structure")
		}

		if data.CurrentPage != "about" {
			t.Errorf("Expected CurrentPage 'about', got %s", data.CurrentPage)
		}

		if data.Description != "About Anshuman Biswas - Software Engineering Leader" {
			t.Errorf("Expected about description, got %s", data.Description)
		}
	})

	t.Run("sets correct page data for /docs/formatting-guide", func(t *testing.T) {
		mockTemplate := &MockTemplate{}
		mockSessionService := &models.SessionService{}

		handler := StaticHandler(mockTemplate, mockSessionService)

		req := httptest.NewRequest(http.MethodGet, "/docs/formatting-guide", nil)
		w := httptest.NewRecorder()

		handler(w, req)

		data, ok := mockTemplate.ExecuteData.(struct {
			Email           string
			LoggedIn        bool
			SignupDisabled  bool
			IsAdmin         bool
			Description     string
			CurrentPage     string
			Username        string
			UserPermissions models.UserPermissions
		})

		if !ok {
			t.Fatal("Data should have correct structure")
		}

		if data.CurrentPage != "admin" {
			t.Errorf("Expected CurrentPage 'admin', got %s", data.CurrentPage)
		}

		if data.Description != "Content Formatting Guide - Anshuman Biswas Blog" {
			t.Errorf("Expected formatting guide description, got %s", data.Description)
		}
	})

	t.Run("sets default page data for unknown paths", func(t *testing.T) {
		mockTemplate := &MockTemplate{}
		mockSessionService := &models.SessionService{}

		handler := StaticHandler(mockTemplate, mockSessionService)

		req := httptest.NewRequest(http.MethodGet, "/unknown-page", nil)
		w := httptest.NewRecorder()

		handler(w, req)

		data, ok := mockTemplate.ExecuteData.(struct {
			Email           string
			LoggedIn        bool
			SignupDisabled  bool
			IsAdmin         bool
			Description     string
			CurrentPage     string
			Username        string
			UserPermissions models.UserPermissions
		})

		if !ok {
			t.Fatal("Data should have correct structure")
		}

		if data.CurrentPage != "static" {
			t.Errorf("Expected CurrentPage 'static', got %s", data.CurrentPage)
		}

		if data.Description != "Anshuman Biswas Blog - Engineering Insights" {
			t.Errorf("Expected default description, got %s", data.Description)
		}
	})

	t.Run("handles user not logged in", func(t *testing.T) {
		mockTemplate := &MockTemplate{}
		mockSessionService := &models.SessionService{} // No DB, so IsUserLoggedIn will fail

		handler := StaticHandler(mockTemplate, mockSessionService)

		req := httptest.NewRequest(http.MethodGet, "/about", nil)
		w := httptest.NewRecorder()

		handler(w, req)

		data, ok := mockTemplate.ExecuteData.(struct {
			Email           string
			LoggedIn        bool
			SignupDisabled  bool
			IsAdmin         bool
			Description     string
			CurrentPage     string
			Username        string
			UserPermissions models.UserPermissions
		})

		if !ok {
			t.Fatal("Data should have correct structure")
		}

		// Should default to not logged in when session check fails
		if data.LoggedIn {
			t.Error("LoggedIn should be false when user is not logged in")
		}

		if data.IsAdmin {
			t.Error("IsAdmin should be false when user is not logged in")
		}

		if data.Email != "" {
			t.Errorf("Email should be empty when not logged in, got %s", data.Email)
		}

		// Should have default commenter permissions
		if data.UserPermissions.CanManageUsers {
			t.Error("Non-logged-in user should not have admin permissions")
		}
	})
}

func TestStaticHandler_PageSwitch(t *testing.T) {
	// Test all the switch cases for page-specific values
	tests := []struct {
		name            string
		path            string
		expectedPage    string
		expectedDescPrefix string
	}{
		{
			name:            "about page",
			path:            "/about",
			expectedPage:    "about",
			expectedDescPrefix: "About Anshuman Biswas",
		},
		{
			name:            "admin formatting guide",
			path:            "/admin/formatting-guide",
			expectedPage:    "admin",
			expectedDescPrefix: "Content Formatting Guide",
		},
		{
			name:            "docs formatting guide",
			path:            "/docs/formatting-guide",
			expectedPage:    "admin",
			expectedDescPrefix: "Content Formatting Guide",
		},
		{
			name:            "unknown page",
			path:            "/some-other-page",
			expectedPage:    "static",
			expectedDescPrefix: "Anshuman Biswas Blog",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTemplate := &MockTemplate{}
			mockSessionService := &models.SessionService{}

			handler := StaticHandler(mockTemplate, mockSessionService)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			handler(w, req)

			data, ok := mockTemplate.ExecuteData.(struct {
				Email           string
				LoggedIn        bool
				SignupDisabled  bool
				IsAdmin         bool
				Description     string
				CurrentPage     string
				Username        string
				UserPermissions models.UserPermissions
			})

			if !ok {
				t.Fatal("Data should have correct structure")
			}

			if data.CurrentPage != tt.expectedPage {
				t.Errorf("Expected CurrentPage %q, got %q", tt.expectedPage, data.CurrentPage)
			}

			if len(data.Description) == 0 {
				t.Error("Description should not be empty")
			}

			// Just check that description starts with expected prefix
			// (we don't need exact match as it might change)
			if len(data.Description) >= len(tt.expectedDescPrefix) {
				if data.Description[:len(tt.expectedDescPrefix)] != tt.expectedDescPrefix {
					t.Errorf("Expected description to start with %q, got %q",
						tt.expectedDescPrefix, data.Description)
				}
			}
		})
	}
}

func TestStaticHandler_HTTPMethods(t *testing.T) {
	// Test that handler works with different HTTP methods
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			mockTemplate := &MockTemplate{}
			mockSessionService := &models.SessionService{}

			handler := StaticHandler(mockTemplate, mockSessionService)

			req := httptest.NewRequest(method, "/about", nil)
			w := httptest.NewRecorder()

			handler(w, req)

			// Should handle all methods (doesn't validate method)
			if !mockTemplate.ExecuteCalled {
				t.Error("Template should have been executed")
			}
		})
	}
}
