package models

import (
	"database/sql"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/lib/pq"
)

type BooksList struct {
	Books []Book
}

type Book struct {
	ID                int
	UserID            int
	Username          string
	AuthorDisplayName string
	AuthorAvatarURL   string
	Title             string
	Slug              string
	BookAuthor        string
	ISBN              string
	Publisher         string
	PageCount         int
	CoverImageURL     string
	Content           string
	ContentHTML       template.HTML
	Description       string
	MyThoughts        string
	MyThoughtsHTML    template.HTML
	LinkURL           string
	ReadingStatus     string
	Rating            int
	DateStarted       string // YYYY-MM-DD or empty
	DateFinished      string // YYYY-MM-DD or empty
	IsPublished       bool
	PublicationDate   string
	LastEditDate      string
	CreatedAt         string
	UpdatedAt         string
	ReadingTime       int         `json:"reading_time,omitempty"`
	Genres            []BookGenre `json:"genres,omitempty"`
}

type BookService struct {
	DB *sql.DB
}

const bookBaseSelect = `
	SELECT b.book_id, b.user_id, u.username, COALESCE(NULLIF(u.full_name, ''), u.username),
	       COALESCE(u.profile_picture_url, ''),
	       b.title, b.slug, b.book_author, COALESCE(b.isbn, ''), COALESCE(b.publisher, ''),
	       COALESCE(b.page_count, 0), COALESCE(b.cover_image_url, ''),
	       b.content, COALESCE(b.description, ''), COALESCE(b.my_thoughts, ''),
	       COALESCE(b.link_url, ''), b.reading_status, COALESCE(b.rating, 0),
	       b.date_started, b.date_finished,
	       b.is_published, b.publication_date, b.last_edit_date, b.created_at, b.updated_at
	FROM Books b
	JOIN Users u ON b.user_id = u.user_id`

// scanBook scans a single book row from the database.
func scanBook(scanner interface{ Scan(...interface{}) error }) (Book, error) {
	var book Book
	var dateStarted, dateFinished sql.NullTime
	err := scanner.Scan(
		&book.ID, &book.UserID, &book.Username, &book.AuthorDisplayName,
		&book.AuthorAvatarURL,
		&book.Title, &book.Slug, &book.BookAuthor, &book.ISBN, &book.Publisher,
		&book.PageCount, &book.CoverImageURL,
		&book.Content, &book.Description, &book.MyThoughts,
		&book.LinkURL, &book.ReadingStatus, &book.Rating,
		&dateStarted, &dateFinished,
		&book.IsPublished, &book.PublicationDate, &book.LastEditDate, &book.CreatedAt, &book.UpdatedAt,
	)
	if err != nil {
		return book, err
	}
	if dateStarted.Valid {
		book.DateStarted = dateStarted.Time.Format("2006-01-02")
	}
	if dateFinished.Valid {
		book.DateFinished = dateFinished.Time.Format("2006-01-02")
	}
	return book, nil
}

