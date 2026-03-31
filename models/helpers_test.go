package models

import (
	"strings"
	"testing"
	"time"
)

func TestFormatPostDates(t *testing.T) {
	tests := []struct {
		name            string
		createdAt       string
		pubDate         string
		lastEdit        string
		wantCreatedAt   string
		wantPubDate     string
		wantLastEdit    string
	}{
		{
			name:          "formats all dates",
			createdAt:     "2024-06-15T10:30:00Z",
			pubDate:       "",
			lastEdit:      "2024-07-01T14:00:00Z",
			wantCreatedAt: "2024-06-15T10:30:00Z",
			wantPubDate:   "June 15, 2024",
			wantLastEdit:  "July 1, 2024",
		},
		{
			name:          "keeps RFC3339 for CreatedAt",
			createdAt:     "2024-01-01T00:00:00Z",
			pubDate:       "",
			lastEdit:      "",
			wantCreatedAt: "2024-01-01T00:00:00Z",
			wantPubDate:   "January 1, 2024",
			wantLastEdit:  "",
		},
		{
			name:          "non-RFC3339 dates left as-is",
			createdAt:     "not a date",
			pubDate:       "also not",
			lastEdit:      "nope",
			wantCreatedAt: "not a date",
			wantPubDate:   "also not",
			wantLastEdit:  "nope",
		},
		{
			name:          "explicit publication date overridden by createdAt",
			createdAt:     "2024-03-01T12:00:00Z",
			pubDate:       "2024-04-01T12:00:00Z",
			lastEdit:      "",
			wantCreatedAt: "2024-03-01T12:00:00Z",
			wantPubDate:   "March 1, 2024",
			wantLastEdit:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			post := &Post{
				CreatedAt:       tt.createdAt,
				PublicationDate: tt.pubDate,
				LastEditDate:    tt.lastEdit,
			}
			formatPostDates(post)
			if post.CreatedAt != tt.wantCreatedAt {
				t.Errorf("CreatedAt = %q, want %q", post.CreatedAt, tt.wantCreatedAt)
			}
			if post.PublicationDate != tt.wantPubDate {
				t.Errorf("PublicationDate = %q, want %q", post.PublicationDate, tt.wantPubDate)
			}
			if post.LastEditDate != tt.wantLastEdit {
				t.Errorf("LastEditDate = %q, want %q", post.LastEditDate, tt.wantLastEdit)
			}
		})
	}
}

