package models

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/lib/pq"
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
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Batch-load categories for all post results
	if len(results) > 0 {
		slugs := make([]string, len(results))
		for i, r := range results {
			slugs[i] = r.Slug
		}
		catMap, _ := ss.batchLoadPostCategories(ctx, slugs)
		for i := range results {
			results[i].Categories = catMap[results[i].Slug]
		}
	}

	return results, nil
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
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Batch-load categories for all slide results
	if len(results) > 0 {
		slugs := make([]string, len(results))
		for i, r := range results {
			slugs[i] = r.Slug
		}
		catMap, _ := ss.batchLoadSlideCategories(ctx, slugs)
		for i := range results {
			results[i].Categories = catMap[results[i].Slug]
		}
	}

	return results, nil
}

// batchLoadPostCategories loads category names for multiple posts by slug in a single query.
func (ss *SearchService) batchLoadPostCategories(ctx context.Context, slugs []string) (map[string][]string, error) {
	query := `
		SELECT p.slug, c.category_name
		FROM categories c
		JOIN post_categories pc ON c.category_id = pc.category_id
		JOIN posts p ON p.post_id = pc.post_id
		WHERE p.slug = ANY($1)
	`
	rows, err := ss.DB.QueryContext(ctx, query, pq.Array(slugs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var slug, name string
		if err := rows.Scan(&slug, &name); err != nil {
			return nil, err
		}
		result[slug] = append(result[slug], name)
	}
	return result, rows.Err()
}

// batchLoadSlideCategories loads category names for multiple slides by slug in a single query.
func (ss *SearchService) batchLoadSlideCategories(ctx context.Context, slugs []string) (map[string][]string, error) {
	query := `
		SELECT s.slug, c.category_name
		FROM categories c
		JOIN Slide_Categories sc ON c.category_id = sc.category_id
		JOIN Slides s ON s.slide_id = sc.slide_id
		WHERE s.slug = ANY($1)
	`
	rows, err := ss.DB.QueryContext(ctx, query, pq.Array(slugs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var slug, name string
		if err := rows.Scan(&slug, &name); err != nil {
			return nil, err
		}
		result[slug] = append(result[slug], name)
	}
	return result, rows.Err()
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
