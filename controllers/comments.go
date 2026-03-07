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

// CommentsController handles CRUD for post comments.
type CommentsController struct {
	DB             *sql.DB
	BlogService    *models.BlogService
}

type commentResponse struct {
	CommentID       int              `json:"comment_id"`
	UserID          int              `json:"user_id"`
	Username        string           `json:"username"`
	ParentCommentID *int             `json:"parent_comment_id,omitempty"`
	Content         string           `json:"content"`
	CommentDate     string           `json:"comment_date"`
	Replies         []commentResponse `json:"replies,omitempty"`
}

// HandleListComments returns comments for a post ordered by date (threaded).
// GET /blog/{slug}/comments
func (cc *CommentsController) HandleListComments(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	postID, err := cc.postIDFromSlug(slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	rows, err := cc.DB.QueryContext(r.Context(), `
		SELECT c.comment_id, c.user_id, u.username, c.parent_comment_id, c.content, c.comment_date
		FROM Comments c
		JOIN Users u ON u.user_id = c.user_id
		WHERE c.post_id = $1
		ORDER BY c.comment_date ASC`, postID)
	if err != nil {
		http.Error(w, "Failed to fetch comments", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var flat []commentResponse
	for rows.Next() {
		var c commentResponse
		var parentID sql.NullInt64
		var commentDate time.Time
		if err := rows.Scan(&c.CommentID, &c.UserID, &c.Username, &parentID, &c.Content, &commentDate); err != nil {
			http.Error(w, "Failed to scan comment", http.StatusInternalServerError)
			return
		}
		if parentID.Valid {
			pid := int(parentID.Int64)
			c.ParentCommentID = &pid
		}
		c.CommentDate = commentDate.Format(time.RFC3339)
		flat = append(flat, c)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "Failed to iterate comments", http.StatusInternalServerError)
		return
	}

	nested := nestComments(flat)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nested)
}

// nestComments builds a tree from flat comment list.
func nestComments(flat []commentResponse) []commentResponse {
	byID := make(map[int]*commentResponse, len(flat))
	for i := range flat {
		byID[flat[i].CommentID] = &flat[i]
	}
	var roots []commentResponse
	for i := range flat {
		c := &flat[i]
		if c.ParentCommentID == nil {
			roots = append(roots, *c)
		} else {
			if parent, ok := byID[*c.ParentCommentID]; ok {
				parent.Replies = append(parent.Replies, *c)
			} else {
				roots = append(roots, *c)
			}
		}
	}
	return roots
}

// HandleCreateComment creates a new comment on a post.
// POST /blog/{slug}/comments
func (cc *CommentsController) HandleCreateComment(w http.ResponseWriter, r *http.Request) {
	user := authmw.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	perms := models.GetPermissions(user.Role)
	if !perms.CanComment {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	slug := chi.URLParam(r, "slug")
	postID, err := cc.postIDFromSlug(slug)
	if err != nil {
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
		http.Error(w, "Comment content cannot be empty", http.StatusBadRequest)
		return
	}
	if len(body.Content) > 5000 {
		http.Error(w, "Comment content too long (max 5000 characters)", http.StatusBadRequest)
		return
	}

	var commentID int
	var commentDate time.Time
	if body.ParentCommentID != nil {
		err = cc.DB.QueryRowContext(r.Context(), `
			INSERT INTO Comments (user_id, post_id, parent_comment_id, content, comment_date)
			VALUES ($1, $2, $3, $4, NOW())
			RETURNING comment_id, comment_date`,
			user.UserID, postID, *body.ParentCommentID, body.Content,
		).Scan(&commentID, &commentDate)
	} else {
		err = cc.DB.QueryRowContext(r.Context(), `
			INSERT INTO Comments (user_id, post_id, content, comment_date)
			VALUES ($1, $2, $3, NOW())
			RETURNING comment_id, comment_date`,
			user.UserID, postID, body.Content,
		).Scan(&commentID, &commentDate)
	}
	if err != nil {
		http.Error(w, "Failed to create comment", http.StatusInternalServerError)
		return
	}

	resp := commentResponse{
		CommentID:       commentID,
		UserID:          user.UserID,
		Username:        user.Username,
		ParentCommentID: body.ParentCommentID,
		Content:         body.Content,
		CommentDate:     commentDate.Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// HandleDeleteComment deletes a comment (author or admin).
// DELETE /comments/{commentID}
func (cc *CommentsController) HandleDeleteComment(w http.ResponseWriter, r *http.Request) {
	user := authmw.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	commentIDStr := chi.URLParam(r, "commentID")
	commentID, err := strconv.Atoi(commentIDStr)
	if err != nil {
		http.Error(w, "Invalid comment ID", http.StatusBadRequest)
		return
	}

	// Fetch comment owner.
	var ownerID int
	if err := cc.DB.QueryRowContext(r.Context(),
		`SELECT user_id FROM Comments WHERE comment_id = $1`, commentID,
	).Scan(&ownerID); err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, "Failed to fetch comment", http.StatusInternalServerError)
		return
	}

	perms := models.GetPermissions(user.Role)
	if user.UserID != ownerID && !perms.CanManageAllPosts {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if _, err := cc.DB.ExecContext(r.Context(),
		`DELETE FROM Comments WHERE comment_id = $1`, commentID,
	); err != nil {
		http.Error(w, "Failed to delete comment", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (cc *CommentsController) postIDFromSlug(slug string) (int, error) {
	var id int
	err := cc.DB.QueryRow(`SELECT post_id FROM Posts WHERE slug = $1`, slug).Scan(&id)
	return id, err
}

// sanitizeCommentContent trims whitespace from comment content.
func sanitizeCommentContent(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		b = append(b, s[i])
	}
	result := string(b)
	// Trim leading/trailing whitespace.
	start := 0
	for start < len(result) && (result[start] == ' ' || result[start] == '\t' || result[start] == '\n' || result[start] == '\r') {
		start++
	}
	end := len(result)
	for end > start && (result[end-1] == ' ' || result[end-1] == '\t' || result[end-1] == '\n' || result[end-1] == '\r') {
		end--
	}
	return result[start:end]
}
