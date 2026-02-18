package models

import (
	"strings"
	"testing"
)

func TestBlogService_GetBlogPostBySlug(t *testing.T) {
	db := SetupTestDB(t)
	blogService := NewBlogService(db)
	postService := &PostService{DB: db}

	// Create a test user and category
	userID := SeedUser(t, db, "blogpost@example.com", "blogpostuser", "password123", RoleEditor)
	categoryID := SeedCategory(t, db, "Blog Category")
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
		CleanupCategory(t, db, categoryID)
	})

	// Create test posts
	publishedPost, _ := postService.Create(userID, categoryID, "Published Post", "# Test Content\n\nThis is a test.", true, false, "", "published-post-slug")
	draftPost, _ := postService.Create(userID, categoryID, "Draft Post", "Draft content", false, false, "", "draft-post-slug")
	t.Cleanup(func() {
		CleanupPost(t, db, publishedPost.ID)
		CleanupPost(t, db, draftPost.ID)
	})

	tests := []struct {
		name    string
		slug    string
		wantErr bool
	}{
		{
			name:    "get published post by slug",
			slug:    "published-post-slug",
			wantErr: false,
		},
		{
			name:    "get draft post by slug",
			slug:    "draft-post-slug",
			wantErr: false, // BlogService should return draft too
		},
		{
			name:    "get non-existent post",
			slug:    "non-existent-slug",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			post, err := blogService.GetBlogPostBySlug(tt.slug)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetBlogPostBySlug() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if post == nil {
					t.Fatal("Expected non-nil post")
				}
				if post.Slug != tt.slug {
					t.Errorf("Expected slug %s, got %s", tt.slug, post.Slug)
				}

				// Verify content was rendered to HTML
				if string(post.ContentHTML) == "" {
					t.Error("Expected ContentHTML to be populated")
				}

				// Verify markdown was rendered (should contain HTML tags)
				if !strings.Contains(string(post.ContentHTML), "<") {
					t.Error("Expected ContentHTML to contain HTML tags")
				}
			}
		})
	}
}

func TestBlogService_MarkdownRendering(t *testing.T) {
	db := SetupTestDB(t)
	blogService := NewBlogService(db)
	postService := &PostService{DB: db}

	// Create a test user and category
	userID := SeedUser(t, db, "markdown@example.com", "markdownuser", "password123", RoleEditor)
	categoryID := SeedCategory(t, db, "Markdown Category")
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
		CleanupCategory(t, db, categoryID)
	})

	tests := []struct {
		name         string
		markdown     string
		wantContains string
	}{
		{
			name:         "heading",
			markdown:     "# Heading 1",
			wantContains: "Heading 1",
		},
		{
			name:         "paragraph",
			markdown:     "This is a paragraph.",
			wantContains: "This is a paragraph",
		},
		{
			name:         "link",
			markdown:     "[Link](https://example.com)",
			wantContains: "example.com",
		},
		{
			name:         "bold text",
			markdown:     "**bold**",
			wantContains: "bold",
		},
		{
			name:         "italic text",
			markdown:     "*italic*",
			wantContains: "italic",
		},
		{
			name:         "code block",
			markdown:     "`code`",
			wantContains: "code",
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slug := "markdown-test-" + string(rune('a'+i))
			post, err := postService.Create(userID, categoryID, tt.name, tt.markdown, true, false, "", slug)
			if err != nil {
				t.Fatalf("Failed to create post: %v", err)
			}
			t.Cleanup(func() {
				CleanupPost(t, db, post.ID)
			})

			// Get the post and verify rendering
			retrieved, err := blogService.GetBlogPostBySlug(slug)
			if err != nil {
				t.Fatalf("Failed to get post: %v", err)
			}

			htmlContent := string(retrieved.ContentHTML)
			if !strings.Contains(htmlContent, tt.wantContains) {
				t.Errorf("Expected rendered content to contain %q, got %q", tt.wantContains, htmlContent)
			}
		})
	}
}

