// seed-images scans existing posts for Cloudinary image URLs and creates
// image_metadata records so they appear in the image manager.
//
// Usage: go run ./cmd/seed-images
//
// Requires PG_USER, PG_PASSWORD, PG_DB, PG_HOST, PG_PORT env vars.
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	_ "github.com/lib/pq"

	"anshumanbiswas.com/blog/models"
)

func main() {
	db, err := openDB()
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	metaSvc := &models.ImageMetadataService{DB: db}

	// Fetch all posts
	rows, err := db.Query(`SELECT id, content, featured_image_url FROM posts`)
	if err != nil {
		log.Fatalf("query posts: %v", err)
	}
	defer rows.Close()

	seen := map[string]bool{}
	var all []imageMatch
	var postCount int

	for rows.Next() {
		var id int
		var content, featuredURL string
		var featuredPtr *string
		if err := rows.Scan(&id, &content, &featuredPtr); err != nil {
			log.Printf("scan post %d: %v", id, err)
			continue
		}
		if featuredPtr != nil {
			featuredURL = *featuredPtr
		}
		postCount++

		// Extract Cloudinary URLs from content
		extracted := extractCloudinaryImages(content)

		// Featured image
		if featuredURL != "" && isCloudinaryURL(featuredURL) {
			extracted = append(extracted, imageMatch{url: featuredURL, alt: "Featured image", caption: ""})
		}

		for _, img := range extracted {
			if seen[img.url] {
				continue
			}
			seen[img.url] = true
			all = append(all, img)
		}
	}
	if err := rows.Err(); err != nil {
		log.Fatalf("rows: %v", err)
	}

	// Upsert each image
	var created int
	for _, img := range all {
		_, err := metaSvc.Upsert(img.url, img.alt, "", img.caption)
		if err != nil {
			log.Printf("upsert %s: %v", img.url, err)
			continue
		}
		created++
	}

	fmt.Printf("Seeded %d images from %d posts\n", created, postCount)
}

var (
	reMarkdownImg = regexp.MustCompile(`!\[([^\]]*)\]\((https://res\.cloudinary\.com/[^)]+)\)`)
	reHTMLImg     = regexp.MustCompile(`<img[^>]*src="(https://res\.cloudinary\.com/[^"]+)"[^>]*alt="([^"]*)"[^>]*>`)
	reFigure      = regexp.MustCompile(`(?s)<figure[^>]*>.*?<img[^>]*src="(https://res\.cloudinary\.com/[^"]+)".*?</figure>`)
	reFigcaption  = regexp.MustCompile(`<figcaption>(.*?)</figcaption>`)
	reImgAlt      = regexp.MustCompile(`alt="([^"]*)"`)
)

type imageMatch struct {
	url     string
	alt     string
	caption string
}

func extractCloudinaryImages(content string) []imageMatch {
	var results []imageMatch

	// Markdown: ![alt](cloudinary-url)
	for _, m := range reMarkdownImg.FindAllStringSubmatch(content, -1) {
		results = append(results, imageMatch{url: m[2], alt: m[1], caption: ""})
	}

	// HTML img: <img src="cloudinary-url" alt="...">
	for _, m := range reHTMLImg.FindAllStringSubmatch(content, -1) {
		results = append(results, imageMatch{url: m[1], alt: m[2], caption: ""})
	}

	// Figure blocks with figcaption
	for _, m := range reFigure.FindAllStringSubmatch(content, -1) {
		url := m[1]
		block := m[0]
		var alt, caption string
		if am := reImgAlt.FindStringSubmatch(block); len(am) > 1 {
			alt = am[1]
		}
		if cm := reFigcaption.FindStringSubmatch(block); len(cm) > 1 {
			caption = cm[1]
		}
		results = append(results, imageMatch{url: url, alt: alt, caption: caption})
	}

	return results
}

func isCloudinaryURL(u string) bool {
	return strings.Contains(u, "res.cloudinary.com")
}

func openDB() (*sql.DB, error) {
	user := os.Getenv("PG_USER")
	pass := os.Getenv("PG_PASSWORD")
	name := os.Getenv("PG_DB")
	host := os.Getenv("PG_HOST")
	port := os.Getenv("PG_PORT")
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "5432"
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, pass, name)
	return sql.Open("postgres", dsn)
}
