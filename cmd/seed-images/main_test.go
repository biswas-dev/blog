package main

import (
	"testing"
)

func TestCollectPostImages(t *testing.T) {
	t.Run("extracts images from markdown content", func(t *testing.T) {
		content := `Here is an image: ![alt text](https://res.cloudinary.com/demo/image/upload/sample.jpg)`
		imgs := collectPostImages(content, nil)
		if len(imgs) == 0 {
			t.Fatal("expected at least one image")
		}
		if imgs[0].url != "https://res.cloudinary.com/demo/image/upload/sample.jpg" {
			t.Errorf("unexpected url: %s", imgs[0].url)
		}
	})

	t.Run("includes featured image when cloudinary", func(t *testing.T) {
		featured := "https://res.cloudinary.com/demo/image/upload/featured.jpg"
		imgs := collectPostImages("no images here", &featured)
		if len(imgs) != 1 {
			t.Fatalf("expected 1 image, got %d", len(imgs))
		}
		if imgs[0].alt != "Featured image" {
			t.Errorf("unexpected alt: %s", imgs[0].alt)
		}
	})

	t.Run("skips non-cloudinary featured image", func(t *testing.T) {
		featured := "/static/uploads/local.jpg"
		imgs := collectPostImages("no images here", &featured)
		if len(imgs) != 0 {
			t.Errorf("expected 0 images for non-cloudinary featured, got %d", len(imgs))
		}
	})

	t.Run("nil featured pointer", func(t *testing.T) {
		imgs := collectPostImages("no images here", nil)
		if len(imgs) != 0 {
			t.Errorf("expected 0 images, got %d", len(imgs))
		}
	})

	t.Run("empty featured string", func(t *testing.T) {
		empty := ""
		imgs := collectPostImages("no images here", &empty)
		if len(imgs) != 0 {
			t.Errorf("expected 0 images, got %d", len(imgs))
		}
	})
}

func TestDeduplicateImages(t *testing.T) {
	t.Run("removes duplicates", func(t *testing.T) {
		imgs := []imageMatch{
			{url: "https://a.com/1.jpg", alt: "first"},
			{url: "https://a.com/1.jpg", alt: "duplicate"},
			{url: "https://a.com/2.jpg", alt: "second"},
		}
		seen := map[string]bool{}
		result := deduplicateImages(imgs, seen)
		if len(result) != 2 {
			t.Fatalf("expected 2 unique images, got %d", len(result))
		}
		if result[0].alt != "first" {
			t.Errorf("expected first occurrence kept, got alt=%s", result[0].alt)
		}
	})

	t.Run("respects existing seen map", func(t *testing.T) {
		imgs := []imageMatch{
			{url: "https://a.com/1.jpg", alt: "first"},
			{url: "https://a.com/2.jpg", alt: "second"},
		}
		seen := map[string]bool{"https://a.com/1.jpg": true}
		result := deduplicateImages(imgs, seen)
		if len(result) != 1 {
			t.Fatalf("expected 1 image, got %d", len(result))
		}
		if result[0].url != "https://a.com/2.jpg" {
			t.Errorf("expected second image, got %s", result[0].url)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		seen := map[string]bool{}
		result := deduplicateImages(nil, seen)
		if len(result) != 0 {
			t.Errorf("expected 0 images, got %d", len(result))
		}
	})
}

func TestExtractCloudinaryImages(t *testing.T) {
	t.Run("extracts markdown images", func(t *testing.T) {
		content := `![photo](https://res.cloudinary.com/demo/image/upload/photo.jpg)`
		results := extractCloudinaryImages(content)
		if len(results) != 1 {
			t.Fatalf("expected 1 image, got %d", len(results))
		}
		if results[0].alt != "photo" {
			t.Errorf("expected alt 'photo', got %s", results[0].alt)
		}
	})

	t.Run("extracts HTML img tags", func(t *testing.T) {
		content := `<img src="https://res.cloudinary.com/demo/image/upload/test.jpg" alt="test">`
		results := extractCloudinaryImages(content)
		if len(results) != 1 {
			t.Fatalf("expected 1 image, got %d", len(results))
		}
	})

	t.Run("ignores non-cloudinary URLs", func(t *testing.T) {
		content := `![photo](https://example.com/photo.jpg)`
		results := extractCloudinaryImages(content)
		if len(results) != 0 {
			t.Errorf("expected 0 images, got %d", len(results))
		}
	})

	t.Run("no images in content", func(t *testing.T) {
		results := extractCloudinaryImages("just plain text")
		if len(results) != 0 {
			t.Errorf("expected 0 images, got %d", len(results))
		}
	})
}

func TestIsCloudinaryURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://res.cloudinary.com/demo/image/upload/test.jpg", true},
		{"https://example.com/test.jpg", false},
		{"/static/uploads/test.jpg", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isCloudinaryURL(tt.url); got != tt.want {
			t.Errorf("isCloudinaryURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}
