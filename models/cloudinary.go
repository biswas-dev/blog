package models

import (
	"crypto/sha1"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"anshumanbiswas.com/blog/internal/crypto"
)

// CloudinarySettings represents the stored Cloudinary configuration
type CloudinarySettings struct {
	CloudName           string     `json:"cloud_name"`
	APIKey              string     `json:"api_key"`
	APISecret           string     `json:"api_secret,omitempty"`
	IsEnabled           bool       `json:"is_enabled"`
	Status              string     `json:"status"`
	LastCheckedAt       *time.Time `json:"last_checked_at,omitempty"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// CloudinaryService handles CRUD for Cloudinary settings
type CloudinaryService struct {
	DB *sql.DB
}

// Get reads settings and decrypts the API secret. Returns nil, nil if not configured.
func (cs *CloudinaryService) Get() (*CloudinarySettings, error) {
	var s CloudinarySettings
	var secretEnc, secretNonce []byte

	query := `
		SELECT cloud_name, api_key, api_secret_encrypted, api_secret_nonce,
		       is_enabled, COALESCE(status, 'unknown'), last_checked_at,
		       COALESCE(consecutive_failures, 0), created_at, updated_at
		FROM cloudinary_settings WHERE id = 1`

	err := cs.DB.QueryRow(query).Scan(
		&s.CloudName, &s.APIKey, &secretEnc, &secretNonce,
		&s.IsEnabled, &s.Status, &s.LastCheckedAt,
		&s.ConsecutiveFailures, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get cloudinary settings: %w", err)
	}

	// Decrypt API secret
	if len(secretEnc) > 0 && len(secretNonce) > 0 {
		key, err := crypto.GetEncryptionKey()
		if err != nil {
			return nil, fmt.Errorf("get encryption key: %w", err)
		}
		plaintext, err := crypto.Decrypt(secretEnc, secretNonce, key)
		if err != nil {
			return nil, fmt.Errorf("decrypt api secret: %w", err)
		}
		s.APISecret = string(plaintext)
	}

	return &s, nil
}

// Save performs an UPSERT of Cloudinary settings. If apiSecret is empty on update, keeps existing.
func (cs *CloudinaryService) Save(cloudName, apiKey, apiSecret string) error {
	key, err := crypto.GetEncryptionKey()
	if err != nil {
		return fmt.Errorf("get encryption key: %w", err)
	}

	// If apiSecret is empty, check if we're updating and should keep existing
	if apiSecret == "" {
		existing, err := cs.Get()
		if err != nil {
			return fmt.Errorf("check existing settings: %w", err)
		}
		if existing != nil {
			apiSecret = existing.APISecret
		}
		if apiSecret == "" {
			return fmt.Errorf("API secret is required")
		}
	}

	secretEnc, secretNonce, err := crypto.Encrypt([]byte(apiSecret), key)
	if err != nil {
		return fmt.Errorf("encrypt api secret: %w", err)
	}

	query := `
		INSERT INTO cloudinary_settings (id, cloud_name, api_key, api_secret_encrypted, api_secret_nonce)
		VALUES (1, $1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET
			cloud_name = EXCLUDED.cloud_name,
			api_key = EXCLUDED.api_key,
			api_secret_encrypted = EXCLUDED.api_secret_encrypted,
			api_secret_nonce = EXCLUDED.api_secret_nonce,
			updated_at = CURRENT_TIMESTAMP`

	_, err = cs.DB.Exec(query, cloudName, apiKey, secretEnc, secretNonce)
	if err != nil {
		return fmt.Errorf("save cloudinary settings: %w", err)
	}

	return nil
}

// Delete removes the Cloudinary settings
func (cs *CloudinaryService) Delete() error {
	_, err := cs.DB.Exec("DELETE FROM cloudinary_settings WHERE id = 1")
	if err != nil {
		return fmt.Errorf("delete cloudinary settings: %w", err)
	}
	return nil
}

// UpdateHealthStatus updates the status and consecutive failure count
func (cs *CloudinaryService) UpdateHealthStatus(status string, failures int) error {
	_, err := cs.DB.Exec(
		`UPDATE cloudinary_settings SET status = $1, consecutive_failures = $2, last_checked_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = 1`,
		status, failures,
	)
	if err != nil {
		return fmt.Errorf("update health status: %w", err)
	}
	return nil
}

// IsConfigured returns true if Cloudinary settings exist and are enabled
func (cs *CloudinaryService) IsConfigured() bool {
	var count int
	err := cs.DB.QueryRow("SELECT COUNT(*) FROM cloudinary_settings WHERE id = 1 AND is_enabled = true").Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

// GenerateSignature generates a Cloudinary upload signature per their spec:
// SHA1(sorted_params_string + api_secret)
func GenerateSignature(params map[string]string, apiSecret string) string {
	// Sort parameter keys
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build the string to sign: key1=value1&key2=value2...
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, params[k]))
	}
	toSign := strings.Join(parts, "&") + apiSecret

	h := sha1.New()
	h.Write([]byte(toSign))
	return fmt.Sprintf("%x", h.Sum(nil))
}
