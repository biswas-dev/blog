package models

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBuildRequest(t *testing.T) {
	t.Run("basic GET without auth", func(t *testing.T) {
		system := &ExternalSystem{}
		req, err := buildRequest("GET", "https://example.com/api/posts/", nil, system)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Method != "GET" {
			t.Errorf("method = %q, want GET", req.Method)
		}
		if req.URL.String() != "https://example.com/api/posts/" {
			t.Errorf("url = %q", req.URL.String())
		}
		if req.Header.Get("Authorization") != "" {
			t.Error("expected no Authorization header")
		}
		if req.Header.Get("Content-Type") != "" {
			t.Error("expected no Content-Type header for GET")
		}
	})

	t.Run("GET with API key", func(t *testing.T) {
		system := &ExternalSystem{APIKey: "test-token-123"}
		req, err := buildRequest("GET", "https://example.com/api/posts/", nil, system)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := req.Header.Get("Authorization"); got != "Bearer test-token-123" {
			t.Errorf("Authorization = %q, want 'Bearer test-token-123'", got)
		}
	})

	t.Run("POST with body sets Content-Type", func(t *testing.T) {
		system := &ExternalSystem{}
		body := bytes.NewReader([]byte(`{"title":"test"}`))
		req, err := buildRequest("POST", "https://example.com/api/posts/", body, system)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Method != "POST" {
			t.Errorf("method = %q, want POST", req.Method)
		}
		if got := req.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
	})

	t.Run("custom headers applied", func(t *testing.T) {
		system := &ExternalSystem{
			CustomHeaders: []CustomHeader{
				{Key: "X-Custom-Header", Value: "custom-value"},
				{Key: "X-Another:", Value: " spaced "},
			},
		}
		req, err := buildRequest("GET", "https://example.com/", nil, system)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := req.Header.Get("X-Custom-Header"); got != "custom-value" {
			t.Errorf("X-Custom-Header = %q, want 'custom-value'", got)
		}
		if got := req.Header.Get("X-Another"); got != "spaced" {
			t.Errorf("X-Another = %q, want 'spaced'", got)
		}
	})

	t.Run("empty custom header key is skipped", func(t *testing.T) {
		system := &ExternalSystem{
			CustomHeaders: []CustomHeader{
				{Key: "", Value: "should-be-ignored"},
			},
		}
		req, err := buildRequest("GET", "https://example.com/", nil, system)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// No extra headers beyond defaults
		if req.Header.Get("") != "" {
			t.Error("empty key header should be skipped")
		}
	})

	t.Run("all features combined", func(t *testing.T) {
		system := &ExternalSystem{
			APIKey: "my-key",
			CustomHeaders: []CustomHeader{
				{Key: "X-Source", Value: "test"},
			},
		}
		body := bytes.NewReader([]byte(`{"data":"test"}`))
		req, err := buildRequest("POST", "https://example.com/api", body, system)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Header.Get("Authorization") != "Bearer my-key" {
			t.Error("Authorization missing or wrong")
		}
		if req.Header.Get("Content-Type") != "application/json" {
			t.Error("Content-Type missing for POST with body")
		}
		if req.Header.Get("X-Source") != "test" {
			t.Error("Custom header X-Source missing")
		}
	})
}

func TestHttpClient(t *testing.T) {
	client := httpClient(5 * time.Second)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Timeout != 5*time.Second {
		t.Errorf("timeout = %v, want 5s", client.Timeout)
	}
}

func TestMarkdownImageRegex(t *testing.T) {
	tests := []struct {
		name  string
		input string
		count int
	}{
		{"matches static upload", "![alt](/static/uploads/post/slug/img.jpg)", 1},
		{"no match for external URL", "![alt](https://example.com/img.jpg)", 0},
		{"multiple images", "![a](/static/uploads/a.jpg) ![b](/static/uploads/b.png)", 2},
		{"no images", "just text", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := markdownImageRe.FindAllStringSubmatch(tt.input, -1)
			if len(matches) != tt.count {
				t.Errorf("got %d matches, want %d", len(matches), tt.count)
			}
		})
	}
}

