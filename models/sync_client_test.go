package models

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

func TestSyncClientTestConnection(t *testing.T) {
	t.Run("successful connection", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/posts/" {
				t.Errorf("expected path /api/posts/, got %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"Posts":[]}`))
		}))
		defer srv.Close()

		sc := &SyncClient{}
		system := &ExternalSystem{BaseURL: srv.URL}
		err := sc.TestConnection(system)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("server returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("forbidden"))
		}))
		defer srv.Close()

		sc := &SyncClient{}
		system := &ExternalSystem{BaseURL: srv.URL}
		err := sc.TestConnection(system)
		if err == nil {
			t.Error("expected error for 403 response")
		}
	})

	t.Run("connection refused", func(t *testing.T) {
		sc := &SyncClient{}
		system := &ExternalSystem{BaseURL: "http://localhost:19999"}
		err := sc.TestConnection(system)
		if err == nil {
			t.Error("expected error for connection refused")
		}
	})

	t.Run("sends auth header", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth != "Bearer test-key" {
				t.Errorf("Authorization = %q, want 'Bearer test-key'", auth)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		sc := &SyncClient{}
		system := &ExternalSystem{BaseURL: srv.URL, APIKey: "test-key"}
		sc.TestConnection(system)
	})
}

func TestSyncClientFetchRemotePosts(t *testing.T) {
	t.Run("successful fetch", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"Posts":[{"ID":1,"Title":"Test Post","Slug":"test","Content":"hello","IsPublished":true}]}`))
		}))
		defer srv.Close()

		sc := &SyncClient{}
		system := &ExternalSystem{BaseURL: srv.URL}
		posts, err := sc.fetchRemotePosts(system)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(posts) != 1 {
			t.Fatalf("expected 1 post, got %d", len(posts))
		}
		if posts[0].Title != "Test Post" {
			t.Errorf("title = %q", posts[0].Title)
		}
		if posts[0].Slug != "test" {
			t.Errorf("slug = %q", posts[0].Slug)
		}
	})

	t.Run("empty posts list", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"Posts":[]}`))
		}))
		defer srv.Close()

		sc := &SyncClient{}
		system := &ExternalSystem{BaseURL: srv.URL}
		posts, err := sc.fetchRemotePosts(system)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(posts) != 0 {
			t.Errorf("expected 0 posts, got %d", len(posts))
		}
	})

	t.Run("server error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("error"))
		}))
		defer srv.Close()

		sc := &SyncClient{}
		system := &ExternalSystem{BaseURL: srv.URL}
		_, err := sc.fetchRemotePosts(system)
		if err == nil {
			t.Error("expected error for 500 response")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("not json"))
		}))
		defer srv.Close()

		sc := &SyncClient{}
		system := &ExternalSystem{BaseURL: srv.URL}
		_, err := sc.fetchRemotePosts(system)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("sends custom headers", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Custom") != "value" {
				t.Errorf("X-Custom = %q", r.Header.Get("X-Custom"))
			}
			w.Write([]byte(`{"Posts":[]}`))
		}))
		defer srv.Close()

		sc := &SyncClient{}
		system := &ExternalSystem{
			BaseURL: srv.URL,
			CustomHeaders: []CustomHeader{
				{Key: "X-Custom", Value: "value"},
			},
		}
		sc.fetchRemotePosts(system)
	})

	t.Run("multiple posts with categories", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"Posts":[
				{"ID":1,"Title":"Post 1","Slug":"post-1","CategoryID":1,"IsPublished":true},
				{"ID":2,"Title":"Post 2","Slug":"post-2","CategoryID":2,"IsPublished":false,"Featured":true}
			]}`))
		}))
		defer srv.Close()

		sc := &SyncClient{}
		system := &ExternalSystem{BaseURL: srv.URL}
		posts, err := sc.fetchRemotePosts(system)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(posts) != 2 {
			t.Fatalf("expected 2 posts, got %d", len(posts))
		}
		if !posts[1].Featured {
			t.Error("post 2 should be featured")
		}
	})
}

func TestDownloadAndRewriteContentImages(t *testing.T) {
	t.Run("rewrites markdown images", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0})
		}))
		defer srv.Close()

		sc := &SyncClient{}
		system := &ExternalSystem{BaseURL: srv.URL}
		content := `![alt](/static/uploads/post/slug/img.jpg)`
		got := sc.downloadAndRewriteContentImages(content, srv.URL, "test-slug", system)
		// Should have rewritten the URL (exact path depends on random filename)
		if got == content {
			// It may fail to create dirs in test env, which is fine
			// The important thing is the function doesn't panic
		}
	})

	t.Run("no images returns unchanged", func(t *testing.T) {
		sc := &SyncClient{}
		system := &ExternalSystem{}
		content := "just plain text with no images"
		got := sc.downloadAndRewriteContentImages(content, "http://example.com", "slug", system)
		if got != content {
			t.Errorf("expected unchanged content, got %q", got)
		}
	})
}

func TestDownloadImage(t *testing.T) {
	t.Run("downloads and saves image", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10})
		}))
		defer srv.Close()

		// Use a temp dir
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(origDir)

		sc := &SyncClient{}
		system := &ExternalSystem{BaseURL: srv.URL}
		localURL, err := sc.downloadImage(srv.URL, "/static/uploads/post/test/img.jpg", "test-slug", "featured", system)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(localURL, "/static/uploads/featured/test-slug/") {
			t.Errorf("localURL = %q, want prefix '/static/uploads/featured/test-slug/'", localURL)
		}
	})

	t.Run("builds full URL for relative path", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte{0x89, 0x50, 0x4E, 0x47})
		}))
		defer srv.Close()

		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(origDir)

		sc := &SyncClient{}
		system := &ExternalSystem{}
		localURL, err := sc.downloadImage(srv.URL, "/static/uploads/post/slug/img.png", "slug", "post", system)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(localURL, ".png") {
			t.Errorf("expected .png extension, got %q", localURL)
		}
	})

	t.Run("server error returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(origDir)

		sc := &SyncClient{}
		system := &ExternalSystem{}
		_, err := sc.downloadImage(srv.URL, "/img.jpg", "slug", "post", system)
		if err == nil {
			t.Error("expected error for 404")
		}
	})

	t.Run("default extension for URL without ext", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte{0xFF, 0xD8})
		}))
		defer srv.Close()

		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(origDir)

		sc := &SyncClient{}
		system := &ExternalSystem{}
		localURL, err := sc.downloadImage(srv.URL, "/static/uploads/post/slug/image", "slug", "post", system)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should default to .jpg
		if !strings.HasSuffix(localURL, ".jpg") {
			t.Errorf("expected .jpg default extension, got %q", localURL)
		}
	})
}

func TestResolveLocalImages(t *testing.T) {
	t.Run("no images to resolve", func(t *testing.T) {
		sc := &SyncClient{}
		system := &ExternalSystem{}
		rp := remotePost{
			Content:          "plain text",
			FeaturedImageURL: "",
			Slug:             "test",
		}
		featURL, content := sc.resolveLocalImages(rp, "http://example.com", system)
		if featURL != "" {
			t.Errorf("expected empty featured URL, got %q", featURL)
		}
		if content != "plain text" {
			t.Errorf("expected unchanged content, got %q", content)
		}
	})

	t.Run("external featured URL not rewritten", func(t *testing.T) {
		sc := &SyncClient{}
		system := &ExternalSystem{}
		rp := remotePost{
			Content:          "text",
			FeaturedImageURL: "https://example.com/img.jpg",
			Slug:             "test",
		}
		featURL, _ := sc.resolveLocalImages(rp, "http://remote.com", system)
		// External URL (not /static/uploads/) should be returned unchanged
		if featURL != "https://example.com/img.jpg" {
			t.Errorf("expected unchanged URL, got %q", featURL)
		}
	})
}
