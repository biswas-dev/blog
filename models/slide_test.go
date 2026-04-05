package models

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// jsonEqual compares two JSON strings structurally (ignoring whitespace differences).
func jsonEqual(a, b string) bool {
	var ja, jb interface{}
	if err := json.Unmarshal([]byte(a), &ja); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(b), &jb); err != nil {
		return false
	}
	na, _ := json.Marshal(ja)
	nb, _ := json.Marshal(jb)
	return string(na) == string(nb)
}

func TestSlideService_Create(t *testing.T) {
	db := SetupTestDB(t)
	slideService := &SlideService{DB: db}

	// Create a test user
	userID := SeedUser(t, db, "slideauthor@example.com", "slideauthor", "password123", RoleEditor)
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
	})

	tests := []struct {
		name        string
		title       string
		slug        string
		content     string
		isPublished bool
		wantErr     bool
	}{
		{
			name:        "create published slide",
			title:       "Test Slide",
			slug:        "test-slide-1",
			content:     "<h1>Test Content</h1>",
			isPublished: true,
			wantErr:     false,
		},
		{
			name:        "create draft slide",
			title:       "Draft Slide",
			slug:        "draft-slide-1",
			content:     "<h1>Draft</h1>",
			isPublished: false,
			wantErr:     false,
		},
		{
			name:        "create slide with empty slug (auto-generate)",
			title:       "Auto Slug Slide",
			slug:        "",
			content:     "<h1>Content</h1>",
			isPublished: true,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slide, err := slideService.Create(userID, tt.title, tt.slug, tt.content, tt.isPublished, nil, "", "{}", "", "")

			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				// Cleanup
				t.Cleanup(func() {
					slideService.Delete(slide.ID)
				})

				if slide.ID == 0 {
					t.Error("Expected non-zero slide ID")
				}
				if slide.Title != tt.title {
					t.Errorf("Expected title %s, got %s", tt.title, slide.Title)
				}
				if slide.Slug == "" {
					t.Error("Expected non-empty slug")
				}
				if slide.IsPublished != tt.isPublished {
					t.Errorf("Expected IsPublished %v, got %v", tt.isPublished, slide.IsPublished)
				}

				// Verify content file was created
				if _, err := os.Stat(slide.ContentFilePath); os.IsNotExist(err) {
					t.Errorf("Content file not created at %s", slide.ContentFilePath)
				}

				// Verify content was written correctly
				fileContent, err := os.ReadFile(slide.ContentFilePath)
				if err != nil {
					t.Errorf("Failed to read content file: %v", err)
				}
				if string(fileContent) != tt.content {
					t.Errorf("Expected content %q, got %q", tt.content, string(fileContent))
				}
			}
		})
	}
}

func TestSlideService_GetByID(t *testing.T) {
	db := SetupTestDB(t)
	slideService := &SlideService{DB: db}

	// Create a test user and slide
	userID := SeedUser(t, db, "getslide@example.com", "getslideuser", "password123", RoleEditor)
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
	})

	slide, err := slideService.Create(userID, "Test Slide", "test-slide-getbyid", "<h1>Content</h1>", true, nil, "", "{}", "", "")
	if err != nil {
		t.Fatalf("Failed to create test slide: %v", err)
	}
	t.Cleanup(func() {
		slideService.Delete(slide.ID)
	})

	// Test retrieving the slide
	retrieved, err := slideService.GetByID(slide.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if retrieved.ID != slide.ID {
		t.Errorf("Expected ID %d, got %d", slide.ID, retrieved.ID)
	}
	if retrieved.Title != slide.Title {
		t.Errorf("Expected title %s, got %s", slide.Title, retrieved.Title)
	}
	if retrieved.Slug != slide.Slug {
		t.Errorf("Expected slug %s, got %s", slide.Slug, retrieved.Slug)
	}

	// Verify content was loaded
	if string(retrieved.ContentHTML) == "" {
		t.Error("Expected content to be loaded")
	}

	// Test retrieving non-existent slide
	_, err = slideService.GetByID(999999)
	if err == nil {
		t.Error("Expected error when retrieving non-existent slide")
	}
}