func TestHtmlImageRegex(t *testing.T) {
	tests := []struct {
		name  string
		input string
		count int
	}{
		{"matches img with static src", `<img src="/static/uploads/post/slug/img.jpg" alt="test">`, 1},
		{"no match for external", `<img src="https://example.com/img.jpg">`, 0},
		{"no images", "just text", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := htmlImageRe.FindAllStringSubmatch(tt.input, -1)
			if len(matches) != tt.count {
				t.Errorf("got %d matches, want %d", len(matches), tt.count)
			}
		})
	}
}

func TestPushOnePost(t *testing.T) {
	t.Run("successful push returns true", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			if len(body) == 0 {
				t.Error("expected non-empty body")
			}
			w.WriteHeader(http.StatusCreated)
		}))
		defer srv.Close()

		post := Post{
			UserID:     1,
			CategoryID: 1,
			Title:      "Test Post",
			Content:    "content",
			Slug:       "test-post",
		}
		system := &ExternalSystem{}
		ok, errMsg := pushOnePost(post, srv.URL, srv.Client(), system)
		if !ok {
			t.Errorf("expected ok=true, got errMsg=%q", errMsg)
		}
		if errMsg != "" {
			t.Errorf("expected empty errMsg, got %q", errMsg)
		}
	})

	t.Run("200 OK also counts as success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		post := Post{Title: "Test", Slug: "test"}
		system := &ExternalSystem{}
		ok, _ := pushOnePost(post, srv.URL, srv.Client(), system)
		if !ok {
			t.Error("expected ok=true for 200 OK")
		}
	})

	t.Run("server error returns false", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		post := Post{Title: "Test", Slug: "test"}
		system := &ExternalSystem{}
		ok, errMsg := pushOnePost(post, srv.URL, srv.Client(), system)
		if ok {
			t.Error("expected ok=false for server error")
		}
		if errMsg == "" {
			t.Error("expected error message")
		}
	})

	t.Run("sends auth header when API key set", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Authorization"); got != "Bearer secret-key" {
				t.Errorf("Authorization = %q, want 'Bearer secret-key'", got)
			}
			w.WriteHeader(http.StatusCreated)
		}))
		defer srv.Close()

		post := Post{Title: "Test", Slug: "test"}
		system := &ExternalSystem{APIKey: "secret-key"}
		ok, _ := pushOnePost(post, srv.URL, srv.Client(), system)
		if !ok {
			t.Error("expected ok=true")
		}
	})
}

func TestRemotePostStruct(t *testing.T) {
	// Verify the struct can be used
	rp := remotePost{
		ID:               1,
		UserID:           1,
		CategoryID:       2,
		Title:            "Test",
		Content:          "content",
		Slug:             "test-slug",
		PublicationDate:  "2024-01-01",
		LastEditDate:     "",
		IsPublished:      true,
		Featured:         false,
		FeaturedImageURL: "/static/uploads/featured/test/img.jpg",
	}
	if rp.Slug != "test-slug" {
		t.Errorf("unexpected slug: %s", rp.Slug)
	}
}

func TestSyncResultStruct(t *testing.T) {
	result := &SyncResult{Direction: "pull"}
	result.ItemsSynced = 5
	result.ItemsSkipped = 3
	result.ItemsFailed = 1

	if result.Direction != "pull" {
		t.Errorf("direction = %q", result.Direction)
	}
	if result.ItemsSynced != 5 {
		t.Errorf("items synced = %d", result.ItemsSynced)
	}
}

func TestSyncPreviewStruct(t *testing.T) {
	preview := &SyncPreview{Direction: "push"}
	preview.Items = append(preview.Items, SyncItem{Title: "Post", Slug: "post", Status: "new"})
	preview.NewCount = 1

	if len(preview.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(preview.Items))
	}
	if preview.Items[0].Status != "new" {
		t.Errorf("status = %q", preview.Items[0].Status)
	}
}
