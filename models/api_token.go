package models

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// tokenCacheEntry caches a validated token -> user mapping with expiry.
type tokenCacheEntry struct {
	user      *User
	tokenID   int
	expiresAt time.Time
}

// tokenCache avoids re-scanning all tokens + bcrypt on every API request.
// Entries expire after 5 minutes.
var tokenCache sync.Map // map[string]*tokenCacheEntry

const tokenCacheTTL = 5 * time.Minute

type APIToken struct {
	ID          int    `json:"id"`
	UserID      int    `json:"user_id"`
	Name        string `json:"name"`
	Token       string `json:"token,omitempty"` // Only returned when creating
	TokenHash   string `json:"-"`              // Never returned in JSON
	CreatedAt   string `json:"created_at"`
	LastUsedAt  *string `json:"last_used_at,omitempty"`
	ExpiresAt   *string `json:"expires_at,omitempty"`
	IsActive    bool   `json:"is_active"`
}

type APITokenService struct {
	DB *sql.DB
}

func (ats *APITokenService) Create(userID int, name string, expiresAt *time.Time) (*APIToken, error) {
	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	
	token := hex.EncodeToString(tokenBytes)
	
	// Hash the token for storage
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash token: %w", err)
	}
	
	tokenHash := string(hashedBytes)
	now := time.Now().UTC()
	
	var tokenID int
	var expiresAtStr *string
	if expiresAt != nil {
		expiresAtFormatted := expiresAt.UTC().Format(time.RFC3339)
		expiresAtStr = &expiresAtFormatted
	}
	
	query := `
		INSERT INTO api_tokens (user_id, name, token_hash, created_at, expires_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	
	err = ats.DB.QueryRow(query, userID, name, tokenHash, now, expiresAtStr, true).Scan(&tokenID)
	if err != nil {
		return nil, fmt.Errorf("failed to create API token: %w", err)
	}
	
	return &APIToken{
		ID:        tokenID,
		UserID:    userID,
		Name:      name,
		Token:     token, // Only return the raw token when creating
		CreatedAt: now.Format(time.RFC3339),
		ExpiresAt: expiresAtStr,
		IsActive:  true,
	}, nil
}

func (ats *APITokenService) GetByUser(userID int) ([]*APIToken, error) {
	query := `
		SELECT id, user_id, name, created_at, last_used_at, expires_at, is_active
		FROM api_tokens
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	
	rows, err := ats.DB.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API tokens: %w", err)
	}
	defer rows.Close()
	
	var tokens []*APIToken
	for rows.Next() {
		token := &APIToken{}
		err := rows.Scan(&token.ID, &token.UserID, &token.Name, &token.CreatedAt, 
			&token.LastUsedAt, &token.ExpiresAt, &token.IsActive)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API token: %w", err)
		}
		tokens = append(tokens, token)
	}
	
	return tokens, nil
}

func (ats *APITokenService) ValidateToken(token string) (*User, error) {
	// Check cache first to avoid expensive DB scan + bcrypt on every request
	if entry, ok := tokenCache.Load(token); ok {
		cached := entry.(*tokenCacheEntry)
		if time.Now().Before(cached.expiresAt) {
			// Update last_used asynchronously
			go ats.updateLastUsed(cached.tokenID)
			return cached.user, nil
		}
		tokenCache.Delete(token) // expired
	}

	query := `
		SELECT at.id, at.user_id, at.token_hash, at.expires_at, at.is_active,
		       u.user_id, u.username, u.email, u.role_id
		FROM api_tokens at
		JOIN users u ON at.user_id = u.user_id
		WHERE at.is_active = true
	`

	rows, err := ats.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query API tokens: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tokenID, userID, userRole int
		var tokenHash, username, email string
		var expiresAt *string
		var isActive bool

		err := rows.Scan(&tokenID, &userID, &tokenHash, &expiresAt, &isActive,
			&userID, &username, &email, &userRole)
		if err != nil {
			continue
		}

		// Check if token is expired
		if expiresAt != nil {
			expiry, err := time.Parse(time.RFC3339, *expiresAt)
			if err == nil && time.Now().UTC().After(expiry) {
				continue
			}
		}

		// Compare the provided token with the hash
		err = bcrypt.CompareHashAndPassword([]byte(tokenHash), []byte(token))
		if err == nil {
			user := &User{
				UserID:   userID,
				Username: username,
				Email:    email,
				Role:     userRole,
			}

			// Cache the validated token
			tokenCache.Store(token, &tokenCacheEntry{
				user:      user,
				tokenID:   tokenID,
				expiresAt: time.Now().Add(tokenCacheTTL),
			})

			ats.updateLastUsed(tokenID)
			return user, nil
		}
	}

	return nil, fmt.Errorf("invalid API token")
}

func (ats *APITokenService) updateLastUsed(tokenID int) {
	query := `UPDATE api_tokens SET last_used_at = $1 WHERE id = $2`
	ats.DB.Exec(query, time.Now().UTC(), tokenID)
}

func (ats *APITokenService) Revoke(tokenID int, userID int) error {
	// Clear cache entries for this token
	tokenCache.Range(func(key, value interface{}) bool {
		if entry, ok := value.(*tokenCacheEntry); ok && entry.tokenID == tokenID {
			tokenCache.Delete(key)
		}
		return true
	})

	query := `UPDATE api_tokens SET is_active = false WHERE id = $1 AND user_id = $2`
	result, err := ats.DB.Exec(query, tokenID, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("token not found or already revoked")
	}
	
	return nil
}

func (ats *APITokenService) Delete(tokenID int, userID int) error {
	query := `DELETE FROM api_tokens WHERE id = $1 AND user_id = $2`
	result, err := ats.DB.Exec(query, tokenID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete token: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("token not found")
	}
	
	return nil
}