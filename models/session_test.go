package models

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestSessionService_GenerateHashedToken(t *testing.T) {
	ss := &SessionService{}

	t.Run("generates valid hash", func(t *testing.T) {
		token := "test-token-123"
		hash, err := ss.GenerateHashedToken(token)
		if err != nil {
			t.Fatalf("GenerateHashedToken returned error: %v", err)
		}
		if hash == "" {
			t.Fatal("expected non-empty hash")
		}

		// Verify hash matches token
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(token)); err != nil {
			t.Fatalf("hashed token does not verify against original: %v", err)
		}
	})

	t.Run("generates salted hashes", func(t *testing.T) {
		token := "same-token"
		h1, err := ss.GenerateHashedToken(token)
		if err != nil {
			t.Fatal(err)
		}
		h2, err := ss.GenerateHashedToken(token)
		if err != nil {
			t.Fatal(err)
		}
		if h1 == h2 {
			t.Fatal("expected different hashes for same token due to salting")
		}

		// Both hashes should verify against the same token
		if err := bcrypt.CompareHashAndPassword([]byte(h1), []byte(token)); err != nil {
			t.Errorf("first hash doesn't verify: %v", err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(h2), []byte(token)); err != nil {
			t.Errorf("second hash doesn't verify: %v", err)
		}
	})
}

func TestSessionService_Create(t *testing.T) {
	db := SetupTestDB(t)
	sessionService := &SessionService{DB: db}

	// Create a test user
	userService := &UserService{DB: db}
	user, err := userService.Create("session@example.com", "sessionuser", "password123", RoleCommenter)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	t.Cleanup(func() {
		CleanupUser(t, db, user.UserID)
	})

	t.Run("create new session", func(t *testing.T) {
		session, err := sessionService.Create(user.UserID)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if session == nil {
			t.Fatal("Expected non-nil session")
		}

		if session.ID == 0 {
			t.Error("Expected non-zero session ID")
		}

		if session.Token == "" {
			t.Error("Expected non-empty token")
		}

		if session.TokenHash == "" {
			t.Error("Expected non-empty token hash")
		}

		// Verify token and hash match
		err = bcrypt.CompareHashAndPassword([]byte(session.TokenHash), []byte(session.Token))
		if err != nil {
			t.Errorf("Token hash does not match token: %v", err)
		}

		// Verify token is not the same as hash
		if session.Token == session.TokenHash {
			t.Error("Token should not be the same as token hash")
		}

		// Cleanup
		sessionService.Logout(user.Email)
	})

	t.Run("update existing session", func(t *testing.T) {
		// Create first session
		session1, err := sessionService.Create(user.UserID)
		if err != nil {
			t.Fatalf("Failed to create first session: %v", err)
		}
		firstToken := session1.Token

		// Create second session for same user (should update)
		session2, err := sessionService.Create(user.UserID)
		if err != nil {
			t.Fatalf("Failed to create second session: %v", err)
		}

		// Token should be different
		if session2.Token == firstToken {
			t.Error("Expected different token when updating session")
		}

		// Cleanup
		sessionService.Logout(user.Email)
	})
}

func TestSessionService_GetSession(t *testing.T) {
	db := SetupTestDB(t)
	sessionService := &SessionService{DB: db}

	// Create a test user
	userService := &UserService{DB: db}
	user, err := userService.Create("getsession@example.com", "getsessionuser", "password123", RoleCommenter)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	t.Cleanup(func() {
		CleanupUser(t, db, user.UserID)
	})

	t.Run("get non-existent session", func(t *testing.T) {
		// Try to get session for user without session
		sessionID, err := sessionService.GetSession(user.UserID)
		if err == nil {
			t.Error("Expected error when getting non-existent session")
		}
		if sessionID != 0 {
			t.Errorf("Expected session ID 0 for non-existent session, got %d", sessionID)
		}
	})

	t.Run("get existing session", func(t *testing.T) {
		// Create a session
		session, err := sessionService.Create(user.UserID)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		t.Cleanup(func() {
			sessionService.Logout(user.Email)
		})

		// Get the session
		sessionID, err := sessionService.GetSession(user.UserID)
		if err != nil {
			t.Errorf("GetSession() error = %v", err)
		}

		if sessionID != session.ID {
			t.Errorf("Expected session ID %d, got %d", session.ID, sessionID)
		}
	})
}

func TestSessionService_User(t *testing.T) {
	db := SetupTestDB(t)
	sessionService := &SessionService{DB: db}

	// Create a test user
	userService := &UserService{DB: db}
	user, err := userService.Create("usertest@example.com", "usertestname", "password123", RoleEditor)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	t.Cleanup(func() {
		CleanupUser(t, db, user.UserID)
	})

	// Create a session for the user
	session, err := sessionService.Create(user.UserID)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	t.Cleanup(func() {
		sessionService.Logout(user.Email)
	})

	tests := []struct {
		name    string
		token   string
		email   string
		wantErr bool
	}{
		{
			name:    "valid token and email",
			token:   session.Token,
			email:   user.Email,
			wantErr: false,
		},
		{
			name:    "valid token and uppercase email",
			token:   session.Token,
			email:   "USERTEST@EXAMPLE.COM",
			wantErr: false, // Should work because email is normalized
		},
		{
			name:    "invalid token",
			token:   "invalid-token-12345",
			email:   user.Email,
			wantErr: true,
		},
		{
			name:    "invalid email",
			token:   session.Token,
			email:   "nonexistent@example.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retrievedUser, err := sessionService.User(tt.token, tt.email)

			if (err != nil) != tt.wantErr {
				t.Errorf("User() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if retrievedUser.UserID != user.UserID {
					t.Errorf("Expected user ID %d, got %d", user.UserID, retrievedUser.UserID)
				}
				if retrievedUser.Username != user.Username {
					t.Errorf("Expected username %s, got %s", user.Username, retrievedUser.Username)
				}
				if retrievedUser.Role != user.Role {
					t.Errorf("Expected role %d, got %d", user.Role, retrievedUser.Role)
				}
			}
		})
	}
}

