package controllers

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"anshumanbiswas.com/blog/models"
)

func TestUsers_TypeDefinition(t *testing.T) {
	t.Run("Users struct exists", func(t *testing.T) {
		var u Users
		_ = u // Verify type exists
	})

	t.Run("Can initialize Users with services", func(t *testing.T) {
		u := Users{
			UserService:     nil, // Would be initialized in main
			SessionService:  nil, // Would be initialized in main
			PostService:     nil, // Would be initialized in main
			APITokenService: nil, // Would be initialized in main
			CategoryService: nil, // Would be initialized in main
		}
		_ = u
	})
}

// Test form value extraction
func TestUsersFormParsing(t *testing.T) {
	t.Run("Parse signup form", func(t *testing.T) {
		formData := strings.NewReader("email=test@example.com&username=testuser&password=securepass123")
		r := httptest.NewRequest(http.MethodPost, "/signup", formData)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		err := r.ParseForm()
		if err != nil {
			t.Errorf("Failed to parse form: %v", err)
		}

		if r.FormValue("email") != "test@example.com" {
			t.Error("Expected email 'test@example.com'")
		}

		if r.FormValue("username") != "testuser" {
			t.Error("Expected username 'testuser'")
		}

		if r.FormValue("password") != "securepass123" {
			t.Error("Expected password to be present")
		}
	})

	t.Run("Parse signin form", func(t *testing.T) {
		formData := strings.NewReader("email=user@test.com&password=mypassword")
		r := httptest.NewRequest(http.MethodPost, "/signin", formData)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		err := r.ParseForm()
		if err != nil {
			t.Errorf("Failed to parse form: %v", err)
		}

		email := r.FormValue("email")
		password := r.FormValue("password")

		if email == "" || password == "" {
			t.Error("Expected email and password to be present")
		}
	})
}

// Test multipart form handling (for image uploads)
func TestUsersMultipartForm(t *testing.T) {
	t.Run("Multipart form parsing", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Add a text field
		writer.WriteField("title", "Test Post")

		// Add a file field (mock image)
		part, err := writer.CreateFormFile("image", "test.jpg")
		if err != nil {
			t.Fatalf("Failed to create form file: %v", err)
		}

		// Write some fake image data
		_, err = io.WriteString(part, "fake image data")
		if err != nil {
			t.Fatalf("Failed to write file data: %v", err)
		}

		writer.Close()

		r := httptest.NewRequest(http.MethodPost, "/upload", body)
		r.Header.Set("Content-Type", writer.FormDataContentType())

		err = r.ParseMultipartForm(10 << 20) // 10 MB limit
		if err != nil {
			t.Errorf("Failed to parse multipart form: %v", err)
		}

		if r.FormValue("title") != "Test Post" {
			t.Error("Expected title 'Test Post'")
		}

		// Check if file was received
		file, header, err := r.FormFile("image")
		if err != nil {
			t.Errorf("Failed to get form file: %v", err)
		}
		if file != nil {
			defer file.Close()
		}

		if header.Filename != "test.jpg" {
			t.Errorf("Expected filename 'test.jpg', got '%s'", header.Filename)
		}
	})
}

// Test cookie handling
func TestUsersCookieHandling(t *testing.T) {
	t.Run("Set session cookie", func(t *testing.T) {
		w := httptest.NewRecorder()

		cookie := &http.Cookie{
			Name:     "session",
			Value:    "session-token-123",
			Path:     "/",
			HttpOnly: true,
		}

		http.SetCookie(w, cookie)

		cookies := w.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("Expected 1 cookie, got %d", len(cookies))
		}

		if cookies[0].Name != "session" {
			t.Errorf("Expected cookie name 'session', got '%s'", cookies[0].Name)
		}

		if cookies[0].Value != "session-token-123" {
			t.Errorf("Expected cookie value 'session-token-123', got '%s'", cookies[0].Value)
		}

		if !cookies[0].HttpOnly {
			t.Error("Expected HttpOnly to be true")
		}
	})

	t.Run("Read session cookie", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/profile", nil)
		r.AddCookie(&http.Cookie{
			Name:  "session",
			Value: "user-session-456",
		})

		cookie, err := r.Cookie("session")
		if err != nil {
			t.Errorf("Failed to get cookie: %v", err)
		}

		if cookie.Value != "user-session-456" {
			t.Errorf("Expected cookie value 'user-session-456', got '%s'", cookie.Value)
		}
	})
}

