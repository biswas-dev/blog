package models

import (
	"testing"
)

func cleanupImageMetadata(t *testing.T, svc *ImageMetadataService, url string) {
	t.Helper()
	svc.Delete(url)
}

func TestImageMetadataService_Upsert(t *testing.T) {
	db := SetupTestDB(t)
	svc := &ImageMetadataService{DB: db}

	t.Run("insert new metadata", func(t *testing.T) {
		url := "https://example.com/test-upsert-insert.jpg"
		t.Cleanup(func() { cleanupImageMetadata(t, svc, url) })

		meta, err := svc.Upsert(url, "alt text", "title text", "caption text")
		if err != nil {
			t.Fatalf("Upsert() error = %v", err)
		}

		if meta.ID == 0 {
			t.Error("Expected non-zero ID")
		}
		if meta.ImageURL != url {
			t.Errorf("Expected URL %q, got %q", url, meta.ImageURL)
		}
		if meta.AltText != "alt text" {
			t.Errorf("Expected alt 'alt text', got %q", meta.AltText)
		}
		if meta.Title != "title text" {
			t.Errorf("Expected title 'title text', got %q", meta.Title)
		}
		if meta.Caption != "caption text" {
			t.Errorf("Expected caption 'caption text', got %q", meta.Caption)
		}
		if meta.CreatedAt.IsZero() {
			t.Error("Expected non-zero created_at")
		}
	})

	t.Run("update existing metadata", func(t *testing.T) {
		url := "https://example.com/test-upsert-update.jpg"
		t.Cleanup(func() { cleanupImageMetadata(t, svc, url) })

		// Insert first
		_, err := svc.Upsert(url, "old alt", "old title", "old caption")
		if err != nil {
			t.Fatalf("Initial Upsert() error = %v", err)
		}

		// Update
		meta, err := svc.Upsert(url, "new alt", "new title", "new caption")
		if err != nil {
			t.Fatalf("Update Upsert() error = %v", err)
		}

		if meta.AltText != "new alt" {
			t.Errorf("Expected alt 'new alt', got %q", meta.AltText)
		}
		if meta.Title != "new title" {
			t.Errorf("Expected title 'new title', got %q", meta.Title)
		}
		if meta.Caption != "new caption" {
			t.Errorf("Expected caption 'new caption', got %q", meta.Caption)
		}
	})

	t.Run("upsert with empty fields", func(t *testing.T) {
		url := "https://example.com/test-upsert-empty.jpg"
		t.Cleanup(func() { cleanupImageMetadata(t, svc, url) })

		meta, err := svc.Upsert(url, "", "", "")
		if err != nil {
			t.Fatalf("Upsert() error = %v", err)
		}

		if meta.AltText != "" {
			t.Errorf("Expected empty alt, got %q", meta.AltText)
		}
	})
}

func TestImageMetadataService_GetByURL(t *testing.T) {
	db := SetupTestDB(t)
	svc := &ImageMetadataService{DB: db}

	t.Run("get existing metadata", func(t *testing.T) {
		url := "https://example.com/test-getbyurl.jpg"
		t.Cleanup(func() { cleanupImageMetadata(t, svc, url) })

		_, err := svc.Upsert(url, "my alt", "my title", "my caption")
		if err != nil {
			t.Fatalf("Upsert() error = %v", err)
		}

		meta, err := svc.GetByURL(url)
		if err != nil {
			t.Fatalf("GetByURL() error = %v", err)
		}
		if meta == nil {
			t.Fatal("Expected non-nil metadata")
		}
		if meta.AltText != "my alt" {
			t.Errorf("Expected alt 'my alt', got %q", meta.AltText)
		}
	})

	t.Run("get non-existent metadata returns nil", func(t *testing.T) {
		meta, err := svc.GetByURL("https://example.com/nonexistent-999.jpg")
		if err != nil {
			t.Fatalf("GetByURL() error = %v", err)
		}
		if meta != nil {
			t.Errorf("Expected nil for non-existent URL, got %+v", meta)
		}
	})
}

func TestImageMetadataService_GetByURLs(t *testing.T) {
	db := SetupTestDB(t)
	svc := &ImageMetadataService{DB: db}

	url1 := "https://example.com/test-bulk-1.jpg"
	url2 := "https://example.com/test-bulk-2.jpg"
	url3 := "https://example.com/test-bulk-missing.jpg"
	t.Cleanup(func() {
		cleanupImageMetadata(t, svc, url1)
		cleanupImageMetadata(t, svc, url2)
	})

	svc.Upsert(url1, "alt1", "title1", "caption1")
	svc.Upsert(url2, "alt2", "title2", "caption2")

	t.Run("bulk lookup with mix of existing and missing", func(t *testing.T) {
		result, err := svc.GetByURLs([]string{url1, url2, url3})
		if err != nil {
			t.Fatalf("GetByURLs() error = %v", err)
		}

		if len(result) != 2 {
			t.Errorf("Expected 2 results, got %d", len(result))
		}
		if result[url1] == nil {
			t.Error("Expected metadata for url1")
		} else if result[url1].AltText != "alt1" {
			t.Errorf("Expected alt 'alt1', got %q", result[url1].AltText)
		}
		if result[url2] == nil {
			t.Error("Expected metadata for url2")
		}
		if result[url3] != nil {
			t.Error("Expected nil for missing url3")
		}
	})

	t.Run("bulk lookup with empty list", func(t *testing.T) {
		result, err := svc.GetByURLs([]string{})
		if err != nil {
			t.Fatalf("GetByURLs() error = %v", err)
		}
		if len(result) != 0 {
			t.Errorf("Expected 0 results, got %d", len(result))
		}
	})
}

func TestImageMetadataService_Delete(t *testing.T) {
	db := SetupTestDB(t)
	svc := &ImageMetadataService{DB: db}

	t.Run("delete existing metadata", func(t *testing.T) {
		url := "https://example.com/test-delete.jpg"

		_, err := svc.Upsert(url, "alt", "title", "caption")
		if err != nil {
			t.Fatalf("Upsert() error = %v", err)
		}

		err = svc.Delete(url)
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		meta, err := svc.GetByURL(url)
		if err != nil {
			t.Fatalf("GetByURL() after delete error = %v", err)
		}
		if meta != nil {
			t.Error("Expected nil after delete")
		}
	})

	t.Run("delete non-existent is no-op", func(t *testing.T) {
		err := svc.Delete("https://example.com/never-existed.jpg")
		if err != nil {
			t.Errorf("Delete() non-existent error = %v", err)
		}
	})
}

func TestPqStringArray(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		expect string
	}{
		{"empty", []string{}, "{}"},
		{"single", []string{"a"}, `{"a"}`},
		{"multiple", []string{"a", "b", "c"}, `{"a","b","c"}`},
		{"with quotes", []string{`a"b`}, `{"a\"b"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pqStringArray(tt.input)
			if got != tt.expect {
				t.Errorf("pqStringArray(%v) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}