func TestSessionService_Logout(t *testing.T) {
	db := SetupTestDB(t)
	sessionService := &SessionService{DB: db}

	// Create a test user
	userService := &UserService{DB: db}
	user, err := userService.Create("logout@example.com", "logoutuser", "password123", RoleCommenter)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	t.Cleanup(func() {
		CleanupUser(t, db, user.UserID)
	})

	// Create a session
	session, err := sessionService.Create(user.UserID)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Verify session exists
	_, err = sessionService.User(session.Token, user.Email)
	if err != nil {
		t.Fatalf("Session should exist before logout: %v", err)
	}

	// Logout
	sessionService.Logout(user.Email)

	// Verify session no longer works
	_, err = sessionService.User(session.Token, user.Email)
	if err == nil {
		t.Error("Session should not work after logout")
	}

	// Verify session was deleted from database
	sessionID, err := sessionService.GetSession(user.UserID)
	if err == nil {
		t.Errorf("Session should not exist after logout, but found session ID %d", sessionID)
	}
}

func TestSessionService_LogoutWithUppercaseEmail(t *testing.T) {
	db := SetupTestDB(t)
	sessionService := &SessionService{DB: db}

	// Create a test user
	userService := &UserService{DB: db}
	user, err := userService.Create("uppercaselogout@example.com", "uppercaselogout", "password123", RoleCommenter)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	t.Cleanup(func() {
		CleanupUser(t, db, user.UserID)
	})

	// Create a session
	session, err := sessionService.Create(user.UserID)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Logout with uppercase email
	sessionService.Logout("UPPERCASELOGOUT@EXAMPLE.COM")

	// Verify session was deleted
	_, err = sessionService.User(session.Token, user.Email)
	if err == nil {
		t.Error("Session should not work after logout with uppercase email")
	}
}

func TestSessionService_TokenSecurity(t *testing.T) {
	db := SetupTestDB(t)
	sessionService := &SessionService{DB: db}

	// Create a test user
	userService := &UserService{DB: db}
	user, err := userService.Create("security@example.com", "securityuser", "password123", RoleCommenter)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	t.Cleanup(func() {
		CleanupUser(t, db, user.UserID)
		sessionService.Logout(user.Email)
	})

	// Create a session
	session, err := sessionService.Create(user.UserID)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Verify raw token is not stored in database
	var storedTokenHash string
	err = db.QueryRow("SELECT token_hash FROM sessions WHERE user_id = $1", user.UserID).Scan(&storedTokenHash)
	if err != nil {
		t.Fatalf("Failed to query session: %v", err)
	}

	if storedTokenHash == session.Token {
		t.Error("Raw token should not be stored in database")
	}

	// Verify stored value is a bcrypt hash
	if len(storedTokenHash) != 60 {
		t.Errorf("Expected bcrypt hash length of 60, got %d", len(storedTokenHash))
	}

	// Verify hash can be used to validate token
	err = bcrypt.CompareHashAndPassword([]byte(storedTokenHash), []byte(session.Token))
	if err != nil {
		t.Errorf("Stored hash should validate against token: %v", err)
	}
}

func TestSessionService_MultipleUsers(t *testing.T) {
	db := SetupTestDB(t)
	sessionService := &SessionService{DB: db}
	userService := &UserService{DB: db}

	// Create two users
	user1, err := userService.Create("multiuser1@example.com", "multiuser1", "password123", RoleCommenter)
	if err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}
	t.Cleanup(func() {
		CleanupUser(t, db, user1.UserID)
		sessionService.Logout(user1.Email)
	})

	user2, err := userService.Create("multiuser2@example.com", "multiuser2", "password123", RoleCommenter)
	if err != nil {
		t.Fatalf("Failed to create user2: %v", err)
	}
	t.Cleanup(func() {
		CleanupUser(t, db, user2.UserID)
		sessionService.Logout(user2.Email)
	})

	// Create sessions for both users
	session1, err := sessionService.Create(user1.UserID)
	if err != nil {
		t.Fatalf("Failed to create session1: %v", err)
	}

	session2, err := sessionService.Create(user2.UserID)
	if err != nil {
		t.Fatalf("Failed to create session2: %v", err)
	}

	// Verify tokens are different
	if session1.Token == session2.Token {
		t.Error("Different users should have different session tokens")
	}

	// Verify each token only works for its own user
	_, err = sessionService.User(session1.Token, user2.Email)
	if err == nil {
		t.Error("User1's token should not work for user2")
	}

	_, err = sessionService.User(session2.Token, user1.Email)
	if err == nil {
		t.Error("User2's token should not work for user1")
	}

	// Verify each token works for its own user
	retrievedUser1, err := sessionService.User(session1.Token, user1.Email)
	if err != nil {
		t.Errorf("User1's token should work for user1: %v", err)
	}
	if retrievedUser1 != nil && retrievedUser1.UserID != user1.UserID {
		t.Error("Retrieved wrong user for user1's token")
	}

	retrievedUser2, err := sessionService.User(session2.Token, user2.Email)
	if err != nil {
		t.Errorf("User2's token should work for user2: %v", err)
	}
	if retrievedUser2 != nil && retrievedUser2.UserID != user2.UserID {
		t.Error("Retrieved wrong user for user2's token")
	}
}
