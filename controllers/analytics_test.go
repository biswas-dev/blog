package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"anshumanbiswas.com/blog/models"
)

func TestAnalyticsDashboard_RequiresAuth(t *testing.T) {
	a := Analytics{SessionService: &models.SessionService{}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/admin/analytics", nil)
	a.Dashboard(w, r)
	if w.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/signin" {
		t.Errorf("expected redirect to /signin, got %q", loc)
	}
}

func TestGetAnalyticsJSON_RequiresAuth(t *testing.T) {
	a := Analytics{SessionService: &models.SessionService{}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/admin/analytics", nil)
	a.GetAnalyticsJSON(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestGetVisitorDetail_RequiresAuth(t *testing.T) {
	a := Analytics{SessionService: &models.SessionService{}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/admin/analytics/visitor?ip=1.2.3.4", nil)
	a.GetVisitorDetail(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}