func TestSlideService_GetBySlug(t *testing.T) {
	db := SetupTestDB(t)
	slideService := &SlideService{DB: db}

	// Create a test user and slide
	userID := SeedUser(t, db, "getbyslug@example.com", "getbysluguser", "password123", RoleEditor)
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
	})

	slide, err := slideService.Create(userID, "Slug Test Slide", "unique-slug-test", "<h1>Content</h1>", true, nil, "", "{}", "", "")
	if err != nil {
		t.Fatalf("Failed to create test slide: %v", err)
	}
	t.Cleanup(func() {
		slideService.Delete(slide.ID)
	})

	// Test retrieving by slug
	retrieved, err := slideService.GetBySlug("unique-slug-test")
	if err != nil {
		t.Fatalf("GetBySlug() error = %v", err)
	}

	if retrieved.ID != slide.ID {
		t.Errorf("Expected ID %d, got %d", slide.ID, retrieved.ID)
	}

	// Test retrieving with non-existent slug
	_, err = slideService.GetBySlug("non-existent-slug")
	if err == nil {
		t.Error("Expected error when retrieving non-existent slug")
	}
}

func TestSlideService_Update(t *testing.T) {
	db := SetupTestDB(t)
	slideService := &SlideService{DB: db}

	// Create a test user and slide
	userID := SeedUser(t, db, "updateslide@example.com", "updateslideuser", "password123", RoleEditor)
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
	})

	slide, err := slideService.Create(userID, "Original Title", "original-slug-update", "<h1>Original</h1>", false, nil, "", "{}", "", "")
	if err != nil {
		t.Fatalf("Failed to create test slide: %v", err)
	}
	t.Cleanup(func() {
		slideService.Delete(slide.ID)
	})

	// Update the slide
	newContent := "<h1>Updated Content</h1>"
	err = slideService.Update(slide.ID, "Updated Title", "updated-slug", newContent, true, nil, "", "", "", "")
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify the update
	updated, err := slideService.GetByID(slide.ID)
	if err != nil {
		t.Fatalf("Failed to get updated slide: %v", err)
	}

	if updated.Title != "Updated Title" {
		t.Errorf("Expected title 'Updated Title', got %s", updated.Title)
	}
	if updated.Slug != "updated-slug" {
		t.Errorf("Expected slug 'updated-slug', got %s", updated.Slug)
	}
	if !updated.IsPublished {
		t.Error("Expected slide to be published")
	}

	// Verify content file was updated
	fileContent, err := os.ReadFile(updated.ContentFilePath)
	if err != nil {
		t.Fatalf("Failed to read content file: %v", err)
	}
	if string(fileContent) != newContent {
		t.Errorf("Expected content %q, got %q", newContent, string(fileContent))
	}
}

func TestSlideService_Delete(t *testing.T) {
	db := SetupTestDB(t)
	slideService := &SlideService{DB: db}

	// Create a test user and slide
	userID := SeedUser(t, db, "deleteslide@example.com", "deleteslideuser", "password123", RoleEditor)
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
	})

	slide, err := slideService.Create(userID, "To Delete", "delete-slide-test", "<h1>Delete Me</h1>", true, nil, "", "{}", "", "")
	if err != nil {
		t.Fatalf("Failed to create test slide: %v", err)
	}

	// Note the file path before deletion
	contentPath := slide.ContentFilePath
	slideDir := filepath.Dir(contentPath)

	// Delete the slide
	err = slideService.Delete(slide.ID)
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}

	// Verify slide was deleted from database
	_, err = slideService.GetByID(slide.ID)
	if err == nil {
		t.Error("Expected error when getting deleted slide")
	}

	// Verify content file and directory were deleted
	if _, err := os.Stat(contentPath); !os.IsNotExist(err) {
		t.Error("Content file should be deleted")
	}
	if _, err := os.Stat(slideDir); !os.IsNotExist(err) {
		t.Error("Slide directory should be deleted")
	}
}

func TestSlideService_GetPublishedSlides(t *testing.T) {
	db := SetupTestDB(t)
	slideService := &SlideService{DB: db}

	// Create a test user
	userID := SeedUser(t, db, "publishedslides@example.com", "publishedslidesuser", "password123", RoleEditor)
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
	})

	// Create published and draft slides
	published1, _ := slideService.Create(userID, "Published 1", "pub-1", "<h1>Pub 1</h1>", true, nil, "", "{}", "", "")
	published2, _ := slideService.Create(userID, "Published 2", "pub-2", "<h1>Pub 2</h1>", true, nil, "", "{}", "", "")
	draft, _ := slideService.Create(userID, "Draft", "draft-1", "<h1>Draft</h1>", false, nil, "", "{}", "", "")

	t.Cleanup(func() {
		slideService.Delete(published1.ID)
		slideService.Delete(published2.ID)
		slideService.Delete(draft.ID)
	})

	// Get published slides
	slidesList, err := slideService.GetPublishedSlides()
	if err != nil {
		t.Fatalf("GetPublishedSlides() error = %v", err)
	}

	// Verify only published slides are returned
	foundPublished1 := false
	foundPublished2 := false
	foundDraft := false

	for _, slide := range slidesList.Slides {
		if slide.ID == published1.ID {
			foundPublished1 = true
		}
		if slide.ID == published2.ID {
			foundPublished2 = true
		}
		if slide.ID == draft.ID {
			foundDraft = true
		}
	}

	if !foundPublished1 {
		t.Error("Published 1 not found in published slides")
	}
	if !foundPublished2 {
		t.Error("Published 2 not found in published slides")
	}
	if foundDraft {
		t.Error("Draft slide should not be in published slides")
	}
}

