package controllers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"

	"anshumanbiswas.com/blog/models"
	"github.com/go-chi/chi/v5"
)

// DeckCraft decks: the deck file is stored in the slides row and edited or shown by the
// vendored DeckCraft web build (static/vendor/deckcraft) through its ?host=parent bridge.

// maxDeckBytes caps an uploaded deck; pictures and video make decks large.
const maxDeckBytes = 100 << 20

// zipMagic starts every .pptx and .deckcraft file (both are zip packages).
var zipMagic = []byte("PK\x03\x04")

// slideGate reports whether the request may see the slide at all (published, or an editor)
// and whether it is past the password gate. The export renderer's token unlocks both.
func (s Slides) slideGate(r *http.Request, slide *models.Slide, user *models.User) (visible, unlocked bool) {
	exportOK := s.hasValidExportToken(r, slide.Slug)
	isEditor := user != nil && models.CanEditSlides(user.Role)
	visible = slide.IsPublished || exportOK || isEditor
	unlocked = slide.PasswordHash == "" || exportOK || isEditor
	if !unlocked {
		cookie, err := r.Cookie(fmt.Sprintf("slide_access_%s", slide.Slug))
		unlocked = err == nil && cookie.Value == "granted"
	}
	return visible, unlocked
}

// ServeDeck streams a DeckCraft deck's file to a viewer who may see the slide.
// GET /slides/{slug}/deck
func (s Slides) ServeDeck(w http.ResponseWriter, r *http.Request) {
	user, _ := s.isUserLoggedIn(r)
	slide, err := s.SlideService.GetBySlug(chi.URLParam(r, "slug"))
	if err != nil || !slide.IsDeck {
		http.Error(w, "Deck not found", http.StatusNotFound)
		return
	}
	if visible, unlocked := s.slideGate(r, slide, user); !visible || !unlocked {
		http.Error(w, "Deck not found", http.StatusNotFound)
		return
	}
	_, data, err := s.SlideService.GetDeck(slide.ID)
	if err != nil {
		log.Printf("ServeDeck %d: %v", slide.ID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if len(data) == 0 {
		http.Error(w, "Deck is empty", http.StatusNotFound)
		return
	}
	w.Header().Set(headerContentType, "application/octet-stream")
	w.Header().Set("Cache-Control", "private, no-cache")
	_, _ = w.Write(data)
}

// SaveDeckFile stores what DeckCraft saved. The page sends only real saves (not exports).
// POST /admin/slides/{slideID}/deck?name=<file name>, body = the file.
func (s Slides) SaveDeckFile(w http.ResponseWriter, r *http.Request) {
	if s.authSlideEditor(r) == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	slideID, err := strconv.Atoi(chi.URLParam(r, "slideID"))
	if err != nil {
		http.Error(w, "Invalid slide ID", http.StatusBadRequest)
		return
	}
	slide, err := s.SlideService.GetByID(slideID)
	if err != nil || !slide.IsDeck {
		http.Error(w, "Deck not found", http.StatusNotFound)
		return
	}
	name, err := deckFileName(r.URL.Query().Get("name"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxDeckBytes))
	if err != nil {
		http.Error(w, "Deck is too large", http.StatusRequestEntityTooLarge)
		return
	}
	if !bytes.HasPrefix(data, zipMagic) {
		http.Error(w, "Not a .pptx or .deckcraft file", http.StatusBadRequest)
		return
	}
	if err := s.SlideService.SaveDeck(slideID, name, data); err != nil {
		log.Printf("SaveDeckFile %d: %v", slideID, err)
		http.Error(w, "Failed to save deck", http.StatusInternalServerError)
		return
	}
	w.Header().Set(headerContentType, "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "name": name, "size": len(data)})
}

// deckFileName keeps a bare .pptx or .deckcraft file name from what the page sent.
func deckFileName(raw string) (string, error) {
	name := path.Base(strings.ReplaceAll(raw, `\`, "/"))
	ext := strings.ToLower(path.Ext(name))
	if (ext != ".pptx" && ext != ".deckcraft") || len(name) > 200 || name == ext {
		return "", errors.New("deck must be a .pptx or .deckcraft file")
	}
	return name, nil
}

// uploadedDeck reads the optional "deck" file (.pptx or .deckcraft) of the new-deck form;
// no file gives an empty name and nil bytes.
func uploadedDeck(r *http.Request) (string, []byte, error) {
	f, h, err := r.FormFile("deck")
	if errors.Is(err, http.ErrMissingFile) || errors.Is(err, http.ErrNotMultipart) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	defer f.Close()
	name, err := deckFileName(h.Filename)
	if err != nil {
		return "", nil, err
	}
	data, err := io.ReadAll(io.LimitReader(f, maxDeckBytes+1))
	if err != nil {
		return "", nil, err
	}
	if len(data) > maxDeckBytes {
		return "", nil, errors.New("deck is too large")
	}
	if !bytes.HasPrefix(data, zipMagic) {
		return "", nil, errors.New("not a .pptx or .deckcraft file")
	}
	return name, data, nil
}
