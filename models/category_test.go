package models

import (
	"strings"
	"testing"
)

func TestCategoryService_Create(t *testing.T) {
	db := SetupTestDB(t)
	categoryService := &CategoryService{DB: db}

	tests := []struct {
		name     string
		catName  string
		wantErr  bool
		errMsg   string
	}{
		{
			name:    "create valid category",
			catName: "Technology",
			wantErr: false,
		},
		{
			name:    "create category with spaces",
			catName: "  Web Development  ",
			wantErr: false, // Should trim spaces
		},
		{
			name:    "create empty category",
			catName: "",
			wantErr: true,
			errMsg:  "category name cannot be empty",
		},
		{
			name:    "create category with only spaces",
			catName: "   ",
			wantErr: true,
			errMsg:  "category name cannot be empty",
		},
		{
			name:    "create category with very long name",
			catName: strings.Repeat("a", 256),
			wantErr: true,
			errMsg:  "category name too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			category, err := categoryService.Create(tt.catName)

			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error message to contain %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if category.ID == 0 {
				t.Error("Expected non-zero category ID")
			}

			// Verify name is trimmed
			expectedName := strings.TrimSpace(tt.catName)
			if category.Name != expectedName {
				t.Errorf("Expected name %q, got %q", expectedName, category.Name)
			}

			if category.CreatedAt.IsZero() {
				t.Error("Expected non-zero created timestamp")
			}

			// Cleanup
			t.Cleanup(func() {
				CleanupCategory(t, db, category.ID)
			})
		})
	}
}

func TestCategoryService_GetByID(t *testing.T) {
	db := SetupTestDB(t)
	categoryService := &CategoryService{DB: db}

	// Create a test category
	created, err := categoryService.Create("Test Category")
	if err != nil {
		t.Fatalf("Failed to create test category: %v", err)
	}
	t.Cleanup(func() {
		CleanupCategory(t, db, created.ID)
	})

	tests := []struct {
		name    string
		id      int
		wantErr bool
	}{
		{
			name:    "get existing category",
			id:      created.ID,
			wantErr: false,
		},
		{
			name:    "get non-existent category",
			id:      999999,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			category, err := categoryService.GetByID(tt.id)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetByID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if category.ID != tt.id {
					t.Errorf("Expected ID %d, got %d", tt.id, category.ID)
				}
				if category.Name != created.Name {
					t.Errorf("Expected name %q, got %q", created.Name, category.Name)
				}
			}
		})
	}
}

func TestCategoryService_GetAll(t *testing.T) {
	db := SetupTestDB(t)
	categoryService := &CategoryService{DB: db}

	// Create multiple test categories
	cat1, err := categoryService.Create("Category A")
	if err != nil {
		t.Fatalf("Failed to create category A: %v", err)
	}
	t.Cleanup(func() {
		CleanupCategory(t, db, cat1.ID)
	})

	cat2, err := categoryService.Create("Category B")
	if err != nil {
		t.Fatalf("Failed to create category B: %v", err)
	}
	t.Cleanup(func() {
		CleanupCategory(t, db, cat2.ID)
	})

	cat3, err := categoryService.Create("Category C")
	if err != nil {
		t.Fatalf("Failed to create category C: %v", err)
	}
	t.Cleanup(func() {
		CleanupCategory(t, db, cat3.ID)
	})

	// Get all categories
	categories, err := categoryService.GetAll()
	if err != nil {
		t.Fatalf("GetAll() error = %v", err)
	}

	// Should have at least our 3 categories
	if len(categories) < 3 {
		t.Errorf("Expected at least 3 categories, got %d", len(categories))
	}

	// Verify our categories are present
	found := make(map[int]bool)
	for _, cat := range categories {
		found[cat.ID] = true
	}

	if !found[cat1.ID] {
		t.Error("Category A not found in GetAll()")
	}
	if !found[cat2.ID] {
		t.Error("Category B not found in GetAll()")
	}
	if !found[cat3.ID] {
		t.Error("Category C not found in GetAll()")
	}

	// Verify categories are sorted alphabetically by name
	for i := 1; i < len(categories); i++ {
		if categories[i-1].Name > categories[i].Name {
			t.Errorf("Categories not sorted alphabetically: %q > %q",
				categories[i-1].Name, categories[i].Name)
			break
		}
	}
}

