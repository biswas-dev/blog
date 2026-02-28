package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"anshumanbiswas.com/blog/models"
)

// Search handles search API requests
type Search struct {
	SearchService *models.SearchService
}

// HandleSearch handles GET /api/search?q=...&limit=10
func (s *Search) HandleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	// Parse limit with default of 10, max of 50
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 50 {
		limit = 50
	}

	// Empty query returns empty response
	if query == "" {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=30")
		json.NewEncoder(w).Encode(&models.SearchResponse{
			Query:  "",
			Posts:  []models.SearchResult{},
			Slides: []models.SearchResult{},
		})
		return
	}

	results, err := s.SearchService.Search(r.Context(), query, limit)
	if err != nil {
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=30")
	json.NewEncoder(w).Encode(results)
}
