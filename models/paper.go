package models

import (
	"database/sql"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/lib/pq"
)

type PapersList struct {
	Papers []Paper
}

type Paper struct {
	ID                int
	UserID            int
	Username          string
	AuthorDisplayName string
	AuthorAvatarURL   string
	Title             string
	Slug              string
	PaperAuthors      string // comma-separated paper author names
	Abstract          string
	PaperYear         int
	Conference        string
	DOI               string
	ArxivID           string
	PDFFileURL        string // internal path, never served publicly
	PDFFileSize       int64
	CoverImageURL     string
	Content           string
	ContentHTML       template.HTML
	Description       string
	MyNotes           string
	MyNotesHTML       template.HTML
	Rating            float64
	IsPublished       bool
	PublicationDate   string
	LastEditDate      string
	CreatedAt         string
	UpdatedAt         string
	ReadingTime       int                 `json:"reading_time,omitempty"`
	ResearchAreas     []PaperResearchArea `json:"research_areas,omitempty"`
}

type PaperService struct {
	DB *sql.DB
}

const paperBaseSelect = `
	SELECT p.paper_id, p.user_id, u.username, COALESCE(NULLIF(u.full_name, ''), u.username),
	       COALESCE(u.profile_picture_url, ''),
	       p.title, p.slug, COALESCE(p.paper_authors, ''), COALESCE(p.abstract, ''),
	       COALESCE(p.paper_year, 0), COALESCE(p.conference, ''),
	       COALESCE(p.doi, ''), COALESCE(p.arxiv_id, ''),
	       COALESCE(p.pdf_file_url, ''), COALESCE(p.pdf_file_size, 0),
	       COALESCE(p.cover_image_url, ''),
	       p.content, COALESCE(p.description, ''), COALESCE(p.my_notes, ''),
	       COALESCE(p.rating, 0),
	       p.is_published, p.publication_date, p.last_edit_date, p.created_at, p.updated_at
	FROM papers p
	JOIN Users u ON p.user_id = u.user_id`

// scanPaper scans a single paper row from the database.
func scanPaper(scanner interface{ Scan(...interface{}) error }) (Paper, error) {
	var paper Paper
	err := scanner.Scan(
		&paper.ID, &paper.UserID, &paper.Username, &paper.AuthorDisplayName,
		&paper.AuthorAvatarURL,
		&paper.Title, &paper.Slug, &paper.PaperAuthors, &paper.Abstract,
		&paper.PaperYear, &paper.Conference,
		&paper.DOI, &paper.ArxivID,
		&paper.PDFFileURL, &paper.PDFFileSize,
		&paper.CoverImageURL,
		&paper.Content, &paper.Description, &paper.MyNotes,
		&paper.Rating,
		&paper.IsPublished, &paper.PublicationDate, &paper.LastEditDate, &paper.CreatedAt, &paper.UpdatedAt,
	)
	return paper, err
}

// Create creates a new paper and returns it.
func (ps *PaperService) Create(userID int, title, slug, paperAuthors, abstract string, paperYear int, conference, doi, arxivID, pdfFileURL string, pdfFileSize int64, coverImageURL, content, description, myNotes string, rating float64, isPublished bool, areaIDs []int) (*Paper, error) {
	if slug == "" {
		slug = generateSlug(title)
	} else {
		slug = sanitizeSlug(slug)
	}

	now := time.Now()

	var paper Paper
	err := ps.DB.QueryRow(`
		INSERT INTO papers (user_id, title, slug, paper_authors, abstract, paper_year, conference, doi, arxiv_id, pdf_file_url, pdf_file_size, cover_image_url, content, description, my_notes, rating, is_published, publication_date, last_edit_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
		RETURNING paper_id, created_at, updated_at`,
		userID, title, slug, paperAuthors, abstract, paperYear, conference, doi, arxivID, pdfFileURL, pdfFileSize, coverImageURL, content, description, myNotes, rating, isPublished, now, now, now, now,
	).Scan(&paper.ID, &paper.CreatedAt, &paper.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create paper: %w", err)
	}

	paper.UserID = userID
	paper.Title = title
	paper.Slug = slug
	paper.PaperAuthors = paperAuthors
	paper.Abstract = abstract
	paper.PaperYear = paperYear
	paper.Conference = conference
	paper.DOI = doi
	paper.ArxivID = arxivID
	paper.PDFFileURL = pdfFileURL
	paper.PDFFileSize = pdfFileSize
	paper.CoverImageURL = coverImageURL
	paper.Content = content
	paper.Description = description
	paper.MyNotes = myNotes
	paper.Rating = rating
	paper.IsPublished = isPublished
	paper.PublicationDate = now.Format(friendlyDateFormat)
	paper.LastEditDate = now.Format(friendlyDateFormat)

	if len(areaIDs) > 0 {
		if err := ps.AddAreas(paper.ID, areaIDs); err != nil {
			return &paper, nil // paper created, areas failed — non-fatal
		}
	}

	return &paper, nil
}