// parseDateToNullTime parses a "2006-01-02" string to sql.NullTime.
func parseDateToNullTime(s string) sql.NullTime {
	if s == "" {
		return sql.NullTime{Valid: false}
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{Time: t, Valid: true}
}

// Create creates a new book and returns it.
func (bs *BookService) Create(userID int, title, slug, bookAuthor, isbn, publisher string, pageCount int, coverImageURL, content, description, myThoughts, linkURL, readingStatus string, rating int, dateStarted, dateFinished string, isPublished bool, genreIDs []int) (*Book, error) {
	if slug == "" {
		slug = generateSlug(title)
	} else {
		slug = sanitizeSlug(slug)
	}

	now := time.Now()
	dsNT := parseDateToNullTime(dateStarted)
	dfNT := parseDateToNullTime(dateFinished)

	var book Book
	err := bs.DB.QueryRow(`
		INSERT INTO Books (user_id, title, slug, book_author, isbn, publisher, page_count, cover_image_url, content, description, my_thoughts, link_url, reading_status, rating, date_started, date_finished, is_published, publication_date, last_edit_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
		RETURNING book_id, created_at, updated_at`,
		userID, title, slug, bookAuthor, isbn, publisher, pageCount, coverImageURL, content, description, myThoughts, linkURL, readingStatus, rating, dsNT, dfNT, isPublished, now, now, now, now,
	).Scan(&book.ID, &book.CreatedAt, &book.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create book: %w", err)
	}

	book.UserID = userID
	book.Title = title
	book.Slug = slug
	book.BookAuthor = bookAuthor
	book.ISBN = isbn
	book.Publisher = publisher
	book.PageCount = pageCount
	book.CoverImageURL = coverImageURL
	book.Content = content
	book.Description = description
	book.MyThoughts = myThoughts
	book.LinkURL = linkURL
	book.ReadingStatus = readingStatus
	book.Rating = rating
	book.DateStarted = dateStarted
	book.DateFinished = dateFinished
	book.IsPublished = isPublished
	book.PublicationDate = now.Format(friendlyDateFormat)
	book.LastEditDate = now.Format(friendlyDateFormat)

	if len(genreIDs) > 0 {
		if err := bs.AddGenres(book.ID, genreIDs); err != nil {
			return &book, nil // book created, genres failed — non-fatal
		}
	}

	return &book, nil
}

// Update updates an existing book.
func (bs *BookService) Update(bookID int, title, slug, bookAuthor, isbn, publisher string, pageCount int, coverImageURL, content, description, myThoughts, linkURL, readingStatus string, rating int, dateStarted, dateFinished string, isPublished bool, genreIDs []int) error {
	if slug == "" {
		slug = generateSlug(title)
	} else {
		slug = sanitizeSlug(slug)
	}

	dsNT := parseDateToNullTime(dateStarted)
	dfNT := parseDateToNullTime(dateFinished)

	_, err := bs.DB.Exec(`
		UPDATE Books SET title=$1, slug=$2, book_author=$3, isbn=$4, publisher=$5, page_count=$6, cover_image_url=$7, content=$8, description=$9, my_thoughts=$10, link_url=$11, reading_status=$12, rating=$13, date_started=$14, date_finished=$15, is_published=$16, last_edit_date=$17, updated_at=$18
		WHERE book_id=$19`,
		title, slug, bookAuthor, isbn, publisher, pageCount, coverImageURL, content, description, myThoughts, linkURL, readingStatus, rating, dsNT, dfNT, isPublished, time.Now(), time.Now(), bookID)
	if err != nil {
		return fmt.Errorf("update book: %w", err)
	}
	if len(genreIDs) > 0 {
		_ = bs.UpdateGenres(bookID, genreIDs)
	}
	return nil
}

// Delete removes a book by ID.
func (bs *BookService) Delete(bookID int) error {
	_, err := bs.DB.Exec(`DELETE FROM Books WHERE book_id = $1`, bookID)
	if err != nil {
		return fmt.Errorf("delete book: %w", err)
	}
	return nil
}

// GetByID retrieves a book by its ID.
func (bs *BookService) GetByID(id int) (*Book, error) {
	book, err := scanBook(bs.DB.QueryRow(bookBaseSelect+` WHERE b.book_id = $1`, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("book not found")
		}
		return nil, fmt.Errorf("get book by id: %w", err)
	}

	formatBookDates(&book)

	books := []Book{book}
	if err := bs.loadGenresForBooks(books); err != nil {
		return nil, fmt.Errorf("load genres: %w", err)
	}
	book.Genres = books[0].Genres

	return &book, nil
}

// GetBySlug retrieves a book by its slug and renders content to HTML.
func (bs *BookService) GetBySlug(slug string) (*Book, error) {
	book, err := scanBook(bs.DB.QueryRow(bookBaseSelect+` WHERE b.slug = $1`, slug))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("book not found")
		}
		return nil, fmt.Errorf("get book by slug: %w", err)
	}

	formatBookDates(&book)

	// Render markdown content to HTML
	book.ContentHTML = template.HTML(RenderContent(book.Content))
	book.MyThoughtsHTML = template.HTML(RenderContent(book.MyThoughts))

	// Calculate reading time
	wordCount := len(strings.Fields(book.Content)) + len(strings.Fields(book.MyThoughts))
	book.ReadingTime = (wordCount + 199) / 200
	if book.ReadingTime < 1 {
		book.ReadingTime = 1
	}

	// Load genres
	books := []Book{book}
	if err := bs.loadGenresForBooks(books); err != nil {
		return nil, fmt.Errorf("load genres: %w", err)
	}
	book.Genres = books[0].Genres

	return &book, nil
}

