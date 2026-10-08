package models

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// GoodreadsItem is one book on a Goodreads user's shelves, from the public
// review-list RSS feed (the Goodreads developer API no longer exists).
type GoodreadsItem struct {
	BookID     string  `xml:"book_id"`
	Title      string  `xml:"title"`
	AuthorName string  `xml:"author_name"`
	ISBN       string  `xml:"isbn"`
	Rating     float64 `xml:"user_rating"`
	ReviewHTML string  `xml:"user_review"`
}

// Review returns the user's review as markdown-ish plain text.
func (g GoodreadsItem) Review() string { return goodreadsReviewText(g.ReviewHTML) }

const goodreadsMaxPages = 50

// FetchGoodreadsShelf returns every book on all of a Goodreads user's shelves.
func FetchGoodreadsShelf(ctx context.Context, client *http.Client, userID string) ([]GoodreadsItem, error) {
	var all []GoodreadsItem
	for page := 1; page <= goodreadsMaxPages; page++ {
		u := fmt.Sprintf("https://www.goodreads.com/review/list_rss/%s?shelf=%s&page=%d",
			url.PathEscape(userID), url.QueryEscape("#ALL#"), page)
		items, err := fetchGoodreadsPage(ctx, client, u)
		if err != nil {
			return nil, fmt.Errorf("goodreads page %d: %w", page, err)
		}
		if len(items) == 0 {
			break
		}
		all = append(all, items...)
	}
	return all, nil
}

func fetchGoodreadsPage(ctx context.Context, client *http.Client, u string) ([]GoodreadsItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; anshumanbiswas.com book sync)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return parseGoodreadsRSS(resp.Body)
}

func parseGoodreadsRSS(r io.Reader) ([]GoodreadsItem, error) {
	var feed struct {
		Items []GoodreadsItem `xml:"channel>item"`
	}
	if err := xml.NewDecoder(r).Decode(&feed); err != nil {
		return nil, fmt.Errorf("parse rss: %w", err)
	}
	for i := range feed.Items {
		it := &feed.Items[i]
		it.BookID = strings.TrimSpace(it.BookID)
		it.ISBN = strings.TrimSpace(it.ISBN)
		it.Title = strings.TrimSpace(it.Title)
		it.AuthorName = strings.Join(strings.Fields(it.AuthorName), " ")
	}
	return feed.Items, nil
}

var (
	reviewBreakRe = regexp.MustCompile(`(?i)<br\s*/?>`)
	reviewBoldRe  = regexp.MustCompile(`(?i)</?(b|strong)>`)
	reviewItalRe  = regexp.MustCompile(`(?i)</?(i|em)>`)
	reviewTagRe   = regexp.MustCompile(`<[^>]+>`)
	blankLinesRe  = regexp.MustCompile(`\n{3,}`)
)

// goodreadsReviewText converts Goodreads review HTML to markdown-ish text.
func goodreadsReviewText(s string) string {
	s = reviewBreakRe.ReplaceAllString(s, "\n")
	s = reviewBoldRe.ReplaceAllString(s, "**")
	s = reviewItalRe.ReplaceAllString(s, "*")
	s = reviewTagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}
	s = blankLinesRe.ReplaceAllString(strings.Join(lines, "\n"), "\n\n")
	return strings.TrimSpace(s)
}

var goodreadsLinkRe = regexp.MustCompile(`goodreads\.com/book/show/(\d+)`)

func goodreadsIDFromURL(link string) string {
	if m := goodreadsLinkRe.FindStringSubmatch(link); m != nil {
		return m[1]
	}
	return ""
}

// normalizeISBN returns the ISBN-13 form of an ISBN-10 or ISBN-13, or "".
func normalizeISBN(s string) string {
	s = strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(s)))
	switch {
	case len(s) == 13 && isDigits(s):
		return s
	case len(s) == 10 && isDigits(s[:9]) && (isDigits(s[9:]) || s[9] == 'X'):
		body := "978" + s[:9]
		sum := 0
		for i, c := range body {
			d := int(c - '0')
			if i%2 == 1 {
				d *= 3
			}
			sum += d
		}
		return body + strconv.Itoa((10-sum%10)%10)
	}
	return ""
}

func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}

var nonAlnumRe = regexp.MustCompile(`[^a-z0-9]+`)

// titleKey reduces a title to its main part ("Siddhartha (Penguin Classics)",
// "The Diary of a CEO: The 33 Laws..." -> "siddhartha", "thediaryofaceo").
func titleKey(t string) string {
	t = strings.ToLower(t)
	if i := strings.IndexAny(t, ":("); i > 0 {
		t = t[:i]
	}
	return nonAlnumRe.ReplaceAllString(t, "")
}

func authorKey(a string) string {
	f := strings.Fields(strings.ToLower(a))
	if len(f) == 0 {
		return ""
	}
	return nonAlnumRe.ReplaceAllString(f[len(f)-1], "")
}

