package models

import (
	"strings"
	"testing"
)

func TestPostService_Create(t *testing.T) {
	db := SetupTestDB(t)
	postService := &PostService{DB: db}

	// Create a test user and category
	userID := SeedUser(t, db, "postauthor@example.com", "postauthor", "password123", RoleEditor)
	categoryID := SeedCategory(t, db, "Test Category")
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
		CleanupCategory(t, db, categoryID)
	})

	tests := []struct {
		name            string
		userID          int
		categoryID      int
		title           string
		content         string
		slug            string
		isPublished     bool
		featured        bool
		featuredImageURL string
		wantErr         bool
	}{
		{
			name:            "create published post",
			userID:          userID,
			categoryID:      categoryID,
			title:           "Test Post",
			content:         "This is test content",
			slug:            "test-post-1",
			isPublished:     true,
			featured:        false,
			featuredImageURL: "",
			wantErr:         false,
		},
		{
			name:            "create draft post",
			userID:          userID,
			categoryID:      categoryID,
			title:           "Draft Post",
			content:         "This is a draft",
			slug:            "draft-post-1",
			isPublished:     false,
			featured:        false,
			featuredImageURL: "",
			wantErr:         false,
		},
		{
			name:            "create featured post",
			userID:          userID,
			categoryID:      categoryID,
			title:           "Featured Post",
			content:         "This is featured",
			slug:            "featured-post-1",
			isPublished:     true,
			featured:        true,
			featuredImageURL: "/static/uploads/featured.jpg",
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			post, err := postService.Create(
				tt.userID,
				tt.categoryID,
				tt.title,
				tt.content,
				tt.isPublished,
				tt.featured,
				tt.featuredImageURL,
				tt.slug,
			)

			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if post.ID == 0 {
					t.Error("Expected non-zero post ID")
				}
				if post.Title != tt.title {
					t.Errorf("Expected title %s, got %s", tt.title, post.Title)
				}
				if post.Content != tt.content {
					t.Errorf("Expected content %s, got %s", tt.content, post.Content)
				}
				if post.Slug != tt.slug {
					t.Errorf("Expected slug %s, got %s", tt.slug, post.Slug)
				}
				if post.IsPublished != tt.isPublished {
					t.Errorf("Expected IsPublished %v, got %v", tt.isPublished, post.IsPublished)
				}
				if post.Featured != tt.featured {
					t.Errorf("Expected Featured %v, got %v", tt.featured, post.Featured)
				}

				// Cleanup
				CleanupPost(t, db, post.ID)
			}
		})
	}
}

func TestPostService_GetByID(t *testing.T) {
	db := SetupTestDB(t)
	postService := &PostService{DB: db}

	// Create a test user, category, and post
	userID := SeedUser(t, db, "getbyid@example.com", "getbyiduser", "password123", RoleEditor)
	categoryID := SeedCategory(t, db, "GetByID Category")
	postID := SeedPost(t, db, userID, categoryID, "Test Post", "Test content", "getbyid-test", true)
	t.Cleanup(func() {
		CleanupPost(t, db, postID)
		CleanupUser(t, db, userID)
		CleanupCategory(t, db, categoryID)
	})

	// Test retrieving the post
	post, err := postService.GetByID(postID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if post.ID != postID {
		t.Errorf("Expected post ID %d, got %d", postID, post.ID)
	}
	if post.Title != "Test Post" {
		t.Errorf("Expected title 'Test Post', got %s", post.Title)
	}
	if post.Content != "Test content" {
		t.Errorf("Expected content 'Test content', got %s", post.Content)
	}

	// Test retrieving non-existent post
	_, err = postService.GetByID(999999)
	if err == nil {
		t.Error("Expected error when retrieving non-existent post")
	}
}

func TestPostService_Update(t *testing.T) {
	db := SetupTestDB(t)
	postService := &PostService{DB: db}

	// Create a test user, category, and post
	userID := SeedUser(t, db, "update@example.com", "updateuser", "password123", RoleEditor)
	categoryID := SeedCategory(t, db, "Update Category")
	postID := SeedPost(t, db, userID, categoryID, "Original Title", "Original content", "original-slug", false)
	t.Cleanup(func() {
		CleanupPost(t, db, postID)
		CleanupUser(t, db, userID)
		CleanupCategory(t, db, categoryID)
	})

	// Update the post
	err := postService.Update(
		postID,
		categoryID,
		"Updated Title",
		"Updated content",
		true,
		true,
		"/static/uploads/featured.jpg",
		"original-slug",
	)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify the update
	post, err := postService.GetByID(postID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if post.Title != "Updated Title" {
		t.Errorf("Expected title 'Updated Title', got %s", post.Title)
	}
	if post.Content != "Updated content" {
		t.Errorf("Expected content 'Updated content', got %s", post.Content)
	}
	if !post.IsPublished {
		t.Error("Expected post to be published")
	}
	if !post.Featured {
		t.Error("Expected post to be featured")
	}
}

func TestPostService_GetTopPosts(t *testing.T) {
	db := SetupTestDB(t)
	postService := &PostService{DB: db}

	// Create test data
	userID := SeedUser(t, db, "topposts@example.com", "toppostsuser", "password123", RoleEditor)
	categoryID := SeedCategory(t, db, "Top Posts Category")
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
		CleanupCategory(t, db, categoryID)
	})

	// Create multiple posts (some published, some not)
	postIDs := make([]int, 0)
	for i := 1; i <= 7; i++ {
		isPublished := i <= 6 // First 6 are published, 7th is draft
		postID := SeedPost(t, db, userID, categoryID,
			"Post "+string(rune('0'+i)),
			"Content "+string(rune('0'+i)),
			"post-"+string(rune('0'+i)),
			isPublished)
		postIDs = append(postIDs, postID)
	}
	t.Cleanup(func() {
		for _, id := range postIDs {
			CleanupPost(t, db, id)
		}
	})

	// Get top posts (should return max 5 published posts)
	postsList, err := postService.GetTopPosts()
	if err != nil {
		t.Fatalf("GetTopPosts() error = %v", err)
	}

	if postsList == nil {
		t.Fatal("Expected non-nil posts list")
	}

	// Should return max 5 posts
	if len(postsList.Posts) > 5 {
		t.Errorf("Expected max 5 posts, got %d", len(postsList.Posts))
	}

	// All returned posts should be published
	for _, post := range postsList.Posts {
		if !post.IsPublished {
			t.Errorf("Expected all posts to be published, found unpublished post: %s", post.Title)
		}
	}
}

