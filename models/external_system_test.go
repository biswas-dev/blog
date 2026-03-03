package models

import (
	"encoding/json"
	"testing"

	"anshumanbiswas.com/blog/internal/crypto"
)

func TestEncryptAPIKey(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	t.Run("empty key returns nil", func(t *testing.T) {
		enc, nonce, err := encryptAPIKey("", key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if enc != nil || nonce != nil {
			t.Error("expected nil for empty API key")
		}
	})

	t.Run("encrypts non-empty key", func(t *testing.T) {
		enc, nonce, err := encryptAPIKey("my-secret-key", key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if enc == nil || nonce == nil {
			t.Fatal("expected non-nil enc and nonce")
		}

		// Verify we can decrypt back
		plain, err := crypto.Decrypt(enc, nonce, key)
		if err != nil {
			t.Fatalf("decrypt failed: %v", err)
		}
		if string(plain) != "my-secret-key" {
			t.Errorf("decrypted = %q, want 'my-secret-key'", string(plain))
		}
	})
}

func TestEncryptHeaders(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	t.Run("nil headers returns nil", func(t *testing.T) {
		enc, nonce, err := encryptHeaders(nil, key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if enc != nil || nonce != nil {
			t.Error("expected nil for nil headers")
		}
	})

	t.Run("encrypts non-nil headers", func(t *testing.T) {
		headers := []CustomHeader{
			{Key: "X-Custom", Value: "val1"},
			{Key: "Authorization", Value: "Bearer tok"},
		}
		enc, nonce, err := encryptHeaders(headers, key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if enc == nil || nonce == nil {
			t.Fatal("expected non-nil enc and nonce")
		}

		// Verify roundtrip
		plain, err := crypto.Decrypt(enc, nonce, key)
		if err != nil {
			t.Fatalf("decrypt failed: %v", err)
		}
		var decoded []CustomHeader
		if err := json.Unmarshal(plain, &decoded); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if len(decoded) != 2 {
			t.Fatalf("expected 2 headers, got %d", len(decoded))
		}
		if decoded[0].Key != "X-Custom" || decoded[0].Value != "val1" {
			t.Errorf("header[0] = %+v", decoded[0])
		}
	})
}

func TestCustomHeaderStruct(t *testing.T) {
	h := CustomHeader{Key: "X-API-Key", Value: "secret"}
	data, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded CustomHeader
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Key != "X-API-Key" || decoded.Value != "secret" {
		t.Errorf("roundtrip failed: %+v", decoded)
	}
}

func TestExternalSystemJSON(t *testing.T) {
	sys := ExternalSystem{
		ID:      1,
		Name:    "Test System",
		BaseURL: "https://example.com",
	}
	data, err := json.Marshal(sys)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded ExternalSystem
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Name != "Test System" {
		t.Errorf("Name = %q", decoded.Name)
	}
	if decoded.BaseURL != "https://example.com" {
		t.Errorf("BaseURL = %q", decoded.BaseURL)
	}
}

func TestSyncLogJSON(t *testing.T) {
	log := SyncLog{
		ID:               1,
		ExternalSystemID: 2,
		Direction:        "pull",
		Status:           "success",
		ItemsSynced:      5,
		ItemsSkipped:     3,
		ItemsFailed:      0,
	}
	data, err := json.Marshal(log)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded SyncLog
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Direction != "pull" {
		t.Errorf("Direction = %q", decoded.Direction)
	}
	if decoded.ItemsSynced != 5 {
		t.Errorf("ItemsSynced = %d", decoded.ItemsSynced)
	}
}