// Update updates an existing paper.
func (ps *PaperService) Update(paperID int, title, slug, paperAuthors, abstract string, paperYear int, conference, doi, arxivID, pdfFileURL string, pdfFileSize int64, coverImageURL, content, description, myNotes string, rating float64, isPublished bool, areaIDs []int) error {
	if slug == "" {
		slug = generateSlug(title)
	} else {
		slug = sanitizeSlug(slug)
	}

	_, err := ps.DB.Exec(`
		UPDATE papers SET title=$1, slug=$2, paper_authors=$3, abstract=$4, paper_year=$5, conference=$6, doi=$7, arxiv_id=$8, pdf_file_url=$9, pdf_file_size=$10, cover_image_url=$11, content=$12, description=$13, my_notes=$14, rating=$15, is_published=$16, last_edit_date=$17, updated_at=$18
		WHERE paper_id=$19`,
		title, slug, paperAuthors, abstract, paperYear, conference, doi, arxivID, pdfFileURL, pdfFileSize, coverImageURL, content, description, myNotes, rating, isPublished, time.Now(), time.Now(), paperID)
	if err != nil {
		return fmt.Errorf("update paper: %w", err)
	}
	if len(areaIDs) > 0 {
		_ = ps.UpdateAreas(paperID, areaIDs)
	}
	return nil
}

// Delete removes a paper by ID.
func (ps *PaperService) Delete(paperID int) error {
	_, err := ps.DB.Exec(`DELETE FROM papers WHERE paper_id = $1`, paperID)
	if err != nil {
		return fmt.Errorf("delete paper: %w", err)
	}
	return nil
}

// GetByID retrieves a paper by its ID.
func (ps *PaperService) GetByID(id int) (*Paper, error) {
	paper, err := scanPaper(ps.DB.QueryRow(paperBaseSelect+` WHERE p.paper_id = $1`, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("paper not found")
		}
		return nil, fmt.Errorf("get paper by id: %w", err)
	}

	formatPaperDates(&paper)

	papers := []Paper{paper}
	if err := ps.loadAreasForPapers(papers); err != nil {
		return nil, fmt.Errorf("load research areas: %w", err)
	}
	paper.ResearchAreas = papers[0].ResearchAreas

	return &paper, nil
}

// GetBySlug retrieves a paper by its slug and renders content to HTML.
func (ps *PaperService) GetBySlug(slug string) (*Paper, error) {
	paper, err := scanPaper(ps.DB.QueryRow(paperBaseSelect+` WHERE p.slug = $1`, slug))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("paper not found")
		}
		return nil, fmt.Errorf("get paper by slug: %w", err)
	}

	formatPaperDates(&paper)

	// Render markdown content to HTML
	paper.ContentHTML = template.HTML(RenderContent(paper.Content))
	paper.MyNotesHTML = template.HTML(RenderContent(paper.MyNotes))

	// Calculate reading time
	wordCount := len(strings.Fields(paper.Content)) + len(strings.Fields(paper.MyNotes))
	paper.ReadingTime = (wordCount + 199) / 200
	if paper.ReadingTime < 1 {
		paper.ReadingTime = 1
	}

	// Load research areas
	papers := []Paper{paper}
	if err := ps.loadAreasForPapers(papers); err != nil {
		return nil, fmt.Errorf("load research areas: %w", err)
	}
	paper.ResearchAreas = papers[0].ResearchAreas

	return &paper, nil
}