func TestSlideService_GetAllSlides(t *testing.T) {
	db := SetupTestDB(t)
	slideService := &SlideService{DB: db}

	// Create a test user
	userID := SeedUser(t, db, "allslides@example.com", "allslidesuser", "password123", RoleEditor)
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
	})

	// Create slides (both published and draft)
	published, _ := slideService.Create(userID, "Published", "all-pub-1", "<h1>Pub</h1>", true, nil, "", "{}", "", "")
	draft, _ := slideService.Create(userID, "Draft", "all-draft-1", "<h1>Draft</h1>", false, nil, "", "{}", "", "")

	t.Cleanup(func() {
		slideService.Delete(published.ID)
		slideService.Delete(draft.ID)
	})

	// Get all slides
	slidesList, err := slideService.GetAllSlides()
	if err != nil {
		t.Fatalf("GetAllSlides() error = %v", err)
	}

	// Should include both published and unpublished
	foundPublished := false
	foundDraft := false

	for _, slide := range slidesList.Slides {
		if slide.ID == published.ID {
			foundPublished = true
		}
		if slide.ID == draft.ID {
			foundDraft = true
		}
	}

	if !foundPublished {
		t.Error("Published slide not found in GetAllSlides()")
	}
	if !foundDraft {
		t.Error("Draft slide not found in GetAllSlides()")
	}
}

func TestSlideService_Categories(t *testing.T) {
	db := SetupTestDB(t)
	slideService := &SlideService{DB: db}
	categoryService := &CategoryService{DB: db}

	// Create a test user
	userID := SeedUser(t, db, "slidecats@example.com", "slidecatsuser", "password123", RoleEditor)
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
	})

	// Create categories
	cat1, _ := categoryService.Create("Slide Cat 1")
	cat2, _ := categoryService.Create("Slide Cat 2")
	t.Cleanup(func() {
		CleanupCategory(t, db, cat1.ID)
		CleanupCategory(t, db, cat2.ID)
	})

	t.Run("create slide with categories", func(t *testing.T) {
		slide, err := slideService.Create(userID, "Cat Slide", "cat-slide-1", "<h1>Content</h1>", true, []int{cat1.ID, cat2.ID}, "", "{}", "", "")
		if err != nil {
			t.Fatalf("Failed to create slide with categories: %v", err)
		}
		t.Cleanup(func() {
			slideService.Delete(slide.ID)
		})

		// Retrieve and verify categories
		retrieved, _ := slideService.GetByID(slide.ID)
		if len(retrieved.Categories) != 2 {
			t.Errorf("Expected 2 categories, got %d", len(retrieved.Categories))
		}
	})

	t.Run("update slide categories", func(t *testing.T) {
		slide, _ := slideService.Create(userID, "Update Cat Slide", "update-cat-slide-1", "<h1>Content</h1>", true, []int{cat1.ID}, "", "{}", "", "")
		t.Cleanup(func() {
			slideService.Delete(slide.ID)
		})

		// Update to different categories
		err := slideService.Update(slide.ID, "Updated", "updated", "<h1>Updated</h1>", true, []int{cat2.ID}, "", "", "", "")
		if err != nil {
			t.Errorf("Update() error = %v", err)
		}

		// Verify categories were updated
		retrieved, _ := slideService.GetByID(slide.ID)
		if len(retrieved.Categories) != 1 {
			t.Errorf("Expected 1 category, got %d", len(retrieved.Categories))
		}
		if len(retrieved.Categories) > 0 && retrieved.Categories[0].ID != cat2.ID {
			t.Error("Expected category to be cat2")
		}
	})
}

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		wantSlug string
	}{
		{
			name:     "simple title",
			title:    "Hello World",
			wantSlug: "hello-world",
		},
		{
			name:     "title with special characters",
			title:    "Hello, World! 123",
			wantSlug: "hello-world-123",
		},
		{
			name:     "title with multiple spaces",
			title:    "Hello   World   Test",
			wantSlug: "hello-world-test",
		},
		{
			name:     "empty title",
			title:    "",
			wantSlug: "slide-", // Should generate timestamp-based slug
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slug := generateSlug(tt.title)
			if tt.wantSlug != "slide-" {
				if slug != tt.wantSlug {
					t.Errorf("Expected slug %q, got %q", tt.wantSlug, slug)
				}
			} else {
				// For empty title, just verify it starts with "slide-"
				if !strings.HasPrefix(slug, tt.wantSlug) {
					t.Errorf("Expected slug to start with %q, got %q", tt.wantSlug, slug)
				}
			}
		})
	}
}

