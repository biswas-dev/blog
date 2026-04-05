package controllers

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// IndexNowKey is the shared secret for IndexNow API authentication.
// The key file must be served at https://<host>/<key>.txt
const IndexNowKey = "1167a900344b4773d7e4d2b864a11b24"

// IndexNowHost is the production hostname used for IndexNow submissions.
const IndexNowHost = "anshumanbiswas.com"

// PingIndexNow notifies search engines (Bing, Yandex, Seznam, Naver) that a URL
// has been created or updated. Runs in a goroutine — never blocks the caller.
// Also pings Google via sitemap ping as a legacy fallback.
func PingIndexNow(urls ...string) {
	if len(urls) == 0 {
		return
	}
	go func() {
		pingIndexNow(urls)
		pingGoogleSitemap()
	}()
}

func pingIndexNow(urls []string) {
	body := map[string]interface{}{
		"host":        IndexNowHost,
		"key":         IndexNowKey,
		"keyLocation": "https://" + IndexNowHost + "/" + IndexNowKey + ".txt",
		"urlList":     urls,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return
	}
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", "https://api.indexnow.org/indexnow", bytes.NewReader(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := client.Do(req)
	if err != nil {
		slog.Warn("indexnow ping failed", "error", err, "urls", urls)
		return
	}
	defer resp.Body.Close()
	slog.Info("indexnow pinged", "status", resp.StatusCode, "urls", urls)
}

func pingGoogleSitemap() {
	// Google deprecated this in 2023 but it still works for Bing and some others.
	// Essentially a best-effort notification.
	client := &http.Client{Timeout: 10 * time.Second}
	sitemapURL := "https://" + IndexNowHost + "/sitemap.xml"
	endpoints := []string{
		"https://www.bing.com/ping?sitemap=" + sitemapURL,
	}
	for _, url := range endpoints {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		slog.Info("sitemap pinged", "url", url, "status", resp.StatusCode)
	}
}

// IndexNowKeyHandler serves the IndexNow key file at /{key}.txt.
// Search engines fetch this to verify the caller owns the domain.
func IndexNowKeyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write([]byte(IndexNowKey))
}