func TestPostService_GetTopPostsWithPagination(t *testing.T) {
	db := SetupTestDB(t)
	postService := &PostService{DB: db}

	// Create test data
	userID := SeedUser(t, db, "pagination@example.com", "paginationuser", "password123", RoleEditor)
	categoryID := SeedCategory(t, db, "Pagination Category")
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
		CleanupCategory(t, db, categoryID)
	})

	// Create 10 published posts
	postIDs := make([]int, 0)
	for i := 1; i <= 10; i++ {
		postID := SeedPost(t, db, userID, categoryID,
			"Pagination Post "+string(rune('0'+i)),
			"Content "+string(rune('0'+i)),
			"pagination-post-"+string(rune('0'+i)),
			true)
		postIDs = append(postIDs, postID)
	}
	t.Cleanup(func() {
		for _, id := range postIDs {
			CleanupPost(t, db, id)
		}
	})

	// Test first page (limit 3, offset 0)
	postsList, err := postService.GetTopPostsWithPagination(3, 0)
	if err != nil {
		t.Fatalf("GetTopPostsWithPagination() error = %v", err)
	}

	if len(postsList.Posts) != 3 {
		t.Errorf("Expected 3 posts on first page, got %d", len(postsList.Posts))
	}

	// Test second page (limit 3, offset 3)
	postsList2, err := postService.GetTopPostsWithPagination(3, 3)
	if err != nil {
		t.Fatalf("GetTopPostsWithPagination() error = %v", err)
	}

	if len(postsList2.Posts) != 3 {
		t.Errorf("Expected 3 posts on second page, got %d", len(postsList2.Posts))
	}

	// Verify pages don't overlap
	if len(postsList.Posts) > 0 && len(postsList2.Posts) > 0 {
		if postsList.Posts[0].ID == postsList2.Posts[0].ID {
			t.Error("Expected different posts on different pages")
		}
	}
}

func TestPostService_GetAllPosts(t *testing.T) {
	db := SetupTestDB(t)
	postService := &PostService{DB: db}

	// Create test user and category
	userID := SeedUser(t, db, "allposts@example.com", "allpostsuser", "password123", RoleEditor)
	categoryID := SeedCategory(t, db, "All Posts Category")
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
		CleanupCategory(t, db, categoryID)
	})

	// Create some posts (both published and unpublished)
	postIDs := make([]int, 0)
	publishedPost := SeedPost(t, db, userID, categoryID, "Published", "Content", "published-1", true)
	draftPost := SeedPost(t, db, userID, categoryID, "Draft", "Content", "draft-1", false)
	postIDs = append(postIDs, publishedPost, draftPost)
	t.Cleanup(func() {
		for _, id := range postIDs {
			CleanupPost(t, db, id)
		}
	})

	// Get all posts (should include both published and unpublished)
	postsList, err := postService.GetAllPosts()
	if err != nil {
		t.Fatalf("GetAllPosts() error = %v", err)
	}

	if postsList == nil {
		t.Fatal("Expected non-nil posts list")
	}

	// Should include both published and unpublished posts
	foundPublished := false
	foundDraft := false
	for _, post := range postsList.Posts {
		if post.ID == publishedPost {
			foundPublished = true
		}
		if post.ID == draftPost {
			foundDraft = true
		}
	}

	if !foundPublished {
		t.Error("Expected to find published post in GetAllPosts()")
	}
	if !foundDraft {
		t.Error("Expected to find draft post in GetAllPosts()")
	}
}