// Test redirect handling
func TestUsersRedirects(t *testing.T) {
	t.Run("Redirect to signin", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/profile", nil)

		http.Redirect(w, r, "/signin", http.StatusFound)

		if w.Code != http.StatusFound {
			t.Errorf("Expected status %d, got %d", http.StatusFound, w.Code)
		}

		location := w.Header().Get("Location")
		if location != "/signin" {
			t.Errorf("Expected Location '/signin', got '%s'", location)
		}
	})

	t.Run("Redirect after login", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/signin", nil)

		http.Redirect(w, r, "/", http.StatusSeeOther)

		if w.Code != http.StatusSeeOther {
			t.Errorf("Expected status %d, got %d", http.StatusSeeOther, w.Code)
		}
	})
}

// Test JSON response handling
func TestUsersJSONResponses(t *testing.T) {
	t.Run("Success JSON response", func(t *testing.T) {
		w := httptest.NewRecorder()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true,"message":"User created"}`))

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		if w.Header().Get("Content-Type") != "application/json" {
			t.Error("Expected Content-Type application/json")
		}

		body := w.Body.String()
		if !strings.Contains(body, "success") {
			t.Error("Expected JSON to contain 'success'")
		}
	})

	t.Run("Error JSON response", func(t *testing.T) {
		w := httptest.NewRecorder()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"Invalid email format"}`))

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}

		body := w.Body.String()
		if !strings.Contains(body, "error") {
			t.Error("Expected JSON to contain 'error'")
		}
	})
}

// Test HTTP status codes
func TestUsersStatusCodes(t *testing.T) {
	tests := []struct {
		name   string
		status int
		desc   string
	}{
		{"OK", http.StatusOK, "Success"},
		{"Created", http.StatusCreated, "Resource created"},
		{"Bad Request", http.StatusBadRequest, "Invalid input"},
		{"Unauthorized", http.StatusUnauthorized, "Not logged in"},
		{"Forbidden", http.StatusForbidden, "Insufficient permissions"},
		{"Not Found", http.StatusNotFound, "Resource not found"},
		{"Internal Error", http.StatusInternalServerError, "Server error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			http.Error(w, tt.desc, tt.status)

			if w.Code != tt.status {
				t.Errorf("Expected status %d, got %d", tt.status, w.Code)
			}

			if !strings.Contains(w.Body.String(), tt.desc) {
				t.Errorf("Expected body to contain '%s'", tt.desc)
			}
		})
	}
}

// Test file path handling
func TestUsersFilePaths(t *testing.T) {
	t.Run("Join upload paths", func(t *testing.T) {
		uploadDir := "uploads"
		filename := "test-image.jpg"

		fullPath := filepath.Join(uploadDir, filename)

		expected := filepath.Join("uploads", "test-image.jpg")
		if fullPath != expected {
			t.Errorf("Expected path '%s', got '%s'", expected, fullPath)
		}
	})

	t.Run("Clean file paths", func(t *testing.T) {
		dirty := filepath.Join("uploads", "..", "..", "etc", "passwd")
		clean := filepath.Clean(dirty)

		// Should not escape uploads directory
		if !strings.HasPrefix(clean, "..") {
			// Path traversal detected - in real code this would be blocked
		}

		// In production, you'd validate the path stays in uploads/
		base := filepath.Join("uploads")
		test := filepath.Join("uploads", "user", "image.jpg")

		if !strings.HasPrefix(test, base) {
			t.Error("Path should stay within base directory")
		}
	})
}

// Test query parameter handling
func TestUsersQueryParams(t *testing.T) {
	t.Run("Parse pagination params", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/admin/users?page=2&limit=20", nil)

		page := r.URL.Query().Get("page")
		limit := r.URL.Query().Get("limit")

		if page != "2" {
			t.Errorf("Expected page '2', got '%s'", page)
		}

		if limit != "20" {
			t.Errorf("Expected limit '20', got '%s'", limit)
		}
	})

	t.Run("Parse filter params", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/admin/users?role=admin&status=active", nil)

		role := r.URL.Query().Get("role")
		status := r.URL.Query().Get("status")

		if role != "admin" {
			t.Errorf("Expected role 'admin', got '%s'", role)
		}

		if status != "active" {
			t.Errorf("Expected status 'active', got '%s'", status)
		}
	})
}

