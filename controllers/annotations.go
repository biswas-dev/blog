package controllers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	authmw "anshumanbiswas.com/blog/middleware"
	"anshumanbiswas.com/blog/models"
	"github.com/go-chi/chi/v5"
)

// AnnotationsController handles post annotations and annotation comments.
type AnnotationsController struct {
	DB *sql.DB
}

type annotationCommentResp struct {
	ID              int    `json:"id"`
	AnnotationID    int    `json:"annotation_id"`
	AuthorID        int    `json:"author_id"`
	AuthorName      string `json:"author_name"`
	ParentCommentID *int   `json:"parent_comment_id,omitempty"`
	Content         string `json:"content"`
	CreatedAt       string `json:"created_at"`
}

type annotationResp struct {
	ID           int                     `json:"id"`
	PostID       int                     `json:"post_id"`
	AuthorID     int                     `json:"author_id"`
	AuthorName   string                  `json:"author_name"`
	StartOffset  int                     `json:"start_offset"`
	EndOffset    int                     `json:"end_offset"`
	SelectedText string                  `json:"selected_text"`
	Color        string                  `json:"color"`
	Resolved     bool                    `json:"resolved"`
	CreatedAt    string                  `json:"created_at"`
	Comments     []annotationCommentResp `json:"comments,omitempty"`
}