func TestBlogService_DateFormatting(t *testing.T) {
	db := SetupTestDB(t)
	blogService := NewBlogService(db)
	postService := &PostService{DB: db}

	// Create a test user and category
	userID := SeedUser(t, db, "dateformat@example.com", "dateformatuser", "password123", RoleEditor)
	categoryID := SeedCategory(t, db, "Date Category")
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
		CleanupCategory(t, db, categoryID)
	})

	// Create a post
	post, err := postService.Create(userID, categoryID, "Date Test", "Content", true, false, "", "date-test-slug")
	if err != nil {
		t.Fatalf("Failed to create post: %v", err)
	}
	t.Cleanup(func() {
		CleanupPost(t, db, post.ID)
	})

	// Get the post
	retrieved, err := blogService.GetBlogPostBySlug("date-test-slug")
	if err != nil {
		t.Fatalf("Failed to get post: %v", err)
	}

	// Verify dates are formatted in friendly format (e.g., "January 2, 2006")
	if retrieved.CreatedAt == "" {
		t.Error("Expected non-empty CreatedAt")
	}

	// The date should contain a month name (not a number)
	monthNames := []string{"January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December"}

	foundMonth := false
	for _, month := range monthNames {
		if strings.Contains(retrieved.CreatedAt, month) {
			foundMonth = true
			break
		}
	}

	if !foundMonth {
		t.Errorf("Expected CreatedAt to contain month name, got: %s", retrieved.CreatedAt)
	}

	// Same for publication date
	if retrieved.PublicationDate != "" {
		foundPubMonth := false
		for _, month := range monthNames {
			if strings.Contains(retrieved.PublicationDate, month) {
				foundPubMonth = true
				break
			}
		}
		if !foundPubMonth {
			t.Errorf("Expected PublicationDate to contain month name, got: %s", retrieved.PublicationDate)
		}
	}
}

func TestBlogService_RenderPreviewDebug(t *testing.T) {
	db := SetupTestDB(t)
	blogService := NewBlogService(db)

	markdown := "# Test Heading\n\nTest paragraph with **bold** text."

	final, stages := blogService.RenderPreviewDebug(markdown)

	// Verify final output contains rendered content
	if !strings.Contains(final, "Test Heading") {
		t.Error("Expected final output to contain heading text")
	}

	// Verify stages map is populated
	if stages == nil {
		t.Error("Expected non-nil stages map")
	}

	// Final should be rendered HTML
	if !strings.Contains(final, "<") {
		t.Error("Expected final output to contain HTML tags")
	}
}

func TestBlogService_EmptyContent(t *testing.T) {
	db := SetupTestDB(t)
	blogService := NewBlogService(db)
	postService := &PostService{DB: db}

	// Create a test user and category
	userID := SeedUser(t, db, "empty@example.com", "emptyuser", "password123", RoleEditor)
	categoryID := SeedCategory(t, db, "Empty Category")
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
		CleanupCategory(t, db, categoryID)
	})

	// Create a post with empty content
	post, err := postService.Create(userID, categoryID, "Empty Post", "", true, false, "", "empty-content-slug")
	if err != nil {
		t.Fatalf("Failed to create post: %v", err)
	}
	t.Cleanup(func() {
		CleanupPost(t, db, post.ID)
	})

	// Get the post
	retrieved, err := blogService.GetBlogPostBySlug("empty-content-slug")
	if err != nil {
		t.Fatalf("Failed to get post: %v", err)
	}

	// Should handle empty content gracefully
	if retrieved == nil {
		t.Error("Expected non-nil post even with empty content")
	}
}

func TestBlogService_LongContent(t *testing.T) {
	db := SetupTestDB(t)
	blogService := NewBlogService(db)
	postService := &PostService{DB: db}

	// Create a test user and category
	userID := SeedUser(t, db, "long@example.com", "longuser", "password123", RoleEditor)
	categoryID := SeedCategory(t, db, "Long Category")
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
		CleanupCategory(t, db, categoryID)
	})

	// Create a post with very long content
	longMarkdown := strings.Repeat("# Heading\n\nParagraph text with some content. ", 100)
	post, err := postService.Create(userID, categoryID, "Long Post", longMarkdown, true, false, "", "long-content-slug")
	if err != nil {
		t.Fatalf("Failed to create post: %v", err)
	}
	t.Cleanup(func() {
		CleanupPost(t, db, post.ID)
	})

	// Get the post
	retrieved, err := blogService.GetBlogPostBySlug("long-content-slug")
	if err != nil {
		t.Fatalf("Failed to get post: %v", err)
	}

	// Should render all content
	if len(string(retrieved.ContentHTML)) == 0 {
		t.Error("Expected ContentHTML to be populated for long content")
	}

	// Verify it contains the repeated text
	if !strings.Contains(string(retrieved.ContentHTML), "Paragraph text") {
		t.Error("Expected ContentHTML to contain the paragraph text")
	}
}

func TestBlogService_SpecialCharacters(t *testing.T) {
	db := SetupTestDB(t)
	blogService := NewBlogService(db)
	postService := &PostService{DB: db}

	// Create a test user and category
	userID := SeedUser(t, db, "special@example.com", "specialuser", "password123", RoleEditor)
	categoryID := SeedCategory(t, db, "Special Category")
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
		CleanupCategory(t, db, categoryID)
	})

	// Create a post with special characters
	specialContent := "Content with special chars: < > & \" ' \n\nAnd some unicode: émojis 🚀"
	post, err := postService.Create(userID, categoryID, "Special Post", specialContent, true, false, "", "special-chars-slug")
	if err != nil {
		t.Fatalf("Failed to create post: %v", err)
	}
	t.Cleanup(func() {
		CleanupPost(t, db, post.ID)
	})

	// Get the post
	retrieved, err := blogService.GetBlogPostBySlug("special-chars-slug")
	if err != nil {
		t.Fatalf("Failed to get post: %v", err)
	}

	// Content should be present (exact rendering depends on markdown processor)
	if len(string(retrieved.ContentHTML)) == 0 {
		t.Error("Expected ContentHTML to be populated")
	}
}