func TestSanitizeSlug(t *testing.T) {
	tests := []struct {
		name     string
		slug     string
		wantSlug string
	}{
		{
			name:     "valid slug",
			slug:     "hello-world",
			wantSlug: "hello-world",
		},
		{
			name:     "slug with uppercase",
			slug:     "Hello-World",
			wantSlug: "hello-world",
		},
		{
			name:     "slug with special characters",
			slug:     "hello_world!@#",
			wantSlug: "hello-world",
		},
		{
			name:     "slug with spaces",
			slug:     "hello world",
			wantSlug: "hello-world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slug := sanitizeSlug(tt.slug)
			if slug != tt.wantSlug {
				t.Errorf("Expected slug %q, got %q", tt.wantSlug, slug)
			}
		})
	}
}

func TestSlideVersionService_MaybeCreateVersion(t *testing.T) {
	db := SetupTestDB(t)
	slideService := &SlideService{DB: db}
	versionService := &SlideVersionService{DB: db}

	userID := SeedUser(t, db, "slideversion@example.com", "slideversionuser", "password123", RoleEditor)
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
	})

	slide, err := slideService.Create(userID, "Version Test", "version-test-slide", "<h1>Original</h1>", true, nil, "", "{}", "", "")
	if err != nil {
		t.Fatalf("Failed to create slide: %v", err)
	}
	t.Cleanup(func() {
		slideService.Delete(slide.ID)
	})

	t.Run("first save creates version 1", func(t *testing.T) {
		err := versionService.MaybeCreateVersion(slide.ID, userID, "Version Test", "<h1>Original</h1>")
		if err != nil {
			t.Fatalf("MaybeCreateVersion() error = %v", err)
		}

		versions, err := versionService.GetVersions(slide.ID)
		if err != nil {
			t.Fatalf("GetVersions() error = %v", err)
		}
		if len(versions) != 1 {
			t.Fatalf("Expected 1 version, got %d", len(versions))
		}
		if versions[0].VersionNumber != 1 {
			t.Errorf("Expected version number 1, got %d", versions[0].VersionNumber)
		}
	})

	t.Run("identical content does not create new version", func(t *testing.T) {
		err := versionService.MaybeCreateVersion(slide.ID, userID, "Version Test", "<h1>Original</h1>")
		if err != nil {
			t.Fatalf("MaybeCreateVersion() error = %v", err)
		}

		versions, _ := versionService.GetVersions(slide.ID)
		if len(versions) != 1 {
			t.Errorf("Expected still 1 version after identical save, got %d", len(versions))
		}
	})

	t.Run("insignificant change does not create new version", func(t *testing.T) {
		err := versionService.MaybeCreateVersion(slide.ID, userID, "Version Test", "<h1>Original!</h1>")
		if err != nil {
			t.Fatalf("MaybeCreateVersion() error = %v", err)
		}

		versions, _ := versionService.GetVersions(slide.ID)
		if len(versions) != 1 {
			t.Errorf("Expected still 1 version after minor change, got %d", len(versions))
		}
	})

	t.Run("significant change creates new version", func(t *testing.T) {
		bigContent := "<h1>Completely new content</h1><p>" + strings.Repeat("Lorem ipsum dolor sit amet. ", 50) + "</p>"
		err := versionService.MaybeCreateVersion(slide.ID, userID, "Version Test Updated", bigContent)
		if err != nil {
			t.Fatalf("MaybeCreateVersion() error = %v", err)
		}

		versions, _ := versionService.GetVersions(slide.ID)
		if len(versions) != 2 {
			t.Errorf("Expected 2 versions after significant change, got %d", len(versions))
		}
	})

	t.Run("get specific version includes content", func(t *testing.T) {
		v, err := versionService.GetVersion(slide.ID, 1)
		if err != nil {
			t.Fatalf("GetVersion() error = %v", err)
		}
		if v.Content != "<h1>Original</h1>" {
			t.Errorf("Expected original content, got %q", v.Content)
		}
	})

	t.Run("delete version", func(t *testing.T) {
		err := versionService.DeleteVersion(slide.ID, 1)
		if err != nil {
			t.Fatalf("DeleteVersion() error = %v", err)
		}

		_, err = versionService.GetVersion(slide.ID, 1)
		if err == nil {
			t.Error("Expected error when getting deleted version")
		}
	})

	t.Run("delete non-existent version returns error", func(t *testing.T) {
		err := versionService.DeleteVersion(slide.ID, 999)
		if err == nil {
			t.Error("Expected error when deleting non-existent version")
		}
	})
}

