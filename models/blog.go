package models

import (
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"time"

	gowiki "github.com/anchoo2kewl/go-wiki"
)

type BlogService struct {
	DB   *sql.DB
	wiki *gowiki.Wiki
}

func NewBlogService(db *sql.DB) *BlogService {
	return &BlogService{
		DB:   db,
		wiki: gowiki.New(gowiki.WithDrawBasePath("/draw")),
	}
}

func (bs *BlogService) GetBlogPostBySlug(slug string) (*Post, error) {
	var post Post

	const query = `
		SELECT p.post_id, p.user_id, u.username, COALESCE(NULLIF(u.full_name, ''), u.username),
		       COALESCE(u.profile_picture_url, ''),
		       p.category_id, p.title, p.content, p.slug, p.publication_date,
		       p.last_edit_date, p.is_published, p.featured_image_url, p.created_at, p.featured
		FROM Posts p
		JOIN Users u ON u.user_id = p.user_id
		WHERE p.slug = $1
		LIMIT 1`
	err := bs.DB.QueryRow(query, slug).Scan(
		&post.ID,
		&post.UserID,
		&post.Username,
		&post.AuthorDisplayName,
		&post.AuthorAvatarURL,
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
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("post not found")
	}
	if err != nil {
		return nil, fmt.Errorf("db query failed: %w", err)
	}

	// Display-friendly dates
	if t, err := time.Parse(time.RFC3339, post.CreatedAt); err == nil {
		post.CreatedAt = t.Format("January 2, 2006")
	}
	if post.PublicationDate != "" {
		if t, err := time.Parse(time.RFC3339, post.PublicationDate); err == nil {
			post.PublicationDate = t.Format("January 2, 2006")
		}
	} else {
		post.PublicationDate = post.CreatedAt
	}
	if post.LastEditDate != "" {
		if t, err := time.Parse(time.RFC3339, post.LastEditDate); err == nil {
			post.LastEditDate = t.Format("January 2, 2006")
		}
	}

	// Render (view-only draw embeds for published content)
	html := stripDrawEditMode(bs.wiki.RenderContent(post.Content))
	post.ContentHTML = template.HTML(html)

	return &post, nil
}

// RenderPreviewDebug returns intermediate stages for debugging.
func (bs *BlogService) RenderPreviewDebug(raw string) (final string, stages map[string]string) {
	return bs.wiki.RenderContentDebug(raw)
}
