package models

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestUserService_Create(t *testing.T) {
	db := SetupTestDB(t)
	userService := &UserService{DB: db}

	tests := []struct {
		name     string
		email    string
		username string
		password string
		roleID   int
		wantErr  bool
	}{
		{
			name:     "create valid user",
			email:    "newuser@example.com",
			username: "newuser",
			password: "password123",
			roleID:   RoleCommenter,
			wantErr:  false,
		},
		{
			name:     "create user with uppercase email",
			email:    "UPPERCASE@EXAMPLE.COM",
			username: "uppercaseuser",
			password: "password123",
			roleID:   RoleEditor,
			wantErr:  false,
		},
		{
			name:     "create admin user",
			email:    "admin@example.com",
			username: "adminuser",
			password: "securepassword",
			roleID:   RoleAdministrator,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := userService.Create(tt.email, tt.username, tt.password, tt.roleID)

			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				// Cleanup
				t.Cleanup(func() {
					CleanupUser(t, db, user.UserID)
				})

				if user.UserID == 0 {
					t.Error("Expected non-zero user ID")
				}

				// Email should be lowercased
				if user.Email != strings.ToLower(tt.email) {
					t.Errorf("Expected email to be lowercased: %s, got %s", strings.ToLower(tt.email), user.Email)
				}

				if user.Username != tt.username {
					t.Errorf("Expected username %s, got %s", tt.username, user.Username)
				}

				if user.Role != tt.roleID {
					t.Errorf("Expected role %d, got %d", tt.roleID, user.Role)
				}

				// Verify password was hashed
				if user.PasswordHash == "" {
					t.Error("Expected non-empty password hash")
				}

				// Verify password hash is valid bcrypt hash
				err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(tt.password))
				if err != nil {
					t.Errorf("Password hash does not match original password: %v", err)
				}

				// Verify the plaintext password is NOT stored
				if strings.Contains(user.PasswordHash, tt.password) {
					t.Error("Password hash should not contain plaintext password")
				}
			}
		})
	}
}

func TestUserService_CreateDuplicateEmail(t *testing.T) {
	db := SetupTestDB(t)
	userService := &UserService{DB: db}

	// Create first user
	email := "duplicate@example.com"
	user1, err := userService.Create(email, "user1", "password123", RoleCommenter)
	if err != nil {
		t.Fatalf("Failed to create first user: %v", err)
	}
	t.Cleanup(func() {
		CleanupUser(t, db, user1.UserID)
	})

	// Attempt to create second user with same email
	_, err = userService.Create(email, "user2", "password456", RoleCommenter)
	if err == nil {
		t.Error("Expected error when creating user with duplicate email")
	}
}

func TestUserService_Authenticate(t *testing.T) {
	db := SetupTestDB(t)
	userService := &UserService{DB: db}

	// Create a test user
	email := "authtest@example.com"
	password := "correctpassword"
	user, err := userService.Create(email, "authuser", password, RoleEditor)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	t.Cleanup(func() {
		CleanupUser(t, db, user.UserID)
	})

	tests := []struct {
		name     string
		email    string
		password string
		wantErr  bool
	}{
		{
			name:     "authenticate with correct password",
			email:    email,
			password: password,
			wantErr:  false,
		},
		{
			name:     "authenticate with incorrect password",
			email:    email,
			password: "wrongpassword",
			wantErr:  true,
		},
		{
			name:     "authenticate with non-existent email",
			email:    "nonexistent@example.com",
			password: password,
			wantErr:  true,
		},
		{
			name:     "authenticate with uppercase email",
			email:    strings.ToUpper(email),
			password: password,
			wantErr:  false, // Should work because email is normalized
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authenticatedUser, err := userService.Authenticate(tt.email, tt.password)

			if (err != nil) != tt.wantErr {
				t.Errorf("Authenticate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if authenticatedUser.UserID != user.UserID {
					t.Errorf("Expected user ID %d, got %d", user.UserID, authenticatedUser.UserID)
				}
				if authenticatedUser.Email != strings.ToLower(email) {
					t.Errorf("Expected email %s, got %s", strings.ToLower(email), authenticatedUser.Email)
				}
				if authenticatedUser.Role != RoleEditor {
					t.Errorf("Expected role %d, got %d", RoleEditor, authenticatedUser.Role)
				}
			}
		})
	}
}