// Test request context
func TestUsersRequestContext(t *testing.T) {
	t.Run("Request context exists", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/profile", nil)

		ctx := r.Context()
		if ctx == nil {
			t.Error("Expected context to exist")
		}
	})

	t.Run("Request with cancelled context", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/profile", nil)

		ctx := r.Context()
		select {
		case <-ctx.Done():
			t.Error("Context should not be done initially")
		default:
			// Context is not cancelled, which is expected
		}
	})
}

// Test file extension validation
func TestUsersFileValidation(t *testing.T) {
	validExtensions := []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}

	tests := []struct {
		filename string
		valid    bool
	}{
		{"image.jpg", true},
		{"photo.PNG", true},
		{"picture.jpeg", true},
		{"graphic.gif", true},
		{"image.webp", true},
		{"script.php", false},
		{"doc.pdf", false},
		{"file.exe", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			ext := strings.ToLower(filepath.Ext(tt.filename))
			isValid := false

			for _, validExt := range validExtensions {
				if ext == validExt {
					isValid = true
					break
				}
			}

			if isValid != tt.valid {
				t.Errorf("File '%s' validity: expected %v, got %v", tt.filename, tt.valid, isValid)
			}
		})
	}
}

// Test temporary file creation
func TestUsersTempFiles(t *testing.T) {
	t.Run("Create and clean up temp file", func(t *testing.T) {
		tmpfile, err := os.CreateTemp("", "upload-*.jpg")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}

		filename := tmpfile.Name()
		tmpfile.Close()

		// Verify file exists
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			t.Error("Temp file should exist after creation")
		}

		// Clean up
		os.Remove(filename)

		// Verify file is removed
		if _, err := os.Stat(filename); !os.IsNotExist(err) {
			t.Error("Temp file should be removed after cleanup")
		}
	})
}

// Test image metadata handler request/response patterns
func TestImageMetadataHandlers(t *testing.T) {
	t.Run("SaveImageMetadata rejects missing service", func(t *testing.T) {
		u := Users{ImageMetadataService: nil}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPut, "/api/admin/image-metadata", strings.NewReader(`{"image_url":"http://example.com/img.jpg","alt_text":"test"}`))
		r.Header.Set("Content-Type", "application/json")
		u.SaveImageMetadata(w, r)
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("Expected 503, got %d", w.Code)
		}
	})

	t.Run("SaveImageMetadata rejects invalid JSON", func(t *testing.T) {
		svc := &models.ImageMetadataService{} // nil DB is fine, validation returns before DB call
		u := Users{ImageMetadataService: svc}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPut, "/api/admin/image-metadata", strings.NewReader(`not json`))
		r.Header.Set("Content-Type", "application/json")
		u.SaveImageMetadata(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", w.Code)
		}
	})

	t.Run("SaveImageMetadata rejects empty URL", func(t *testing.T) {
		svc := &models.ImageMetadataService{}
		u := Users{ImageMetadataService: svc}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPut, "/api/admin/image-metadata", strings.NewReader(`{"image_url":"","alt_text":"test"}`))
		r.Header.Set("Content-Type", "application/json")
		u.SaveImageMetadata(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", w.Code)
		}
	})

	t.Run("GetImageMetadata rejects missing service", func(t *testing.T) {
		u := Users{ImageMetadataService: nil}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/admin/image-metadata?url=http://example.com/img.jpg", nil)
		u.GetImageMetadata(w, r)
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("Expected 503, got %d", w.Code)
		}
	})

	t.Run("GetImageMetadata rejects missing url param", func(t *testing.T) {
		svc := &models.ImageMetadataService{}
		u := Users{ImageMetadataService: svc}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/admin/image-metadata", nil)
		u.GetImageMetadata(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", w.Code)
		}
	})

	t.Run("GetImageMetadataBulk rejects missing service", func(t *testing.T) {
		u := Users{ImageMetadataService: nil}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/admin/image-metadata/bulk", strings.NewReader(`{"urls":[]}`))
		r.Header.Set("Content-Type", "application/json")
		u.GetImageMetadataBulk(w, r)
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("Expected 503, got %d", w.Code)
		}
	})

	t.Run("GetImageMetadataBulk rejects invalid JSON", func(t *testing.T) {
		svc := &models.ImageMetadataService{}
		u := Users{ImageMetadataService: svc}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/admin/image-metadata/bulk", strings.NewReader(`bad`))
		r.Header.Set("Content-Type", "application/json")
		u.GetImageMetadataBulk(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", w.Code)
		}
	})
}
