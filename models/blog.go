package models

import (
	"database/sql"
	"fmt"
	"html/template"
	"time"

	render "anshumanbiswas.com/blog/internal/render"
)

type BlogService struct {
	DB       *sql.DB
	renderer *render.Renderer
}

func NewBlogService(db *sql.DB) *BlogService {
	return &BlogService{
		DB:       db,
		renderer: render.NewRenderer(render.DefaultOptions()),
	}
}

func (bs *BlogService) GetBlogPostBySlug(slug string) (*Post, error) {
	post := Post{}

	const query = `SELECT post_id, user_id, category_id, title, content, slug, publication_date, last_edit_date, is_published, featured_image_url, created_at, featured FROM posts WHERE slug = $1 LIMIT 1`
	rows, err := bs.DB.Query(query, slug)
	if err != nil {
		return nil, fmt.Errorf("db query failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		err := rows.Scan(
			&post.ID,
			&post.UserID,
			&post.CategoryID,
			&post.Title,
			&post.Content,
			&post.Slug,
			&post.PublicationDate,
			&post.LastEditDate,
			&post.IsPublished,
			&post.FeaturedImageURL,
			&post.CreatedAt,
			&post.Featured,
		)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		// Display-friendly dates (assumes RFC3339 strings)
		if post.CreatedAt != "" {
			if t, err := time.Parse(time.RFC3339, post.CreatedAt); err == nil {
				post.CreatedAt = t.Format("January 2, 2006")
			}
		}
		if post.PublicationDate != "" {
			if pt, err := time.Parse(time.RFC3339, post.PublicationDate); err == nil {
				post.PublicationDate = pt.Format("January 2, 2006")
			}
		} else {
			post.PublicationDate = post.CreatedAt
		}
		if post.LastEditDate != "" {
			if lt, err := time.Parse(time.RFC3339, post.LastEditDate); err == nil {
				post.LastEditDate = lt.Format("January 2, 2006")
			}
		}

		// --- Render ---
		html := bs.renderer.Render(post.Content)
		post.ContentHTML = template.HTML(html)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration failed: %w", err)
	}

	if post.ID == 0 {
		return nil, fmt.Errorf("post not found")
	}

	return &post, nil
}

// If you want to expose a preview that returns intermediate stages for debugging:
func (bs *BlogService) RenderPreviewDebug(raw string) (final string, stages map[string]string) {
	return bs.renderer.RenderWithDebug(raw, true)
}
