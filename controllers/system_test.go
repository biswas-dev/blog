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

func TestListExternalSystems_RequiresAuth(t *testing.T) {
	s := System{SessionService: &models.SessionService{}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/admin/external-systems", nil)
	s.ListExternalSystems(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestGetExternalSystem_RequiresAuth(t *testing.T) {
	s := System{SessionService: &models.SessionService{}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/admin/external-systems/1", nil)
	s.GetExternalSystem(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestCreateExternalSystem_RequiresAuth(t *testing.T) {
	s := System{SessionService: &models.SessionService{}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/admin/external-systems", nil)
	s.CreateExternalSystem(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestUpdateExternalSystem_RequiresAuth(t *testing.T) {
	s := System{SessionService: &models.SessionService{}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/api/admin/external-systems/1", nil)
	s.UpdateExternalSystem(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestDeleteExternalSystem_RequiresAuth(t *testing.T) {
	s := System{SessionService: &models.SessionService{}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/api/admin/external-systems/1", nil)
	s.DeleteExternalSystem(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestTestExternalConnection_RequiresAuth(t *testing.T) {
	s := System{SessionService: &models.SessionService{}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/admin/external-systems/1/test", nil)
	s.TestExternalConnection(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestPreviewSync_RequiresAuth(t *testing.T) {
	s := System{SessionService: &models.SessionService{}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/admin/external-systems/1/preview", nil)
	s.PreviewSync(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestExecuteSync_RequiresAuth(t *testing.T) {
	s := System{SessionService: &models.SessionService{}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/admin/external-systems/1/sync", nil)
	s.ExecuteSync(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestGetSyncLogs_RequiresAuth(t *testing.T) {
	s := System{SessionService: &models.SessionService{}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/admin/external-systems/1/logs", nil)
	s.GetSyncLogs(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestGetCloudinarySettings_RequiresAuth(t *testing.T) {
	s := System{SessionService: &models.SessionService{}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/admin/cloudinary/settings", nil)
	s.GetCloudinarySettings(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestSaveCloudinarySettings_RequiresAuth(t *testing.T) {
	s := System{SessionService: &models.SessionService{}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/admin/cloudinary/settings", nil)
	s.SaveCloudinarySettings(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestDeleteCloudinarySettings_RequiresAuth(t *testing.T) {
	s := System{SessionService: &models.SessionService{}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/api/admin/cloudinary/settings", nil)
	s.DeleteCloudinarySettings(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestTestCloudinaryConnection_RequiresAuth(t *testing.T) {
	s := System{SessionService: &models.SessionService{}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/admin/cloudinary/test", nil)
	s.TestCloudinaryConnection(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestGetCloudinarySignature_RequiresAuth(t *testing.T) {
	s := System{SessionService: &models.SessionService{}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/admin/cloudinary/signature", nil)
	s.GetCloudinarySignature(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}