// HandleListAnnotations returns all non-resolved annotations + their comments.
// GET /blog/{slug}/annotations
func (ac *AnnotationsController) HandleListAnnotations(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	postID, err := ac.postIDFromSlug(slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	rows, err := ac.DB.QueryContext(r.Context(), `
		SELECT a.id, a.post_id, a.author_id, COALESCE(NULLIF(u.full_name, ''), u.username), a.start_offset, a.end_offset,
		       a.selected_text, a.color, a.resolved, a.created_at
		FROM post_annotations a
		JOIN Users u ON u.user_id = a.author_id
		WHERE a.post_id = $1 AND a.resolved = false
		ORDER BY a.start_offset ASC`, postID)
	if err != nil {
		http.Error(w, "Failed to fetch annotations", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	annotations := []annotationResp{}
	annotationIDs := []int{}
	byID := map[int]*annotationResp{}

	for rows.Next() {
		var a annotationResp
		var createdAt time.Time
		if err := rows.Scan(&a.ID, &a.PostID, &a.AuthorID, &a.AuthorName,
			&a.StartOffset, &a.EndOffset, &a.SelectedText, &a.Color, &a.Resolved, &createdAt); err != nil {
			http.Error(w, "Failed to scan annotation", http.StatusInternalServerError)
			return
		}
		a.CreatedAt = createdAt.Format(time.RFC3339)
		a.Comments = []annotationCommentResp{}
		annotations = append(annotations, a)
		annotationIDs = append(annotationIDs, a.ID)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "Failed to iterate annotations", http.StatusInternalServerError)
		return
	}

	// Build lookup for attaching comments.
	for i := range annotations {
		byID[annotations[i].ID] = &annotations[i]
	}

	// Fetch comments for all annotations.
	if len(annotationIDs) > 0 {
		// Build $1,$2,... placeholders.
		placeholders := make([]interface{}, len(annotationIDs))
		ph := ""
		for i, id := range annotationIDs {
			if i > 0 {
				ph += ","
			}
			ph += "$" + strconv.Itoa(i+1)
			placeholders[i] = id
		}
		commentRows, err := ac.DB.QueryContext(r.Context(), `
			SELECT ac.id, ac.annotation_id, ac.author_id, COALESCE(NULLIF(u.full_name, ''), u.username), ac.parent_comment_id,
			       ac.content, ac.created_at
			FROM post_annotation_comments ac
			JOIN Users u ON u.user_id = ac.author_id
			WHERE ac.annotation_id IN (`+ph+`)
			ORDER BY ac.created_at ASC`, placeholders...)
		if err == nil {
			defer commentRows.Close()
			for commentRows.Next() {
				var c annotationCommentResp
				var parentID sql.NullInt64
				var createdAt time.Time
				if err := commentRows.Scan(&c.ID, &c.AnnotationID, &c.AuthorID, &c.AuthorName,
					&parentID, &c.Content, &createdAt); err != nil {
					continue
				}
				if parentID.Valid {
					pid := int(parentID.Int64)
					c.ParentCommentID = &pid
				}
				c.CreatedAt = createdAt.Format(time.RFC3339)
				if ann, ok := byID[c.AnnotationID]; ok {
					ann.Comments = append(ann.Comments, c)
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(annotations)
}

// HandleCreateAnnotation creates a new annotation on a post.
// POST /blog/{slug}/annotations
func (ac *AnnotationsController) HandleCreateAnnotation(w http.ResponseWriter, r *http.Request) {
	user := authmw.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	slug := chi.URLParam(r, "slug")
	postID, err := ac.postIDFromSlug(slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var body struct {
		StartOffset  int    `json:"start_offset"`
		EndOffset    int    `json:"end_offset"`
		SelectedText string `json:"selected_text"`
		Color        string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if body.SelectedText == "" {
		http.Error(w, "selected_text is required", http.StatusBadRequest)
		return
	}
	if body.Color == "" {
		body.Color = "yellow"
	}

	var a annotationResp
	var createdAt time.Time
	err = ac.DB.QueryRowContext(r.Context(), `
		INSERT INTO post_annotations (post_id, author_id, start_offset, end_offset, selected_text, color)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, post_id, author_id, start_offset, end_offset, selected_text, color, resolved, created_at`,
		postID, user.UserID, body.StartOffset, body.EndOffset, body.SelectedText, body.Color,
	).Scan(&a.ID, &a.PostID, &a.AuthorID, &a.StartOffset, &a.EndOffset,
		&a.SelectedText, &a.Color, &a.Resolved, &createdAt)
	if err != nil {
		http.Error(w, "Failed to create annotation", http.StatusInternalServerError)
		return
	}
	a.AuthorName = user.DisplayName()
	a.CreatedAt = createdAt.Format(time.RFC3339)
	a.Comments = []annotationCommentResp{}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(a)
}

// HandleUpdateAnnotation updates color or resolved status.
// PATCH /annotations/{annotationID}
func (ac *AnnotationsController) HandleUpdateAnnotation(w http.ResponseWriter, r *http.Request) {
	user := authmw.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	annotationID, err := strconv.Atoi(chi.URLParam(r, "annotationID"))
	if err != nil {
		http.Error(w, "Invalid annotation ID", http.StatusBadRequest)
		return
	}

	// Check access: author or admin.
	var ownerID int
	if err := ac.DB.QueryRowContext(r.Context(),
		`SELECT author_id FROM post_annotations WHERE id = $1`, annotationID,
	).Scan(&ownerID); err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, "Failed to fetch annotation", http.StatusInternalServerError)
		return
	}
	perms := models.GetPermissions(user.Role)
	if user.UserID != ownerID && !perms.CanManageAllPosts {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var body struct {
		Color    *string `json:"color,omitempty"`
		Resolved *bool   `json:"resolved,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if body.Color != nil {
		if _, err := ac.DB.ExecContext(r.Context(),
			`UPDATE post_annotations SET color = $1 WHERE id = $2`, *body.Color, annotationID,
		); err != nil {
			http.Error(w, "Failed to update annotation", http.StatusInternalServerError)
			return
		}
	}
	if body.Resolved != nil {
		if _, err := ac.DB.ExecContext(r.Context(),
			`UPDATE post_annotations SET resolved = $1 WHERE id = $2`, *body.Resolved, annotationID,
		); err != nil {
			http.Error(w, "Failed to update annotation", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleDeleteAnnotation deletes an annotation (author only).
// DELETE /annotations/{annotationID}
func (ac *AnnotationsController) HandleDeleteAnnotation(w http.ResponseWriter, r *http.Request) {
	user := authmw.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	annotationID, err := strconv.Atoi(chi.URLParam(r, "annotationID"))
	if err != nil {
		http.Error(w, "Invalid annotation ID", http.StatusBadRequest)
		return
	}

	var ownerID int
	if err := ac.DB.QueryRowContext(r.Context(),
		`SELECT author_id FROM post_annotations WHERE id = $1`, annotationID,
	).Scan(&ownerID); err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, "Failed to fetch annotation", http.StatusInternalServerError)
		return
	}
	perms := models.GetPermissions(user.Role)
	if user.UserID != ownerID && !perms.CanManageAllPosts {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if _, err := ac.DB.ExecContext(r.Context(),
		`DELETE FROM post_annotations WHERE id = $1`, annotationID,
	); err != nil {
		http.Error(w, "Failed to delete annotation", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleCreateAnnotationComment adds a comment to an annotation.
// POST /annotations/{annotationID}/comments
func (ac *AnnotationsController) HandleCreateAnnotationComment(w http.ResponseWriter, r *http.Request) {
	user := authmw.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	annotationID, err := strconv.Atoi(chi.URLParam(r, "annotationID"))
	if err != nil {
		http.Error(w, "Invalid annotation ID", http.StatusBadRequest)
		return
	}

	// Ensure annotation exists.
	var exists bool
	if err := ac.DB.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM post_annotations WHERE id = $1)`, annotationID,
	).Scan(&exists); err != nil || !exists {
		http.NotFound(w, r)
		return
	}

	var body struct {
		Content         string `json:"content"`
		ParentCommentID *int   `json:"parent_comment_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	body.Content = sanitizeCommentContent(body.Content)
	if body.Content == "" {
		http.Error(w, "content is required", http.StatusBadRequest)
		return
	}

	var c annotationCommentResp
	var createdAt time.Time

	if body.ParentCommentID != nil {
		err = ac.DB.QueryRowContext(r.Context(), `
			INSERT INTO post_annotation_comments (annotation_id, author_id, parent_comment_id, content)
			VALUES ($1, $2, $3, $4)
			RETURNING id, annotation_id, author_id, parent_comment_id, content, created_at`,
			annotationID, user.UserID, *body.ParentCommentID, body.Content,
		).Scan(&c.ID, &c.AnnotationID, &c.AuthorID, &body.ParentCommentID, &c.Content, &createdAt)
		c.ParentCommentID = body.ParentCommentID
	} else {
		err = ac.DB.QueryRowContext(r.Context(), `
			INSERT INTO post_annotation_comments (annotation_id, author_id, content)
			VALUES ($1, $2, $3)
			RETURNING id, annotation_id, author_id, content, created_at`,
			annotationID, user.UserID, body.Content,
		).Scan(&c.ID, &c.AnnotationID, &c.AuthorID, &c.Content, &createdAt)
	}
	if err != nil {
		http.Error(w, "Failed to create annotation comment", http.StatusInternalServerError)
		return
	}
	c.AuthorName = user.DisplayName()
	c.CreatedAt = createdAt.Format(time.RFC3339)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(c)
}

// HandleUpdateAnnotationComment updates comment content (author only).
// PATCH /annotation-comments/{commentID}
func (ac *AnnotationsController) HandleUpdateAnnotationComment(w http.ResponseWriter, r *http.Request) {
	user := authmw.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	commentID, err := strconv.Atoi(chi.URLParam(r, "commentID"))
	if err != nil {
		http.Error(w, "Invalid comment ID", http.StatusBadRequest)
		return
	}

	var ownerID int
	if err := ac.DB.QueryRowContext(r.Context(),
		`SELECT author_id FROM post_annotation_comments WHERE id = $1`, commentID,
	).Scan(&ownerID); err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, "Failed to fetch annotation comment", http.StatusInternalServerError)
		return
	}
	if user.UserID != ownerID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	body.Content = sanitizeCommentContent(body.Content)
	if body.Content == "" {
		http.Error(w, "content is required", http.StatusBadRequest)
		return
	}

	if _, err := ac.DB.ExecContext(r.Context(),
		`UPDATE post_annotation_comments SET content = $1, updated_at = NOW() WHERE id = $2`,
		body.Content, commentID,
	); err != nil {
		http.Error(w, "Failed to update annotation comment", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleDeleteAnnotationComment deletes an annotation comment (author only).
// DELETE /annotation-comments/{commentID}
func (ac *AnnotationsController) HandleDeleteAnnotationComment(w http.ResponseWriter, r *http.Request) {
	user := authmw.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	commentID, err := strconv.Atoi(chi.URLParam(r, "commentID"))
	if err != nil {
		http.Error(w, "Invalid comment ID", http.StatusBadRequest)
		return
	}

	var ownerID int
	if err := ac.DB.QueryRowContext(r.Context(),
		`SELECT author_id FROM post_annotation_comments WHERE id = $1`, commentID,
	).Scan(&ownerID); err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, "Failed to fetch annotation comment", http.StatusInternalServerError)
		return
	}
	perms := models.GetPermissions(user.Role)
	if user.UserID != ownerID && !perms.CanManageAllPosts {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if _, err := ac.DB.ExecContext(r.Context(),
		`DELETE FROM post_annotation_comments WHERE id = $1`, commentID,
	); err != nil {
		http.Error(w, "Failed to delete annotation comment", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (ac *AnnotationsController) postIDFromSlug(slug string) (int, error) {
	var id int
	err := ac.DB.QueryRow(`SELECT post_id FROM Posts WHERE slug = $1`, slug).Scan(&id)
	return id, err
}
