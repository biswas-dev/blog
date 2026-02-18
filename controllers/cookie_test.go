package controllers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewCookie(t *testing.T) {
	t.Run("creates cookie with correct properties", func(t *testing.T) {
		expire := time.Now().Add(7 * 24 * time.Hour)
		cookie := newCookie("test", "value", expire)

		if cookie.Name != "test" {
			t.Errorf("Expected name 'test', got %s", cookie.Name)
		}

		if cookie.Value != "value" {
			t.Errorf("Expected value 'value', got %s", cookie.Value)
		}

		if cookie.Path != "/" {
			t.Errorf("Expected path '/', got %s", cookie.Path)
		}

		if !cookie.HttpOnly {
			t.Error("Cookie should be HttpOnly")
		}

		if cookie.Expires != expire {
			t.Errorf("Expected expires %v, got %v", expire, cookie.Expires)
		}
	})

	t.Run("creates cookie with past expiration", func(t *testing.T) {
		expire := time.Now().Add(-7 * 24 * time.Hour)
		cookie := newCookie("expired", "old", expire)

		if cookie.Expires.After(time.Now()) {
			t.Error("Cookie should have past expiration")
		}
	})

	t.Run("creates cookie with empty values", func(t *testing.T) {
		expire := time.Now()
		cookie := newCookie("", "", expire)

		if cookie.Name != "" {
			t.Error("Should handle empty name")
		}

		if cookie.Value != "" {
			t.Error("Should handle empty value")
		}
	})
}

func TestSetCookie(t *testing.T) {
	t.Run("sets cookie on response", func(t *testing.T) {
		w := httptest.NewRecorder()
		setCookie(w, "test", "value")

		cookies := w.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("Expected 1 cookie, got %d", len(cookies))
		}

		cookie := cookies[0]
		if cookie.Name != "test" {
			t.Errorf("Expected name 'test', got %s", cookie.Name)
		}

		if cookie.Value != "value" {
			t.Errorf("Expected value 'value', got %s", cookie.Value)
		}

		if !cookie.HttpOnly {
			t.Error("Cookie should be HttpOnly")
		}

		if cookie.Path != "/" {
			t.Errorf("Expected path '/', got %s", cookie.Path)
		}
	})

	t.Run("cookie expires in 7 days", func(t *testing.T) {
		w := httptest.NewRecorder()
		setCookie(w, "test", "value")

		cookies := w.Result().Cookies()
		cookie := cookies[0]

		// Cookie should expire approximately 7 days from now (allow 1 minute variance)
		expectedExpiry := time.Now().Add(7 * 24 * time.Hour)
		diff := cookie.Expires.Sub(expectedExpiry)

		if diff < -1*time.Minute || diff > 1*time.Minute {
			t.Errorf("Cookie expiration %v differs from expected by %v", cookie.Expires, diff)
		}

		// Should definitely expire in the future
		if cookie.Expires.Before(time.Now()) {
			t.Error("Cookie should expire in the future")
		}
	})

	t.Run("sets session cookie", func(t *testing.T) {
		w := httptest.NewRecorder()
		setCookie(w, CookieSession, "session-token-123")

		cookies := w.Result().Cookies()
		cookie := cookies[0]

		if cookie.Name != CookieSession {
			t.Errorf("Expected name %s, got %s", CookieSession, cookie.Name)
		}

		if cookie.Value != "session-token-123" {
			t.Errorf("Expected session token, got %s", cookie.Value)
		}
	})

	t.Run("sets user email cookie", func(t *testing.T) {
		w := httptest.NewRecorder()
		setCookie(w, CookieUserEmail, "test@example.com")

		cookies := w.Result().Cookies()
		cookie := cookies[0]

		if cookie.Name != CookieUserEmail {
			t.Errorf("Expected name %s, got %s", CookieUserEmail, cookie.Name)
		}

		if cookie.Value != "test@example.com" {
			t.Errorf("Expected email, got %s", cookie.Value)
		}
	})
}

func TestReadCookie(t *testing.T) {
	t.Run("reads existing cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{
			Name:  "test",
			Value: "value",
		})

		value, err := readCookie(req, "test")
		if err != nil {
			t.Fatalf("readCookie returned error: %v", err)
		}

		if value != "value" {
			t.Errorf("Expected value 'value', got %s", value)
		}
	})

	t.Run("returns error for missing cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		_, err := readCookie(req, "nonexistent")
		if err == nil {
			t.Error("Expected error for missing cookie")
		}

		// Error message should contain cookie name
		if err != nil && !strings.Contains(err.Error(), "nonexistent") {
			t.Errorf("Error should mention cookie name: %v", err)
		}
	})

	t.Run("reads session cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{
			Name:  CookieSession,
			Value: "session-123",
		})

		value, err := readCookie(req, CookieSession)
		if err != nil {
			t.Fatalf("readCookie returned error: %v", err)
		}

		if value != "session-123" {
			t.Errorf("Expected session value, got %s", value)
		}
	})

	t.Run("reads user email cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{
			Name:  CookieUserEmail,
			Value: "user@example.com",
		})

		value, err := readCookie(req, CookieUserEmail)
		if err != nil {
			t.Fatalf("readCookie returned error: %v", err)
		}

		if value != "user@example.com" {
			t.Errorf("Expected email value, got %s", value)
		}
	})

	t.Run("handles empty cookie value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{
			Name:  "empty",
			Value: "",
		})

		value, err := readCookie(req, "empty")
		if err != nil {
			t.Fatalf("readCookie returned error: %v", err)
		}

		if value != "" {
			t.Errorf("Expected empty value, got %s", value)
		}
	})
}

