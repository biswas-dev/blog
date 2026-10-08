package models

import (
	"strings"
	"testing"
)

const sampleGoodreadsRSS = `<?xml version="1.0"?>
<rss version="2.0"><channel><title>Anshuman's bookshelf: all</title>
<item>
  <title>The 48 Laws of Power</title>
  <book_id>1303</book_id>
  <author_name>Robert   Greene</author_name>
  <isbn>0140280197</isbn>
  <user_rating>4</user_rating>
  <user_review><![CDATA[This book is a <b>fascinating</b> read.<br /><br /><br />Laws &amp; history, <i>told well</i>.]]></user_review>
</item>
<item>
  <title>Open to Work: How to Get Ahead in the Age of AI</title>
  <book_id> 243013855 </book_id>
  <author_name>Ryan Roslansky</author_name>
  <isbn></isbn>
  <user_rating>0</user_rating>
  <user_review></user_review>
</item>
</channel></rss>`

func TestParseGoodreadsRSS(t *testing.T) {
	items, err := parseGoodreadsRSS(strings.NewReader(sampleGoodreadsRSS))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].BookID != "1303" || items[0].Rating != 4 || items[0].AuthorName != "Robert Greene" {
		t.Errorf("item 0 = %+v", items[0])
	}
	if items[1].BookID != "243013855" || items[1].Rating != 0 {
		t.Errorf("item 1 = %+v", items[1])
	}
	want := "This book is a **fascinating** read.\n\nLaws & history, *told well*."
	if got := items[0].Review(); got != want {
		t.Errorf("Review() = %q, want %q", got, want)
	}
}

func TestNormalizeISBN(t *testing.T) {
	tests := map[string]string{
		"0140280197":        "9780140280197",
		"013211917X":        "9780132119177",
		"978-0-14-028019-7": "9780140280197",
		"9780593715833":     "9780593715833",
		"B0H6ZR5W9W":        "",
		"":                  "",
	}
	for in, want := range tests {
		if got := normalizeISBN(in); got != want {
			t.Errorf("normalizeISBN(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchGoodreadsItem(t *testing.T) {
	items := []GoodreadsItem{
		{BookID: "1303", Title: "The 48 Laws of Power", AuthorName: "Robert Greene", ISBN: "0140280197"},
		{BookID: "243013855", Title: "Open to Work: How to Get Ahead in the Age of AI", AuthorName: "Ryan Roslansky"},
		{BookID: "52036", Title: "Siddhartha", AuthorName: "Hermann Hesse"},
	}
	tests := []struct {
		name  string
		book  Book
		known string
		want  string
	}{
		{"remembered id", Book{Title: "x"}, "52036", "52036"},
		{"goodreads link", Book{Title: "x", LinkURL: "https://www.goodreads.com/book/show/1303-the-48-laws"}, "", "1303"},
		{"isbn13 vs isbn10", Book{Title: "x", ISBN: "9780140280197"}, "", "1303"},
		{"title + author", Book{Title: "Open to Work", BookAuthor: "Ryan Roslansky"}, "", "243013855"},
		{"different edition title", Book{Title: "Siddhartha (A New Directions Paperback)", BookAuthor: "Hermann Hesse", LinkURL: "https://www.goodreads.com/book/show/18867923-siddhartha"}, "", "52036"},
		{"author mismatch", Book{Title: "Siddhartha", BookAuthor: "Someone Else"}, "", ""},
		{"no match", Book{Title: "Marble Hall Murders", BookAuthor: "Anthony Horowitz"}, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchGoodreadsItem(tt.book, tt.known, items)
			if (got == nil) != (tt.want == "") || (got != nil && got.BookID != tt.want) {
				t.Errorf("match = %+v, want %q", got, tt.want)
			}
		})
	}
}

func TestPlanGoodreadsReview(t *testing.T) {
	book := Book{Rating: 3, MyThoughts: "blog thoughts"}
	item := GoodreadsItem{Rating: 5, ReviewHTML: "gr review"}

	tests := []struct {
		name         string
		book         Book
		item         GoodreadsItem
		prev         *goodreadsState
		wantRating   float64
		wantThoughts string
	}{
		{"first sync pulls goodreads", book, item, nil, 5, "gr review"},
		{"unchanged on goodreads keeps blog edits", book, item, &goodreadsState{Rating: 5, Review: "gr review"}, 3, "blog thoughts"},
		{"goodreads rating changed", book, item, &goodreadsState{Rating: 4, Review: "gr review"}, 5, "blog thoughts"},
		{"goodreads review changed", book, item, &goodreadsState{Rating: 5, Review: "old"}, 3, "gr review"},
		{"empty goodreads never clears blog", book, GoodreadsItem{}, nil, 3, "blog thoughts"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, th := planGoodreadsReview(tt.book, tt.item, tt.prev)
			if r != tt.wantRating || th != tt.wantThoughts {
				t.Errorf("got (%v, %q), want (%v, %q)", r, th, tt.wantRating, tt.wantThoughts)
			}
		})
	}
}