func TestSlide_PasswordProtection(t *testing.T) {
	t.Run("set and check password", func(t *testing.T) {
		slide := &Slide{}
		err := slide.SetPassword("mysecret123")
		if err != nil {
			t.Fatalf("SetPassword() error = %v", err)
		}
		if slide.PasswordHash == "" {
			t.Fatal("Expected password hash to be set")
		}
		if slide.PasswordHash == "mysecret123" {
			t.Fatal("Password hash should not be plaintext")
		}

		// Correct password
		if !slide.CheckPassword("mysecret123") {
			t.Error("CheckPassword() should return true for correct password")
		}

		// Wrong password
		if slide.CheckPassword("wrongpassword") {
			t.Error("CheckPassword() should return false for wrong password")
		}
	})

	t.Run("empty password hash always passes", func(t *testing.T) {
		slide := &Slide{PasswordHash: ""}
		if !slide.CheckPassword("anything") {
			t.Error("Empty password hash should always pass")
		}
	})

	t.Run("create slide with password", func(t *testing.T) {
		db := SetupTestDB(t)
		slideService := &SlideService{DB: db}

		userID := SeedUser(t, db, "slidepwd@example.com", "slidepwduser", "password123", RoleEditor)
		t.Cleanup(func() {
			CleanupUser(t, db, userID)
		})

		slide, err := slideService.Create(userID, "Protected Slide", "protected-slide", "<h1>Secret</h1>", true, nil, "", "{}", "viewerpass", "")
		if err != nil {
			t.Fatalf("Failed to create slide with password: %v", err)
		}
		t.Cleanup(func() {
			slideService.Delete(slide.ID)
		})

		// Retrieve and verify password hash was stored
		retrieved, err := slideService.GetBySlug("protected-slide")
		if err != nil {
			t.Fatalf("GetBySlug() error = %v", err)
		}
		if retrieved.PasswordHash == "" {
			t.Error("Expected password hash to be stored")
		}

		// Verify bcrypt hash is correct
		err = bcrypt.CompareHashAndPassword([]byte(retrieved.PasswordHash), []byte("viewerpass"))
		if err != nil {
			t.Error("Password hash does not match 'viewerpass'")
		}
	})
}

