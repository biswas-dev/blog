package controllers

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

const (
	CookieSession   = "session"
	CookieUserEmail = "user_email"
)

func newCookie(name, value string, expire time.Time) *http.Cookie {
	cookie := http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   os.Getenv("APP_ENV") != "development",
		SameSite: http.SameSiteLaxMode,
		Expires:  expire,
	}
	return &cookie
}

func setCookie(w http.ResponseWriter, name, value string) {
	expire := time.Now().Add(7 * 24 * time.Hour)
	cookie := newCookie(name, value, expire)
	http.SetCookie(w, cookie)
}

func readCookie(r *http.Request, name string) (string, error) {
	c, err := r.Cookie(name)
	if err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return c.Value, nil
}

func deleteCookie(w http.ResponseWriter, name, value string) {
	expire := time.Now().Add(-7 * 24 * time.Hour)
	cookie := newCookie(name, value, expire)
	http.SetCookie(w, cookie)
}