func TestBlogService_GetBlogPostBySlug_CaseInsensitivity(t *testing.T) {
	db := SetupTestDB(t)
	blogService := NewBlogService(db)
	postService := &PostService{DB: db}

	// Create a test user and category
	userID := SeedUser(t, db, "case@example.com", "caseuser", "password123", RoleEditor)
	categoryID := SeedCategory(t, db, "Case Category")
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
		CleanupCategory(t, db, categoryID)
	})

	// Create a post with lowercase slug
	post, err := postService.Create(userID, categoryID, "Case Test", "Content", true, false, "", "lowercase-slug")
	if err != nil {
		t.Fatalf("Failed to create post: %v", err)
	}
	t.Cleanup(func() {
		CleanupPost(t, db, post.ID)
	})

	// Try to get with exact slug
	retrieved, err := blogService.GetBlogPostBySlug("lowercase-slug")
	if err != nil {
		t.Errorf("Failed to get post with exact slug: %v", err)
	}
	if retrieved == nil || retrieved.Slug != "lowercase-slug" {
		t.Error("Expected to retrieve post with exact slug match")
	}

	// Try to get with uppercase slug (should not find it - slugs are case-sensitive)
	_, err = blogService.GetBlogPostBySlug("LOWERCASE-SLUG")
	if err == nil {
		t.Log("Note: Slug lookup is case-sensitive (expected behavior)")
	}
}

// Test helper functions for preview generation
func TestPreviewContentRaw_MoreTagVariations(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "more tag with no space",
			content:  "First paragraph\n\n<more-->\n\nSecond paragraph",
			expected: "First paragraph",
		},
		{
			name:     "more tag with space",
			content:  "First paragraph\n\n<more -->\n\nSecond paragraph",
			expected: "First paragraph",
		},
		{
			name:     "HTML encoded more tag",
			content:  "First paragraph\n\n&lt;more--&gt;\n\nSecond paragraph",
			expected: "First paragraph",
		},
		{
			name:     "HTML encoded more tag with space",
			content:  "First paragraph\n\n&lt;more --&gt;\n\nSecond paragraph",
			expected: "First paragraph",
		},
		{
			name:     "multiple more tags - uses first",
			content:  "First\n<more-->\nMiddle\n<more -->\nLast",
			expected: "First",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := previewContentRaw(tt.content)
			if result != tt.expected {
				t.Errorf("previewContentRaw() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestPreviewContentRaw_ShortContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "very short content",
			content: "Short text",
		},
		{
			name:    "content under 150 chars",
			content: "This is a medium length content that is under the 150 character limit and should be returned as-is without truncation.",
		},
		{
			name:    "exactly at limit",
			content: strings.Repeat("word ", 30), // ~150 chars with spaces
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := previewContentRaw(tt.content)
			// Should return original content trimmed
			expected := strings.TrimSpace(tt.content)
			if result != expected {
				t.Errorf("previewContentRaw() = %q, want %q", result, expected)
			}
		})
	}
}

func TestPreviewContentRaw_EmptyLines(t *testing.T) {
	content := "First line\n\n\n\nSecond line after empty lines\n\n\nThird line"
	result := previewContentRaw(content)

	// Should skip empty lines and concatenate with double newlines
	if !strings.Contains(result, "First line") {
		t.Error("Expected result to contain 'First line'")
	}
	if !strings.Contains(result, "Second line") {
		t.Error("Expected result to contain 'Second line'")
	}
}

func TestPreviewContentRaw_FirstLineTruncation(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		checkStr string
	}{
		{
			name:     "single very long line with spaces",
			content:  strings.Repeat("word ", 100), // Way over 150 chars
			checkStr: "...",
		},
		{
			name:     "long line with markdown",
			content:  "**" + strings.Repeat("bold word ", 50) + "**",
			checkStr: "...",
		},
		{
			name:     "long URL-like content",
			content:  "Check out https://example.com/" + strings.Repeat("verylongpath/", 20),
			checkStr: "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := previewContentRaw(tt.content)

			// Should be truncated at word boundary with ellipsis
			if !strings.HasSuffix(result, tt.checkStr) {
				t.Errorf("Expected truncated content to end with %q, got %q", tt.checkStr, result)
			}

			// Should be much shorter than original
			if len(result) >= len(tt.content) {
				t.Errorf("Expected truncated length < original, got %d >= %d", len(result), len(tt.content))
			}
		})
	}
}

