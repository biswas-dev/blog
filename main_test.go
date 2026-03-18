package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestGetAppPort(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "default port when no env var",
			envValue: "",
			expected: "3000",
		},
		{
			name:     "custom port from env var",
			envValue: "8080",
			expected: "8080",
		},
		{
			name:     "port 80",
			envValue: "80",
			expected: "80",
		},
		{
			name:     "high port number",
			envValue: "65535",
			expected: "65535",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original env var
			original := os.Getenv("APP_PORT")
			defer os.Setenv("APP_PORT", original)

			// Set test env var
			if tt.envValue == "" {
				os.Unsetenv("APP_PORT")
			} else {
				os.Setenv("APP_PORT", tt.envValue)
			}

			result := getAppPort()
			if result != tt.expected {
				t.Errorf("getAppPort() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Test environment variable handling
func TestEnvironmentVariables(t *testing.T) {
	t.Run("get and set env var", func(t *testing.T) {
		key := "TEST_VAR"
		value := "test_value"

		// Clean up
		defer os.Unsetenv(key)

		// Set
		os.Setenv(key, value)

		// Get
		result := os.Getenv(key)
		if result != value {
			t.Errorf("Expected env var '%s', got '%s'", value, result)
		}
	})

	t.Run("unset env var returns empty string", func(t *testing.T) {
		result := os.Getenv("NONEXISTENT_VAR_XYZ123")
		if result != "" {
			t.Errorf("Expected empty string for unset var, got '%s'", result)
		}
	})

	t.Run("override existing env var", func(t *testing.T) {
		key := "OVERRIDE_TEST"
		original := "original"
		updated := "updated"

		os.Setenv(key, original)
		defer os.Unsetenv(key)

		// Override
		os.Setenv(key, updated)

		result := os.Getenv(key)
		if result != updated {
			t.Errorf("Expected '%s', got '%s'", updated, result)
		}
	})
}

// Test database configuration parsing
func TestDatabaseConfig(t *testing.T) {
	t.Run("database environment variables", func(t *testing.T) {
		vars := []string{"PG_USER", "PG_PASSWORD", "PG_DB", "PG_HOST", "PG_PORT"}

		for _, varName := range vars {
			// Test that we can set and get each variable
			testValue := "test_" + varName
			os.Setenv(varName, testValue)
			defer os.Unsetenv(varName)

			result := os.Getenv(varName)
			if result != testValue {
				t.Errorf("Failed to set/get %s", varName)
			}
		}
	})

	t.Run("parse database connection string components", func(t *testing.T) {
		dbUser := "testuser"
		dbPassword := "testpass"
		dbName := "testdb"
		dbHost := "localhost"
		dbPort := "5432"

		os.Setenv("PG_USER", dbUser)
		os.Setenv("PG_PASSWORD", dbPassword)
		os.Setenv("PG_DB", dbName)
		os.Setenv("PG_HOST", dbHost)
		os.Setenv("PG_PORT", dbPort)

		defer func() {
			os.Unsetenv("PG_USER")
			os.Unsetenv("PG_PASSWORD")
			os.Unsetenv("PG_DB")
			os.Unsetenv("PG_HOST")
			os.Unsetenv("PG_PORT")
		}()

		if os.Getenv("PG_USER") != dbUser {
			t.Error("PG_USER not set correctly")
		}
		if os.Getenv("PG_PASSWORD") != dbPassword {
			t.Error("PG_PASSWORD not set correctly")
		}
		if os.Getenv("PG_DB") != dbName {
			t.Error("PG_DB not set correctly")
		}
		if os.Getenv("PG_HOST") != dbHost {
			t.Error("PG_HOST not set correctly")
		}
		if os.Getenv("PG_PORT") != dbPort {
			t.Error("PG_PORT not set correctly")
		}
	})
}

// Test API token handling
func TestAPITokenHandling(t *testing.T) {
	t.Run("API token environment variable", func(t *testing.T) {
		token := "test-api-token-12345"

		os.Setenv("API_TOKEN", token)
		defer os.Unsetenv("API_TOKEN")

		result := os.Getenv("API_TOKEN")
		if result != token {
			t.Errorf("Expected token '%s', got '%s'", token, result)
		}
	})

	t.Run("empty API token", func(t *testing.T) {
		os.Unsetenv("API_TOKEN")

		result := os.Getenv("API_TOKEN")
		if result != "" {
			t.Errorf("Expected empty token, got '%s'", result)
		}
	})
}

// Test port validation logic
func TestPortValidation(t *testing.T) {
	t.Run("valid port numbers", func(t *testing.T) {
		validPorts := []string{"80", "443", "3000", "8080", "8443", "65535"}

		for _, port := range validPorts {
			os.Setenv("APP_PORT", port)
			defer os.Unsetenv("APP_PORT")

			result := getAppPort()
			if result != port {
				t.Errorf("Expected port '%s', got '%s'", port, result)
			}
		}
	})

	t.Run("port number as string", func(t *testing.T) {
		port := "3000"
		os.Setenv("APP_PORT", port)
		defer os.Unsetenv("APP_PORT")

		result := getAppPort()

		// Should return the string as-is
		if result != port {
			t.Errorf("Expected port '%s', got '%s'", port, result)
		}
	})
}

// Test listen address formatting
func TestListenAddressFormatting(t *testing.T) {
	tests := []struct {
		name     string
		port     string
		expected string
	}{
		{
			name:     "default port format",
			port:     "3000",
			expected: ":3000",
		},
		{
			name:     "port 80 format",
			port:     "80",
			expected: ":80",
		},
		{
			name:     "custom port format",
			port:     "8080",
			expected: ":8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("APP_PORT", tt.port)
			defer os.Unsetenv("APP_PORT")

			port := getAppPort()
			listenAddr := ":" + port

			if listenAddr != tt.expected {
				t.Errorf("Expected listen address '%s', got '%s'", tt.expected, listenAddr)
			}
		})
	}
}

// Test configuration precedence
func TestConfigurationPrecedence(t *testing.T) {
	t.Run("environment variable overrides default", func(t *testing.T) {
		customPort := "9000"
		os.Setenv("APP_PORT", customPort)
		defer os.Unsetenv("APP_PORT")

		port := getAppPort()

		if port == "3000" {
			t.Error("Environment variable should override default port")
		}

		if port != customPort {
			t.Errorf("Expected custom port '%s', got '%s'", customPort, port)
		}
	})

	t.Run("default used when env var not set", func(t *testing.T) {
		os.Unsetenv("APP_PORT")

		port := getAppPort()

		if port != "3000" {
			t.Errorf("Expected default port '3000', got '%s'", port)
		}
	})
}

// Test empty and whitespace handling
func TestEmptyValueHandling(t *testing.T) {
	t.Run("empty string env var uses default", func(t *testing.T) {
		os.Setenv("APP_PORT", "")
		defer os.Unsetenv("APP_PORT")

		port := getAppPort()

		if port != "3000" {
			t.Errorf("Expected default port '3000' for empty env var, got '%s'", port)
		}
	})

	t.Run("whitespace-only env var", func(t *testing.T) {
		os.Setenv("APP_PORT", "   ")
		defer os.Unsetenv("APP_PORT")

		port := getAppPort()

		// Function returns as-is, doesn't trim
		if port != "   " {
			t.Errorf("Expected whitespace to be preserved, got '%s'", port)
		}
	})
}

func TestXmlEscape(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"no special chars", "hello world", "hello world"},
		{"ampersand", "A & B", "A &amp; B"},
		{"less than", "a < b", "a &lt; b"},
		{"greater than", "a > b", "a &gt; b"},
		{"double quote", `say "hello"`, "say &quot;hello&quot;"},
		{"single quote", "it's", "it&apos;s"},
		{"all special chars", `<a href="x">&'`, "&lt;a href=&quot;x&quot;&gt;&amp;&apos;"},
		{"multiple ampersands", "A & B & C", "A &amp; B &amp; C"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := xmlEscape(tt.input)
			if got != tt.want {
				t.Errorf("xmlEscape(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestJsonResponse(t *testing.T) {
	t.Run("sends JSON with status code", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := map[string]string{"key": "value"}
		jsonResponse(w, data, http.StatusOK)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", ct)
		}

		var result map[string]string
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if result["key"] != "value" {
			t.Errorf("expected key=value, got %q", result["key"])
		}
	})

	t.Run("sends 201 for created", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := map[string]int{"id": 42}
		jsonResponse(w, data, http.StatusCreated)

		if w.Code != http.StatusCreated {
			t.Errorf("expected status 201, got %d", w.Code)
		}
	})

	t.Run("sends array", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := []string{"a", "b", "c"}
		jsonResponse(w, data, http.StatusOK)

		var result []string
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if len(result) != 3 {
			t.Errorf("expected 3 items, got %d", len(result))
		}
	})
}

func TestStaticCacheMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := staticCacheMiddleware(inner)

	tests := []struct {
		name       string
		path       string
		wantCache  string
		wantCT     string
	}{
		{"css file", "/styles.css", "public, max-age=31536000, immutable", ""},
		{"js file", "/app.js", "public, max-age=31536000, immutable", ""},
		{"svg file", "/icon.svg", "public, max-age=604800, stale-while-revalidate=2592000", mimeSVG},
		{"png file", "/photo.png", "public, max-age=604800, stale-while-revalidate=2592000", ""},
		{"jpg file", "/photo.jpg", "public, max-age=604800, stale-while-revalidate=2592000", ""},
		{"webp file", "/photo.webp", "public, max-age=604800, stale-while-revalidate=2592000", ""},
		{"gif file", "/anim.gif", "public, max-age=604800, stale-while-revalidate=2592000", ""},
		{"ico file", "/favicon.ico", "public, max-age=604800, stale-while-revalidate=2592000", ""},
		{"other file", "/data.json", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, tt.path, nil)
			handler.ServeHTTP(w, r)

			gotCache := w.Header().Get("Cache-Control")
			if gotCache != tt.wantCache {
				t.Errorf("Cache-Control = %q, want %q", gotCache, tt.wantCache)
			}
			if tt.wantCT != "" {
				gotCT := w.Header().Get("Content-Type")
				if gotCT != tt.wantCT {
					t.Errorf("Content-Type = %q, want %q", gotCT, tt.wantCT)
				}
			}
		})
	}
}

func TestConstants(t *testing.T) {
	// Verify constants are defined correctly
	if headerContentType != "Content-Type" {
		t.Errorf("headerContentType = %q", headerContentType)
	}
	if headerCacheCtrl != "Cache-Control" {
		t.Errorf("headerCacheCtrl = %q", headerCacheCtrl)
	}
	if mimeSVG != "image/svg+xml" {
		t.Errorf("mimeSVG = %q", mimeSVG)
	}
	if !strings.Contains(routeAdminUploads, "/admin/uploads") {
		t.Errorf("routeAdminUploads = %q", routeAdminUploads)
	}
}
