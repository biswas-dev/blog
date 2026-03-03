package models

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"testing"
)

func TestGenerateSignature(t *testing.T) {
	t.Run("single param", func(t *testing.T) {
		params := map[string]string{"timestamp": "1234567890"}
		secret := "my-secret"
		got := GenerateSignature(params, secret)

		// Expected: SHA1("timestamp=1234567890my-secret")
		h := sha1.New()
		h.Write([]byte("timestamp=1234567890my-secret"))
		want := fmt.Sprintf("%x", h.Sum(nil))
		if got != want {
			t.Errorf("GenerateSignature() = %q, want %q", got, want)
		}
	})

	t.Run("multiple params sorted", func(t *testing.T) {
		params := map[string]string{
			"timestamp": "123",
			"folder":    "uploads",
		}
		secret := "secret"
		got := GenerateSignature(params, secret)

		// Keys sorted: folder, timestamp → "folder=uploads&timestamp=123secret"
		h := sha1.New()
		h.Write([]byte("folder=uploads&timestamp=123secret"))
		want := fmt.Sprintf("%x", h.Sum(nil))
		if got != want {
			t.Errorf("GenerateSignature() = %q, want %q", got, want)
		}
	})

	t.Run("empty params", func(t *testing.T) {
		params := map[string]string{}
		secret := "secret"
		got := GenerateSignature(params, secret)

		// SHA1("secret")
		h := sha1.New()
		h.Write([]byte("secret"))
		want := fmt.Sprintf("%x", h.Sum(nil))
		if got != want {
			t.Errorf("GenerateSignature() = %q, want %q", got, want)
		}
	})

	t.Run("deterministic output", func(t *testing.T) {
		params := map[string]string{"a": "1", "b": "2"}
		got1 := GenerateSignature(params, "secret")
		got2 := GenerateSignature(params, "secret")
		if got1 != got2 {
			t.Errorf("expected deterministic output, got %q and %q", got1, got2)
		}
	})

	t.Run("different secret produces different signature", func(t *testing.T) {
		params := map[string]string{"timestamp": "123"}
		got1 := GenerateSignature(params, "secret1")
		got2 := GenerateSignature(params, "secret2")
		if got1 == got2 {
			t.Error("different secrets should produce different signatures")
		}
	})
}

func TestCloudinarySettingsJSON(t *testing.T) {
	settings := CloudinarySettings{
		CloudName: "my-cloud",
		APIKey:    "api-key-123",
		IsEnabled: true,
		Status:    "healthy",
	}

	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded CloudinarySettings
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.CloudName != "my-cloud" {
		t.Errorf("CloudName = %q", decoded.CloudName)
	}
	if decoded.APIKey != "api-key-123" {
		t.Errorf("APIKey = %q", decoded.APIKey)
	}
	if !decoded.IsEnabled {
		t.Error("expected IsEnabled = true")
	}
	if decoded.Status != "healthy" {
		t.Errorf("Status = %q", decoded.Status)
	}
}

func TestCloudinarySettingsOmitsSecret(t *testing.T) {
	settings := CloudinarySettings{
		CloudName: "cloud",
		APIKey:    "key",
		APISecret: "",
	}

	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}

	// api_secret should be omitted when empty
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)
	if _, exists := raw["api_secret"]; exists {
		t.Error("api_secret should be omitted when empty")
	}
}
