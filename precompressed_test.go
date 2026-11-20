package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestServePrecompressed(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "app_bg.wasm")
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	_, _ = zw.Write([]byte("wasm-bytes"))
	_ = zw.Close()
	if err := os.WriteFile(name+".gz", gz.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/static/vendor/x/app_bg.wasm", nil)
	req.Header.Set("Accept-Encoding", "gzip, br")
	w := httptest.NewRecorder()
	if !servePrecompressed(w, req, name) {
		t.Fatal("expected the .gz to be served")
	}
	if w.Header().Get("Content-Encoding") != "gzip" || w.Header().Get("Content-Type") != "application/wasm" {
		t.Fatalf("headers = %v", w.Header())
	}
	zr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := io.ReadAll(zr); string(got) != "wasm-bytes" {
		t.Fatalf("body = %q", got)
	}

	plain := httptest.NewRequest("GET", "/static/vendor/x/app_bg.wasm", nil)
	if servePrecompressed(httptest.NewRecorder(), plain, name) {
		t.Fatal("a client without gzip must get the plain file")
	}
	if servePrecompressed(httptest.NewRecorder(), req, filepath.Join(dir, "missing.js")) {
		t.Fatal("no .gz must fall through")
	}
	if servePrecompressed(httptest.NewRecorder(), req, dir+"/../x.wasm") {
		t.Fatal("a path with .. must fall through")
	}
}
