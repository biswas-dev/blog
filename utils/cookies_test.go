package utils

import (
	"net/http"
	"testing"
)

func TestReadCookie_FindsCookie(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	r.AddCookie(&http.Cookie{Name: CookieSession, Value: "tok123"})
	val, err := ReadCookie(r, CookieSession)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "tok123" {
		t.Fatalf("expected 'tok123', got %q", val)
	}
}

func TestReadCookie_MissingCookie(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if _, err := ReadCookie(r, CookieUserEmail); err == nil {
		t.Fatalf("expected error for missing cookie")
	}
}
