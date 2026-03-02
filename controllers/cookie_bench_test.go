package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func BenchmarkNewCookie(b *testing.B) {
	expire := time.Now().Add(7 * 24 * time.Hour)
	for i := 0; i < b.N; i++ {
		newCookie("session", "token-value-here", expire)
	}
}

func BenchmarkSetCookie(b *testing.B) {
	w := httptest.NewRecorder()
	for i := 0; i < b.N; i++ {
		setCookie(w, "session", "token-value-here")
	}
}

func BenchmarkReadCookie(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "token-value-here"})
	for i := 0; i < b.N; i++ {
		readCookie(req, "session")
	}
}
