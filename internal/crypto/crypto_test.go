package crypto

import (
	"encoding/hex"
	"os"
	"testing"
)

func TestGetEncryptionKey(t *testing.T) {
	original := os.Getenv("ENCRYPTION_KEY")
	defer os.Setenv("ENCRYPTION_KEY", original)

	t.Run("missing env var", func(t *testing.T) {
		os.Unsetenv("ENCRYPTION_KEY")
		_, err := GetEncryptionKey()
		if err == nil {
			t.Fatal("expected error for missing ENCRYPTION_KEY")
		}
	})

	t.Run("invalid hex", func(t *testing.T) {
		os.Setenv("ENCRYPTION_KEY", "not-hex")
		_, err := GetEncryptionKey()
		if err == nil {
			t.Fatal("expected error for invalid hex")
		}
	})

	t.Run("wrong length", func(t *testing.T) {
		os.Setenv("ENCRYPTION_KEY", hex.EncodeToString([]byte("short")))
		_, err := GetEncryptionKey()
		if err == nil {
			t.Fatal("expected error for wrong length key")
		}
	})

	t.Run("valid 32-byte key", func(t *testing.T) {
		key := make([]byte, 32)
		for i := range key {
			key[i] = byte(i)
		}
		os.Setenv("ENCRYPTION_KEY", hex.EncodeToString(key))
		got, err := GetEncryptionKey()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 32 {
			t.Errorf("expected 32 bytes, got %d", len(got))
		}
	})
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	tests := []struct {
		name      string
		plaintext string
	}{
		{"empty string", ""},
		{"short text", "hello"},
		{"longer text", "this is a secret API key that needs encryption"},
		{"special chars", `{"key":"value","headers":[{"k":"Auth","v":"Bearer tok123"}]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, nonce, err := Encrypt([]byte(tt.plaintext), key)
			if err != nil {
				t.Fatalf("Encrypt error: %v", err)
			}
			if len(nonce) == 0 {
				t.Fatal("nonce should not be empty")
			}
			if tt.plaintext != "" && len(ciphertext) == 0 {
				t.Fatal("ciphertext should not be empty for non-empty plaintext")
			}

			decrypted, err := Decrypt(ciphertext, nonce, key)
			if err != nil {
				t.Fatalf("Decrypt error: %v", err)
			}
			if string(decrypted) != tt.plaintext {
				t.Errorf("roundtrip failed: got %q, want %q", string(decrypted), tt.plaintext)
			}
		})
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	for i := range key2 {
		key2[i] = byte(i + 1)
	}

	ciphertext, nonce, err := Encrypt([]byte("secret"), key1)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}

	_, err = Decrypt(ciphertext, nonce, key2)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}

func TestDecryptWithCorruptedNonce(t *testing.T) {
	key := make([]byte, 32)
	ciphertext, nonce, err := Encrypt([]byte("secret"), key)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}

	// Corrupt nonce
	nonce[0] ^= 0xff

	_, err = Decrypt(ciphertext, nonce, key)
	if err == nil {
		t.Fatal("expected error when decrypting with corrupted nonce")
	}
}

func TestEncryptProducesDifferentCiphertext(t *testing.T) {
	key := make([]byte, 32)
	plaintext := []byte("same plaintext")

	ct1, _, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("first encrypt: %v", err)
	}

	ct2, _, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("second encrypt: %v", err)
	}

	if hex.EncodeToString(ct1) == hex.EncodeToString(ct2) {
		t.Error("two encryptions of the same plaintext should produce different ciphertexts")
	}
}