// matchGoodreadsItem finds the Goodreads entry for a blog book: by remembered
// or linked Goodreads book ID, then ISBN, then a unique title + author surname.
func matchGoodreadsItem(book Book, knownGoodreadsID string, items []GoodreadsItem) *GoodreadsItem {
	for _, id := range []string{knownGoodreadsID, goodreadsIDFromURL(book.LinkURL)} {
		if id == "" {
			continue
		}
		for i := range items {
			if items[i].BookID == id {
				return &items[i]
			}
		}
	}

	isbns := map[string]bool{}
	for _, s := range []string{book.ISBN, book.AmazonASIN} {
		if n := normalizeISBN(s); n != "" {
			isbns[n] = true
		}
	}
	for i := range items {
		if n := normalizeISBN(items[i].ISBN); n != "" && isbns[n] {
			return &items[i]
		}
	}

	tk, ak := titleKey(book.Title), authorKey(book.BookAuthor)
	if tk == "" || ak == "" {
		return nil
	}
	var found *GoodreadsItem
	for i := range items {
		if titleKey(items[i].Title) == tk && authorKey(items[i].AuthorName) == ak {
			if found != nil {
				return nil // ambiguous
			}
			found = &items[i]
		}
	}
	return found
}

type goodreadsState struct {
	GoodreadsBookID string
	Rating          float64
	Review          string
}

// planGoodreadsReview decides the blog's rating/thoughts after a sync. A
// Goodreads value is applied only when it is set and has changed since the
// last sync (or was never synced), so blog edits and reverts are not undone.
func planGoodreadsReview(book Book, item GoodreadsItem, prev *goodreadsState) (rating float64, thoughts string) {
	rating, thoughts = book.Rating, book.MyThoughts
	review := item.Review()
	if item.Rating > 0 && (prev == nil || !sameRating(prev.Rating, item.Rating)) {
		rating = item.Rating
	}
	if review != "" && (prev == nil || prev.Review != review) {
		thoughts = review
	}
	return rating, thoughts
}

// GoodreadsSyncResult summarises one sync run.
type GoodreadsSyncResult struct {
	GoodreadsBooks int      `json:"goodreads_books"`
	Matched        int      `json:"matched"`
	Updated        []string `json:"updated"`
}

// GoodreadsSyncService pulls star ratings and reviews from a Goodreads
// user's shelves into matching books.
type GoodreadsSyncService struct {
	DB          *sql.DB
	BookService *BookService
	Client      *http.Client
}

// Sync fetches the Goodreads shelves for userID and updates matching books.
func (s *GoodreadsSyncService) Sync(ctx context.Context, userID string) (*GoodreadsSyncResult, error) {
	if userID == "" {
		return nil, fmt.Errorf("goodreads user id not configured")
	}
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	items, err := FetchGoodreadsShelf(ctx, client, userID)
	if err != nil {
		return nil, err
	}
	books, err := s.BookService.GetAllBooks()
	if err != nil {
		return nil, err
	}

	res := &GoodreadsSyncResult{GoodreadsBooks: len(items), Updated: []string{}}
	for _, book := range books.Books {
		prev, err := s.loadState(book.ID)
		if err != nil {
			return res, err
		}
		known := ""
		if prev != nil {
			known = prev.GoodreadsBookID
		}
		item := matchGoodreadsItem(book, known, items)
		if item == nil {
			continue
		}
		res.Matched++

		rating, thoughts := planGoodreadsReview(book, *item, prev)
		if !sameRating(rating, book.Rating) || thoughts != book.MyThoughts {
			if err := s.BookService.SetReview(book.ID, rating, thoughts, ReviewChangeGoodreads); err != nil {
				return res, fmt.Errorf("book %d: %w", book.ID, err)
			}
			res.Updated = append(res.Updated, book.Title)
		}
		if err := s.saveState(book.ID, goodreadsState{item.BookID, item.Rating, item.Review()}); err != nil {
			return res, err
		}
	}
	return res, nil
}

func (s *GoodreadsSyncService) loadState(bookID int) (*goodreadsState, error) {
	var st goodreadsState
	err := s.DB.QueryRow(`SELECT goodreads_book_id, rating, review FROM goodreads_sync_state WHERE book_id = $1`, bookID).
		Scan(&st.GoodreadsBookID, &st.Rating, &st.Review)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load goodreads state: %w", err)
	}
	return &st, nil
}

func (s *GoodreadsSyncService) saveState(bookID int, st goodreadsState) error {
	_, err := s.DB.Exec(`
		INSERT INTO goodreads_sync_state (book_id, goodreads_book_id, rating, review, synced_at)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
		ON CONFLICT (book_id) DO UPDATE SET goodreads_book_id = $2, rating = $3, review = $4, synced_at = CURRENT_TIMESTAMP`,
		bookID, st.GoodreadsBookID, st.Rating, st.Review)
	if err != nil {
		return fmt.Errorf("save goodreads state: %w", err)
	}
	return nil
}
