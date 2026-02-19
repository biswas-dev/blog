package models

import (
	"testing"

	_ "github.com/lib/pq"
)

func TestAPITokenCreation(t *testing.T) {
	db := SetupTestDB(t)
	apiTokenService := &APITokenService{DB: db}

	// Always create a fresh test user for this test
	userService := &UserService{DB: db}
	user, err := userService.Create("test-apitoken@example.com", "testuser", "password123", RoleCommenter)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	t.Cleanup(func() {
		CleanupUser(t, db, user.UserID)
	})

	// Use the created user's ID for the test
	testAPITokenCreationForUser(t, apiTokenService, user.UserID)
}

func testAPITokenCreationForUser(t *testing.T, apiTokenService *APITokenService, userID int) {
	t.Helper()

	// Test creating API token
	tokenName := "test-token-integration"
	t.Logf("Creating API token '%s' for user ID %d...", tokenName, userID)

	token, err := apiTokenService.Create(userID, tokenName, nil)
	if err != nil {
		t.Fatalf("Failed to create API token: %v", err)
	}

	// Verify token was created
	if token == nil {
		t.Fatal("Token is nil")
	}

	if token.ID == 0 {
		t.Error("Token ID should not be 0")
	}

	if token.Name != tokenName {
		t.Errorf("Expected token name '%s', got '%s'", tokenName, token.Name)
	}

	if token.UserID != userID {
		t.Errorf("Expected user ID %d, got %d", userID, token.UserID)
	}

	if token.Token == "" {
		t.Error("Token string should not be empty")
	}

	if !token.IsActive {
		t.Error("Token should be active")
	}

	t.Logf("✅ API Token created successfully!")
	t.Logf("   Token ID: %d", token.ID)
	t.Logf("   Token Name: %s", token.Name)
	t.Logf("   Token Length: %d characters", len(token.Token))
	t.Logf("   Created At: %s", token.CreatedAt)

	// Clean up: Delete the test token
	err = apiTokenService.Delete(token.ID, userID)
	if err != nil {
		t.Errorf("Failed to clean up test token: %v", err)
	}
}

func TestAPITokenValidation(t *testing.T) {
	db := SetupTestDB(t)
	apiTokenService := &APITokenService{DB: db}

	// Create a test user
	userService := &UserService{DB: db}
	user, err := userService.Create("test-validation@example.com", "validationuser", "password123", RoleCommenter)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	t.Cleanup(func() {
		CleanupUser(t, db, user.UserID)
	})

	// Create a test token
	tokenName := "test-validation-token"
	token, err := apiTokenService.Create(user.UserID, tokenName, nil)
	if err != nil {
		t.Fatalf("Failed to create test token: %v", err)
	}
	t.Cleanup(func() {
		apiTokenService.Delete(token.ID, user.UserID)
	})

	// Test validation with the token
	validatedUser, err := apiTokenService.ValidateToken(token.Token)
	if err != nil {
		t.Errorf("Failed to validate token: %v", err)
	}

	if validatedUser == nil {
		t.Error("User should not be nil for valid token")
	} else {
		if validatedUser.UserID != user.UserID {
			t.Errorf("Expected user ID %d, got %d", user.UserID, validatedUser.UserID)
		}
	}

	// Test validation with invalid token
	_, err = apiTokenService.ValidateToken("invalid-token-12345")
	if err == nil {
		t.Error("Should fail to validate invalid token")
	}

	t.Log("✅ API Token validation test completed successfully!")
}

func TestAPITokenRevoke(t *testing.T) {
	db := SetupTestDB(t)
	apiTokenService := &APITokenService{DB: db}

	// Create a test user
	userService := &UserService{DB: db}
	user, err := userService.Create("test-revoke@example.com", "revokeuser", "password123", RoleCommenter)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	t.Cleanup(func() {
		CleanupUser(t, db, user.UserID)
	})

	// Create a test token
	token, err := apiTokenService.Create(user.UserID, "test-revoke-token", nil)
	if err != nil {
		t.Fatalf("Failed to create test token: %v", err)
	}
	t.Cleanup(func() {
		apiTokenService.Delete(token.ID, user.UserID)
	})

	// Revoke the token
	err = apiTokenService.Revoke(token.ID, user.UserID)
	if err != nil {
		t.Errorf("Failed to revoke token: %v", err)
	}

	// Verify token can no longer be validated
	_, err = apiTokenService.ValidateToken(token.Token)
	if err == nil {
		t.Error("Should not validate revoked token")
	}

	t.Log("✅ API Token revoke test completed successfully!")
}

func TestAPITokenGetByUser(t *testing.T) {
	db := SetupTestDB(t)
	apiTokenService := &APITokenService{DB: db}

	// Create a test user
	userService := &UserService{DB: db}
	user, err := userService.Create("test-getbyuser@example.com", "getbyusertest", "password123", RoleCommenter)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	t.Cleanup(func() {
		CleanupUser(t, db, user.UserID)
	})

	// Create multiple test tokens
	token1, err := apiTokenService.Create(user.UserID, "token-1", nil)
	if err != nil {
		t.Fatalf("Failed to create token 1: %v", err)
	}
	t.Cleanup(func() {
		apiTokenService.Delete(token1.ID, user.UserID)
	})

	token2, err := apiTokenService.Create(user.UserID, "token-2", nil)
	if err != nil {
		t.Fatalf("Failed to create token 2: %v", err)
	}
	t.Cleanup(func() {
		apiTokenService.Delete(token2.ID, user.UserID)
	})

	// Get tokens for user
	tokens, err := apiTokenService.GetByUser(user.UserID)
	if err != nil {
		t.Fatalf("Failed to get tokens by user: %v", err)
	}

	if len(tokens) < 2 {
		t.Errorf("Expected at least 2 tokens, got %d", len(tokens))
	}

	t.Log("✅ API Token GetByUser test completed successfully!")
}
