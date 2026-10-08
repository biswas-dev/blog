package models

import (
	"database/sql"
	"fmt"
	"math"
	"time"
)

// What replaced a book's rating/thoughts, recorded on each history row.
const (
	ReviewChangeEdit      = "edit"
	ReviewChangeGoodreads = "goodreads"
	ReviewChangeRevert    = "revert"
)

// BookReviewVersion is a book's rating and my_thoughts as they were before
// a change replaced them.
type BookReviewVersion struct {
	ID         int
	BookID     int
	Rating     float64
	MyThoughts string
	ReplacedBy string
	CreatedAt  time.Time
}

func sameRating(a, b float64) bool { return math.Abs(a-b) < 0.01 }

// snapshotReview saves the book's current rating/thoughts to book_review_history
// when the incoming values differ. Must run inside the tx that applies the change.
func snapshotReview(tx *sql.Tx, bookID int, newRating float64, newThoughts, replacedBy string) error {
	var oldRating float64
	var oldThoughts string
	err := tx.QueryRow(`SELECT COALESCE(rating, 0), COALESCE(my_thoughts, '') FROM Books WHERE book_id = $1 FOR UPDATE`, bookID).
		Scan(&oldRating, &oldThoughts)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read current review: %w", err)
	}
	if sameRating(oldRating, newRating) && oldThoughts == newThoughts {
		return nil
	}
	_, err = tx.Exec(`INSERT INTO book_review_history (book_id, rating, my_thoughts, replaced_by) VALUES ($1, $2, $3, $4)`,
		bookID, oldRating, oldThoughts, replacedBy)
	if err != nil {
		return fmt.Errorf("save review history: %w", err)
	}
	return nil
}

// SetReview changes only a book's rating and my_thoughts, keeping the
// previous values in book_review_history.
func (bs *BookService) SetReview(bookID int, rating float64, myThoughts, replacedBy string) error {
	tx, err := bs.DB.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	if err := snapshotReview(tx, bookID, rating, myThoughts, replacedBy); err != nil {
		return err
	}
	now := time.Now()
	if _, err := tx.Exec(`UPDATE Books SET rating = $1, my_thoughts = $2, last_edit_date = $3, updated_at = $3 WHERE book_id = $4`,
		rating, myThoughts, now, bookID); err != nil {
		return fmt.Errorf("update review: %w", err)
	}
	return tx.Commit()
}

// GetReviewHistory returns a book's previous ratings/thoughts, newest first.
func (bs *BookService) GetReviewHistory(bookID int) ([]BookReviewVersion, error) {
	rows, err := bs.DB.Query(`
		SELECT id, book_id, rating, my_thoughts, replaced_by, created_at
		FROM book_review_history WHERE book_id = $1
		ORDER BY created_at DESC, id DESC`, bookID)
	if err != nil {
		return nil, fmt.Errorf("list review history: %w", err)
	}
	defer rows.Close()

	var versions []BookReviewVersion
	for rows.Next() {
		var v BookReviewVersion
		if err := rows.Scan(&v.ID, &v.BookID, &v.Rating, &v.MyThoughts, &v.ReplacedBy, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan review history: %w", err)
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// RevertReview restores the rating/thoughts from a history entry. The values
// being replaced are themselves saved, so a revert can be undone.
func (bs *BookService) RevertReview(bookID, historyID int) error {
	var rating float64
	var thoughts string
	err := bs.DB.QueryRow(`SELECT rating, my_thoughts FROM book_review_history WHERE id = $1 AND book_id = $2`, historyID, bookID).
		Scan(&rating, &thoughts)
	if err == sql.ErrNoRows {
		return fmt.Errorf("review version not found")
	}
	if err != nil {
		return fmt.Errorf("get review version: %w", err)
	}
	return bs.SetReview(bookID, rating, thoughts, ReviewChangeRevert)
}