func TestUserService_UpdatePassword(t *testing.T) {
	db := SetupTestDB(t)
	userService := &UserService{DB: db}

	// Create a test user
	user, err := userService.Create("updatepw@example.com", "updatepwuser", "oldpassword", RoleCommenter)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	t.Cleanup(func() {
		CleanupUser(t, db, user.UserID)
	})

	// Update password
	newPassword := "newpassword123"
	err = userService.UpdatePassword(user.UserID, newPassword)
	if err != nil {
		t.Fatalf("UpdatePassword() error = %v", err)
	}

	// Verify old password no longer works
	_, err = userService.Authenticate(user.Email, "oldpassword")
	if err == nil {
		t.Error("Old password should not work after update")
	}

	// Verify new password works
	authenticatedUser, err := userService.Authenticate(user.Email, newPassword)
	if err != nil {
		t.Errorf("New password should work after update: %v", err)
	}
	if authenticatedUser != nil && authenticatedUser.UserID != user.UserID {
		t.Errorf("Expected user ID %d, got %d", user.UserID, authenticatedUser.UserID)
	}
}

func TestUserService_UpdateEmail(t *testing.T) {
	db := SetupTestDB(t)
	userService := &UserService{DB: db}

	// Create a test user
	user, err := userService.Create("oldemail@example.com", "updateemailuser", "password123", RoleCommenter)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	t.Cleanup(func() {
		CleanupUser(t, db, user.UserID)
	})

	tests := []struct {
		name     string
		newEmail string
		wantErr  bool
	}{
		{
			name:     "update to new email",
			newEmail: "newemail@example.com",
			wantErr:  false,
		},
		{
			name:     "update to uppercase email",
			newEmail: "UPPERCASE@EXAMPLE.COM",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := userService.UpdateEmail(user.UserID, tt.newEmail)

			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateEmail() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				// Verify email was updated and lowercased
				var updatedEmail string
				err = db.QueryRow("SELECT email FROM users WHERE user_id = $1", user.UserID).Scan(&updatedEmail)
				if err != nil {
					t.Fatalf("Failed to query updated email: %v", err)
				}

				expectedEmail := strings.ToLower(tt.newEmail)
				if updatedEmail != expectedEmail {
					t.Errorf("Expected email %s, got %s", expectedEmail, updatedEmail)
				}

				// Verify old email no longer works for authentication
				_, err = userService.Authenticate("oldemail@example.com", "password123")
				if err == nil {
					t.Error("Old email should not work after update")
				}

				// Verify new email works for authentication
				authenticatedUser, err := userService.Authenticate(tt.newEmail, "password123")
				if err != nil {
					t.Errorf("New email should work for authentication: %v", err)
				}
				if authenticatedUser != nil && authenticatedUser.UserID != user.UserID {
					t.Errorf("Expected user ID %d, got %d", user.UserID, authenticatedUser.UserID)
				}
			}
		})
	}
}

func TestUserService_GetAllUsers(t *testing.T) {
	db := SetupTestDB(t)
	userService := &UserService{DB: db}

	// Create multiple test users
	user1, err := userService.Create("user1@example.com", "user1", "password123", RoleCommenter)
	if err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}
	t.Cleanup(func() {
		CleanupUser(t, db, user1.UserID)
	})

	user2, err := userService.Create("user2@example.com", "user2", "password123", RoleEditor)
	if err != nil {
		t.Fatalf("Failed to create user2: %v", err)
	}
	t.Cleanup(func() {
		CleanupUser(t, db, user2.UserID)
	})

	user3, err := userService.Create("user3@example.com", "user3", "password123", RoleAdministrator)
	if err != nil {
		t.Fatalf("Failed to create user3: %v", err)
	}
	t.Cleanup(func() {
		CleanupUser(t, db, user3.UserID)
	})

	// Get all users
	users, err := userService.GetAllUsers()
	if err != nil {
		t.Fatalf("GetAllUsers() error = %v", err)
	}

	if len(users) < 3 {
		t.Errorf("Expected at least 3 users, got %d", len(users))
	}

	// Verify our created users are in the list
	foundUsers := make(map[int]bool)
	for _, u := range users {
		if u.UserID == user1.UserID || u.UserID == user2.UserID || u.UserID == user3.UserID {
			foundUsers[u.UserID] = true
		}
	}

	if !foundUsers[user1.UserID] {
		t.Error("user1 not found in GetAllUsers()")
	}
	if !foundUsers[user2.UserID] {
		t.Error("user2 not found in GetAllUsers()")
	}
	if !foundUsers[user3.UserID] {
		t.Error("user3 not found in GetAllUsers()")
	}
}