// GetPublishedPapers returns all published papers ordered by year and creation date.
func (ps *PaperService) GetPublishedPapers() (*PapersList, error) {
	list := PapersList{}

	rows, err := ps.DB.Query(paperBaseSelect + `
		WHERE p.is_published = true
		ORDER BY p.paper_year DESC, p.created_at DESC`)
	if err != nil {
		return &list, fmt.Errorf("query published papers: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		paper, err := scanPaper(rows)
		if err != nil {
			return nil, fmt.Errorf("scan published paper: %w", err)
		}
		formatPaperDates(&paper)

		// Calculate reading time
		wordCount := len(strings.Fields(paper.Content))
		paper.ReadingTime = (wordCount + 199) / 200
		if paper.ReadingTime < 1 {
			paper.ReadingTime = 1
		}

		// Content preview
		paper.Content = previewContentRaw(paper.Content)
		paper.ContentHTML = template.HTML(RenderContent(paper.Content))

		list.Papers = append(list.Papers, paper)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate published papers: %w", err)
	}

	if err := ps.loadAreasForPapers(list.Papers); err != nil {
		return nil, fmt.Errorf("load research areas: %w", err)
	}

	return &list, nil
}

// GetAllPapers returns all papers (for admin) ordered by creation date.
func (ps *PaperService) GetAllPapers() (*PapersList, error) {
	list := PapersList{}

	rows, err := ps.DB.Query(paperBaseSelect + ` ORDER BY p.created_at DESC`)
	if err != nil {
		return &list, fmt.Errorf("query all papers: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		paper, err := scanPaper(rows)
		if err != nil {
			return nil, fmt.Errorf("scan paper: %w", err)
		}
		formatPaperDates(&paper)
		list.Papers = append(list.Papers, paper)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate all papers: %w", err)
	}

	if err := ps.loadAreasForPapers(list.Papers); err != nil {
		return nil, fmt.Errorf("load research areas: %w", err)
	}

	return &list, nil
}

// GetPublishedPapersByArea returns published papers that belong to a specific research area.
func (ps *PaperService) GetPublishedPapersByArea(areaID int) ([]Paper, error) {
	rows, err := ps.DB.Query(paperBaseSelect+`
		JOIN paper_research_area_map pram ON p.paper_id = pram.paper_id
		WHERE pram.area_id = $1 AND p.is_published = true
		ORDER BY p.paper_year DESC, p.created_at DESC`, areaID)
	if err != nil {
		return nil, fmt.Errorf("query papers by area: %w", err)
	}
	defer rows.Close()

	var papers []Paper
	for rows.Next() {
		paper, err := scanPaper(rows)
		if err != nil {
			return nil, fmt.Errorf("scan paper by area: %w", err)
		}
		formatPaperDates(&paper)

		wordCount := len(strings.Fields(paper.Content))
		paper.ReadingTime = (wordCount + 199) / 200
		if paper.ReadingTime < 1 {
			paper.ReadingTime = 1
		}

		paper.Content = previewContentRaw(paper.Content)
		paper.ContentHTML = template.HTML(RenderContent(paper.Content))

		papers = append(papers, paper)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate papers by area: %w", err)
	}

	if err := ps.loadAreasForPapers(papers); err != nil {
		return nil, fmt.Errorf("load research areas: %w", err)
	}

	return papers, nil
}

// GetPublishedPapersByUser returns published papers by a specific user.
func (ps *PaperService) GetPublishedPapersByUser(userID int) ([]Paper, error) {
	rows, err := ps.DB.Query(paperBaseSelect+`
		WHERE p.user_id = $1 AND p.is_published = true
		ORDER BY p.paper_year DESC, p.created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("query papers by user: %w", err)
	}
	defer rows.Close()

	var papers []Paper
	for rows.Next() {
		paper, err := scanPaper(rows)
		if err != nil {
			return nil, fmt.Errorf("scan paper by user: %w", err)
		}
		formatPaperDates(&paper)

		wordCount := len(strings.Fields(paper.Content))
		paper.ReadingTime = (wordCount + 199) / 200
		if paper.ReadingTime < 1 {
			paper.ReadingTime = 1
		}

		paper.Content = previewContentRaw(paper.Content)
		paper.ContentHTML = template.HTML(RenderContent(paper.Content))

		papers = append(papers, paper)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate papers by user: %w", err)
	}

	if err := ps.loadAreasForPapers(papers); err != nil {
		return nil, fmt.Errorf("load research areas: %w", err)
	}

	return papers, nil
}

// AddAreas adds research areas to a paper.
func (ps *PaperService) AddAreas(paperID int, areaIDs []int) error {
	if len(areaIDs) == 0 {
		return nil
	}
	_, err := ps.DB.Exec(`
		INSERT INTO paper_research_area_map (paper_id, area_id)
		SELECT $1, unnest($2::int[]) ON CONFLICT DO NOTHING`,
		paperID, pq.Array(areaIDs))
	if err != nil {
		return fmt.Errorf("add paper research areas: %w", err)
	}
	return nil
}

// UpdateAreas replaces all research areas for a paper.
func (ps *PaperService) UpdateAreas(paperID int, areaIDs []int) error {
	_, err := ps.DB.Exec(`DELETE FROM paper_research_area_map WHERE paper_id = $1`, paperID)
	if err != nil {
		return fmt.Errorf("remove existing paper research areas: %w", err)
	}
	if len(areaIDs) > 0 {
		return ps.AddAreas(paperID, areaIDs)
	}
	return nil
}

// loadAreasForPapers batch-loads research areas for all papers in a single query.
func (ps *PaperService) loadAreasForPapers(papers []Paper) error {
	if len(papers) == 0 {
		return nil
	}
	ids := make([]int, len(papers))
	idIdx := make(map[int][]int)
	for i, p := range papers {
		ids[i] = p.ID
		idIdx[p.ID] = append(idIdx[p.ID], i)
	}

	rows, err := ps.DB.Query(`
		SELECT pram.paper_id, pra.area_id, pra.area_name, pra.created_at
		FROM paper_research_areas pra
		JOIN paper_research_area_map pram ON pra.area_id = pram.area_id
		WHERE pram.paper_id = ANY($1)`, pq.Array(ids))
	if err != nil {
		return fmt.Errorf("batch-load paper research areas: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var paperID int
		var area PaperResearchArea
		if err := rows.Scan(&paperID, &area.ID, &area.Name, &area.CreatedAt); err != nil {
			return err
		}
		for _, idx := range idIdx[paperID] {
			papers[idx].ResearchAreas = append(papers[idx].ResearchAreas, area)
		}
	}
	return rows.Err()
}

// formatPaperDates normalises date fields for display.
func formatPaperDates(paper *Paper) {
	if t, err := time.Parse(time.RFC3339, paper.CreatedAt); err == nil {
		paper.CreatedAt = t.Format(time.RFC3339)
		paper.PublicationDate = t.Format(friendlyDateFormat)
	}
	if paper.PublicationDate != "" && paper.PublicationDate != paper.CreatedAt {
		if t, err := time.Parse(time.RFC3339, paper.PublicationDate); err == nil {
			paper.PublicationDate = t.Format(friendlyDateFormat)
		}
	}
	if paper.LastEditDate != "" {
		if t, err := time.Parse(time.RFC3339, paper.LastEditDate); err == nil {
			paper.LastEditDate = t.Format(friendlyDateFormat)
		}
	}
}
