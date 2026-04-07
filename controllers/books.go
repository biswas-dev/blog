package controllers

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"anshumanbiswas.com/blog/models"
	"anshumanbiswas.com/blog/utils"
	"anshumanbiswas.com/blog/views"
	gowiki "github.com/anchoo2kewl/go-wiki"
	"github.com/go-chi/chi/v5"
)

// Books handles book CRUD and public display.
type Books struct {
	Templates struct {
		BooksList  views.Template
		BookPage   views.Template
		AdminBooks views.Template
		BookEditor views.Template
	}
	BookService        *models.BookService
	BookVersionService *models.BookVersionService
	SessionService     *models.SessionService
	BookGenreService   *models.BookGenreService
	BlogWiki           *gowiki.Wiki
}

// PublicBooksList displays the public books listing page.
// GET /books
func (b Books) PublicBooksList(w http.ResponseWriter, r *http.Request) {
	user, _ := utils.IsUserLoggedIn(r, b.SessionService)

	booksList, err := b.BookService.GetPublishedBooks()
	if err != nil {
		log.Printf("Error fetching books: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Separate books by reading status
	var currentlyReading, completed, wantToRead, abandoned []models.Book
	for _, book := range booksList.Books {
		switch book.ReadingStatus {
		case "reading":
			currentlyReading = append(currentlyReading, book)
		case "completed":
			completed = append(completed, book)
		case "want-to-read":
			wantToRead = append(wantToRead, book)
		case "abandoned":
			abandoned = append(abandoned, book)
		default:
			completed = append(completed, book)
		}
	}

	userPerms := models.GetPermissions(models.RoleCommenter)
	if user != nil {
		userPerms = models.GetPermissions(user.Role)
	}

	data := struct {
		LoggedIn         bool
		Username         string
		IsAdmin          bool
		SignupDisabled   bool
		Description      string
		CurrentPage      string
		Books            *models.BooksList
		CurrentlyReading []models.Book
		Completed        []models.Book
		WantToRead       []models.Book
		Abandoned        []models.Book
		UserPermissions  models.UserPermissions
	}{
		LoggedIn:         user != nil,
		Username:         getUsername(user),
		IsAdmin:          user != nil && models.IsAdmin(user.Role),
		SignupDisabled:   true,
		Description:      "Book reviews and reading list - Anshuman Biswas Blog",
		CurrentPage:      "books",
		Books:            booksList,
		CurrentlyReading: currentlyReading,
		Completed:        completed,
		WantToRead:       wantToRead,
		Abandoned:        abandoned,
		UserPermissions:  userPerms,
	}

	b.Templates.BooksList.Execute(w, r, data)
}

// ViewBook displays a single book with content.
// GET /books/{slug}
func (b Books) ViewBook(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	user, _ := utils.IsUserLoggedIn(r, b.SessionService)

	book, err := b.BookService.GetBySlug(slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Check if book is published (unless user can view unpublished)
	if !book.IsPublished && (user == nil || !models.CanViewUnpublished(user.Role)) {
		http.NotFound(w, r)
		return
	}

	userPerms := models.GetPermissions(models.RoleCommenter)
	if user != nil {
		userPerms = models.GetPermissions(user.Role)
	}

	// Compute full URL for share links
	scheme := "https"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if r.TLS == nil {
		scheme = "http"
	}
	fullURL := fmt.Sprintf("%s://%s/books/%s", scheme, r.Host, slug)

	data := struct {
		LoggedIn        bool
		Username        string
		IsAdmin         bool
		SignupDisabled  bool
		UserID          int
		Description     string
		CurrentPage     string
		Book            *models.Book
		FullURL         string
		UserPermissions models.UserPermissions
	}{
		LoggedIn:        user != nil,
		Username:        getUsername(user),
		IsAdmin:         user != nil && models.IsAdmin(user.Role),
		SignupDisabled:  true,
		Description:     book.Title,
		CurrentPage:     "books",
		Book:            book,
		FullURL:         fullURL,
		UserPermissions: userPerms,
	}

	if user != nil {
		data.UserID = user.UserID
	}

	b.Templates.BookPage.Execute(w, r, data)
}

// GenrePage displays books filtered by a specific genre.
// GET /books/genre/{name}
func (b Books) GenrePage(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	user, _ := utils.IsUserLoggedIn(r, b.SessionService)

	genre, err := b.BookGenreService.GetByName(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	books, err := b.BookService.GetPublishedBooksByGenre(genre.ID)
	if err != nil {
		log.Printf("Error fetching books by genre: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Separate books by reading status
	var currentlyReading, completed, wantToRead, abandoned []models.Book
	for _, book := range books {
		switch book.ReadingStatus {
		case "reading":
			currentlyReading = append(currentlyReading, book)
		case "completed":
			completed = append(completed, book)
		case "want-to-read":
			wantToRead = append(wantToRead, book)
		case "abandoned":
			abandoned = append(abandoned, book)
		default:
			completed = append(completed, book)
		}
	}

	userPerms := models.GetPermissions(models.RoleCommenter)
	if user != nil {
		userPerms = models.GetPermissions(user.Role)
	}

	data := struct {
		LoggedIn         bool
		Username         string
		IsAdmin          bool
		SignupDisabled   bool
		Description      string
		CurrentPage      string
		GenreName        string
		GenreGroup       string
		Books            []models.Book
		CurrentlyReading []models.Book
		Completed        []models.Book
		WantToRead       []models.Book
		Abandoned        []models.Book
		UserPermissions  models.UserPermissions
	}{
		LoggedIn:         user != nil,
		Username:         getUsername(user),
		IsAdmin:          user != nil && models.IsAdmin(user.Role),
		SignupDisabled:   true,
		Description:      fmt.Sprintf("Books in %s - Anshuman Biswas Blog", genre.Name),
		CurrentPage:      "books",
		GenreName:        genre.Name,
		GenreGroup:       genre.Group,
		Books:            books,
		CurrentlyReading: currentlyReading,
		Completed:        completed,
		WantToRead:       wantToRead,
		Abandoned:        abandoned,
		UserPermissions:  userPerms,
	}

	b.Templates.BooksList.Execute(w, r, data)
}

// AdminBooks displays the admin books management page.
// GET /admin/books
func (b Books) AdminBooks(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, b.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	if !models.IsAdmin(user.Role) {
		http.Error(w, errForbiddenAdmin, http.StatusForbidden)
		return
	}

	booksList, err := b.BookService.GetAllBooks()
	if err != nil {
		log.Printf("Error fetching books: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	genres, err := b.BookGenreService.GetAll()
	if err != nil {
		log.Printf("Error fetching genres: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Apply status filter if provided
	statusFilter := r.URL.Query().Get("status")
	if statusFilter != "" {
		var filtered []models.Book
		for _, book := range booksList.Books {
			if book.ReadingStatus == statusFilter {
				filtered = append(filtered, book)
			}
		}
		booksList.Books = filtered
	}

	data := struct {
		LoggedIn        bool
		Username        string
		IsAdmin         bool
		SignupDisabled  bool
		Description     string
		CurrentPage     string
		Books           []models.Book
		Genres          []models.BookGenre
		StatusFilter    string
		UserPermissions models.UserPermissions
	}{
		LoggedIn:        true,
		Username:        user.Username,
		IsAdmin:         models.IsAdmin(user.Role),
		SignupDisabled:  true,
		Description:     "Manage Books - Anshuman Biswas Blog",
		CurrentPage:     "admin-books",
		Books:           booksList.Books,
		Genres:          genres,
		StatusFilter:    statusFilter,
		UserPermissions: models.GetPermissions(user.Role),
	}

	b.Templates.AdminBooks.Execute(w, r, data)
}

// NewBook displays the book creation editor.
// GET /admin/books/new
func (b Books) NewBook(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, b.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	if !models.IsAdmin(user.Role) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	genres, err := b.BookGenreService.GetAll()
	if err != nil {
		log.Printf("Error fetching genres: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Generate editor HTML from go-wiki
	editorHTML, err := b.BlogWiki.EditorHTML("")
	if err != nil {
		log.Printf("Error generating editor HTML: %v", err)
	}

	genresByGroup := groupGenres(genres)

	data := struct {
		LoggedIn         bool
		Username         string
		IsAdmin          bool
		SignupDisabled   bool
		Description      string
		CurrentPage      string
		Genres           []models.BookGenre
		GenresByGroup    map[string][]models.BookGenre
		SelectedGenreMap map[int]bool
		Book             *models.Book
		Mode             string
		EditorHTML       template.HTML
		UserPermissions  models.UserPermissions
	}{
		LoggedIn:         true,
		Username:         user.Username,
		IsAdmin:         models.IsAdmin(user.Role),
		SignupDisabled:   true,
		Description:      "Create New Book - Anshuman Biswas Blog",
		CurrentPage:      "admin-books",
		Genres:           genres,
		GenresByGroup:    genresByGroup,
		SelectedGenreMap: map[int]bool{},
		Book:             &models.Book{},
		Mode:             "new",
		EditorHTML:       editorHTML,
		UserPermissions:  models.GetPermissions(user.Role),
	}

	b.Templates.BookEditor.Execute(w, r, data)
}

// CreateBook handles book creation.
// POST /admin/books
func (b Books) CreateBook(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, b.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	if !models.IsAdmin(user.Role) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	slug := r.FormValue("slug")
	bookAuthor := r.FormValue("book_author")
	isbn := r.FormValue("isbn")
	publisher := r.FormValue("publisher")
	pageCountStr := r.FormValue("page_count")
	coverImageURL := r.FormValue("cover_image_url")
	content := r.FormValue("content")
	description := r.FormValue("description")
	myThoughts := r.FormValue("my_thoughts")
	linkURL := r.FormValue("link_url")
	readingStatus := r.FormValue("reading_status")
	ratingStr := r.FormValue("rating")
	dateStarted := r.FormValue("date_started")
	dateFinished := r.FormValue("date_finished")
	isPublished := r.FormValue("is_published") == "on" || r.FormValue("is_published") == "true"
	genresStr := r.FormValue("genres")

	if title == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}

	pageCount, _ := strconv.Atoi(pageCountStr)
	rating, _ := strconv.Atoi(ratingStr)

	var genreIDs []int
	if genresStr != "" {
		for _, gStr := range strings.Split(genresStr, ",") {
			if gID, err := strconv.Atoi(strings.TrimSpace(gStr)); err == nil {
				genreIDs = append(genreIDs, gID)
			}
		}
	}

	book, err := b.BookService.Create(user.UserID, title, slug, bookAuthor, isbn, publisher, pageCount, coverImageURL, content, description, myThoughts, linkURL, readingStatus, rating, dateStarted, dateFinished, isPublished, genreIDs)
	if err != nil {
		log.Printf("Error creating book: %v", err)
		http.Error(w, "Failed to create book", http.StatusInternalServerError)
		return
	}

	_ = b.BookVersionService.MaybeCreateVersion(book.ID, user.UserID, title, content)

	http.Redirect(w, r, fmt.Sprintf("/admin/books/%d/edit", book.ID), http.StatusFound)
}

// EditBook displays the book editing page.
// GET /admin/books/{bookID}/edit
func (b Books) EditBook(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, b.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	if !models.IsAdmin(user.Role) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	bookIDStr := chi.URLParam(r, "bookID")
	bookID, err := strconv.Atoi(bookIDStr)
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}

	book, err := b.BookService.GetByID(bookID)
	if err != nil {
		log.Printf("Error fetching book: %v", err)
		http.Error(w, "Book not found", http.StatusNotFound)
		return
	}

	genres, err := b.BookGenreService.GetAll()
	if err != nil {
		log.Printf("Error fetching genres: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Convert book genres to selected genre map
	selectedGenreMap := make(map[int]bool)
	for _, genre := range book.Genres {
		selectedGenreMap[genre.ID] = true
	}

	genresByGroup := groupGenres(genres)

	// Generate editor HTML from go-wiki
	editorHTML, err := b.BlogWiki.EditorHTML(book.Content)
	if err != nil {
		log.Printf("Error generating editor HTML: %v", err)
	}

	data := struct {
		LoggedIn         bool
		Username         string
		IsAdmin          bool
		SignupDisabled   bool
		Description      string
		CurrentPage      string
		Genres           []models.BookGenre
		GenresByGroup    map[string][]models.BookGenre
		SelectedGenreMap map[int]bool
		Book             *models.Book
		Mode             string
		EditorHTML       template.HTML
		UserPermissions  models.UserPermissions
	}{
		LoggedIn:         true,
		Username:         user.Username,
		IsAdmin:         models.IsAdmin(user.Role),
		SignupDisabled:   true,
		Description:      "Edit Book - Anshuman Biswas Blog",
		CurrentPage:      "admin-books",
		Genres:           genres,
		GenresByGroup:    genresByGroup,
		SelectedGenreMap: selectedGenreMap,
		Book:             book,
		Mode:             "edit",
		EditorHTML:       editorHTML,
		UserPermissions:  models.GetPermissions(user.Role),
	}

	b.Templates.BookEditor.Execute(w, r, data)
}

// UpdateBook handles book updates.
// POST /admin/books/{bookID}
func (b Books) UpdateBook(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, b.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	if !models.IsAdmin(user.Role) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	bookIDStr := chi.URLParam(r, "bookID")
	bookID, err := strconv.Atoi(bookIDStr)
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	slug := r.FormValue("slug")
	bookAuthor := r.FormValue("book_author")
	isbn := r.FormValue("isbn")
	publisher := r.FormValue("publisher")
	pageCountStr := r.FormValue("page_count")
	coverImageURL := r.FormValue("cover_image_url")
	content := r.FormValue("content")
	description := r.FormValue("description")
	myThoughts := r.FormValue("my_thoughts")
	linkURL := r.FormValue("link_url")
	readingStatus := r.FormValue("reading_status")
	ratingStr := r.FormValue("rating")
	dateStarted := r.FormValue("date_started")
	dateFinished := r.FormValue("date_finished")
	isPublished := r.FormValue("is_published") == "on" || r.FormValue("is_published") == "true"
	genresStr := r.FormValue("genres")

	if title == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}

	pageCount, _ := strconv.Atoi(pageCountStr)
	rating, _ := strconv.Atoi(ratingStr)

	var genreIDs []int
	if genresStr != "" {
		for _, gStr := range strings.Split(genresStr, ",") {
			if gID, err := strconv.Atoi(strings.TrimSpace(gStr)); err == nil {
				genreIDs = append(genreIDs, gID)
			}
		}
	}

	err = b.BookService.Update(bookID, title, slug, bookAuthor, isbn, publisher, pageCount, coverImageURL, content, description, myThoughts, linkURL, readingStatus, rating, dateStarted, dateFinished, isPublished, genreIDs)
	if err != nil {
		log.Printf("Error updating book: %v", err)
		http.Error(w, "Failed to update book", http.StatusInternalServerError)
		return
	}

	_ = b.BookVersionService.MaybeCreateVersion(bookID, user.UserID, title, content)

	http.Redirect(w, r, fmt.Sprintf("/admin/books/%d/edit", bookID), http.StatusFound)
}

// DeleteBook handles book deletion.
// POST /admin/books/{bookID}/delete
func (b Books) DeleteBook(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, b.SessionService)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	if !models.IsAdmin(user.Role) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	bookIDStr := chi.URLParam(r, "bookID")
	bookID, err := strconv.Atoi(bookIDStr)
	if err != nil {
		http.Error(w, "Invalid book ID", http.StatusBadRequest)
		return
	}

	err = b.BookService.Delete(bookID)
	if err != nil {
		log.Printf("Error deleting book: %v", err)
		http.Error(w, "Failed to delete book", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/books", http.StatusFound)
}

// PreviewBook handles markdown preview for books.
// POST /admin/books/preview
func (b Books) PreviewBook(w http.ResponseWriter, r *http.Request) {
	user, err := utils.IsUserLoggedIn(r, b.SessionService)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !models.IsAdmin(user.Role) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	content := r.FormValue("content")
	rendered := models.RenderContent(content)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, template.HTML(rendered))
}

// groupGenres organizes genres by their Group field for the editor template.
func groupGenres(genres []models.BookGenre) map[string][]models.BookGenre {
	m := make(map[string][]models.BookGenre)
	for _, g := range genres {
		group := g.Group
		if group == "" {
			group = "other"
		}
		m[group] = append(m[group], g)
	}
	return m
}