func TestPostService_GetPostsByUser(t *testing.T) {
	db := SetupTestDB(t)
	postService := &PostService{DB: db}

	// Create two test users
	user1ID := SeedUser(t, db, "user1@example.com", "user1", "password123", RoleEditor)
	user2ID := SeedUser(t, db, "user2@example.com", "user2", "password123", RoleEditor)
	categoryID := SeedCategory(t, db, "User Posts Category")
	t.Cleanup(func() {
		CleanupUser(t, db, user1ID)
		CleanupUser(t, db, user2ID)
		CleanupCategory(t, db, categoryID)
	})

	// Create posts for both users
	user1Post1 := SeedPost(t, db, user1ID, categoryID, "User1 Post1", "Content", "user1-post1", true)
	user1Post2 := SeedPost(t, db, user1ID, categoryID, "User1 Post2", "Content", "user1-post2", false)
	user2Post := SeedPost(t, db, user2ID, categoryID, "User2 Post", "Content", "user2-post", true)
	t.Cleanup(func() {
		CleanupPost(t, db, user1Post1)
		CleanupPost(t, db, user1Post2)
		CleanupPost(t, db, user2Post)
	})

	// Get posts by user1
	postsList, err := postService.GetPostsByUser(user1ID)
	if err != nil {
		t.Fatalf("GetPostsByUser() error = %v", err)
	}

	if postsList == nil {
		t.Fatal("Expected non-nil posts list")
	}

	// Should only return user1's posts
	for _, post := range postsList.Posts {
		if post.UserID != user1ID {
			t.Errorf("Expected only user1's posts, found post by user %d", post.UserID)
		}
	}

	// Should have at least 2 posts for user1
	foundCount := 0
	for _, post := range postsList.Posts {
		if post.ID == user1Post1 || post.ID == user1Post2 {
			foundCount++
		}
	}
	if foundCount < 2 {
		t.Errorf("Expected to find 2 posts for user1, found %d", foundCount)
	}
}

func TestTrimContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantContains string
		wantNotContains string
	}{
		{
			name:     "content with more tag",
			content:  "This is the preview content<more-->This should be trimmed",
			wantContains: "preview content",
			wantNotContains: "should be trimmed",
		},
		{
			name:     "content with escaped more tag",
			content:  "This is the preview&lt;more--&gt;This should be trimmed",
			wantContains: "preview",
			wantNotContains: "should be trimmed",
		},
		{
			name:     "content without more tag",
			content:  strings.Repeat("word ", 50), // 50 words
			wantContains: "word",
			wantNotContains: "",
		},
		{
			name:     "content with code blocks",
			content:  "Text before ```code block``` text after",
			wantContains: "Text",
			wantNotContains: "```",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trimContent(tt.content)

			if tt.wantContains != "" && !strings.Contains(result, tt.wantContains) {
				t.Errorf("trimContent() result should contain %q, got %q", tt.wantContains, result)
			}

			if tt.wantNotContains != "" && strings.Contains(result, tt.wantNotContains) {
				t.Errorf("trimContent() result should not contain %q, got %q", tt.wantNotContains, result)
			}
		})
	}
}

func TestPreviewContentRaw(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantContains string
		wantNotContains string
	}{
		{
			name:     "content with <more--> tag",
			content:  "This is preview content\n\n<more-->\n\nThis is full content",
			wantContains: "preview content",
			wantNotContains: "full content",
		},
		{
			name:     "content with <more --> tag (with space)",
			content:  "Preview here<more -->Full content here",
			wantContains: "Preview",
			wantNotContains: "Full content",
		},
		{
			name:     "content with escaped more tag",
			content:  "Preview&lt;more--&gt;Full content",
			wantContains: "Preview",
			wantNotContains: "Full content",
		},
		{
			name:     "short content without more tag",
			content:  "This is a short post",
			wantContains: "short post",
			wantNotContains: "",
		},
		{
			name:     "long content without more tag",
			content:  strings.Repeat("Lorem ipsum dolor sit amet. ", 30),
			wantContains: "Lorem ipsum",
			wantNotContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := previewContentRaw(tt.content)

			if tt.wantContains != "" && !strings.Contains(result, tt.wantContains) {
				t.Errorf("previewContentRaw() result should contain %q, got %q", tt.wantContains, result)
			}

			if tt.wantNotContains != "" && strings.Contains(result, tt.wantNotContains) {
				t.Errorf("previewContentRaw() result should not contain %q, got %q", tt.wantNotContains, result)
			}

			// Verify result doesn't contain the more tag itself
			if strings.Contains(result, "<more-->") || strings.Contains(result, "&lt;more--&gt;") {
				t.Error("previewContentRaw() result should not contain the more tag")
			}
		})
	}
}

func TestRenderContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantContains string
	}{
		{
			name:     "markdown heading",
			content:  "# Heading 1",
			wantContains: "Heading 1",
		},
		{
			name:     "markdown link",
			content:  "[Link](https://example.com)",
			wantContains: "example.com",
		},
		{
			name:     "markdown code",
			content:  "`inline code`",
			wantContains: "inline code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderContent(tt.content)

			if !strings.Contains(result, tt.wantContains) {
				t.Errorf("RenderContent() result should contain %q, got %q", tt.wantContains, result)
			}
		})
	}
}
