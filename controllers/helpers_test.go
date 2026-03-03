package controllers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestStripMarkdownLinks(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no links", "plain text", "plain text"},
		{"single link", "See [Google](https://google.com) now", "See Google now"},
		{"multiple links", "[A](url1) and [B](url2)", "A and B"},
		{"nested brackets", "[[text]](url)", "[text]"},
		{"no closing paren", "[text](url", "[text](url"},
		{"empty text", "[](url)", ""},
		{"link at start", "[Start](url) rest", "Start rest"},
		{"link at end", "text [End](url)", "text End"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdownLinks(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdownLinks(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestOgExcerpt(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "short text",
			input:  "Hello world",
			maxLen: 200,
			want:   "Hello world",
		},
		{
			name:   "skips headings",
			input:  "# Heading\nActual content here",
			maxLen: 200,
			want:   "Actual content here",
		},
		{
			name:   "skips images",
			input:  "![alt](url)\nText after image",
			maxLen: 200,
			want:   "Text after image",
		},
		{
			name:   "skips code fences",
			input:  "```\ncode\n```\nAfter code",
			maxLen: 200,
			want:   "code After code",
		},
		{
			name:   "skips horizontal rules",
			input:  "---\nAfter rule",
			maxLen: 200,
			want:   "After rule",
		},
		{
			name:   "skips more tags",
			input:  "<more-->\nAfter more",
			maxLen: 200,
			want:   "After more",
		},
		{
			name:   "strips bold markdown",
			input:  "This is **bold** text",
			maxLen: 200,
			want:   "This is bold text",
		},
		{
			name:   "strips inline code",
			input:  "Use `fmt.Println` here",
			maxLen: 200,
			want:   "Use fmt.Println here",
		},
		{
			name:   "truncates long text",
			input:  "This is a very long sentence that goes on and on and should be truncated at some point",
			maxLen: 30,
			want:   "This is a very long sentence...",
		},
		{
			name:   "strips markdown links",
			input:  "Visit [my site](https://example.com) today",
			maxLen: 200,
			want:   "Visit my site today",
		},
		{
			name:   "skips tables",
			input:  "| Header |\nContent after",
			maxLen: 200,
			want:   "Content after",
		},
		{
			name:   "empty content",
			input:  "",
			maxLen: 200,
			want:   "",
		},
		{
			name:   "only headings",
			input:  "# Heading\n## Sub",
			maxLen: 200,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ogExcerpt(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("ogExcerpt(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestExtensionForContent(t *testing.T) {
	tests := []struct {
		name     string
		filetype string
		filename string
		wantExt  string
		wantOK   bool
	}{
		{"jpeg content type", "image/jpeg", "photo.jpg", ".jpg", true},
		{"png content type", "image/png", "image.png", ".png", true},
		{"gif content type", "image/gif", "anim.gif", ".gif", true},
		{"webp content type", "image/webp", "photo.webp", ".webp", true},
		{"svg content type", "image/svg+xml", "icon.svg", ".svg", true},
		{"unknown type falls back to ext", "application/octet-stream", "photo.jpg", ".jpg", true},
		{"unknown type with jpeg ext", "application/octet-stream", "photo.jpeg", ".jpg", true},
		{"unknown type with png ext", "application/octet-stream", "photo.png", ".png", true},
		{"unknown type with gif ext", "application/octet-stream", "photo.gif", ".gif", true},
		{"unknown type with webp ext", "application/octet-stream", "photo.webp", ".webp", true},
		{"unknown type with svg ext", "application/octet-stream", "photo.svg", ".svg", true},
		{"unsupported type and ext", "text/plain", "file.txt", "", false},
		{"unsupported type no ext", "text/plain", "file", "", false},
		{"uppercase ext fallback", "application/octet-stream", "PHOTO.JPG", ".jpg", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext, ok := extensionForContent(tt.filetype, tt.filename)
			if ok != tt.wantOK {
				t.Errorf("extensionForContent(%q, %q) ok = %v, want %v", tt.filetype, tt.filename, ok, tt.wantOK)
			}
			if ext != tt.wantExt {
				t.Errorf("extensionForContent(%q, %q) ext = %q, want %q", tt.filetype, tt.filename, ext, tt.wantExt)
			}
		})
	}
}

func TestRequireAdminOrRedirect(t *testing.T) {
	t.Run("ajax unauthorized returns 401", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/admin", nil)
		user, ok := requireAdminOrRedirect(w, r, nil, true)
		if ok {
			t.Error("expected ok=false for nil session service")
		}
		if user != nil {
			t.Error("expected nil user")
		}
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("non-ajax unauthorized redirects to signin", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/admin", nil)
		user, ok := requireAdminOrRedirect(w, r, nil, false)
		if ok {
			t.Error("expected ok=false")
		}
		if user != nil {
			t.Error("expected nil user")
		}
		if w.Code != http.StatusFound {
			t.Errorf("expected 302, got %d", w.Code)
		}
		if loc := w.Header().Get("Location"); loc != "/signin" {
			t.Errorf("expected redirect to /signin, got %q", loc)
		}
	})
}

func TestSaveUploadedFile(t *testing.T) {
	// Create a temp directory to act as CWD
	tmpDir := t.TempDir()
	// Save current dir and change to temp
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create a minimal JPEG-like file (starts with FF D8 for JPEG detection)
	jpegHeader := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	jpegHeader = append(jpegHeader, make([]byte, 512)...)

	t.Run("saves featured image with slug", func(t *testing.T) {
		file := bytes.NewReader(jpegHeader)
		url, err := saveUploadedFile(file, "photo.jpg", "featured", "my-post")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(url, "/static/uploads/featured/my-post/") {
			t.Errorf("url = %q, want prefix '/static/uploads/featured/my-post/'", url)
		}
		if !strings.HasSuffix(url, ".jpg") {
			t.Errorf("url = %q, want .jpg suffix", url)
		}
	})

	t.Run("saves post image with slug", func(t *testing.T) {
		file := bytes.NewReader(jpegHeader)
		url, err := saveUploadedFile(file, "photo.jpg", "post", "my-post")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(url, "/static/uploads/post/my-post/") {
			t.Errorf("url = %q, want prefix '/static/uploads/post/my-post/'", url)
		}
	})

	t.Run("saves without slug", func(t *testing.T) {
		file := bytes.NewReader(jpegHeader)
		url, err := saveUploadedFile(file, "photo.jpg", "inline", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(url, "/static/uploads/") {
			t.Errorf("url = %q, want prefix '/static/uploads/'", url)
		}
	})

	t.Run("rejects unsupported file type", func(t *testing.T) {
		textContent := bytes.NewReader([]byte("this is plain text, not an image"))
		_, err := saveUploadedFile(textContent, "file.txt", "post", "slug")
		if err == nil {
			t.Fatal("expected error for unsupported file type")
		}
	})
}
