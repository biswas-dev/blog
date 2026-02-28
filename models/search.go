package models

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"
)

// SearchResult represents a single search result (post or slide)
type SearchResult struct {
	Type       string   `json:"type"`
	Title      string   `json:"title"`
	Slug       string   `json:"slug"`
	Excerpt    string   `json:"excerpt"`
	Date       string   `json:"date"`
	Categories []string `json:"categories"`
	Rank       float64  `json:"rank"`
}

// SearchResponse is the API response for search queries
type SearchResponse struct {
	Query      string         `json:"query"`
	TotalCount int            `json:"total_count"`
	Posts      []SearchResult `json:"posts"`
	Slides     []SearchResult `json:"slides"`
}

// SearchService provides full-text search across posts and slides
type SearchService struct {
	DB *sql.DB
}

// Search performs a parallel full-text search across posts and slides
func (ss *SearchService) Search(ctx context.Context, query string, limit int) (*SearchResponse, error) {
	if query == "" {
		return &SearchResponse{Query: query, Posts: []SearchResult{}, Slides: []SearchResult{}}, nil
	}

	var (
		posts  []SearchResult
		slides []SearchResult
		postErr, slideErr error
		wg sync.WaitGroup
	)

	wg.Add(2)

	// Search posts in parallel
	go func() {
		defer wg.Done()
		posts, postErr = ss.searchPosts(ctx, query, limit)
	}()

	// Search slides in parallel
	go func() {
		defer wg.Done()
		slides, slideErr = ss.searchSlides(ctx, query, limit)
	}()

	wg.Wait()

	if postErr != nil {
		return nil, fmt.Errorf("search posts: %w", postErr)
	}
	if slideErr != nil {
		return nil, fmt.Errorf("search slides: %w", slideErr)
	}

	if posts == nil {
		posts = []SearchResult{}
	}
	if slides == nil {
		slides = []SearchResult{}
	}

	return &SearchResponse{
		Query:      query,
		TotalCount: len(posts) + len(slides),
		Posts:      posts,
		Slides:     slides,
	}, nil
}

// searchPosts searches published posts using full-text search
func (ss *SearchService) searchPosts(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	sqlQuery := `
		SELECT
			p.title,
			p.slug,
			ts_headline('english', p.content, plainto_tsquery('english', $1),
				'StartSel=<mark>, StopSel=</mark>, MaxWords=35, MinWords=15, MaxFragments=1') AS excerpt,
			COALESCE(p.publication_date::text, p.created_at::text) AS date,
			ts_rank(p.search_vector, plainto_tsquery('english', $1)) AS rank
		FROM posts p
		WHERE p.is_published = true
			AND p.search_vector @@ plainto_tsquery('english', $1)
		ORDER BY rank DESC
		LIMIT $2
	`

	rows, err := ss.DB.QueryContext(ctx, sqlQuery, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		r.Type = "post"
		if err := rows.Scan(&r.Title, &r.Slug, &r.Excerpt, &r.Date, &r.Rank); err != nil {
			return nil, err
		}

		// Load categories for this post
		r.Categories, _ = ss.loadPostCategories(ctx, r.Slug)

		results = append(results, r)
	}

	return results, rows.Err()
}

// searchSlides searches published slides using full-text search
func (ss *SearchService) searchSlides(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	sqlQuery := `
		SELECT
			s.title,
			s.slug,
			ts_headline('english', s.search_content, plainto_tsquery('english', $1),
				'StartSel=<mark>, StopSel=</mark>, MaxWords=35, MinWords=15, MaxFragments=1') AS excerpt,
			s.created_at::text AS date,
			ts_rank(s.search_vector, plainto_tsquery('english', $1)) AS rank
		FROM Slides s
		WHERE s.is_published = true
			AND s.search_vector @@ plainto_tsquery('english', $1)
		ORDER BY rank DESC
		LIMIT $2
	`

	rows, err := ss.DB.QueryContext(ctx, sqlQuery, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		r.Type = "slide"
		if err := rows.Scan(&r.Title, &r.Slug, &r.Excerpt, &r.Date, &r.Rank); err != nil {
			return nil, err
		}

		// Load categories for this slide
		r.Categories, _ = ss.loadSlideCategories(ctx, r.Slug)

		results = append(results, r)
	}

	return results, rows.Err()
}

// loadPostCategories loads category names for a post by slug
func (ss *SearchService) loadPostCategories(ctx context.Context, slug string) ([]string, error) {
	query := `
		SELECT c.category_name
		FROM categories c
		JOIN post_categories pc ON c.category_id = pc.category_id
		JOIN posts p ON p.post_id = pc.post_id
		WHERE p.slug = $1
	`
	rows, err := ss.DB.QueryContext(ctx, query, slug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cats []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		cats = append(cats, name)
	}
	return cats, rows.Err()
}

// loadSlideCategories loads category names for a slide by slug
func (ss *SearchService) loadSlideCategories(ctx context.Context, slug string) ([]string, error) {
	query := `
		SELECT c.category_name
		FROM categories c
		JOIN Slide_Categories sc ON c.category_id = sc.category_id
		JOIN Slides s ON s.slide_id = sc.slide_id
		WHERE s.slug = $1
	`
	rows, err := ss.DB.QueryContext(ctx, query, slug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cats []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		cats = append(cats, name)
	}
	return cats, rows.Err()
}

// BackfillSlideContent reads HTML files for all slides and updates search_content
func (ss *SearchService) BackfillSlideContent() {
	rows, err := ss.DB.Query(`SELECT slide_id, content_file_path FROM Slides`)
	if err != nil {
		log.Printf("search: failed to query slides for backfill: %v", err)
		return
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var id int
		var filePath string
		if err := rows.Scan(&id, &filePath); err != nil {
			log.Printf("search: failed to scan slide row: %v", err)
			continue
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			log.Printf("search: failed to read slide file %s: %v", filePath, err)
			continue
		}

		plainText := stripHTML(string(content))

		_, err = ss.DB.Exec(`UPDATE Slides SET search_content = $1 WHERE slide_id = $2`, plainText, id)
		if err != nil {
			log.Printf("search: failed to update slide %d search_content: %v", id, err)
			continue
		}
		count++
	}

	if count > 0 {
		log.Printf("search: backfilled search_content for %d slides", count)
	}
}
