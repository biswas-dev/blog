package controllers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"anshumanbiswas.com/blog/models"
)

func TestDeckFileName(t *testing.T) {
	ok := map[string]string{
		"Talk.pptx":            "Talk.pptx",
		"My Deck.deckcraft":    "My Deck.deckcraft",
		"docs/Talk.PPTX":       "Talk.PPTX",
		`..\..\evil.deckcraft`: "evil.deckcraft",
	}
	for in, want := range ok {
		if got, err := deckFileName(in); err != nil || got != want {
			t.Errorf("deckFileName(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "Talk.pdf", "Talk.png", ".pptx", "Talk.pptx.exe", "../"} {
		if got, err := deckFileName(in); err == nil {
			t.Errorf("deckFileName(%q) = %q; want an error", in, got)
		}
	}
}

func TestSlideGate(t *testing.T) {
	s := Slides{}
	editor := &models.User{Role: models.RoleAdministrator}
	req := func(cookie string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/slides/talk/deck", nil)
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: "slide_access_talk", Value: cookie})
		}
		return r
	}
	cases := []struct {
		name              string
		slide             models.Slide
		user              *models.User
		cookie            string
		visible, unlocked bool
	}{
		{"published public", models.Slide{Slug: "talk", IsPublished: true}, nil, "", true, true},
		{"unpublished visitor", models.Slide{Slug: "talk"}, nil, "", false, true},
		{"unpublished editor", models.Slide{Slug: "talk"}, editor, "", true, true},
		{"password, no cookie", models.Slide{Slug: "talk", IsPublished: true, PasswordHash: "x"}, nil, "", true, false},
		{"password, wrong cookie", models.Slide{Slug: "talk", IsPublished: true, PasswordHash: "x"}, nil, "nope", true, false},
		{"password, granted", models.Slide{Slug: "talk", IsPublished: true, PasswordHash: "x"}, nil, "granted", true, true},
		{"password, editor", models.Slide{Slug: "talk", PasswordHash: "x"}, editor, "", true, true},
	}
	for _, c := range cases {
		v, u := s.slideGate(req(c.cookie), &c.slide, c.user)
		if v != c.visible || u != c.unlocked {
			t.Errorf("%s: slideGate = (%v, %v); want (%v, %v)", c.name, v, u, c.visible, c.unlocked)
		}
	}
}

func TestPaperPDFPath(t *testing.T) {
	sep := string(filepath.Separator)
	if got, ok := paperPDFPath("/static/uploads/papers/attn/abc.pdf"); !ok || got != filepath.Join("static", "uploads", "papers", "attn", "abc.pdf") {
		t.Errorf("valid URL: got %q, %v", got, ok)
	}
	for _, bad := range []string{"", "/static/uploads/papers/../../main.go", "/static/uploads/avatars/a.pdf", "/static/uploads/papers/a.png", "/etc/passwd", "static/uploads/papers" + sep} {
		if got, ok := paperPDFPath(bad); ok {
			t.Errorf("paperPDFPath(%q) = %q accepted; want rejected", bad, got)
		}
	}
}

func TestReplacePaperPDFKeepsFirstOriginal(t *testing.T) {
	dir := t.TempDir()
	pdf := filepath.Join(dir, "paper.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-original"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"%PDF-v1", "%PDF-v2"} {
		if err := replacePaperPDF(pdf, []byte(v)); err != nil {
			t.Fatal(err)
		}
	}
	if b, _ := os.ReadFile(pdf); string(b) != "%PDF-v2" {
		t.Errorf("pdf = %q; want the last save", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "paper.orig.pdf")); string(b) != "%PDF-original" {
		t.Errorf("orig = %q; want the first upload", b)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".save-*")); len(left) != 0 {
		t.Errorf("temp files left behind: %v", left)
	}
}