func TestCategoryService_Update(t *testing.T) {
	db := SetupTestDB(t)
	categoryService := &CategoryService{DB: db}

	// Create a test category
	category, err := categoryService.Create("Original Name")
	if err != nil {
		t.Fatalf("Failed to create test category: %v", err)
	}
	t.Cleanup(func() {
		CleanupCategory(t, db, category.ID)
	})

	tests := []struct {
		name    string
		id      int
		newName string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "update with valid name",
			id:      category.ID,
			newName: "Updated Name",
			wantErr: false,
		},
		{
			name:    "update with name containing spaces",
			id:      category.ID,
			newName: "  Trimmed Name  ",
			wantErr: false,
		},
		{
			name:    "update with empty name",
			id:      category.ID,
			newName: "",
			wantErr: true,
			errMsg:  "category name cannot be empty",
		},
		{
			name:    "update non-existent category",
			id:      999999,
			newName: "New Name",
			wantErr: true,
			errMsg:  "category not found",
		},
		{
			name:    "update with very long name",
			id:      category.ID,
			newName: strings.Repeat("a", 256),
			wantErr: true,
			errMsg:  "category name too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated, err := categoryService.Update(tt.id, tt.newName)

			if (err != nil) != tt.wantErr {
				t.Errorf("Update() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error message to contain %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if updated.ID != tt.id {
				t.Errorf("Expected ID %d, got %d", tt.id, updated.ID)
			}

			expectedName := strings.TrimSpace(tt.newName)
			if updated.Name != expectedName {
				t.Errorf("Expected name %q, got %q", expectedName, updated.Name)
			}

			// Verify update persisted
			fetched, err := categoryService.GetByID(tt.id)
			if err != nil {
				t.Fatalf("Failed to fetch updated category: %v", err)
			}
			if fetched.Name != expectedName {
				t.Errorf("Updated name not persisted: expected %q, got %q", expectedName, fetched.Name)
			}
		})
	}
}

func TestCategoryService_Delete(t *testing.T) {
	db := SetupTestDB(t)
	categoryService := &CategoryService{DB: db}

	t.Run("delete unused category", func(t *testing.T) {
		// Create a category
		category, err := categoryService.Create("To Be Deleted")
		if err != nil {
			t.Fatalf("Failed to create test category: %v", err)
		}

		// Delete it
		err = categoryService.Delete(category.ID)
		if err != nil {
			t.Errorf("Delete() error = %v", err)
		}

		// Verify it's deleted
		_, err = categoryService.GetByID(category.ID)
		if err == nil {
			t.Error("Expected error when getting deleted category")
		}
	})

	t.Run("delete non-existent category", func(t *testing.T) {
		err := categoryService.Delete(999999)
		if err == nil {
			t.Error("Expected error when deleting non-existent category")
		}
	})

	t.Run("delete category in use", func(t *testing.T) {
		// Create a category, user, and post
		category, err := categoryService.Create("In Use Category")
		if err != nil {
			t.Fatalf("Failed to create category: %v", err)
		}
		t.Cleanup(func() {
			CleanupCategory(t, db, category.ID)
		})

		userID := SeedUser(t, db, "catdelete@example.com", "catdeleteuser", "password123", RoleEditor)
		t.Cleanup(func() {
			CleanupUser(t, db, userID)
		})

		postID := SeedPost(t, db, userID, category.ID, "Test Post", "Content", "test-delete-cat", true)
		t.Cleanup(func() {
			CleanupPost(t, db, postID)
		})

		// Associate category with post
		err = categoryService.AssignCategoriesToPost(postID, []int{category.ID})
		if err != nil {
			t.Fatalf("Failed to assign category to post: %v", err)
		}

		// Try to delete category that's in use
		err = categoryService.Delete(category.ID)
		if err == nil {
			t.Error("Expected error when deleting category in use")
		}
		if !strings.Contains(err.Error(), "cannot delete category") {
			t.Errorf("Expected 'cannot delete category' error, got: %v", err)
		}
	})
}