func TestSlide_Contributors(t *testing.T) {
	db := SetupTestDB(t)
	slideService := &SlideService{DB: db}
	versionService := &SlideVersionService{DB: db}

	user1ID := SeedUser(t, db, "contrib1@example.com", "contrib1", "password123", RoleEditor)
	user2ID := SeedUser(t, db, "contrib2@example.com", "contrib2", "password123", RoleEditor)
	t.Cleanup(func() {
		CleanupUser(t, db, user1ID)
		CleanupUser(t, db, user2ID)
	})

	slide, err := slideService.Create(user1ID, "Contrib Test", "contrib-test-slide", "<h1>Original</h1>", true, nil, "", "{}", "", "")
	if err != nil {
		t.Fatalf("Failed to create slide: %v", err)
	}
	t.Cleanup(func() {
		slideService.Delete(slide.ID)
	})

	t.Run("first edit creates contributor record", func(t *testing.T) {
		err := versionService.MaybeCreateVersion(slide.ID, user1ID, "Contrib Test", "<h1>Original</h1>")
		if err != nil {
			t.Fatalf("MaybeCreateVersion() error = %v", err)
		}

		contributors, err := versionService.GetContributors(slide.ID)
		if err != nil {
			t.Fatalf("GetContributors() error = %v", err)
		}
		if len(contributors) != 1 {
			t.Fatalf("Expected 1 contributor, got %d", len(contributors))
		}
		if contributors[0].UserID != user1ID {
			t.Errorf("Expected contributor user ID %d, got %d", user1ID, contributors[0].UserID)
		}
	})

	t.Run("second user edit adds another contributor", func(t *testing.T) {
		bigContent := "<h1>User 2 edit</h1><p>" + strings.Repeat("New content by user 2. ", 50) + "</p>"
		err := versionService.MaybeCreateVersion(slide.ID, user2ID, "Contrib Test", bigContent)
		if err != nil {
			t.Fatalf("MaybeCreateVersion() error = %v", err)
		}

		contributors, err := versionService.GetContributors(slide.ID)
		if err != nil {
			t.Fatalf("GetContributors() error = %v", err)
		}
		if len(contributors) != 2 {
			t.Fatalf("Expected 2 contributors, got %d", len(contributors))
		}
	})

	t.Run("contributed slides excludes authored slides", func(t *testing.T) {
		contributed, err := versionService.GetContributedSlides(user2ID)
		if err != nil {
			t.Fatalf("GetContributedSlides() error = %v", err)
		}

		found := false
		for _, s := range contributed {
			if s.ID == slide.ID {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected user2's contributed slides to include the test slide")
		}

		// user1 authored the slide, so it should NOT appear in their contributed list
		contributedByAuthor, err := versionService.GetContributedSlides(user1ID)
		if err != nil {
			t.Fatalf("GetContributedSlides() error = %v", err)
		}
		for _, s := range contributedByAuthor {
			if s.ID == slide.ID {
				t.Error("Author's own slide should not appear in contributed slides")
				break
			}
		}
	})
}

func TestSlide_Metadata(t *testing.T) {
	db := SetupTestDB(t)
	slideService := &SlideService{DB: db}

	userID := SeedUser(t, db, "slidemeta@example.com", "slidemetauser", "password123", RoleEditor)
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
	})

	t.Run("create slide with metadata", func(t *testing.T) {
		metadata := `{"theme":"moon","transition":"slide"}`
		slide, err := slideService.Create(userID, "Meta Slide", "meta-slide-1", "<h1>Content</h1>", true, nil, "A test slide", metadata, "", "")
		if err != nil {
			t.Fatalf("Failed to create slide with metadata: %v", err)
		}
		t.Cleanup(func() {
			slideService.Delete(slide.ID)
		})

		retrieved, err := slideService.GetByID(slide.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if retrieved.Description != "A test slide" {
			t.Errorf("Expected description 'A test slide', got %q", retrieved.Description)
		}
		if !jsonEqual(retrieved.SlideMetadata, metadata) {
			t.Errorf("Expected metadata %q, got %q", metadata, retrieved.SlideMetadata)
		}
	})

	t.Run("update slide metadata", func(t *testing.T) {
		slide, err := slideService.Create(userID, "Meta Update", "meta-update-1", "<h1>Content</h1>", true, nil, "Initial desc", `{"theme":"black"}`, "", "")
		if err != nil {
			t.Fatalf("Failed to create slide: %v", err)
		}
		t.Cleanup(func() {
			slideService.Delete(slide.ID)
		})

		newMeta := `{"theme":"white","transition":"fade"}`
		err = slideService.Update(slide.ID, "Meta Update", "meta-update-1", "<h1>Content</h1>", true, nil, "Updated desc", newMeta, "", "")
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		retrieved, err := slideService.GetByID(slide.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if retrieved.Description != "Updated desc" {
			t.Errorf("Expected description 'Updated desc', got %q", retrieved.Description)
		}
		if !jsonEqual(retrieved.SlideMetadata, newMeta) {
			t.Errorf("Expected metadata %q, got %q", newMeta, retrieved.SlideMetadata)
		}
	})

	t.Run("default empty metadata round-trips as empty JSON", func(t *testing.T) {
		slide, err := slideService.Create(userID, "No Meta", "no-meta-1", "<h1>Content</h1>", true, nil, "", "", "", "")
		if err != nil {
			t.Fatalf("Failed to create slide: %v", err)
		}
		t.Cleanup(func() {
			slideService.Delete(slide.ID)
		})

		retrieved, err := slideService.GetByID(slide.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if retrieved.SlideMetadata != "{}" {
			t.Errorf("Expected default metadata '{}', got %q", retrieved.SlideMetadata)
		}
	})
}
