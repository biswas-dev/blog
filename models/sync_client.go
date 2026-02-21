package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SyncClient handles syncing posts between blog instances
type SyncClient struct {
	PostService           *PostService
	CategoryService       *CategoryService
	ExternalSystemService *ExternalSystemService
}

// remotePost represents a post as returned by the remote /api/posts/ endpoint
type remotePost struct {
	ID               int        `json:"ID"`
	UserID           int        `json:"UserID"`
	CategoryID       int        `json:"CategoryID"`
	Title            string     `json:"Title"`
	Content          string     `json:"Content"`
	Slug             string     `json:"Slug"`
	PublicationDate  string     `json:"PublicationDate"`
	LastEditDate     string     `json:"LastEditDate"`
	IsPublished      bool       `json:"IsPublished"`
	Featured         bool       `json:"Featured"`
	FeaturedImageURL string     `json:"FeaturedImageURL"`
	CreatedAt        string     `json:"CreatedAt"`
	Categories       []Category `json:"categories,omitempty"`
}

// buildRequest creates an HTTP request with auth and custom headers
func buildRequest(method, url string, body io.Reader, system *ExternalSystem) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	if system.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+system.APIKey)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	for _, h := range system.CustomHeaders {
		if h.Key != "" {
			req.Header.Set(h.Key, h.Value)
		}
	}

	return req, nil
}

// httpClient returns an HTTP client with a given timeout
func httpClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

// TestConnection verifies connectivity to a remote blog instance
func (sc *SyncClient) TestConnection(system *ExternalSystem) error {
	url := strings.TrimRight(system.BaseURL, "/") + "/api/posts/"

	req, err := buildRequest("GET", url, nil, system)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	client := httpClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("remote returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// fetchRemotePosts fetches posts from the remote system
func (sc *SyncClient) fetchRemotePosts(system *ExternalSystem) ([]remotePost, error) {
	url := strings.TrimRight(system.BaseURL, "/") + "/api/posts/"

	req, err := buildRequest("GET", url, nil, system)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	client := httpClient(30 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch remote posts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("remote returned status %d: %s", resp.StatusCode, string(body))
	}

	// The /api/posts/ endpoint returns a PostsList with a "Posts" field
	var result struct {
		Posts []remotePost `json:"Posts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.Posts, nil
}

// getLocalSlugs returns a set of all local post slugs
func (sc *SyncClient) getLocalSlugs() (map[string]bool, error) {
	posts, err := sc.PostService.GetAllPosts()
	if err != nil {
		return nil, fmt.Errorf("get local posts: %w", err)
	}

	slugs := make(map[string]bool, len(posts.Posts))
	for _, p := range posts.Posts {
		slugs[p.Slug] = true
	}
	return slugs, nil
}

// PreviewPull shows which remote posts would be pulled (new vs existing locally)
func (sc *SyncClient) PreviewPull(system *ExternalSystem) (*SyncPreview, error) {
	remotePosts, err := sc.fetchRemotePosts(system)
	if err != nil {
		return nil, err
	}

	localSlugs, err := sc.getLocalSlugs()
	if err != nil {
		return nil, err
	}

	preview := &SyncPreview{Direction: "pull"}
	for _, rp := range remotePosts {
		item := SyncItem{Title: rp.Title, Slug: rp.Slug}
		if localSlugs[rp.Slug] {
			item.Status = "exists"
			preview.SkipCount++
		} else {
			item.Status = "new"
			preview.NewCount++
		}
		preview.Items = append(preview.Items, item)
	}

	return preview, nil
}

// PreviewPush shows which local posts would be pushed (new vs existing remotely)
func (sc *SyncClient) PreviewPush(system *ExternalSystem) (*SyncPreview, error) {
	remotePosts, err := sc.fetchRemotePosts(system)
	if err != nil {
		return nil, err
	}

	remoteSlugs := make(map[string]bool, len(remotePosts))
	for _, rp := range remotePosts {
		remoteSlugs[rp.Slug] = true
	}

	localPosts, err := sc.PostService.GetAllPosts()
	if err != nil {
		return nil, fmt.Errorf("get local posts: %w", err)
	}

	preview := &SyncPreview{Direction: "push"}
	for _, lp := range localPosts.Posts {
		item := SyncItem{Title: lp.Title, Slug: lp.Slug}
		if remoteSlugs[lp.Slug] {
			item.Status = "exists"
			preview.SkipCount++
		} else {
			item.Status = "new"
			preview.NewCount++
		}
		preview.Items = append(preview.Items, item)
	}

	return preview, nil
}

// ExecutePull pulls new posts from the remote system and creates them locally
func (sc *SyncClient) ExecutePull(system *ExternalSystem, userID int) (*SyncResult, error) {
	remotePosts, err := sc.fetchRemotePosts(system)
	if err != nil {
		return nil, err
	}

	localSlugs, err := sc.getLocalSlugs()
	if err != nil {
		return nil, err
	}

	result := &SyncResult{Direction: "pull"}

	for _, rp := range remotePosts {
		if localSlugs[rp.Slug] {
			result.ItemsSkipped++
			continue
		}

		// Create the post locally with the pulling user as author
		_, err := sc.PostService.Create(
			userID,
			rp.CategoryID,
			rp.Title,
			rp.Content,
			rp.IsPublished,
			rp.Featured,
			rp.FeaturedImageURL,
			rp.Slug,
		)
		if err != nil {
			result.ItemsFailed++
			if result.ErrorMessage == "" {
				result.ErrorMessage = fmt.Sprintf("failed to create '%s': %v", rp.Slug, err)
			}
			continue
		}

		result.ItemsSynced++
	}

	return result, nil
}

// ExecutePush pushes local posts to the remote system
func (sc *SyncClient) ExecutePush(system *ExternalSystem) (*SyncResult, error) {
	remotePosts, err := sc.fetchRemotePosts(system)
	if err != nil {
		return nil, err
	}

	remoteSlugs := make(map[string]bool, len(remotePosts))
	for _, rp := range remotePosts {
		remoteSlugs[rp.Slug] = true
	}

	localPosts, err := sc.PostService.GetAllPosts()
	if err != nil {
		return nil, fmt.Errorf("get local posts: %w", err)
	}

	result := &SyncResult{Direction: "push"}
	client := httpClient(30 * time.Second)
	pushURL := strings.TrimRight(system.BaseURL, "/") + "/api/posts/"

	for _, lp := range localPosts.Posts {
		if remoteSlugs[lp.Slug] {
			result.ItemsSkipped++
			continue
		}

		payload := map[string]interface{}{
			"UserID":           lp.UserID,
			"CategoryID":       lp.CategoryID,
			"Title":            lp.Title,
			"Content":          lp.Content,
			"Slug":             lp.Slug,
			"IsPublished":      lp.IsPublished,
			"Featured":         lp.Featured,
			"FeaturedImageURL": lp.FeaturedImageURL,
		}

		body, err := json.Marshal(payload)
		if err != nil {
			result.ItemsFailed++
			continue
		}

		req, err := buildRequest("POST", pushURL, bytes.NewReader(body), system)
		if err != nil {
			result.ItemsFailed++
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			result.ItemsFailed++
			if result.ErrorMessage == "" {
				result.ErrorMessage = fmt.Sprintf("failed to push '%s': %v", lp.Slug, err)
			}
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
			result.ItemsSynced++
		} else {
			result.ItemsFailed++
			if result.ErrorMessage == "" {
				result.ErrorMessage = fmt.Sprintf("push '%s' returned status %d", lp.Slug, resp.StatusCode)
			}
		}
	}

	return result, nil
}
