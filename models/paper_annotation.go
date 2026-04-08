package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type PaperAnnotation struct {
	ID           int             `json:"id"`
	PaperID      int             `json:"paper_id"`
	AuthorID     int             `json:"author_id"`
	AuthorName   string          `json:"author_name"`
	PageNumber   int             `json:"page_number"`
	SelectedText string          `json:"selected_text"`
	BoundingBox  json.RawMessage `json:"bounding_box"` // JSONB stored as-is
	Color        string          `json:"color"`
	Note         string          `json:"note"`
	IsPublic     bool            `json:"is_public"`
	SortOrder    int             `json:"sort_order"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type PaperAnnotationComment struct {
	ID              int       `json:"id"`
	AnnotationID    int       `json:"annotation_id"`
	AuthorID        int       `json:"author_id"`
	AuthorName      string    `json:"author_name"`
	ParentCommentID *int      `json:"parent_comment_id,omitempty"`
	Content         string    `json:"content"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type PaperAnnotationService struct {
	DB *sql.DB
}

// Create creates a new paper annotation.
func (s *PaperAnnotationService) Create(paperID, authorID, pageNumber int, selectedText string, boundingBox json.RawMessage, color, note string, isPublic bool) (*PaperAnnotation, error) {
	ann := &PaperAnnotation{
		PaperID:      paperID,
		AuthorID:     authorID,
		PageNumber:   pageNumber,
		SelectedText: selectedText,
		BoundingBox:  boundingBox,
		Color:        color,
		Note:         note,
		IsPublic:     isPublic,
	}

	err := s.DB.QueryRow(`
		INSERT INTO paper_annotations (paper_id, author_id, page_number, selected_text, bounding_box, color, note, is_public)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING annotation_id, created_at`,
		paperID, authorID, pageNumber, selectedText, boundingBox, color, note, isPublic,
	).Scan(&ann.ID, &ann.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create annotation: %w", err)
	}
	ann.UpdatedAt = ann.CreatedAt

	return ann, nil
}

// Update updates an existing paper annotation.
func (s *PaperAnnotationService) Update(annotationID int, color, note string, isPublic bool, sortOrder int) error {
	_, err := s.DB.Exec(`
		UPDATE paper_annotations
		SET color = $2, note = $3, is_public = $4, sort_order = $5, updated_at = NOW()
		WHERE annotation_id = $1`,
		annotationID, color, note, isPublic, sortOrder)
	if err != nil {
		return fmt.Errorf("update annotation: %w", err)
	}
	return nil
}

// Delete removes a paper annotation by ID.
func (s *PaperAnnotationService) Delete(annotationID int) error {
	_, err := s.DB.Exec(`DELETE FROM paper_annotations WHERE annotation_id = $1`, annotationID)
	if err != nil {
		return fmt.Errorf("delete annotation: %w", err)
	}
	return nil
}

// GetByPaper returns all annotations for a paper.
func (s *PaperAnnotationService) GetByPaper(paperID int) ([]PaperAnnotation, error) {
	rows, err := s.DB.Query(`
		SELECT a.annotation_id, a.paper_id, a.author_id,
		       COALESCE(NULLIF(u.full_name, ''), u.username),
		       a.page_number, a.selected_text, a.bounding_box,
		       a.color, a.note, a.is_public, a.sort_order, a.created_at, a.updated_at
		FROM paper_annotations a
		JOIN Users u ON a.author_id = u.user_id
		WHERE a.paper_id = $1
		ORDER BY a.page_number, a.sort_order`, paperID)
	if err != nil {
		return nil, fmt.Errorf("get annotations by paper: %w", err)
	}
	defer rows.Close()

	var annotations []PaperAnnotation
	for rows.Next() {
		var ann PaperAnnotation
		if err := rows.Scan(
			&ann.ID, &ann.PaperID, &ann.AuthorID, &ann.AuthorName,
			&ann.PageNumber, &ann.SelectedText, &ann.BoundingBox,
			&ann.Color, &ann.Note, &ann.IsPublic, &ann.SortOrder, &ann.CreatedAt, &ann.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan annotation: %w", err)
		}
		annotations = append(annotations, ann)
	}
	return annotations, rows.Err()
}

// GetPublicByPaper returns all public annotations for a paper.
func (s *PaperAnnotationService) GetPublicByPaper(paperID int) ([]PaperAnnotation, error) {
	rows, err := s.DB.Query(`
		SELECT a.annotation_id, a.paper_id, a.author_id,
		       COALESCE(NULLIF(u.full_name, ''), u.username),
		       a.page_number, a.selected_text, a.bounding_box,
		       a.color, a.note, a.is_public, a.sort_order, a.created_at, a.updated_at
		FROM paper_annotations a
		JOIN Users u ON a.author_id = u.user_id
		WHERE a.paper_id = $1 AND a.is_public = true
		ORDER BY a.sort_order, a.page_number`, paperID)
	if err != nil {
		return nil, fmt.Errorf("get public annotations by paper: %w", err)
	}
	defer rows.Close()

	var annotations []PaperAnnotation
	for rows.Next() {
		var ann PaperAnnotation
		if err := rows.Scan(
			&ann.ID, &ann.PaperID, &ann.AuthorID, &ann.AuthorName,
			&ann.PageNumber, &ann.SelectedText, &ann.BoundingBox,
			&ann.Color, &ann.Note, &ann.IsPublic, &ann.SortOrder, &ann.CreatedAt, &ann.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan public annotation: %w", err)
		}
		annotations = append(annotations, ann)
	}
	return annotations, rows.Err()
}

// ReorderAnnotations sets sort_order based on the position in the provided slice.
func (s *PaperAnnotationService) ReorderAnnotations(paperID int, annotationIDs []int) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return fmt.Errorf("begin reorder tx: %w", err)
	}
	defer tx.Rollback()

	for i, id := range annotationIDs {
		_, err := tx.Exec(`
			UPDATE paper_annotations SET sort_order = $1 WHERE annotation_id = $2 AND paper_id = $3`,
			i, id, paperID)
		if err != nil {
			return fmt.Errorf("reorder annotation %d: %w", id, err)
		}
	}

	return tx.Commit()
}

