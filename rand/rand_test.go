package rand

import (
	"encoding/base64"
	"testing"
)

func TestBytes(t *testing.T) {
	t.Run("generates bytes of correct length", func(t *testing.T) {
		sizes := []int{1, 16, 32, 64, 128}
		for _, size := range sizes {
			b, err := Bytes(size)
			if err != nil {
				t.Errorf("Bytes(%d) unexpected error: %v", size, err)
			}
			if len(b) != size {
				t.Errorf("Bytes(%d) = %d bytes, want %d bytes", size, len(b), size)
			}
		}
	})

	t.Run("generates different bytes on each call", func(t *testing.T) {
		b1, err1 := Bytes(32)
		b2, err2 := Bytes(32)

		if err1 != nil || err2 != nil {
			t.Fatalf("Bytes() errors: %v, %v", err1, err2)
		}

		// Check that they're different (statistically should always be different)
		same := true
		for i := range b1 {
			if b1[i] != b2[i] {
				same = false
				break
			}
		}
		if same {
			t.Error("Bytes() generated identical random bytes twice (very unlikely)")
		}
	})

	t.Run("handles zero length", func(t *testing.T) {
		b, err := Bytes(0)
		if err != nil {
			t.Errorf("Bytes(0) unexpected error: %v", err)
		}
		if len(b) != 0 {
			t.Errorf("Bytes(0) = %d bytes, want 0 bytes", len(b))
		}
	})

	t.Run("generates cryptographically random bytes", func(t *testing.T) {
		// Test that bytes are not all zeros (basic sanity check)
		b, err := Bytes(32)
		if err != nil {
			t.Fatalf("Bytes() error: %v", err)
		}

		allZero := true
		for _, byte := range b {
			if byte != 0 {
				allZero = false
				break
			}
		}

		if allZero {
			t.Error("Bytes() returned all zeros (not random)")
		}
	})
}

func TestString(t *testing.T) {
	t.Run("generates base64 URL-encoded string", func(t *testing.T) {
		s, err := String(32)
		if err != nil {
			t.Fatalf("String() error: %v", err)
		}

		if s == "" {
			t.Error("String() returned empty string")
		}

		// Verify it's valid base64 URL encoding
		_, err = base64.URLEncoding.DecodeString(s)
		if err != nil {
			t.Errorf("String() result is not valid base64 URL encoding: %v", err)
		}
	})

	t.Run("generates different strings on each call", func(t *testing.T) {
		s1, err1 := String(32)
		s2, err2 := String(32)

		if err1 != nil || err2 != nil {
			t.Fatalf("String() errors: %v, %v", err1, err2)
		}

		if s1 == s2 {
			t.Error("String() generated identical strings twice (very unlikely)")
		}
	})

	t.Run("generates strings of expected length range", func(t *testing.T) {
		// Base64 encoding of n bytes produces roughly 4*ceil(n/3) characters
		sizes := []struct {
			bytes int
			minLen int
			maxLen int
		}{
			{16, 20, 30},
			{32, 40, 50},
			{64, 80, 100},
		}

		for _, tc := range sizes {
			s, err := String(tc.bytes)
			if err != nil {
				t.Errorf("String(%d) error: %v", tc.bytes, err)
			}
			if len(s) < tc.minLen || len(s) > tc.maxLen {
				t.Errorf("String(%d) length = %d, want between %d and %d",
					tc.bytes, len(s), tc.minLen, tc.maxLen)
			}
		}
	})

	t.Run("generates URL-safe characters only", func(t *testing.T) {
		s, err := String(32)
		if err != nil {
			t.Fatalf("String() error: %v", err)
		}

		// Base64 URL encoding uses: A-Z, a-z, 0-9, -, _
		for _, char := range s {
			if !((char >= 'A' && char <= 'Z') ||
				(char >= 'a' && char <= 'z') ||
				(char >= '0' && char <= '9') ||
				char == '-' || char == '_' || char == '=') {
				t.Errorf("String() contains non-URL-safe character: %c", char)
			}
		}
	})
}

func TestSessionToken(t *testing.T) {
	t.Run("generates session token", func(t *testing.T) {
		token, err := SessionToken()
		if err != nil {
			t.Fatalf("SessionToken() error: %v", err)
		}

		if token == "" {
			t.Error("SessionToken() returned empty string")
		}

		// Should be base64 URL encoded
		_, err = base64.URLEncoding.DecodeString(token)
		if err != nil {
			t.Errorf("SessionToken() result is not valid base64 URL encoding: %v", err)
		}
	})

	t.Run("generates unique session tokens", func(t *testing.T) {
		token1, err1 := SessionToken()
		token2, err2 := SessionToken()

		if err1 != nil || err2 != nil {
			t.Fatalf("SessionToken() errors: %v, %v", err1, err2)
		}

		if token1 == token2 {
			t.Error("SessionToken() generated identical tokens twice (very unlikely)")
		}
	})

	t.Run("generates tokens of consistent length", func(t *testing.T) {
		// SessionTokenBytes = 32, so base64 encoding should be ~44 chars
		tokens := make([]string, 10)
		for i := range tokens {
			token, err := SessionToken()
			if err != nil {
				t.Fatalf("SessionToken() error: %v", err)
			}
			tokens[i] = token
		}

		// All tokens should be the same length
		firstLen := len(tokens[0])
		for i, token := range tokens {
			if len(token) != firstLen {
				t.Errorf("Token %d length = %d, want %d", i, len(token), firstLen)
			}
		}

		// Should be approximately 44 characters (32 bytes -> 43 base64 chars + padding)
		if firstLen < 40 || firstLen > 50 {
			t.Errorf("SessionToken() length = %d, expected around 44", firstLen)
		}
	})

	t.Run("uses SessionTokenBytes constant", func(t *testing.T) {
		if SessionTokenBytes != 32 {
			t.Errorf("SessionTokenBytes = %d, want 32", SessionTokenBytes)
		}
	})

	t.Run("generates cryptographically strong tokens", func(t *testing.T) {
		// Generate multiple tokens and check for patterns
		tokens := make(map[string]bool)
		for i := 0; i < 100; i++ {
			token, err := SessionToken()
			if err != nil {
				t.Fatalf("SessionToken() error: %v", err)
			}

			if tokens[token] {
				t.Error("SessionToken() generated duplicate token")
			}
			tokens[token] = true
		}

		// Should have generated 100 unique tokens
		if len(tokens) != 100 {
			t.Errorf("Generated %d unique tokens out of 100", len(tokens))
		}
	})
}

func BenchmarkBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = Bytes(32)
	}
}

func BenchmarkString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = String(32)
	}
}

func BenchmarkSessionToken(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = SessionToken()
	}
}