func TestDeleteCookie(t *testing.T) {
	t.Run("sets cookie with past expiration", func(t *testing.T) {
		w := httptest.NewRecorder()
		deleteCookie(w, "test", "value")

		cookies := w.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("Expected 1 cookie, got %d", len(cookies))
		}

		cookie := cookies[0]
		if cookie.Name != "test" {
			t.Errorf("Expected name 'test', got %s", cookie.Name)
		}

		// Cookie should have past expiration to delete it
		if cookie.Expires.After(time.Now()) {
			t.Error("Delete cookie should have past expiration")
		}
	})

	t.Run("deletes session cookie", func(t *testing.T) {
		w := httptest.NewRecorder()
		deleteCookie(w, CookieSession, "old-session")

		cookies := w.Result().Cookies()
		cookie := cookies[0]

		if cookie.Name != CookieSession {
			t.Errorf("Expected name %s, got %s", CookieSession, cookie.Name)
		}

		if !cookie.Expires.Before(time.Now()) {
			t.Error("Cookie should be expired")
		}
	})

	t.Run("deletes user email cookie", func(t *testing.T) {
		w := httptest.NewRecorder()
		deleteCookie(w, CookieUserEmail, "old@example.com")

		cookies := w.Result().Cookies()
		cookie := cookies[0]

		if cookie.Name != CookieUserEmail {
			t.Errorf("Expected name %s, got %s", CookieUserEmail, cookie.Name)
		}

		if !cookie.Expires.Before(time.Now()) {
			t.Error("Cookie should be expired")
		}
	})

	t.Run("cookie is HttpOnly", func(t *testing.T) {
		w := httptest.NewRecorder()
		deleteCookie(w, "test", "value")

		cookies := w.Result().Cookies()
		cookie := cookies[0]

		if !cookie.HttpOnly {
			t.Error("Delete cookie should still be HttpOnly")
		}
	})
}

func TestCookieConstants(t *testing.T) {
	t.Run("CookieSession constant", func(t *testing.T) {
		if CookieSession != "session" {
			t.Errorf("Expected CookieSession to be 'session', got %s", CookieSession)
		}
	})

	t.Run("CookieUserEmail constant", func(t *testing.T) {
		if CookieUserEmail != "user_email" {
			t.Errorf("Expected CookieUserEmail to be 'user_email', got %s", CookieUserEmail)
		}
	})
}

func TestCookieLifecycle(t *testing.T) {
	// Test full cookie lifecycle: set -> read -> delete
	t.Run("complete cookie lifecycle", func(t *testing.T) {
		// Set cookie
		w := httptest.NewRecorder()
		setCookie(w, "lifecycle", "test-value")

		// Simulate reading the cookie in a new request
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		for _, cookie := range w.Result().Cookies() {
			req.AddCookie(cookie)
		}

		// Read cookie
		value, err := readCookie(req, "lifecycle")
		if err != nil {
			t.Fatalf("Failed to read cookie: %v", err)
		}

		if value != "test-value" {
			t.Errorf("Expected 'test-value', got %s", value)
		}

		// Delete cookie
		w2 := httptest.NewRecorder()
		deleteCookie(w2, "lifecycle", "test-value")

		cookies := w2.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("Expected 1 cookie, got %d", len(cookies))
		}

		// Verify cookie is expired
		if !cookies[0].Expires.Before(time.Now()) {
			t.Error("Cookie should be expired after delete")
		}
	})
}

func TestCookieSecurity(t *testing.T) {
	t.Run("cookies are HttpOnly", func(t *testing.T) {
		w := httptest.NewRecorder()
		setCookie(w, "security", "test")

		cookies := w.Result().Cookies()
		if !cookies[0].HttpOnly {
			t.Error("Cookie must be HttpOnly for security")
		}
	})

	t.Run("cookies have path set", func(t *testing.T) {
		w := httptest.NewRecorder()
		setCookie(w, "path-test", "value")

		cookies := w.Result().Cookies()
		if cookies[0].Path != "/" {
			t.Errorf("Cookie path should be '/', got %s", cookies[0].Path)
		}
	})
}