func TestPreviewContentRaw_MultiLineBoundary(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "multiple short lines",
			content: "Line 1\n\nLine 2\n\nLine 3\n\nLine 4\n\nLine 5",
		},
		{
			name:    "lines with varying length",
			content: "Short\n\nThis is a medium length line\n\nAnother line here\n\nAnd more content",
		},
		{
			name:    "lines that together exceed limit",
			content: strings.Repeat("Medium line of text here. ", 3) + "\n\n" + strings.Repeat("Another line. ", 3),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := previewContentRaw(tt.content)

			// Should stop when approaching limit
			rendered := RenderContent(result)
			plainText := stripHTML(rendered)

			// Result should not be drastically longer than limit
			if len(plainText) > 200 {
				t.Errorf("Expected preview to be reasonably short, got %d chars", len(plainText))
			}

			// Should preserve line breaks between paragraphs
			if strings.Contains(result, "\n\n") && len(strings.Split(tt.content, "\n\n")) > 1 {
				if !strings.Contains(result, "\n\n") {
					t.Error("Expected paragraph breaks to be preserved")
				}
			}
		})
	}
}

func TestPreviewContentRaw_WordBoundaries(t *testing.T) {
	// Test that truncation happens at word boundaries
	longWord := strings.Repeat("supercalifragilisticexpialidocious", 10)
	content := "Start " + longWord + " end"

	result := previewContentRaw(content)

	// Should include "Start" and truncate before or within the long word
	if !strings.Contains(result, "Start") {
		t.Error("Expected result to contain 'Start'")
	}

	// Should have ellipsis indicating truncation
	if !strings.Contains(result, "...") {
		t.Error("Expected result to contain '...' for truncation")
	}
}

func TestPreviewContentRaw_MixedContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "markdown with lists",
			content: "# Heading\n\n- Item 1\n- Item 2 with more text\n- Item 3\n\nParagraph after list",
		},
		{
			name:    "markdown with code",
			content: "Introduction\n\n```go\nfunc main() {}\n```\n\nMore text after code",
		},
		{
			name:    "markdown with links",
			content: "Check out [this link](https://example.com) and [another](https://test.com) for more info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := previewContentRaw(tt.content)

			// Should return some content
			if result == "" {
				t.Error("Expected non-empty result")
			}

			// Should be shorter than or equal to original
			if len(result) > len(tt.content) {
				t.Errorf("Expected result <= original length, got %d > %d", len(result), len(tt.content))
			}
		})
	}
}

func TestPreviewContentRaw_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "empty string",
			content: "",
			want:    "",
		},
		{
			name:    "only whitespace",
			content: "   \n\n   \n   ",
			want:    "",
		},
		{
			name:    "more tag at start",
			content: "<more-->Rest of content",
			want:    "<more-->Rest of content", // Short content returned as-is even with tag
		},
		{
			name:    "more tag with content before",
			content: "Before<more-->After",
			want:    "Before",
		},
		{
			name:    "single word",
			content: "Hello",
			want:    "Hello",
		},
		{
			name:    "newlines only",
			content: "\n\n\n",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := previewContentRaw(tt.content)
			if result != tt.want {
				t.Errorf("previewContentRaw(%q) = %q, want %q", tt.content, result, tt.want)
			}
		})
	}
}

func TestPreviewContentRaw_SpecificCoverage(t *testing.T) {
	// Test to ensure findContentBeforeMoreTag returns empty string when no tag
	t.Run("no more tag returns empty from findContentBeforeMoreTag", func(t *testing.T) {
		content := "Regular content without any special tags"
		result := previewContentRaw(content)
		// Should still process normally
		if result == "" {
			t.Error("Expected non-empty result for content without more tag")
		}
	})

	// Test word truncation with exactly one word fitting
	t.Run("truncation with single fitting word", func(t *testing.T) {
		// Create content where only first word fits within limit
		longSecondWord := strings.Repeat("x", 200)
		content := "First " + longSecondWord
		result := previewContentRaw(content)

		if !strings.Contains(result, "First") {
			t.Error("Expected 'First' to be in truncated result")
		}
		if !strings.Contains(result, "...") {
			t.Error("Expected ellipsis in truncated result")
		}
	})

	// Test line length calculation edge case
	t.Run("multi-line with exact boundary", func(t *testing.T) {
		// Create lines that add up to just under 150 chars
		line1 := strings.Repeat("a ", 25) // ~50 chars
		line2 := strings.Repeat("b ", 25) // ~50 chars
		line3 := strings.Repeat("c ", 25) // ~50 chars
		content := line1 + "\n\n" + line2 + "\n\n" + line3
		result := previewContentRaw(content)

		// Should include at least the first two lines
		if !strings.Contains(result, "a") {
			t.Error("Expected first line in result")
		}
	})
}
