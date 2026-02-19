package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"anshumanbiswas.com/blog/models"
)

func TestSystemDashboard_RequiresAuth(t *testing.T) {
	// Create a System controller with nil services (we're just testing auth)
	systemC := System{
		SessionService: &models.SessionService{},
	}

	// Create a request without authentication
	req := httptest.NewRequest("GET", "/admin/system", nil)
	w := httptest.NewRecorder()

	systemC.Dashboard(w, req)

	// Should redirect to signin
	if w.Code != http.StatusFound {
		t.Errorf("Expected redirect (302), got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if location != "/signin" {
		t.Errorf("Expected redirect to /signin, got %s", location)
	}
}

func TestGetSystemInfoJSON_RequiresAuth(t *testing.T) {
	systemC := System{
		SessionService: &models.SessionService{},
	}

	req := httptest.NewRequest("GET", "/api/admin/system", nil)
	w := httptest.NewRecorder()

	systemC.GetSystemInfoJSON(w, req)

	// Should return 401 Unauthorized
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", w.Code)
	}
}

func TestExportDatabase_RequiresAuth(t *testing.T) {
	systemC := System{
		SessionService: &models.SessionService{},
	}

	req := httptest.NewRequest("GET", "/api/admin/db/export", nil)
	w := httptest.NewRecorder()

	systemC.ExportDatabase(w, req)

	// Should return 401 Unauthorized
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", w.Code)
	}
}

func TestImportDatabase_RequiresAuth(t *testing.T) {
	systemC := System{
		SessionService: &models.SessionService{},
	}

	req := httptest.NewRequest("POST", "/api/admin/db/import", nil)
	w := httptest.NewRecorder()

	systemC.ImportDatabase(w, req)

	// Should return 401 Unauthorized
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", w.Code)
	}
}

func TestSystemService_GetSystemInfo_Structure(t *testing.T) {
	t.Skip("Skipping test that requires database connection")
	// This test is skipped because GetSystemInfo requires a real database connection
	// Integration tests with a test database should be used instead
}
