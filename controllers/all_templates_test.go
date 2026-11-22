package controllers

import (
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"anshumanbiswas.com/blog/models"
	"anshumanbiswas.com/blog/templates"
	"anshumanbiswas.com/blog/views"
)

// TestAllTemplatesExecute renders every page template in templates/ with the
// exact data struct shapes produced by the corresponding controllers (both
// populated and empty / alternate modes). Any field mismatch, missing func,
// or template execution error fails the build immediately.
func TestAllTemplatesExecute(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	lastLogin := "2026-05-01 12:00:00"
	lastIP := "127.0.0.1"
	adminPerms := models.GetPermissions(models.RoleAdministrator)
	commenterPerms := models.GetPermissions(models.RoleCommenter)

	sampleUser := &models.User{
		UserID:           1,
		Username:         "admin",
		FullName:         "Anshuman Biswas",
		Email:            "admin@example.com",
		Role:             models.RoleAdministrator,
		AvatarURL:        "/static/images/anshuman-biswas-portrait.jpg",
		Bio:              "Software engineer",
		RegistrationDate: "2025-01-01",
	}
	sampleCategory := models.Category{ID: 1, Name: "Go", CreatedAt: now}
	sampleGenre := models.BookGenre{ID: 1, Name: "Systems", Group: "tech", CreatedAt: now}
	sampleArea := models.PaperResearchArea{ID: 1, Name: "Distributed Systems"}

	samplePost := models.Post{
		ID:                42,
		UserID:            1,
		Username:          "admin",
		AuthorDisplayName: "Anshuman Biswas",
		AuthorAvatarURL:   "/static/images/anshuman-biswas-portrait.jpg",
		AuthorBio:         "Software engineer",
		Title:             "Sample Post",
		Content:           "# Hello\nWorld",
		ContentHTML:       template.HTML("<h1>Hello</h1><p>World</p>"),
		Slug:              "sample-post",
		PublicationDate:   "May 1, 2026",
		LastEditDate:      "May 2, 2026",
		IsPublished:       true,
		Featured:          true,
		FeaturedImageURL:  "/static/placeholder-featured.svg",
		CreatedAt:         "2026-05-01T12:00:00Z",
		ReadingTime:       4,
		Categories:        []models.Category{sampleCategory},
		Contributors:      []models.User{*sampleUser},
	}

	sampleSlide := models.Slide{
		ID:                7,
		UserID:            1,
		Username:          "admin",
		AuthorDisplayName: "Anshuman Biswas",
		AuthorAvatarURL:   "/static/images/anshuman-biswas-portrait.jpg",
		Title:             "Sample Deck",
		Slug:              "sample-deck",
		IsPublished:       true,
		PasswordHash:      "$2a$10$hash",
		SlideMetadata:     `{"format":"pptx"}`,
		Description:       "A sample slide deck",
		FeaturedImageURL:  "/static/placeholder-featured.svg",
		SlideCount:        10,
		CreatedAt:         "2026-05-01",
		UpdatedAt:         "2026-05-02",
		RelativeTime:      "1 day ago",
		Categories:        []models.Category{sampleCategory},
		Contributors:      []models.User{*sampleUser},
		IsDeck:            true,
		DeckName:          "sample.deckcraft",
	}

	sampleGuide := models.Guide{
		ID:                9,
		UserID:            1,
		Username:          "admin",
		AuthorDisplayName: "Anshuman Biswas",
		AuthorAvatarURL:   "/static/images/anshuman-biswas-portrait.jpg",
		Title:             "Sample Guide",
		Content:           "# Guide",
		ContentHTML:       template.HTML("<h1>Guide</h1>"),
		Slug:              "sample-guide",
		Description:       "Guide description",
		FeaturedImageURL:  "/static/placeholder-featured.svg",
		IsPublished:       true,
		PublicationDate:   "May 1, 2026",
		LastEditDate:      "May 2, 2026",
		CreatedAt:         "2026-05-01",
		UpdatedAt:         "2026-05-02",
		ReadingTime:       8,
		Categories:        []models.Category{sampleCategory},
	}

	sampleBook := models.Book{
		ID:                11,
		UserID:            1,
		Username:          "admin",
		AuthorDisplayName: "Anshuman Biswas",
		AuthorAvatarURL:   "/static/images/anshuman-biswas-portrait.jpg",
		Title:             "Designing Data-Intensive Applications",
		Slug:              "designing-data-intensive-applications",
		BookAuthor:        "Martin Kleppmann",
		ISBN:              "9781449373320",
		Publisher:         "O'Reilly",
		PageCount:         616,
		CoverImageURL:     "/static/placeholder-featured.svg",
		Content:           "Review",
		ContentHTML:       template.HTML("<p>Review</p>"),
		Description:       "Great book",
		MyThoughts:        "Notes",
		MyThoughtsHTML:    template.HTML("<p>Notes</p>"),
		LinkURL:           "https://www.goodreads.com/book/show/23463279",
		ReadingStatus:     "completed",
		Rating:            4.5,
		AmazonASIN:        "1449373321",
		Medium:            "ebook",
		EbookReader:       "Kindle Scribe",
		SourceName:        "Audible",
		SourceURL:         "https://audible.com",
		DateStarted:       "2026-01-01",
		DateFinished:      "2026-02-01",
		IsPublished:       true,
		PublicationDate:   "May 1, 2026",
		CreatedAt:         "2026-05-01",
		UpdatedAt:         "2026-05-02",
		ReadingTime:       5,
		Genres:            []models.BookGenre{sampleGenre},
	}

	samplePaper := models.Paper{
		ID:                15,
		UserID:            1,
		Username:          "admin",
		AuthorDisplayName: "Anshuman Biswas",
		AuthorAvatarURL:   "/static/images/anshuman-biswas-portrait.jpg",
		Title:             "MapReduce: Simplified Data Processing on Large Clusters",
		Slug:              "mapreduce",
		PaperAuthors:      "Jeffrey Dean, Sanjay Ghemawat",
		Abstract:          "Abstract text",
		PaperYear:         2004,
		Conference:        "OSDI",
		DOI:               "10.1145/1327452.1327492",
		ArxivID:           "1234.5678",
		PDFFileURL:        "/tmp/sample.pdf",
		PDFFileSize:       102400,
		CoverImageURL:     "/static/placeholder-featured.svg",
		Content:           "Paper review",
		ContentHTML:       template.HTML("<p>Paper review</p>"),
		Description:       "Classic distributed systems paper",
		MyNotes:           "My notes",
		MyNotesHTML:       template.HTML("<p>My notes</p>"),
		Rating:            5.0,
		IsPublished:       true,
		PublicationDate:   "May 1, 2026",
		CreatedAt:         "2026-05-01",
		UpdatedAt:         "2026-05-02",
		ReadingTime:       6,
		ResearchAreas:     []models.PaperResearchArea{sampleArea},
	}

	samplePaperAnnotation := models.PaperAnnotation{
		ID:           3,
		PaperID:      15,
		AuthorID:     1,
		AuthorName:   "Anshuman Biswas",
		PageNumber:   2,
		SelectedText: "MapReduce is a programming model",
		BoundingBox:  json.RawMessage(`{}`),
		Color:        "yellow",
		Note:         "Key insight",
		IsPublic:     true,
		SortOrder:    1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	sampleReviewVersion := models.BookReviewVersion{
		ID:         1,
		BookID:     11,
		Rating:     4.0,
		MyThoughts: "Earlier thoughts",
		ReplacedBy: "goodreads-sync",
		CreatedAt:  now,
	}

	sampleReq := httptest.NewRequest(http.MethodGet, "https://anshumanbiswas.com/", nil)

	staticData := struct {
		Email           string
		LoggedIn        bool
		SignupDisabled  bool
		IsAdmin         bool
		Description     string
		CurrentPage     string
		Username        string
		UserPermissions models.UserPermissions
	}{
		Email:           "admin@example.com",
		LoggedIn:        true,
		SignupDisabled:  true,
		IsAdmin:         true,
		Description:     "Static page",
		CurrentPage:     "about",
		Username:        "admin",
		UserPermissions: adminPerms,
	}

	cases := []struct {
		name     string
		template string
		data     any
	}{
		{
			name:     "about.gohtml",
			template: "about.gohtml",
			data:     staticData,
		},
		{
			name:     "admin-formatting-guide.gohtml",
			template: "admin-formatting-guide.gohtml",
			data:     staticData,
		},
		{
			name:     "complete-formatting-guide.gohtml",
			template: "complete-formatting-guide.gohtml",
			data:     staticData,
		},
		{
			name:     "admin-analytics.gohtml",
			template: "admin-analytics.gohtml",
			data: struct {
				Email           string
				LoggedIn        bool
				Username        string
				IsAdmin         bool
				SignupDisabled  bool
				Description     string
				CurrentPage     string
				User            *models.User
				UserPermissions models.UserPermissions
				LiveStats       *models.LiveStats
			}{
				Email:           "admin@example.com",
				LoggedIn:        true,
				Username:        "admin",
				IsAdmin:         true,
				SignupDisabled:  true,
				Description:     "Analytics",
				CurrentPage:     "admin-analytics",
				User:            sampleUser,
				UserPermissions: adminPerms,
				LiveStats:       &models.LiveStats{TodayViews: 100, TodayUniques: 50},
			},
		},
		{
			name:     "admin-books.gohtml (with cover and without cover)",
			template: "admin-books.gohtml",
			data: struct {
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
				LoggedIn:       true,
				Username:       "admin",
				IsAdmin:        true,
				SignupDisabled: true,
				Description:    "Manage Books",
				CurrentPage:    "admin-books",
				Books: []models.Book{
					sampleBook,
					{ID: 12, Title: "No Cover Book", Slug: "no-cover", BookAuthor: "Author", ReadingStatus: "reading"},
				},
				Genres:          []models.BookGenre{sampleGenre},
				StatusFilter:    "",
				UserPermissions: adminPerms,
			},
		},
		{
			name:     "admin-categories.gohtml",
			template: "admin-categories.gohtml",
			data: struct {
				Email           string
				LoggedIn        bool
				Username        string
				IsAdmin         bool
				SignupDisabled  bool
				Description     string
				CurrentPage     string
				User            *models.User
				Categories      []models.Category
				PostCounts      map[int]int
				Flash           string
				UserPermissions models.UserPermissions
			}{
				Email:           "admin@example.com",
				LoggedIn:        true,
				Username:        "admin",
				IsAdmin:         true,
				SignupDisabled:  true,
				Description:     "Manage Categories",
				CurrentPage:     "admin-categories",
				User:            sampleUser,
				Categories:      []models.Category{sampleCategory},
				PostCounts:      map[int]int{1: 5},
				Flash:           "Updated",
				UserPermissions: adminPerms,
			},
		},
		{
			name:     "admin-guides.gohtml",
			template: "admin-guides.gohtml",
			data: struct {
				LoggedIn        bool
				Username        string
				IsAdmin         bool
				SignupDisabled  bool
				Description     string
				CurrentPage     string
				Guides          *models.GuidesList
				UserPermissions models.UserPermissions
			}{
				LoggedIn:        true,
				Username:        "admin",
				IsAdmin:         true,
				SignupDisabled:  true,
				Description:     "Manage Guides",
				CurrentPage:     "admin-guides",
				Guides:          &models.GuidesList{Guides: []models.Guide{sampleGuide}},
				UserPermissions: adminPerms,
			},
		},
		{
			name:     "admin-papers.gohtml (populated + empty cover)",
			template: "admin-papers.gohtml",
			data: struct {
				LoggedIn        bool
				Username        string
				IsAdmin         bool
				SignupDisabled  bool
				Description     string
				CurrentPage     string
				Papers          []models.Paper
				ResearchAreas   []models.PaperResearchArea
				UserPermissions models.UserPermissions
			}{
				LoggedIn:       true,
				Username:       "admin",
				IsAdmin:        true,
				SignupDisabled: true,
				Description:    "Manage Papers",
				CurrentPage:    "admin-papers",
				Papers: []models.Paper{
					samplePaper,
					{ID: 16, Title: "No Cover Paper", Slug: "no-cover-paper", PaperAuthors: "Author", PaperYear: 2025},
				},
				ResearchAreas:   []models.PaperResearchArea{sampleArea},
				UserPermissions: adminPerms,
			},
		},
		{
			name:     "admin-papers.gohtml (empty)",
			template: "admin-papers.gohtml",
			data: struct {
				LoggedIn        bool
				Username        string
				IsAdmin         bool
				SignupDisabled  bool
				Description     string
				CurrentPage     string
				Papers          []models.Paper
				ResearchAreas   []models.PaperResearchArea
				UserPermissions models.UserPermissions
			}{
				LoggedIn:        true,
				Username:        "admin",
				IsAdmin:         true,
				SignupDisabled:  true,
				Description:     "Manage Papers",
				CurrentPage:     "admin-papers",
				UserPermissions: adminPerms,
			},
		},
		{
			name:     "admin-posts.gohtml",
			template: "admin-posts.gohtml",
			data: struct {
				Email           string
				LoggedIn        bool
				Username        string
				IsAdmin         bool
				SignupDisabled  bool
				Description     string
				CurrentPage     string
				Posts           *models.PostsList
				UserPermissions models.UserPermissions
			}{
				Email:           "admin@example.com",
				LoggedIn:        true,
				Username:        "admin",
				IsAdmin:         true,
				SignupDisabled:  true,
				Description:     "Manage Posts",
				CurrentPage:     "admin-posts",
				Posts:           &models.PostsList{Posts: []models.Post{samplePost}},
				UserPermissions: adminPerms,
			},
		},
		{
			name:     "admin-security.gohtml",
			template: "admin-security.gohtml",
			data: securityDashboardData{
				Email:           "admin@example.com",
				LoggedIn:        true,
				Username:        "admin",
				IsAdmin:         true,
				SignupDisabled:  true,
				Description:     "Security",
				CurrentPage:     "admin-security",
				User:            sampleUser,
				BannedRules:     []models.IPRule{{ID: 1, IPAddress: "1.2.3.4", Action: "ban", Reason: "spam", CreatedAt: "2026-05-01"}},
				AllowedRules:    []models.IPRule{{ID: 2, IPAddress: "5.6.7.8", Action: "allow", Reason: "office", CreatedAt: "2026-05-01"}},
				UserPermissions: adminPerms,
			},
		},
		{
			name:     "admin-slides.gohtml",
			template: "admin-slides.gohtml",
			data: struct {
				LoggedIn        bool
				Username        string
				IsAdmin         bool
				SignupDisabled  bool
				Description     string
				CurrentPage     string
				Slides          *models.SlidesList
				Categories      []models.Category
				UserPermissions models.UserPermissions
			}{
				LoggedIn:        true,
				Username:        "admin",
				IsAdmin:         true,
				SignupDisabled:  true,
				Description:     "Manage Slides",
				CurrentPage:     "admin-slides",
				Slides:          &models.SlidesList{Slides: []models.Slide{sampleSlide}},
				Categories:      []models.Category{sampleCategory},
				UserPermissions: adminPerms,
			},
		},
		{
			name:     "admin-system.gohtml",
			template: "admin-system.gohtml",
			data: struct {
				Email           string
				LoggedIn        bool
				Username        string
				IsAdmin         bool
				SignupDisabled  bool
				Description     string
				CurrentPage     string
				User            *models.User
				SystemInfo      *models.SystemInfo
				UserPermissions models.UserPermissions
			}{
				Email:           "admin@example.com",
				LoggedIn:        true,
				Username:        "admin",
				IsAdmin:         true,
				SignupDisabled:  true,
				Description:     "System",
				CurrentPage:     "admin-system",
				User:            sampleUser,
				SystemInfo:      &models.SystemInfo{},
				UserPermissions: adminPerms,
			},
		},
		{
			name:     "admin-users.gohtml",
			template: "admin-users.gohtml",
			data: adminUsersDashboardData{
				Email:          "admin@example.com",
				LoggedIn:       true,
				Username:       "admin",
				IsAdmin:        true,
				SignupDisabled: true,
				Description:    "User Management",
				CurrentPage:    "admin",
				User:           sampleUser,
				Users: []models.UserWithStats{{
					UserID:           1,
					Username:         "admin",
					FullName:         "Anshuman Biswas",
					Email:            "admin@example.com",
					Role:             models.RoleAdministrator,
					RoleName:         "Admin",
					RegistrationDate: "2025-01-01",
					AuthProvider:     "local",
					LoginCount:       10,
					FailedAttempts:   1,
					LastLoginAt:      &lastLogin,
					LastLoginIP:      &lastIP,
				}},
				UserPermissions: adminPerms,
			},
		},
		{
			name:     "api-access.gohtml",
			template: "api-access.gohtml",
			data: struct {
				Email           string
				LoggedIn        bool
				Username        string
				IsAdmin         bool
				SignupDisabled  bool
				Description     string
				CurrentPage     string
				Message         string
				Tokens          []*models.APIToken
				UserPermissions models.UserPermissions
				Request         *http.Request
			}{
				Email:           "admin@example.com",
				LoggedIn:        true,
				Username:        "admin",
				IsAdmin:         true,
				SignupDisabled:  true,
				Description:     "API Access",
				CurrentPage:     "api-access",
				Message:         "Created",
				Tokens:          []*models.APIToken{{ID: 1, UserID: 1, Name: "CLI", CreatedAt: "2026-05-01", IsActive: true}},
				UserPermissions: adminPerms,
				Request:         sampleReq,
			},
		},
		{
			name:     "blogpost.gohtml",
			template: "blogpost.gohtml",
			data: struct {
				LoggedIn        bool
				Email           string
				Username        string
				IsAdmin         bool
				CanAnnotate     bool
				UserID          int
				SignupDisabled  bool
				Description     string
				CurrentPage     string
				ReadTime        string
				FullURL         string
				OGImage         string
				OGDescription   string
				Post            *models.Post
				PrevPost        *models.Post
				NextPost        *models.Post
				UserPermissions models.UserPermissions
				Contributors    []models.User
			}{
				LoggedIn:        true,
				Email:           "admin@example.com",
				Username:        "admin",
				IsAdmin:         true,
				CanAnnotate:     true,
				UserID:          1,
				SignupDisabled:  true,
				Description:     "Sample Post",
				CurrentPage:     "blog",
				ReadTime:        "4",
				FullURL:         "https://anshumanbiswas.com/blog/sample-post",
				OGImage:         "https://anshumanbiswas.com/static/placeholder-featured.svg",
				OGDescription:   "Sample description",
				Post:            &samplePost,
				PrevPost:        &samplePost,
				NextPost:        &samplePost,
				UserPermissions: adminPerms,
				Contributors:    []models.User{*sampleUser},
			},
		},
		{
			name:     "book-editor.gohtml (edit)",
			template: "book-editor.gohtml",
			data: struct {
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
				ReviewHistory    []models.BookReviewVersion
				UserPermissions  models.UserPermissions
			}{
				LoggedIn:         true,
				Username:         "admin",
				IsAdmin:          true,
				SignupDisabled:   true,
				Description:      "Edit Book",
				CurrentPage:      "admin-books",
				Genres:           []models.BookGenre{sampleGenre},
				GenresByGroup:    map[string][]models.BookGenre{"tech": {sampleGenre}},
				SelectedGenreMap: map[int]bool{1: true},
				Book:             &sampleBook,
				Mode:             "edit",
				EditorHTML:       template.HTML("<textarea></textarea>"),
				ReviewHistory:    []models.BookReviewVersion{sampleReviewVersion},
				UserPermissions:  adminPerms,
			},
		},
		{
			name:     "book-page.gohtml",
			template: "book-page.gohtml",
			data: struct {
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
				LoggedIn:        true,
				Username:        "admin",
				IsAdmin:         true,
				SignupDisabled:  true,
				UserID:          1,
				Description:     sampleBook.Title,
				CurrentPage:     "books",
				Book:            &sampleBook,
				FullURL:         "https://anshumanbiswas.com/books/designing-data-intensive-applications",
				UserPermissions: adminPerms,
			},
		},
		{
			name:     "books-list.gohtml (public list)",
			template: "books-list.gohtml",
			data: struct {
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
				FilterName       string
				FilterType       string
			}{
				LoggedIn:         true,
				Username:         "admin",
				IsAdmin:          true,
				SignupDisabled:   true,
				Description:      "Books",
				CurrentPage:      "books",
				Books:            &models.BooksList{Books: []models.Book{sampleBook}},
				CurrentlyReading: []models.Book{sampleBook},
				Completed:        []models.Book{sampleBook},
				WantToRead:       []models.Book{sampleBook},
				Abandoned:        []models.Book{sampleBook},
				UserPermissions:  adminPerms,
			},
		},
		{
			name:     "books-list.gohtml (GenrePage)",
			template: "books-list.gohtml",
			data: struct {
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
				FilterName       string
				FilterType       string
			}{
				LoggedIn:         false,
				SignupDisabled:   true,
				Description:      "Books in Systems",
				CurrentPage:      "books",
				GenreName:        "Systems",
				GenreGroup:       "tech",
				Books:            []models.Book{sampleBook},
				Completed:        []models.Book{sampleBook},
				UserPermissions:  commenterPerms,
				FilterName:       "Systems",
				FilterType:       "genre",
			},
		},
		{
			name:     "guide-editor.gohtml (edit)",
			template: "guide-editor.gohtml",
			data: struct {
				LoggedIn           bool
				Username           string
				IsAdmin            bool
				SignupDisabled     bool
				Description        string
				CurrentPage        string
				Categories         []models.Category
				SelectedCategories []int
				Guide              *models.Guide
				Mode               string
				EditorHTML         template.HTML
				UserPermissions    models.UserPermissions
			}{
				LoggedIn:           true,
				Username:           "admin",
				IsAdmin:            true,
				SignupDisabled:     true,
				Description:        "Edit Guide",
				CurrentPage:        "admin-guides",
				Categories:         []models.Category{sampleCategory},
				SelectedCategories: []int{1},
				Guide:              &sampleGuide,
				Mode:               "edit",
				EditorHTML:         template.HTML("<textarea></textarea>"),
				UserPermissions:    adminPerms,
			},
		},
		{
			name:     "guide-page.gohtml",
			template: "guide-page.gohtml",
			data: struct {
				LoggedIn        bool
				Username        string
				IsAdmin         bool
				SignupDisabled  bool
				UserID          int
				Description     string
				CurrentPage     string
				Guide           *models.Guide
				FullURL         string
				UserPermissions models.UserPermissions
			}{
				LoggedIn:        true,
				Username:        "admin",
				IsAdmin:         true,
				SignupDisabled:  true,
				UserID:          1,
				Description:     sampleGuide.Title,
				CurrentPage:     "guides",
				Guide:           &sampleGuide,
				FullURL:         "https://anshumanbiswas.com/guides/sample-guide",
				UserPermissions: adminPerms,
			},
		},
		{
			name:     "guides-list.gohtml",
			template: "guides-list.gohtml",
			data: struct {
				LoggedIn        bool
				Username        string
				IsAdmin         bool
				SignupDisabled  bool
				Description     string
				CurrentPage     string
				Guides          *models.GuidesList
				UserPermissions models.UserPermissions
			}{
				LoggedIn:        true,
				Username:        "admin",
				IsAdmin:         true,
				SignupDisabled:  true,
				Description:     "Guides",
				CurrentPage:     "guides",
				Guides:          &models.GuidesList{Guides: []models.Guide{sampleGuide}},
				UserPermissions: adminPerms,
			},
		},
		{
			name:     "home.gohtml (admin)",
			template: "home.gohtml",
			data: struct {
				Email           string
				LoggedIn        bool
				Posts           *models.PostsList
				Username        string
				IsAdmin         bool
				SignupDisabled  bool
				Description     string
				CurrentPage     string
				UserPermissions models.UserPermissions
			}{
				Email:           "admin@example.com",
				LoggedIn:        true,
				Posts:           &models.PostsList{Posts: []models.Post{samplePost}},
				Username:        "admin",
				IsAdmin:         true,
				SignupDisabled:  true,
				Description:     "Home",
				CurrentPage:     "home",
				UserPermissions: adminPerms,
			},
		},
		{
			name:     "home.gohtml (commenter nav check)",
			template: "home.gohtml",
			data: struct {
				Email           string
				LoggedIn        bool
				Posts           *models.PostsList
				Username        string
				IsAdmin         bool
				SignupDisabled  bool
				Description     string
				CurrentPage     string
				UserPermissions models.UserPermissions
			}{
				Email:           "reader@example.com",
				LoggedIn:        true,
				Posts:           &models.PostsList{Posts: []models.Post{samplePost}},
				Username:        "reader",
				IsAdmin:         false,
				SignupDisabled:  true,
				Description:     "Home",
				CurrentPage:     "home",
				UserPermissions: commenterPerms,
			},
		},
		{
			name:     "paper-editor.gohtml (new)",
			template: "paper-editor.gohtml",
			data: struct {
				LoggedIn        bool
				Username        string
				IsAdmin         bool
				SignupDisabled  bool
				Description     string
				CurrentPage     string
				ResearchAreas   []models.PaperResearchArea
				SelectedAreaMap map[int]bool
				Paper           *models.Paper
				Mode            string
				EditorHTML      template.HTML
				UserPermissions models.UserPermissions
			}{
				LoggedIn:        true,
				Username:        "admin",
				IsAdmin:         true,
				SignupDisabled:  true,
				Description:     "New Paper",
				CurrentPage:     "admin-papers",
				ResearchAreas:   []models.PaperResearchArea{sampleArea},
				SelectedAreaMap: map[int]bool{},
				Paper:           &models.Paper{},
				Mode:            "new",
				EditorHTML:      template.HTML("<textarea></textarea>"),
				UserPermissions: adminPerms,
			},
		},
		{
			name:     "paper-editor.gohtml (edit)",
			template: "paper-editor.gohtml",
			data: struct {
				LoggedIn        bool
				Username        string
				IsAdmin         bool
				SignupDisabled  bool
				Description     string
				CurrentPage     string
				ResearchAreas   []models.PaperResearchArea
				SelectedAreaMap map[int]bool
				Paper           *models.Paper
				Mode            string
				EditorHTML      template.HTML
				Annotations     []models.PaperAnnotation
				UserPermissions models.UserPermissions
			}{
				LoggedIn:        true,
				Username:        "admin",
				IsAdmin:         true,
				SignupDisabled:  true,
				Description:     "Edit Paper",
				CurrentPage:     "admin-papers",
				ResearchAreas:   []models.PaperResearchArea{sampleArea},
				SelectedAreaMap: map[int]bool{1: true},
				Paper:           &samplePaper,
				Mode:            "edit",
				EditorHTML:      template.HTML("<textarea></textarea>"),
				Annotations:     []models.PaperAnnotation{samplePaperAnnotation},
				UserPermissions: adminPerms,
			},
		},
		{
			name:     "paper-page.gohtml",
			template: "paper-page.gohtml",
			data: struct {
				LoggedIn          bool
				Username          string
				IsAdmin           bool
				SignupDisabled    bool
				UserID            int
				Description       string
				CurrentPage       string
				Paper             *models.Paper
				PublicAnnotations []models.PaperAnnotation
				FullURL           string
				UserPermissions   models.UserPermissions
			}{
				LoggedIn:          true,
				Username:          "admin",
				IsAdmin:           true,
				SignupDisabled:    true,
				UserID:            1,
				Description:       samplePaper.Title,
				CurrentPage:       "papers",
				Paper:             &samplePaper,
				PublicAnnotations: []models.PaperAnnotation{samplePaperAnnotation},
				FullURL:           "https://anshumanbiswas.com/papers/mapreduce",
				UserPermissions:   adminPerms,
			},
		},
		{
			name:     "papers-list.gohtml",
			template: "papers-list.gohtml",
			data: struct {
				LoggedIn        bool
				Username        string
				IsAdmin         bool
				SignupDisabled  bool
				Description     string
				CurrentPage     string
				Papers          []models.Paper
				UserPermissions models.UserPermissions
				FilterName      string
				FilterType      string
			}{
				LoggedIn:        true,
				Username:        "admin",
				IsAdmin:         true,
				SignupDisabled:  true,
				Description:     "Papers",
				CurrentPage:     "papers",
				Papers:          []models.Paper{samplePaper},
				UserPermissions: adminPerms,
			},
		},
		{
			name:     "post-editor.gohtml (edit)",
			template: "post-editor.gohtml",
			data: struct {
				Email              string
				LoggedIn           bool
				Username           string
				IsAdmin            bool
				SignupDisabled     bool
				Description        string
				CurrentPage        string
				UserPermissions    models.UserPermissions
				Mode               string
				Post               *models.Post
				Categories         []models.Category
				SelectedCategories []int
				EditorHTML         template.HTML
			}{
				Email:              "admin@example.com",
				LoggedIn:           true,
				Username:           "admin",
				IsAdmin:            true,
				SignupDisabled:     true,
				Description:        "Edit Post",
				CurrentPage:        "admin-posts",
				UserPermissions:    adminPerms,
				Mode:               "edit",
				Post:               &samplePost,
				Categories:         []models.Category{sampleCategory},
				SelectedCategories: []int{1},
				EditorHTML:         template.HTML("<textarea></textarea>"),
			},
		},
		{
			name:     "profile.gohtml",
			template: "profile.gohtml",
			data: struct {
				Email           string
				LoggedIn        bool
				Username        string
				FullName        string
				AvatarURL       string
				Bio             string
				IsAdmin         bool
				SignupDisabled  bool
				Description     string
				CurrentPage     string
				Message         string
				UserPermissions models.UserPermissions
			}{
				Email:           "admin@example.com",
				LoggedIn:        true,
				Username:        "admin",
				FullName:        "Anshuman Biswas",
				AvatarURL:       "/static/images/anshuman-biswas-portrait.jpg",
				Bio:             "Engineer",
				IsAdmin:         true,
				SignupDisabled:  true,
				Description:     "Profile",
				CurrentPage:     "profile",
				Message:         "Saved",
				UserPermissions: adminPerms,
			},
		},
		{
			name:     "signin.gohtml",
			template: "signin.gohtml",
			data: struct {
				Email           string
				LoggedIn        bool
				SignupDisabled  bool
				IsAdmin         bool
				Description     string
				CurrentPage     string
				Username        string
				UserPermissions models.UserPermissions
			}{
				Email:           "user@example.com",
				LoggedIn:        false,
				SignupDisabled:  false,
				IsAdmin:         false,
				Description:     "Sign in",
				CurrentPage:     "signin",
				Username:        "",
				UserPermissions: commenterPerms,
			},
		},
		{
			name:     "signup.gohtml (enabled)",
			template: "signup.gohtml",
			data: struct {
				Email            string
				Username         string
				LoggedIn         bool
				IsSignupDisabled bool
				SignupDisabled   bool
				IsAdmin          bool
				Description      string
				CurrentPage      string
				UserPermissions  models.UserPermissions
			}{
				Email:            "user@example.com",
				Username:         "newuser",
				LoggedIn:         false,
				IsSignupDisabled: false,
				SignupDisabled:   false,
				IsAdmin:          false,
				Description:      "Sign up",
				CurrentPage:      "signup",
				UserPermissions:  commenterPerms,
			},
		},
		{
			name:     "slide-deck.gohtml",
			template: "slide-deck.gohtml",
			data: struct {
				LoggedIn        bool
				Username        string
				IsAdmin         bool
				SignupDisabled  bool
				Description     string
				CurrentPage     string
				Slide           *models.Slide
				UserPermissions models.UserPermissions
			}{
				LoggedIn:        true,
				Username:        "admin",
				IsAdmin:         true,
				SignupDisabled:  true,
				Description:     sampleSlide.Title,
				CurrentPage:     "slides",
				Slide:           &sampleSlide,
				UserPermissions: adminPerms,
			},
		},
		{
			name:     "slide-editor.gohtml",
			template: "slide-editor.gohtml",
			data: struct {
				LoggedIn        bool
				Username        string
				IsAdmin         bool
				SignupDisabled  bool
				Description     string
				CurrentPage     string
				Categories      []models.Category
				Slide           *models.Slide
				IsEdit          bool
				UserPermissions models.UserPermissions
			}{
				LoggedIn:        true,
				Username:        "admin",
				IsAdmin:         true,
				SignupDisabled:  true,
				Description:     "Edit Slide",
				CurrentPage:     "admin-slides",
				Categories:      []models.Category{sampleCategory},
				Slide:           &sampleSlide,
				IsEdit:          true,
				UserPermissions: adminPerms,
			},
		},
		{
			name:     "slide-password.gohtml",
			template: "slide-password.gohtml",
			data: struct {
				LoggedIn        bool
				Username        string
				IsAdmin         bool
				SignupDisabled  bool
				Description     string
				CurrentPage     string
				Slide           *models.Slide
				Error           string
				UserPermissions models.UserPermissions
			}{
				LoggedIn:        false,
				SignupDisabled:  true,
				Description:     "Password Required",
				CurrentPage:     "slides",
				Slide:           &sampleSlide,
				Error:           "Incorrect password",
				UserPermissions: commenterPerms,
			},
		},
		{
			name:     "slides-list.gohtml",
			template: "slides-list.gohtml",
			data: struct {
				LoggedIn        bool
				Username        string
				IsAdmin         bool
				SignupDisabled  bool
				Description     string
				CurrentPage     string
				Slides          *models.SlidesList
				UserPermissions models.UserPermissions
			}{
				LoggedIn:        true,
				Username:        "admin",
				IsAdmin:         true,
				SignupDisabled:  true,
				Description:     "Slides",
				CurrentPage:     "slides",
				Slides:          &models.SlidesList{Slides: []models.Slide{sampleSlide}},
				UserPermissions: adminPerms,
			},
		},
		{
			name:     "tag-page.gohtml",
			template: "tag-page.gohtml",
			data: struct {
				TagName         string
				Posts           []models.Post
				Slides          []models.Slide
				Guides          []models.Guide
				PostCount       int
				SlideCount      int
				GuideCount      int
				TotalCount      int
				LoggedIn        bool
				IsAdmin         bool
				Username        string
				Description     string
				CurrentPage     string
				UserPermissions models.UserPermissions
				SignupDisabled  bool
			}{
				TagName:         "Go",
				Posts:           []models.Post{samplePost},
				Slides:          []models.Slide{sampleSlide},
				Guides:          []models.Guide{sampleGuide},
				PostCount:       1,
				SlideCount:      1,
				GuideCount:      1,
				TotalCount:      3,
				LoggedIn:        true,
				IsAdmin:         true,
				Username:        "admin",
				Description:     "Tag Go",
				CurrentPage:     "tags",
				UserPermissions: adminPerms,
				SignupDisabled:  true,
			},
		},
		{
			name:     "user-posts.gohtml",
			template: "user-posts.gohtml",
			data: struct {
				Email           string
				LoggedIn        bool
				Username        string
				IsAdmin         bool
				SignupDisabled  bool
				Description     string
				CurrentPage     string
				Posts           *models.PostsList
				UserPermissions models.UserPermissions
				UserComments    []UserComment
				UserAnnotations []UserAnnotation
			}{
				Email:           "admin@example.com",
				LoggedIn:        true,
				Username:        "admin",
				IsAdmin:         true,
				SignupDisabled:  true,
				Description:     "My Posts",
				CurrentPage:     "my-posts",
				Posts:           &models.PostsList{Posts: []models.Post{samplePost}},
				UserPermissions: adminPerms,
				UserComments:    []UserComment{{CommentID: 1, PostTitle: "Sample Post", PostSlug: "sample-post", Content: "Nice!", CommentDate: now}},
				UserAnnotations: []UserAnnotation{{ID: 1, PostTitle: "Sample Post", PostSlug: "sample-post", SelectedText: "World", Color: "yellow", CreatedAt: now}},
			},
		},
		{
			name:     "user-profile.gohtml",
			template: "user-profile.gohtml",
			data: struct {
				LoggedIn         bool
				Email            string
				Username         string
				IsAdmin          bool
				SignupDisabled   bool
				Description      string
				CurrentPage      string
				UserPermissions  models.UserPermissions
				ProfileUser      *models.User
				AuthoredPosts    []models.Post
				ContributedPosts []models.Post
				AuthoredSlides   []models.Slide
				AuthoredGuides   []models.Guide
				UserComments     []UserComment
			}{
				LoggedIn:         true,
				Email:            "admin@example.com",
				Username:         "admin",
				IsAdmin:          true,
				SignupDisabled:   true,
				Description:      "Profile",
				CurrentPage:      "profile",
				UserPermissions:  adminPerms,
				ProfileUser:      sampleUser,
				AuthoredPosts:    []models.Post{samplePost},
				ContributedPosts: []models.Post{samplePost},
				AuthoredSlides:   []models.Slide{sampleSlide},
				AuthoredGuides:   []models.Guide{sampleGuide},
				UserComments:     []UserComment{{CommentID: 1, PostTitle: "Sample Post", PostSlug: "sample-post", Content: "Nice!", CommentDate: now}},
			},
		},
	}

	tested := map[string]bool{}
	nonIntAdminEdit := regexp.MustCompile(`/admin/(?:posts|slides|guides|books|papers)/([^/]+)/edit`)

	for _, tc := range cases {
		tested[tc.template] = true
		t.Run(tc.name, func(t *testing.T) {
			tpl, err := views.ParseFS(templates.FS, tc.template, "tailwind.gohtml")
			if err != nil {
				t.Fatalf("ParseFS(%s) failed: %v", tc.template, err)
			}
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "https://anshumanbiswas.com/", nil)
			tpl.Execute(w, r, tc.data)

			body := w.Body.String()
			if w.Code != http.StatusOK {
				t.Fatalf("%s returned status %d: %s", tc.template, w.Code, body)
			}
			if strings.Contains(body, "There was an error executing the template") {
				t.Fatalf("%s failed template execution", tc.template)
			}
			if strings.Contains(body, "<!-- icon not found:") {
				t.Errorf("%s rendered a missing icon placeholder", tc.template)
			}
			if strings.Contains(body, `href="/reset-pw"`) {
				t.Errorf("%s contains dead link /reset-pw", tc.template)
			}
			if strings.Contains(body, `href="/admin/slides/new"`) {
				t.Errorf("%s contains dead link /admin/slides/new", tc.template)
			}
			if (tc.template == "paper-page.gohtml" || tc.template == "papers-list.gohtml") &&
				(strings.Contains(body, `href="/books/author/`) || strings.Contains(body, `href="/books/publisher/`)) {
				t.Errorf("%s links paper author/conference into /books/ filter routes", tc.template)
			}
			if tc.name == "home.gohtml (commenter nav check)" && strings.Contains(body, `href="/admin/categories"`) {
				t.Errorf("commenter nav should not expose admin-only /admin/categories link")
			}
			for _, m := range nonIntAdminEdit.FindAllStringSubmatch(body, -1) {
				id := m[1]
				for _, ch := range id {
					if ch < '0' || ch > '9' {
						t.Errorf("%s rendered non-numeric admin edit link %q (controller expects numeric ID)", tc.template, m[0])
						break
					}
				}
			}
		})
	}

	// Verify every .gohtml template in templates/ (other than layout tailwind.gohtml and standalone NotFoundPage.gohtml) is covered.
	entries, err := fs.ReadDir(templates.FS, ".")
	if err != nil {
		t.Fatalf("ReadDir templates.FS: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".gohtml") {
			continue
		}
		if name == "tailwind.gohtml" || name == "NotFoundPage.gohtml" {
			continue
		}
		if !tested[name] {
			t.Errorf("template %q is not covered by TestAllTemplatesExecute; add a test case for it", name)
		}
	}

	// Ensure standalone NotFoundPage.gohtml exists and has valid static assets.
	notFoundPath := filepath.Join("..", "templates", "NotFoundPage.gohtml")
	if _, err := os.Stat(notFoundPath); err != nil {
		t.Errorf("NotFoundPage.gohtml missing: %v", err)
	}
}

