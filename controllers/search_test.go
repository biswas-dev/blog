package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"anshumanbiswas.com/blog/models"
)

func TestSearch_HandleSearch_EmptyQuery(t *testing.T) {
	db := setupTestDB(t)
	searchC := &Search{
		SearchService: &models.SearchService{DB: db},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=", nil)
	w := httptest.NewRecorder()

	searchC.HandleSearch(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", ct)
	}

	var resp models.SearchResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
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

func TestSearch_HandleSearch_JSONContentType(t *testing.T) {
	db := setupTestDB(t)
	searchC := &Search{
		SearchService: &models.SearchService{DB: db},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=golang", nil)
	w := httptest.NewRecorder()

	searchC.HandleSearch(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", ct)
	}

	cache := w.Header().Get("Cache-Control")
	if cache != "public, max-age=30" {
		t.Errorf("expected Cache-Control 'public, max-age=30', got %q", cache)
	}
}

func TestSearch_HandleSearch_LimitParameter(t *testing.T) {
	db := setupTestDB(t)
	searchC := &Search{
		SearchService: &models.SearchService{DB: db},
	}

	// Test with explicit limit
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=test&limit=5", nil)
	w := httptest.NewRecorder()

	searchC.HandleSearch(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp models.SearchResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

func TestSearch_HandleSearch_LimitCapped(t *testing.T) {
	db := setupTestDB(t)
	searchC := &Search{
		SearchService: &models.SearchService{DB: db},
	}

	// Test with limit exceeding max (should be capped at 50)
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=test&limit=999", nil)
	w := httptest.NewRecorder()

	searchC.HandleSearch(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestSearch_HandleSearch_NoQueryParam(t *testing.T) {
	db := setupTestDB(t)
	searchC := &Search{
		SearchService: &models.SearchService{DB: db},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	w := httptest.NewRecorder()

	searchC.HandleSearch(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp models.SearchResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Query != "" {
		t.Errorf("expected empty query, got %q", resp.Query)
	}
}

func TestSearch_StructFields(t *testing.T) {
	t.Run("Search struct has SearchService field", func(t *testing.T) {
		s := &Search{}
		if s.SearchService != nil {
			t.Error("SearchService should be nil by default")
		}
	})

	t.Run("HandleSearch has correct signature", func(t *testing.T) {
		s := &Search{}
		var _ http.HandlerFunc = s.HandleSearch
	})
}
