package models

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"anshumanbiswas.com/blog/internal/crypto"
)

// BrevoSettings represents the stored Brevo email configuration
type BrevoSettings struct {
	APIKey              string     `json:"api_key,omitempty"`
	FromEmail           string     `json:"from_email"`
	FromName            string     `json:"from_name"`
	IsEnabled           bool       `json:"is_enabled"`
	Status              string     `json:"status"`
	LastCheckedAt       *time.Time `json:"last_checked_at,omitempty"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// BrevoService handles CRUD for Brevo settings and sending emails
type BrevoService struct {
	DB *sql.DB
}

// Get reads settings and decrypts the API key. Returns nil, nil if not configured.
func (bs *BrevoService) Get() (*BrevoSettings, error) {
	var s BrevoSettings
	var keyEnc, keyNonce []byte

	query := `
		SELECT api_key_encrypted, api_key_nonce,
		       from_email, from_name, is_enabled,
		       COALESCE(status, 'unknown'), last_checked_at,
		       COALESCE(consecutive_failures, 0), created_at, updated_at
		FROM brevo_settings WHERE id = 1`

	err := bs.DB.QueryRow(query).Scan(
		&keyEnc, &keyNonce,
		&s.FromEmail, &s.FromName, &s.IsEnabled,
		&s.Status, &s.LastCheckedAt,
		&s.ConsecutiveFailures, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get brevo settings: %w", err)
	}

	if len(keyEnc) > 0 && len(keyNonce) > 0 {
		key, err := crypto.GetEncryptionKey()
		if err != nil {
			return nil, fmt.Errorf("get encryption key: %w", err)
		}
		plaintext, err := crypto.Decrypt(keyEnc, keyNonce, key)
		if err != nil {
			return nil, fmt.Errorf("decrypt api key: %w", err)
		}
		s.APIKey = string(plaintext)
	}

	return &s, nil
}

// Save performs an UPSERT of Brevo settings. If apiKey is empty on update, keeps existing.
func (bs *BrevoService) Save(apiKey, fromEmail, fromName string) error {
	key, err := crypto.GetEncryptionKey()
	if err != nil {
		return fmt.Errorf("get encryption key: %w", err)
	}

	if apiKey == "" {
		existing, err := bs.Get()
		if err != nil {
			return fmt.Errorf("check existing settings: %w", err)
		}
		if existing != nil {
			apiKey = existing.APIKey
		}
		if apiKey == "" {
			return fmt.Errorf("API key is required")
		}
	}

	keyEnc, keyNonce, err := crypto.Encrypt([]byte(apiKey), key)
	if err != nil {
		return fmt.Errorf("encrypt api key: %w", err)
	}

	query := `
		INSERT INTO brevo_settings (id, api_key_encrypted, api_key_nonce, from_email, from_name)
		VALUES (1, $1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET
			api_key_encrypted = EXCLUDED.api_key_encrypted,
			api_key_nonce = EXCLUDED.api_key_nonce,
			from_email = EXCLUDED.from_email,
			from_name = EXCLUDED.from_name,
			updated_at = CURRENT_TIMESTAMP`

	_, err = bs.DB.Exec(query, keyEnc, keyNonce, fromEmail, fromName)
	if err != nil {
		return fmt.Errorf("save brevo settings: %w", err)
	}

	return nil
}

// Delete removes the Brevo settings
func (bs *BrevoService) Delete() error {
	_, err := bs.DB.Exec("DELETE FROM brevo_settings WHERE id = 1")
	if err != nil {
		return fmt.Errorf("delete brevo settings: %w", err)
	}
	return nil
}

// UpdateHealthStatus updates the status and consecutive failure count
func (bs *BrevoService) UpdateHealthStatus(status string, failures int) error {
	_, err := bs.DB.Exec(
		`UPDATE brevo_settings SET status = $1, consecutive_failures = $2, last_checked_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = 1`,
		status, failures,
	)
	if err != nil {
		return fmt.Errorf("update health status: %w", err)
	}
	return nil
}

// IsConfigured returns true if Brevo settings exist and are enabled
func (bs *BrevoService) IsConfigured() bool {
	var count int
	err := bs.DB.QueryRow("SELECT COUNT(*) FROM brevo_settings WHERE id = 1 AND is_enabled = true").Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

// TestConnection sends a GET request to the Brevo account endpoint to verify the API key.
func (bs *BrevoService) TestConnection(apiKey string) (bool, string) {
	req, err := http.NewRequest("GET", "https://api.brevo.com/v3/account", nil)
	if err != nil {
		return false, fmt.Sprintf("Failed to create request: %v", err)
	}
	req.Header.Set("api-key", apiKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Sprintf("Connection failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))

	if resp.StatusCode == http.StatusOK {
		var account struct {
			Email   string `json:"email"`
			Company string `json:"companyName"`
		}
		if json.Unmarshal(body, &account) == nil && account.Email != "" {
			return true, fmt.Sprintf("Connected as %s (%s)", account.Email, account.Company)
		}
		return true, "Connection successful"
	}

	return false, fmt.Sprintf("Brevo returned status %d: %s", resp.StatusCode, string(body))
}

// SendTestEmail sends a small test email via the Brevo transactional API.
func (bs *BrevoService) SendTestEmail(apiKey, fromEmail, fromName, toEmail string) (bool, string) {
	payload := map[string]interface{}{
		"sender": map[string]string{
			"name":  fromName,
			"email": fromEmail,
		},
		"to": []map[string]string{
			{"email": toEmail},
		},
		"subject":     "Test Email from Blog Admin",
		"htmlContent": "<html><body><h2>Test Email</h2><p>This is a test email from your blog's Brevo integration.</p><p>If you received this, your email settings are configured correctly.</p></body></html>",
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Sprintf("Failed to build request: %v", err)
	}

	req, err := http.NewRequest("POST", "https://api.brevo.com/v3/smtp/email", bytes.NewReader(jsonBody))
	if err != nil {
		return false, fmt.Sprintf("Failed to create request: %v", err)
	}
	req.Header.Set("api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Sprintf("Send failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		return true, fmt.Sprintf("Test email sent to %s", toEmail)
	}

	return false, fmt.Sprintf("Brevo returned status %d: %s", resp.StatusCode, string(body))
}