func TestUserService_GenerateHashedToken(t *testing.T) {
	userService := &UserService{}

	token := "test-token-12345"
	hash, err := userService.GenerateHashedToken(token)
	if err != nil {
		t.Fatalf("GenerateHashedToken() error = %v", err)
	}

	if hash == "" {
		t.Error("Expected non-empty hash")
	}

	// Verify hash is different from token
	if hash == token {
		t.Error("Hash should be different from original token")
	}

	// Verify hash is valid bcrypt hash
	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(token))
	if err != nil {
		t.Errorf("Hash does not match original token: %v", err)
	}

	// Verify same token produces different hashes (salted)
	hash2, err := userService.GenerateHashedToken(token)
	if err != nil {
		t.Fatalf("GenerateHashedToken() error = %v", err)
	}

	if hash == hash2 {
		t.Error("Expected different hashes for same token (should be salted)")
	}
}

func TestUserService_PasswordSecurity(t *testing.T) {
	db := SetupTestDB(t)
	userService := &UserService{DB: db}

	// Create user with a password
	password := "mySecureP@ssw0rd"
	user, err := userService.Create("security@example.com", "securityuser", password, RoleCommenter)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	t.Cleanup(func() {
		CleanupUser(t, db, user.UserID)
	})

	// Verify password is never stored in plaintext
	var storedPassword string
	err = db.QueryRow("SELECT password FROM users WHERE user_id = $1", user.UserID).Scan(&storedPassword)
	if err != nil {
		t.Fatalf("Failed to query password: %v", err)
	}

	if storedPassword == password {
		t.Error("Password should not be stored in plaintext")
	}

	// Verify it starts with bcrypt identifier
	if !strings.HasPrefix(storedPassword, "$2a$") && !strings.HasPrefix(storedPassword, "$2b$") {
		t.Errorf("Password hash should use bcrypt format, got: %s", storedPassword[:10])
	}

	// Verify password hash length is reasonable (bcrypt hashes are 60 characters)
	if len(storedPassword) != 60 {
		t.Errorf("Expected bcrypt hash length of 60, got %d", len(storedPassword))
	}
}

func TestUserService_EmailNormalization(t *testing.T) {
	db := SetupTestDB(t)
	userService := &UserService{DB: db}

	tests := []struct {
		name          string
		inputEmail    string
		expectedEmail string
	}{
		{
			name:          "lowercase email",
			inputEmail:    "test@example.com",
			expectedEmail: "test@example.com",
		},
		{
			name:          "uppercase email",
			inputEmail:    "TEST@EXAMPLE.COM",
			expectedEmail: "test@example.com",
		},
		{
			name:          "mixed case email",
			inputEmail:    "TeSt@ExAmPlE.CoM",
			expectedEmail: "test@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := userService.Create(tt.inputEmail, "emailnormuser-"+tt.name, "password123", RoleCommenter)
			if err != nil {
				t.Fatalf("Failed to create user: %v", err)
			}
			t.Cleanup(func() {
				CleanupUser(t, db, user.UserID)
			})

			if user.Email != tt.expectedEmail {
				t.Errorf("Expected email %s, got %s", tt.expectedEmail, user.Email)
			}

			// Verify authentication works with any case variation
			_, err = userService.Authenticate(tt.inputEmail, "password123")
			if err != nil {
				t.Errorf("Authentication should work with input email case: %v", err)
			}

			_, err = userService.Authenticate(strings.ToUpper(tt.inputEmail), "password123")
			if err != nil {
				t.Errorf("Authentication should work with uppercase email: %v", err)
			}

			_, err = userService.Authenticate(strings.ToLower(tt.inputEmail), "password123")
			if err != nil {
				t.Errorf("Authentication should work with lowercase email: %v", err)
			}
		})
	}
}