// TestTemplateInternalLinksMatchRoutes parses every route registered in main.go
// and checks every internal href/action/src in templates/*.gohtml so dead links
// and missing static assets fail CI immediately.
func TestTemplateInternalLinksMatchRoutes(t *testing.T) {
	mainBytes, err := os.ReadFile(filepath.Join("..", "main.go"))
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	mainSrc := string(mainBytes)

	// Resolve string constants in main.go.
	consts := map[string]string{}
	constRe := regexp.MustCompile(`(?m)^\s*(\w+)\s*=\s*"([^"]+)"`)
	for _, m := range constRe.FindAllStringSubmatch(mainSrc, -1) {
		consts[m[1]] = m[2]
	}

	// Extract route patterns (accounting for r.Route("/prefix", ...)).
	routeRe := regexp.MustCompile(`r\.(?:Get|Post|Put|Delete|Patch|Handle|HandleFunc)\(([^,]+),`)
	groupRe := regexp.MustCompile(`r\.Route\("([^"]+)",`)

	type prefixEntry struct {
		prefix string
		depth  int
	}
	var stack []prefixEntry
	depth := 0
	var compiled []*regexp.Regexp

	toRegex := func(pat string) *regexp.Regexp {
		if strings.HasSuffix(pat, "/*") {
			return regexp.MustCompile("^" + regexp.QuoteMeta(strings.TrimSuffix(pat, "/*")) + "(/.*)?$")
		}
		paramRe := regexp.MustCompile(`\{[^}]+\}`)
		parts := paramRe.Split(pat, -1)
		var b strings.Builder
		b.WriteString("^")
		for i, p := range parts {
			b.WriteString(regexp.QuoteMeta(p))
			if i < len(parts)-1 {
				b.WriteString(`[^/]+`)
			}
		}
		b.WriteString("/?$")
		return regexp.MustCompile(b.String())
	}

	for _, line := range strings.Split(mainSrc, "\n") {
		if gm := groupRe.FindStringSubmatch(line); gm != nil {
			stack = append(stack, prefixEntry{prefix: gm[1], depth: depth})
		}
		depth += strings.Count(line, "{") - strings.Count(line, "}")
		for len(stack) > 0 && depth <= stack[len(stack)-1].depth {
			stack = stack[:len(stack)-1]
		}
		if rm := routeRe.FindStringSubmatch(line); rm != nil {
			expr := strings.TrimSpace(rm[1])
			parts := strings.Split(expr, "+")
			var resolved strings.Builder
			ok := true
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if len(part) >= 2 && part[0] == '"' && part[len(part)-1] == '"' {
					resolved.WriteString(part[1 : len(part)-1])
				} else if v, exists := consts[part]; exists {
					resolved.WriteString(v)
				} else {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			var full strings.Builder
			for _, s := range stack {
				full.WriteString(s.prefix)
			}
			full.WriteString(resolved.String())
			p := full.String()
			if len(p) > 1 {
				p = strings.TrimRight(p, "/")
			}
			compiled = append(compiled, toRegex(p))
		}
	}
	if len(compiled) == 0 {
		t.Fatalf("expected routes from main.go, found 0")
	}

	matchesRoute := func(path string) bool {
		clean := strings.Split(strings.Split(path, "?")[0], "#")[0]
		if clean == "" {
			return true
		}
		norm := regexp.MustCompile(`\{\{[^}]+\}\}|\$\{[^}]+\}`).ReplaceAllString(clean, "DUMMY")
		for _, rx := range compiled {
			if rx.MatchString(norm) {
				return true
			}
		}
		return false
	}

	attrRe := regexp.MustCompile(`(?:href|action|src)=["']([^"']+)["']`)
	codeBlockRe := regexp.MustCompile(`(?s)<pre\b[^>]*>.*?</pre>`)
	scriptBlockRe := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`)

	entries, err := fs.ReadDir(templates.FS, ".")
	if err != nil {
		t.Fatalf("ReadDir templates.FS: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".gohtml") {
			continue
		}
		raw, err := fs.ReadFile(templates.FS, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		// Strip code-example <pre> and <script> blocks so documentation examples and JS template strings aren't falsely flagged.
		cleaned := codeBlockRe.ReplaceAllString(string(raw), "")
		cleaned = scriptBlockRe.ReplaceAllString(cleaned, "")
		for _, m := range attrRe.FindAllStringSubmatch(cleaned, -1) {
			val := m[1]
			if strings.HasPrefix(val, "#") || strings.HasPrefix(val, "mailto:") ||
				strings.HasPrefix(val, "javascript:") || strings.HasPrefix(val, "data:") ||
				strings.HasPrefix(val, "tel:") || strings.HasPrefix(val, "{{") {
				continue
			}
			if strings.HasPrefix(val, "https://anshumanbiswas.com") {
				val = strings.TrimPrefix(val, "https://anshumanbiswas.com")
				if val == "" {
					val = "/"
				}
			} else if strings.HasPrefix(val, "http://") || strings.HasPrefix(val, "https://") || strings.HasPrefix(val, "//") {
				continue
			}
			if !strings.HasPrefix(val, "/") {
				continue
			}
			clean := strings.Split(strings.Split(val, "?")[0], "#")[0]
			if strings.HasPrefix(clean, "/static/") || strings.HasPrefix(clean, "/css/") {
				// Vendored craft builds are populated from crafts.lock during image build.
				if strings.HasPrefix(clean, "/static/vendor/deckcraft/") || strings.HasPrefix(clean, "/static/vendor/pdfcraft/") {
					continue
				}
				if strings.Contains(clean, "{{") || strings.Contains(clean, "${") {
					continue
				}
				diskPath := filepath.Join("..", strings.TrimPrefix(clean, "/"))
				if _, err := os.Stat(diskPath); err != nil {
					t.Errorf("%s references missing static asset %q", name, val)
				}
				continue
			}
			if !matchesRoute(val) {
				t.Errorf("%s contains dead internal link %q (no matching route in main.go)", name, val)
			}
		}
	}
}

// TestPaperMyNotesArePrivate ensures paper.MyNotes never leaks onto the public
// paper-page.gohtml view and that paper-editor.gohtml clearly marks My Notes as
// private / not published.
func TestPaperMyNotesArePrivate(t *testing.T) {
	sentinel := "TOP_SECRET_PRIVATE_PAPER_NOTE_98765"
	paper := &models.Paper{
		ID:          1,
		Title:       "Private Notes Test Paper",
		Slug:        "private-notes-test-paper",
		Content:     "Public summary",
		ContentHTML: template.HTML("<p>Public summary</p>"),
		MyNotes:     sentinel,
		MyNotesHTML: template.HTML("<p>" + sentinel + "</p>"),
		IsPublished: true,
	}

	req := httptest.NewRequest(http.MethodGet, "https://anshumanbiswas.com/papers/private-notes-test-paper", nil)

	// Public page must NOT render MyNotes or MyNotesHTML even if populated on the struct.
	pubTpl := views.Must(views.ParseFS(templates.FS, "paper-page.gohtml", "tailwind.gohtml"))
	pubRec := httptest.NewRecorder()
	pubTpl.Execute(pubRec, req, struct {
		LoggedIn          bool
		Username          string
		IsAdmin           bool
		SignupDisabled    bool
		UserID            int
		Description       string
		CurrentPage       string
		Paper             *models.Paper
		PublicAnnotations []models.PaperAnnotation
		FullURL           string
		UserPermissions   models.UserPermissions
	}{
		Paper:           paper,
		CurrentPage:     "papers",
		FullURL:         "https://anshumanbiswas.com/papers/private-notes-test-paper",
		UserPermissions: models.GetPermissions(models.RoleCommenter),
	})
	if strings.Contains(pubRec.Body.String(), sentinel) {
		t.Fatalf("paper-page.gohtml leaked private MyNotes into public HTML output")
	}
	if strings.Contains(pubRec.Body.String(), "My Notes") {
		t.Fatalf("paper-page.gohtml should not render a public My Notes section")
	}

	// Admin editor MUST render MyNotes and clearly label it as private / not published.
	edTpl := views.Must(views.ParseFS(templates.FS, "paper-editor.gohtml", "tailwind.gohtml"))
	edRec := httptest.NewRecorder()
	edTpl.Execute(edRec, req, struct {
		LoggedIn        bool
		Username        string
		IsAdmin         bool
		SignupDisabled  bool
		Description     string
		CurrentPage     string
		ResearchAreas   []models.PaperResearchArea
		SelectedAreaMap map[int]bool
		Paper           *models.Paper
		Mode            string
		EditorHTML      template.HTML
		Annotations     []models.PaperAnnotation
		UserPermissions models.UserPermissions
	}{
		LoggedIn:        true,
		Username:        "admin",
		IsAdmin:         true,
		CurrentPage:     "admin-papers",
		Paper:           paper,
		Mode:            "edit",
		EditorHTML:      template.HTML("<textarea></textarea>"),
		UserPermissions: models.GetPermissions(models.RoleAdministrator),
	})
	edHTML := edRec.Body.String()
	if !strings.Contains(edHTML, sentinel) {
		t.Fatalf("paper-editor.gohtml did not render Paper.MyNotes in the editor textarea")
	}
	if !strings.Contains(edHTML, "Not Published") {
		t.Fatalf("paper-editor.gohtml missing clear 'Not Published' indicator for My Notes")
	}
}