// GetPublishedBooks returns all published books with custom ordering.
func (bs *BookService) GetPublishedBooks() (*BooksList, error) {
	list := BooksList{}

	rows, err := bs.DB.Query(bookBaseSelect + `
		WHERE b.is_published = true
		ORDER BY CASE b.reading_status WHEN 'reading' THEN 0 WHEN 'completed' THEN 1 WHEN 'want-to-read' THEN 2 ELSE 3 END,
		         COALESCE(b.date_finished, b.date_started, b.created_at) DESC`)
	if err != nil {
		return &list, fmt.Errorf("query published books: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		book, err := scanBook(rows)
		if err != nil {
			return nil, fmt.Errorf("scan published book: %w", err)
		}
		formatBookDates(&book)

		// Calculate reading time
		wordCount := len(strings.Fields(book.Content))
		book.ReadingTime = (wordCount + 199) / 200
		if book.ReadingTime < 1 {
			book.ReadingTime = 1
		}

		// Content preview
		book.Content = previewContentRaw(book.Content)
		book.ContentHTML = template.HTML(RenderContent(book.Content))

		list.Books = append(list.Books, book)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate published books: %w", err)
	}

	if err := bs.loadGenresForBooks(list.Books); err != nil {
		return nil, fmt.Errorf("load genres: %w", err)
	}

	return &list, nil
}

// GetAllBooks returns all books (for admin) ordered by creation date.
func (bs *BookService) GetAllBooks() (*BooksList, error) {
	list := BooksList{}

	rows, err := bs.DB.Query(bookBaseSelect + ` ORDER BY b.created_at DESC`)
	if err != nil {
		return &list, fmt.Errorf("query all books: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		book, err := scanBook(rows)
		if err != nil {
			return nil, fmt.Errorf("scan book: %w", err)
		}
		formatBookDates(&book)
		list.Books = append(list.Books, book)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate all books: %w", err)
	}

	if err := bs.loadGenresForBooks(list.Books); err != nil {
		return nil, fmt.Errorf("load genres: %w", err)
	}

	return &list, nil
}

// GetPublishedBooksByGenre returns published books that belong to a specific genre.
func (bs *BookService) GetPublishedBooksByGenre(genreID int) ([]Book, error) {
	rows, err := bs.DB.Query(bookBaseSelect+`
		JOIN Book_Genre_Map bgm ON b.book_id = bgm.book_id
		WHERE bgm.genre_id = $1 AND b.is_published = true
		ORDER BY CASE b.reading_status WHEN 'reading' THEN 0 WHEN 'completed' THEN 1 WHEN 'want-to-read' THEN 2 ELSE 3 END,
		         COALESCE(b.date_finished, b.date_started, b.created_at) DESC`, genreID)
	if err != nil {
		return nil, fmt.Errorf("query books by genre: %w", err)
	}
	defer rows.Close()

	var books []Book
	for rows.Next() {
		book, err := scanBook(rows)
		if err != nil {
			return nil, fmt.Errorf("scan book by genre: %w", err)
		}
		formatBookDates(&book)

		wordCount := len(strings.Fields(book.Content))
		book.ReadingTime = (wordCount + 199) / 200
		if book.ReadingTime < 1 {
			book.ReadingTime = 1
		}

		book.Content = previewContentRaw(book.Content)
		book.ContentHTML = template.HTML(RenderContent(book.Content))

		books = append(books, book)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate books by genre: %w", err)
	}

	if err := bs.loadGenresForBooks(books); err != nil {
		return nil, fmt.Errorf("load genres: %w", err)
	}

	return books, nil
}

// GetPublishedBooksByUser returns published books by a specific user.
func (bs *BookService) GetPublishedBooksByUser(userID int) ([]Book, error) {
	rows, err := bs.DB.Query(bookBaseSelect+`
		WHERE b.user_id = $1 AND b.is_published = true
		ORDER BY CASE b.reading_status WHEN 'reading' THEN 0 WHEN 'completed' THEN 1 WHEN 'want-to-read' THEN 2 ELSE 3 END,
		         COALESCE(b.date_finished, b.date_started, b.created_at) DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("query books by user: %w", err)
	}
	defer rows.Close()

	var books []Book
	for rows.Next() {
		book, err := scanBook(rows)
		if err != nil {
			return nil, fmt.Errorf("scan book by user: %w", err)
		}
		formatBookDates(&book)

		wordCount := len(strings.Fields(book.Content))
		book.ReadingTime = (wordCount + 199) / 200
		if book.ReadingTime < 1 {
			book.ReadingTime = 1
		}

		book.Content = previewContentRaw(book.Content)
		book.ContentHTML = template.HTML(RenderContent(book.Content))

		books = append(books, book)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate books by user: %w", err)
	}

	if err := bs.loadGenresForBooks(books); err != nil {
		return nil, fmt.Errorf("load genres: %w", err)
	}

	return books, nil
}

// AddGenres adds genres to a book.
func (bs *BookService) AddGenres(bookID int, genreIDs []int) error {
	if len(genreIDs) == 0 {
		return nil
	}
	_, err := bs.DB.Exec(`
		INSERT INTO Book_Genre_Map (book_id, genre_id)
		SELECT $1, unnest($2::int[]) ON CONFLICT DO NOTHING`,
		bookID, pq.Array(genreIDs))
	if err != nil {
		return fmt.Errorf("add book genres: %w", err)
	}
	return nil
}

// UpdateGenres replaces all genres for a book.
func (bs *BookService) UpdateGenres(bookID int, genreIDs []int) error {
	_, err := bs.DB.Exec(`DELETE FROM Book_Genre_Map WHERE book_id = $1`, bookID)
	if err != nil {
		return fmt.Errorf("remove existing book genres: %w", err)
	}
	if len(genreIDs) > 0 {
		return bs.AddGenres(bookID, genreIDs)
	}
	return nil
}

// loadGenresForBooks batch-loads genres for all books in a single query.
func (bs *BookService) loadGenresForBooks(books []Book) error {
	if len(books) == 0 {
		return nil
	}
	ids := make([]int, len(books))
	idIdx := make(map[int][]int)
	for i, b := range books {
		ids[i] = b.ID
		idIdx[b.ID] = append(idIdx[b.ID], i)
	}

	rows, err := bs.DB.Query(`
		SELECT bgm.book_id, bg.genre_id, bg.genre_name, bg.genre_group, bg.created_at
		FROM Book_Genres bg
		JOIN Book_Genre_Map bgm ON bg.genre_id = bgm.genre_id
		WHERE bgm.book_id = ANY($1)`, pq.Array(ids))
	if err != nil {
		return fmt.Errorf("batch-load book genres: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var bookID int
		var genre BookGenre
		if err := rows.Scan(&bookID, &genre.ID, &genre.Name, &genre.Group, &genre.CreatedAt); err != nil {
			return err
		}
		for _, idx := range idIdx[bookID] {
			books[idx].Genres = append(books[idx].Genres, genre)
		}
	}
	return rows.Err()
}

// formatBookDates normalises date fields for display.
func formatBookDates(book *Book) {
	if t, err := time.Parse(time.RFC3339, book.CreatedAt); err == nil {
		book.CreatedAt = t.Format(time.RFC3339)
		book.PublicationDate = t.Format(friendlyDateFormat)
	}
	if book.PublicationDate != "" && book.PublicationDate != book.CreatedAt {
		if t, err := time.Parse(time.RFC3339, book.PublicationDate); err == nil {
			book.PublicationDate = t.Format(friendlyDateFormat)
		}
	}
	if book.LastEditDate != "" {
		if t, err := time.Parse(time.RFC3339, book.LastEditDate); err == nil {
			book.LastEditDate = t.Format(friendlyDateFormat)
		}
	}
	// DateStarted and DateFinished stay as YYYY-MM-DD (already user-friendly)
}