func TestCategoryService_GetCategoriesByPostID(t *testing.T) {
	db := SetupTestDB(t)
	categoryService := &CategoryService{DB: db}

	// Create test data
	userID := SeedUser(t, db, "postcats@example.com", "postcatsuser", "password123", RoleEditor)
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
	})

	cat1, _ := categoryService.Create("Category 1")
	cat2, _ := categoryService.Create("Category 2")
	cat3, _ := categoryService.Create("Category 3")
	t.Cleanup(func() {
		CleanupCategory(t, db, cat1.ID)
		CleanupCategory(t, db, cat2.ID)
		CleanupCategory(t, db, cat3.ID)
	})

	postID := SeedPost(t, db, userID, cat1.ID, "Test Post", "Content", "test-post-cats", true)
	t.Cleanup(func() {
		CleanupPost(t, db, postID)
	})

	// Assign multiple categories to the post
	err := categoryService.AssignCategoriesToPost(postID, []int{cat1.ID, cat2.ID})
	if err != nil {
		t.Fatalf("Failed to assign categories: %v", err)
	}

	// Get categories for the post
	categories, err := categoryService.GetCategoriesByPostID(postID)
	if err != nil {
		t.Fatalf("GetCategoriesByPostID() error = %v", err)
	}

	if len(categories) != 2 {
		t.Errorf("Expected 2 categories, got %d", len(categories))
	}

	// Verify correct categories are returned
	foundCat1 := false
	foundCat2 := false
	for _, cat := range categories {
		if cat.ID == cat1.ID {
			foundCat1 = true
		}
		if cat.ID == cat2.ID {
			foundCat2 = true
		}
		if cat.ID == cat3.ID {
			t.Error("Cat3 should not be associated with the post")
		}
	}

	if !foundCat1 {
		t.Error("Category 1 not found")
	}
	if !foundCat2 {
		t.Error("Category 2 not found")
	}
}

func TestCategoryService_AssignCategoriesToPost(t *testing.T) {
	db := SetupTestDB(t)
	categoryService := &CategoryService{DB: db}

	// Create test data
	userID := SeedUser(t, db, "assigncats@example.com", "assigncatsuser", "password123", RoleEditor)
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
	})

	cat1, _ := categoryService.Create("Assign Cat 1")
	cat2, _ := categoryService.Create("Assign Cat 2")
	cat3, _ := categoryService.Create("Assign Cat 3")
	t.Cleanup(func() {
		CleanupCategory(t, db, cat1.ID)
		CleanupCategory(t, db, cat2.ID)
		CleanupCategory(t, db, cat3.ID)
	})

	postID := SeedPost(t, db, userID, cat1.ID, "Assign Test", "Content", "test-assign-cats", true)
	t.Cleanup(func() {
		CleanupPost(t, db, postID)
	})

	t.Run("assign multiple categories", func(t *testing.T) {
		err := categoryService.AssignCategoriesToPost(postID, []int{cat1.ID, cat2.ID})
		if err != nil {
			t.Errorf("AssignCategoriesToPost() error = %v", err)
		}

		// Verify assignment
		categories, _ := categoryService.GetCategoriesByPostID(postID)
		if len(categories) != 2 {
			t.Errorf("Expected 2 categories, got %d", len(categories))
		}
	})

	t.Run("reassign categories (replace existing)", func(t *testing.T) {
		// Assign different set of categories
		err := categoryService.AssignCategoriesToPost(postID, []int{cat2.ID, cat3.ID})
		if err != nil {
			t.Errorf("AssignCategoriesToPost() error = %v", err)
		}

		// Verify old categories were removed and new ones added
		categories, _ := categoryService.GetCategoriesByPostID(postID)
		if len(categories) != 2 {
			t.Errorf("Expected 2 categories, got %d", len(categories))
		}

		foundCat1 := false
		for _, cat := range categories {
			if cat.ID == cat1.ID {
				foundCat1 = true
			}
		}
		if foundCat1 {
			t.Error("Cat1 should have been removed")
		}
	})

	t.Run("assign empty category list", func(t *testing.T) {
		err := categoryService.AssignCategoriesToPost(postID, []int{})
		if err == nil {
			t.Error("Expected error when assigning empty category list")
		}
	})
}

