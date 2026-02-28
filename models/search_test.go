package models

import (
	"context"
	"testing"
)

func TestSearchService_EmptyQuery(t *testing.T) {
	db := SetupTestDB(t)
	ss := &SearchService{DB: db}

	resp, err := ss.Search(context.Background(), "", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Query != "" {
		t.Errorf("expected empty query, got %q", resp.Query)
	}
	if len(resp.Posts) != 0 {
		t.Errorf("expected 0 posts, got %d", len(resp.Posts))
	}
	if len(resp.Slides) != 0 {
		t.Errorf("expected 0 slides, got %d", len(resp.Slides))
	}
}

func TestSearchService_SearchByTitle(t *testing.T) {
	db := SetupTestDB(t)
	ss := &SearchService{DB: db}

	SeedRoles(t, db)
	userID := SeedUser(t, db, "search-test@test.com", "searchtest", "hash123", 2)
	defer CleanupUser(t, db, userID)

	catID := SeedCategory(t, db, "SearchTestCat")
	defer CleanupCategory(t, db, catID)

	postID := SeedPost(t, db, userID, catID, "Xylophone Kubernetes Tutorial", "Learn about deploying with Kubernetes", "xylophone-k8s-test", true)
	defer CleanupPost(t, db, postID)

	resp, err := ss.Search(context.Background(), "Xylophone Kubernetes", 10)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}

	if len(resp.Posts) == 0 {
		t.Fatal("expected at least 1 post result")
	}

	if resp.Posts[0].Slug != "xylophone-k8s-test" {
		t.Errorf("expected slug 'xylophone-k8s-test', got %q", resp.Posts[0].Slug)
	}
}

func TestSearchService_DraftsNotReturned(t *testing.T) {
	db := SetupTestDB(t)
	ss := &SearchService{DB: db}

	SeedRoles(t, db)
	userID := SeedUser(t, db, "search-draft@test.com", "searchdraft", "hash123", 2)
	defer CleanupUser(t, db, userID)

	catID := SeedCategory(t, db, "DraftTestCat")
	defer CleanupCategory(t, db, catID)

	postID := SeedPost(t, db, userID, catID, "Zephyr Draft Unicorn Post", "This draft should not appear in search", "zephyr-draft-unicorn", false)
	defer CleanupPost(t, db, postID)

	resp, err := ss.Search(context.Background(), "Zephyr Draft Unicorn", 10)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}

	if len(resp.Posts) != 0 {
		t.Errorf("expected 0 posts for draft, got %d", len(resp.Posts))
	}
}

func TestSearchService_SearchByContent(t *testing.T) {
	db := SetupTestDB(t)
	ss := &SearchService{DB: db}

	SeedRoles(t, db)
	userID := SeedUser(t, db, "search-content@test.com", "searchcontent", "hash123", 2)
	defer CleanupUser(t, db, userID)

	catID := SeedCategory(t, db, "ContentTestCat")
	defer CleanupCategory(t, db, catID)

	postID := SeedPost(t, db, userID, catID, "Generic Title Here", "This post contains the unique word qwertyuiop for content search testing", "content-search-test", true)
	defer CleanupPost(t, db, postID)

	resp, err := ss.Search(context.Background(), "qwertyuiop", 10)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}

	if len(resp.Posts) == 0 {
		t.Fatal("expected at least 1 post when searching by content")
	}

	if resp.Posts[0].Slug != "content-search-test" {
		t.Errorf("expected slug 'content-search-test', got %q", resp.Posts[0].Slug)
	}
}

func TestSearchService_TitleRanksHigher(t *testing.T) {
	db := SetupTestDB(t)
	ss := &SearchService{DB: db}

	SeedRoles(t, db)
	userID := SeedUser(t, db, "search-rank@test.com", "searchrank", "hash123", 2)
	defer CleanupUser(t, db, userID)

	catID := SeedCategory(t, db, "RankTestCat")
	defer CleanupCategory(t, db, catID)

	// Post with "flamingo" in title
	postIDTitle := SeedPost(t, db, userID, catID, "Flamingo Migration Guide", "How to handle data migration effectively", "flamingo-title-rank", true)
	defer CleanupPost(t, db, postIDTitle)

	// Post with "flamingo" in content only
	postIDContent := SeedPost(t, db, userID, catID, "General Bird Facts", "The flamingo is a beautiful pink bird found in many regions", "flamingo-content-rank", true)
	defer CleanupPost(t, db, postIDContent)

	resp, err := ss.Search(context.Background(), "flamingo", 10)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}

	if len(resp.Posts) < 2 {
		t.Fatalf("expected at least 2 posts, got %d", len(resp.Posts))
	}

	// Title match should rank higher
	if resp.Posts[0].Slug != "flamingo-title-rank" {
		t.Errorf("expected title match 'flamingo-title-rank' to rank first, got %q", resp.Posts[0].Slug)
	}
}

func TestSearchResponse_Structure(t *testing.T) {
	resp := &SearchResponse{
		Query:      "test",
		TotalCount: 2,
		Posts:      []SearchResult{{Type: "post", Title: "Test Post", Slug: "test-post"}},
		Slides:     []SearchResult{{Type: "slide", Title: "Test Slide", Slug: "test-slide"}},
	}

	if resp.TotalCount != 2 {
		t.Errorf("expected TotalCount 2, got %d", resp.TotalCount)
	}
	if resp.Posts[0].Type != "post" {
		t.Errorf("expected type 'post', got %q", resp.Posts[0].Type)
	}
	if resp.Slides[0].Type != "slide" {
		t.Errorf("expected type 'slide', got %q", resp.Slides[0].Type)
	}
}
