package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"anshumanbiswas.com/blog/internal/crypto"
)

// ExternalSystem represents a registered external blog instance
type ExternalSystem struct {
	ID              int            `json:"id"`
	Name            string         `json:"name"`
	BaseURL         string         `json:"base_url"`
	APIKey          string         `json:"api_key,omitempty"` // Only populated when decrypted
	CustomHeaders   []CustomHeader `json:"custom_headers,omitempty"`
	IsActive        bool           `json:"is_active"`
	LastSyncAt      *time.Time     `json:"last_sync_at,omitempty"`
	LastSyncStatus  string         `json:"last_sync_status,omitempty"`
	LastSyncMessage string         `json:"last_sync_message,omitempty"`
	CreatedBy       int            `json:"created_by"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// CustomHeader represents a key-value HTTP header pair
type CustomHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// SyncLog represents a record of a sync operation
type SyncLog struct {
	ID               int        `json:"id"`
	ExternalSystemID int        `json:"external_system_id"`
	Direction        string     `json:"direction"`
	ContentType      string     `json:"content_type"`
	Status           string     `json:"status"`
	ItemsSynced      int        `json:"items_synced"`
	ItemsSkipped     int        `json:"items_skipped"`
	ItemsFailed      int        `json:"items_failed"`
	ErrorMessage     string     `json:"error_message,omitempty"`
	InitiatedBy      int        `json:"initiated_by"`
	StartedAt        time.Time  `json:"started_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

// SyncPreview represents a preview of what a sync operation would do
type SyncPreview struct {
	Direction string     `json:"direction"`
	Items     []SyncItem `json:"items"`
	NewCount  int        `json:"new_count"`
	SkipCount int        `json:"skip_count"`
}

// SyncItem represents a single item in a sync preview
type SyncItem struct {
	Title  string `json:"title"`
	Slug   string `json:"slug"`
	Status string `json:"status"` // "new" or "exists"
}

// SyncResult represents the result of an executed sync
type SyncResult struct {
	Direction    string `json:"direction"`
	ItemsSynced  int    `json:"items_synced"`
	ItemsSkipped int    `json:"items_skipped"`
	ItemsFailed  int    `json:"items_failed"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// ExternalSystemService handles CRUD for external systems and sync logs
type ExternalSystemService struct {
	DB *sql.DB
}

// Create inserts a new external system with encrypted credentials
func (es *ExternalSystemService) Create(name, baseURL, apiKey string, headers []CustomHeader, createdBy int) (*ExternalSystem, error) {
	key, err := crypto.GetEncryptionKey()
	if err != nil {
		return nil, fmt.Errorf("get encryption key: %w", err)
	}

	// Encrypt API key
	var apiKeyEnc, apiKeyNonce []byte
	if apiKey != "" {
		apiKeyEnc, apiKeyNonce, err = crypto.Encrypt([]byte(apiKey), key)
		if err != nil {
			return nil, fmt.Errorf("encrypt api key: %w", err)
		}
	}

	// Encrypt custom headers as JSON
	var headersEnc, headersNonce []byte
	if len(headers) > 0 {
		headersJSON, err := json.Marshal(headers)
		if err != nil {
			return nil, fmt.Errorf("marshal headers: %w", err)
		}
		headersEnc, headersNonce, err = crypto.Encrypt(headersJSON, key)
		if err != nil {
			return nil, fmt.Errorf("encrypt headers: %w", err)
		}
	}

	system := &ExternalSystem{}
	query := `
		INSERT INTO external_systems (name, base_url, api_key_encrypted, api_key_nonce, custom_headers_encrypted, custom_headers_nonce, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, name, base_url, is_active, COALESCE(created_by, 0), created_at, updated_at`

	err = es.DB.QueryRow(query, name, baseURL, apiKeyEnc, apiKeyNonce, headersEnc, headersNonce, createdBy).Scan(
		&system.ID, &system.Name, &system.BaseURL, &system.IsActive, &system.CreatedBy, &system.CreatedAt, &system.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create external system: %w", err)
	}

	return system, nil
}

// GetAll returns all external systems without decrypting credentials
func (es *ExternalSystemService) GetAll() ([]ExternalSystem, error) {
	systems := []ExternalSystem{}
	query := `
		SELECT id, name, base_url, is_active, last_sync_at,
		       COALESCE(last_sync_status, ''), COALESCE(last_sync_message, ''),
		       COALESCE(created_by, 0), created_at, updated_at
		FROM external_systems
		ORDER BY name ASC`

	rows, err := es.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("list external systems: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var s ExternalSystem
		err := rows.Scan(
			&s.ID, &s.Name, &s.BaseURL, &s.IsActive, &s.LastSyncAt,
			&s.LastSyncStatus, &s.LastSyncMessage,
			&s.CreatedBy, &s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan external system: %w", err)
		}
		systems = append(systems, s)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate external systems: %w", err)
	}

	return systems, nil
}

// GetByID returns an external system with decrypted credentials
func (es *ExternalSystemService) GetByID(id int) (*ExternalSystem, error) {
	key, err := crypto.GetEncryptionKey()
	if err != nil {
		return nil, fmt.Errorf("get encryption key: %w", err)
	}

	var s ExternalSystem
	var apiKeyEnc, apiKeyNonce, headersEnc, headersNonce []byte

	query := `
		SELECT id, name, base_url, api_key_encrypted, api_key_nonce, custom_headers_encrypted, custom_headers_nonce,
		       is_active, last_sync_at, COALESCE(last_sync_status, ''), COALESCE(last_sync_message, ''),
		       COALESCE(created_by, 0), created_at, updated_at
		FROM external_systems WHERE id = $1`

	err = es.DB.QueryRow(query, id).Scan(
		&s.ID, &s.Name, &s.BaseURL, &apiKeyEnc, &apiKeyNonce, &headersEnc, &headersNonce,
		&s.IsActive, &s.LastSyncAt, &s.LastSyncStatus, &s.LastSyncMessage, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("external system not found")
		}
		return nil, fmt.Errorf("get external system: %w", err)
	}

	// Decrypt API key
	if len(apiKeyEnc) > 0 && len(apiKeyNonce) > 0 {
		plaintext, err := crypto.Decrypt(apiKeyEnc, apiKeyNonce, key)
		if err != nil {
			return nil, fmt.Errorf("decrypt api key: %w", err)
		}
		s.APIKey = string(plaintext)
	}

	// Decrypt custom headers
	if len(headersEnc) > 0 && len(headersNonce) > 0 {
		plaintext, err := crypto.Decrypt(headersEnc, headersNonce, key)
		if err != nil {
			return nil, fmt.Errorf("decrypt headers: %w", err)
		}
		var headers []CustomHeader
		if err := json.Unmarshal(plaintext, &headers); err != nil {
			return nil, fmt.Errorf("unmarshal headers: %w", err)
		}
		s.CustomHeaders = headers
	}

	return &s, nil
}

// Update updates an external system, re-encrypting credentials if provided
func (es *ExternalSystemService) Update(id int, name, baseURL, apiKey string, headers []CustomHeader) (*ExternalSystem, error) {
	key, err := crypto.GetEncryptionKey()
	if err != nil {
		return nil, fmt.Errorf("get encryption key: %w", err)
	}

	// If apiKey is empty, keep existing encrypted values
	if apiKey == "" && headers == nil {
		// Only update name and base_url
		query := `
			UPDATE external_systems SET name = $1, base_url = $2, updated_at = CURRENT_TIMESTAMP
			WHERE id = $3
			RETURNING id, name, base_url, is_active, COALESCE(created_by, 0), created_at, updated_at`
		s := &ExternalSystem{}
		err = es.DB.QueryRow(query, name, baseURL, id).Scan(
			&s.ID, &s.Name, &s.BaseURL, &s.IsActive, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("external system not found")
			}
			return nil, fmt.Errorf("update external system: %w", err)
		}
		return s, nil
	}

	// Re-encrypt API key if provided
	var apiKeyEnc, apiKeyNonce []byte
	if apiKey != "" {
		apiKeyEnc, apiKeyNonce, err = crypto.Encrypt([]byte(apiKey), key)
		if err != nil {
			return nil, fmt.Errorf("encrypt api key: %w", err)
		}
	}

	// Re-encrypt headers if provided
	var headersEnc, headersNonce []byte
	if headers != nil {
		headersJSON, err := json.Marshal(headers)
		if err != nil {
			return nil, fmt.Errorf("marshal headers: %w", err)
		}
		headersEnc, headersNonce, err = crypto.Encrypt(headersJSON, key)
		if err != nil {
			return nil, fmt.Errorf("encrypt headers: %w", err)
		}
	}

	// Build dynamic update
	s := &ExternalSystem{}
	if apiKey != "" && headers != nil {
		query := `
			UPDATE external_systems
			SET name = $1, base_url = $2, api_key_encrypted = $3, api_key_nonce = $4,
			    custom_headers_encrypted = $5, custom_headers_nonce = $6, updated_at = CURRENT_TIMESTAMP
			WHERE id = $7
			RETURNING id, name, base_url, is_active, COALESCE(created_by, 0), created_at, updated_at`
		err = es.DB.QueryRow(query, name, baseURL, apiKeyEnc, apiKeyNonce, headersEnc, headersNonce, id).Scan(
			&s.ID, &s.Name, &s.BaseURL, &s.IsActive, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt,
		)
	} else if apiKey != "" {
		query := `
			UPDATE external_systems
			SET name = $1, base_url = $2, api_key_encrypted = $3, api_key_nonce = $4, updated_at = CURRENT_TIMESTAMP
			WHERE id = $5
			RETURNING id, name, base_url, is_active, COALESCE(created_by, 0), created_at, updated_at`
		err = es.DB.QueryRow(query, name, baseURL, apiKeyEnc, apiKeyNonce, id).Scan(
			&s.ID, &s.Name, &s.BaseURL, &s.IsActive, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt,
		)
	} else {
		query := `
			UPDATE external_systems
			SET name = $1, base_url = $2, custom_headers_encrypted = $3, custom_headers_nonce = $4, updated_at = CURRENT_TIMESTAMP
			WHERE id = $5
			RETURNING id, name, base_url, is_active, COALESCE(created_by, 0), created_at, updated_at`
		err = es.DB.QueryRow(query, name, baseURL, headersEnc, headersNonce, id).Scan(
			&s.ID, &s.Name, &s.BaseURL, &s.IsActive, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt,
		)
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("external system not found")
		}
		return nil, fmt.Errorf("update external system: %w", err)
	}

	return s, nil
}

// Delete removes an external system and its sync logs (CASCADE)
func (es *ExternalSystemService) Delete(id int) error {
	result, err := es.DB.Exec("DELETE FROM external_systems WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete external system: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete result: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("external system not found")
	}

	return nil
}

// UpdateSyncStatus updates the last sync fields on an external system
func (es *ExternalSystemService) UpdateSyncStatus(id int, status, message string) error {
	_, err := es.DB.Exec(
		"UPDATE external_systems SET last_sync_at = CURRENT_TIMESTAMP, last_sync_status = $1, last_sync_message = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $3",
		status, message, id,
	)
	if err != nil {
		return fmt.Errorf("update sync status: %w", err)
	}
	return nil
}

// CreateSyncLog inserts a new sync log entry and returns its ID
func (es *ExternalSystemService) CreateSyncLog(systemID int, direction, contentType string, initiatedBy int) (int, error) {
	var logID int
	query := `
		INSERT INTO sync_logs (external_system_id, direction, content_type, status, initiated_by)
		VALUES ($1, $2, $3, 'started', $4)
		RETURNING id`
	err := es.DB.QueryRow(query, systemID, direction, contentType, initiatedBy).Scan(&logID)
	if err != nil {
		return 0, fmt.Errorf("create sync log: %w", err)
	}
	return logID, nil
}

// UpdateSyncLog updates a sync log with results
func (es *ExternalSystemService) UpdateSyncLog(logID int, status string, synced, skipped, failed int, errMsg string) error {
	_, err := es.DB.Exec(
		`UPDATE sync_logs SET status = $1, items_synced = $2, items_skipped = $3, items_failed = $4, error_message = $5, completed_at = CURRENT_TIMESTAMP WHERE id = $6`,
		status, synced, skipped, failed, errMsg, logID,
	)
	if err != nil {
		return fmt.Errorf("update sync log: %w", err)
	}
	return nil
}

// GetSyncLogs returns recent sync logs for a system
func (es *ExternalSystemService) GetSyncLogs(systemID, limit int) ([]SyncLog, error) {
	if limit <= 0 {
		limit = 20
	}

	var logs []SyncLog
	query := `
		SELECT id, external_system_id, direction, content_type, status, items_synced, items_skipped, items_failed,
		       COALESCE(error_message, ''), COALESCE(initiated_by, 0), started_at, completed_at
		FROM sync_logs
		WHERE external_system_id = $1
		ORDER BY started_at DESC
		LIMIT $2`

	rows, err := es.DB.Query(query, systemID, limit)
	if err != nil {
		return nil, fmt.Errorf("get sync logs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var l SyncLog
		err := rows.Scan(
			&l.ID, &l.ExternalSystemID, &l.Direction, &l.ContentType, &l.Status,
			&l.ItemsSynced, &l.ItemsSkipped, &l.ItemsFailed, &l.ErrorMessage,
			&l.InitiatedBy, &l.StartedAt, &l.CompletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan sync log: %w", err)
		}
		logs = append(logs, l)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sync logs: %w", err)
	}

	return logs, nil
}