func TestStripHTMLLocal(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"no tags", "hello", "hello"},
		{"simple tag", "<b>bold</b>", "bold"},
		{"nested tags", "<div><p>text</p></div>", "text"},
		{"self-closing", "a<br/>b", "ab"},
		{"attributes", `<a href="url">link</a>`, "link"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripHTML(tt.input)
			if got != tt.want {
				t.Errorf("stripHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFindContentBeforeMoreTag(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no more tag", "Hello world", ""},
		{"standard more tag", "Before<more-->After", "Before"},
		{"spaced more tag", "Before<more -->After", "Before"},
		{"html entity more", "Before&lt;more--&gt;After", "Before"},
		{"html entity spaced", "Before&lt;more --&gt;After", "Before"},
		{"trims whitespace", "  Before  <more-->After", "Before"},
		{"empty before more", "<more-->After", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findContentBeforeMoreTag(tt.input)
			if got != tt.want {
				t.Errorf("findContentBeforeMoreTag(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSetFirstError(t *testing.T) {
	t.Run("sets on empty", func(t *testing.T) {
		r := &SyncResult{}
		setFirstError(r, "first error")
		if r.ErrorMessage != "first error" {
			t.Errorf("expected 'first error', got %q", r.ErrorMessage)
		}
	})

	t.Run("does not overwrite", func(t *testing.T) {
		r := &SyncResult{ErrorMessage: "existing"}
		setFirstError(r, "second error")
		if r.ErrorMessage != "existing" {
			t.Errorf("expected 'existing', got %q", r.ErrorMessage)
		}
	})

	t.Run("empty message on empty result", func(t *testing.T) {
		r := &SyncResult{}
		setFirstError(r, "")
		if r.ErrorMessage != "" {
			t.Errorf("expected empty, got %q", r.ErrorMessage)
		}
	})
}

func TestFriendlyDateFormat(t *testing.T) {
	// Verify the constant matches what time.Format produces
	d := time.Date(2024, time.March, 15, 0, 0, 0, 0, time.UTC)
	got := d.Format(friendlyDateFormat)
	if got != "March 15, 2024" {
		t.Errorf("friendlyDateFormat produced %q, want 'March 15, 2024'", got)
	}
}

func TestPreviewContentRawLocal(t *testing.T) {
	tests := []struct {
		name    string
		content string
		check   func(string) bool
		desc    string
	}{
		{
			name:    "uses more tag when present",
			content: "Before content<more-->After content",
			check:   func(s string) bool { return s == "Before content" },
			desc:    "should return content before more tag",
		},
		{
			name:    "short content returned as-is",
			content: "Short text",
			check:   func(s string) bool { return s == "Short text" },
			desc:    "short content should be returned trimmed",
		},
		{
			name:    "empty content",
			content: "",
			check:   func(s string) bool { return s == "" },
			desc:    "empty should return empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := previewContentRaw(tt.content)
			if !tt.check(got) {
				t.Errorf("previewContentRaw(%q) = %q, %s", tt.content, got, tt.desc)
			}
		})
	}
}

func TestTruncateFirstLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		maxChars int
		wantEnd  string
	}{
		{
			name:     "truncates long line",
			line:     "This is a very long first line that exceeds the maximum character limit for a preview",
			maxChars: 30,
			wantEnd:  "...",
		},
		{
			name:     "single long word",
			line:     "superlongwordwithnobreaks and more",
			maxChars: 10,
			wantEnd:  "...",
		},
		{
			name:     "empty line",
			line:     "",
			maxChars: 100,
			wantEnd:  "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateFirstLine(tt.line, tt.maxChars)
			if !strings.HasSuffix(got, tt.wantEnd) {
				t.Errorf("truncateFirstLine(%q, %d) = %q, want suffix %q", tt.line, tt.maxChars, got, tt.wantEnd)
			}
		})
	}
}

func TestBuildPreviewFromLines(t *testing.T) {
	t.Run("multiple short lines", func(t *testing.T) {
		content := "Line one\n\nLine two\n\nLine three"
		got := buildPreviewFromLines(content, 200)
		if !strings.Contains(got, "Line one") {
			t.Errorf("expected 'Line one' in output, got %q", got)
		}
	})

	t.Run("stops at maxChars", func(t *testing.T) {
		content := "Short\n\nThis is a medium length line\n\nAnother line here"
		got := buildPreviewFromLines(content, 20)
		// Should stop before including all lines
		if strings.Contains(got, "Another line") {
			t.Errorf("should have stopped before 'Another line', got %q", got)
		}
	})

	t.Run("skips empty lines", func(t *testing.T) {
		content := "\n\n\nActual content"
		got := buildPreviewFromLines(content, 200)
		if strings.TrimSpace(got) != "Actual content" {
			t.Errorf("expected 'Actual content', got %q", got)
		}
	})

	t.Run("long first line gets truncated", func(t *testing.T) {
		content := "This is a very long first line that exceeds the maximum character limit by quite a bit and should be truncated at a word boundary"
		got := buildPreviewFromLines(content, 30)
		if !strings.HasSuffix(got, "...") {
			t.Errorf("expected '...' suffix for truncation, got %q", got)
		}
	})
}

func TestExtractPreviewByLength(t *testing.T) {
	t.Run("short content returns full content", func(t *testing.T) {
		got := extractPreviewByLength("Hello world")
		if got != "Hello world" {
			t.Errorf("expected 'Hello world', got %q", got)
		}
	})

	t.Run("long content gets truncated", func(t *testing.T) {
		long := strings.Repeat("word ", 100)
		got := extractPreviewByLength(long)
		if len(got) > 200 {
			t.Errorf("expected truncated output, got len=%d", len(got))
		}
	})
}

func TestRenderContentLocal(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check string
	}{
		{"bold text", "**bold**", "<strong>bold</strong>"},
		{"italic text", "*italic*", "<em>italic</em>"},
		{"heading", "# Title", "<h1"},
		{"link", "[text](http://example.com)", `href="http://example.com"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderContent(tt.input)
			if !strings.Contains(got, tt.check) {
				t.Errorf("RenderContent(%q) = %q, missing %q", tt.input, got, tt.check)
			}
		})
	}
}