func TestCategoryService_GetPostCountByCategory(t *testing.T) {
	db := SetupTestDB(t)
	categoryService := &CategoryService{DB: db}

	// Create test data
	userID := SeedUser(t, db, "countcats@example.com", "countcatsuser", "password123", RoleEditor)
	t.Cleanup(func() {
		CleanupUser(t, db, userID)
	})

	cat1, _ := categoryService.Create("Count Cat 1")
	cat2, _ := categoryService.Create("Count Cat 2")
	cat3, _ := categoryService.Create("Count Cat 3")
	t.Cleanup(func() {
		CleanupCategory(t, db, cat1.ID)
		CleanupCategory(t, db, cat2.ID)
		CleanupCategory(t, db, cat3.ID)
	})

	// Create posts with different categories
	post1 := SeedPost(t, db, userID, cat1.ID, "Post 1", "Content", "count-post-1", true)
	post2 := SeedPost(t, db, userID, cat1.ID, "Post 2", "Content", "count-post-2", true)
	post3 := SeedPost(t, db, userID, cat2.ID, "Post 3", "Content", "count-post-3", true)
	t.Cleanup(func() {
		CleanupPost(t, db, post1)
		CleanupPost(t, db, post2)
		CleanupPost(t, db, post3)
	})

	// Assign categories
	categoryService.AssignCategoriesToPost(post1, []int{cat1.ID})
	categoryService.AssignCategoriesToPost(post2, []int{cat1.ID})
	categoryService.AssignCategoriesToPost(post3, []int{cat2.ID})

	// Get post counts
	counts, err := categoryService.GetPostCountByCategory()
	if err != nil {
		t.Fatalf("GetPostCountByCategory() error = %v", err)
	}

	// Cat1 should have 2 posts
	if counts[cat1.ID] != 2 {
		t.Errorf("Expected cat1 to have 2 posts, got %d", counts[cat1.ID])
	}

	// Cat2 should have 1 post
	if counts[cat2.ID] != 1 {
		t.Errorf("Expected cat2 to have 1 post, got %d", counts[cat2.ID])
	}

	// Cat3 should have 0 posts
	if counts[cat3.ID] != 0 {
		t.Errorf("Expected cat3 to have 0 posts, got %d", counts[cat3.ID])
	}
}

func TestCategoryService_NameTrimming(t *testing.T) {
	db := SetupTestDB(t)
	categoryService := &CategoryService{DB: db}

	// Create category with leading/trailing spaces
	category, err := categoryService.Create("  Trimmed Category  ")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}
	t.Cleanup(func() {
		CleanupCategory(t, db, category.ID)
	})

	// Verify name was trimmed
	if category.Name != "Trimmed Category" {
		t.Errorf("Expected trimmed name 'Trimmed Category', got %q", category.Name)
	}

	// Update with spaces
	updated, err := categoryService.Update(category.ID, "  Updated Name  ")
	if err != nil {
		t.Fatalf("Failed to update category: %v", err)
	}

	if updated.Name != "Updated Name" {
		t.Errorf("Expected trimmed name 'Updated Name', got %q", updated.Name)
	}
}