// CreateComment creates a new comment on an annotation.
func (s *PaperAnnotationService) CreateComment(annotationID, authorID int, parentID *int, content string) (*PaperAnnotationComment, error) {
	comment := &PaperAnnotationComment{
		AnnotationID:    annotationID,
		AuthorID:        authorID,
		ParentCommentID: parentID,
		Content:         content,
	}

	err := s.DB.QueryRow(`
		INSERT INTO paper_annotation_comments (annotation_id, author_id, parent_comment_id, content)
		VALUES ($1, $2, $3, $4)
		RETURNING comment_id, created_at, updated_at`,
		annotationID, authorID, parentID, content,
	).Scan(&comment.ID, &comment.CreatedAt, &comment.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create annotation comment: %w", err)
	}

	return comment, nil
}

// GetComments returns all comments for an annotation.
func (s *PaperAnnotationService) GetComments(annotationID int) ([]PaperAnnotationComment, error) {
	rows, err := s.DB.Query(`
		SELECT c.comment_id, c.annotation_id, c.author_id,
		       COALESCE(NULLIF(u.full_name, ''), u.username),
		       c.parent_comment_id, c.content, c.created_at, c.updated_at
		FROM paper_annotation_comments c
		JOIN Users u ON c.author_id = u.user_id
		WHERE c.annotation_id = $1
		ORDER BY c.created_at`, annotationID)
	if err != nil {
		return nil, fmt.Errorf("get annotation comments: %w", err)
	}
	defer rows.Close()

	var comments []PaperAnnotationComment
	for rows.Next() {
		var c PaperAnnotationComment
		if err := rows.Scan(
			&c.ID, &c.AnnotationID, &c.AuthorID, &c.AuthorName,
			&c.ParentCommentID, &c.Content, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan annotation comment: %w", err)
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

// DeleteComment removes a comment by ID.
func (s *PaperAnnotationService) DeleteComment(commentID int) error {
	_, err := s.DB.Exec(`DELETE FROM paper_annotation_comments WHERE comment_id = $1`, commentID)
	if err != nil {
		return fmt.Errorf("delete annotation comment: %w", err)
	}
	return nil
}
