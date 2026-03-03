package models

import "testing"

func TestContentTypeForPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/blog/my-post", "page"},
		{"/blog", "page"},
		{"/blog/", "page"},
		{"/slides/my-deck", "slide"},
		{"/slides", "slide"},
		{"/slides/", "slide"},
		{"/", "other"},
		{"/about", "other"},
		{"/admin", "other"},
		{"", "other"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := ContentTypeForPath(tt.path)
			if got != tt.want {
				t.Errorf("ContentTypeForPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestClassifyBrowser(t *testing.T) {
	tests := []struct {
		name string
		ua   string
		want string
	}{
		{"chrome", "Mozilla/5.0 Chrome/120.0.0.0 Safari/537.36", "Chrome"},
		{"firefox", "Mozilla/5.0 Firefox/121.0", "Firefox"},
		{"safari", "Mozilla/5.0 AppleWebKit/605.1.15 Safari/605.1.15", "Safari"},
		{"edge", "Mozilla/5.0 Chrome/120.0.0.0 Edg/120.0.0.0", "Edge"},
		{"googlebot", "Googlebot/2.1 (+http://www.google.com/bot.html)", "Bot"},
		{"crawler", "SomeCrawler/1.0", "Bot"},
		{"spider", "BaiduSpider/2.0", "Bot"},
		{"empty", "", "Unknown"},
		{"unknown agent", "SomeRandomAgent/1.0", "Other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyBrowser(tt.ua)
			if got != tt.want {
				t.Errorf("classifyBrowser(%q) = %q, want %q", tt.ua, got, tt.want)
			}
		})
	}
}

func TestParsePeriodDays(t *testing.T) {
	tests := []struct {
		period string
		want   int
	}{
		{"7d", 7},
		{"30d", 30},
		{"90d", 90},
		{"1y", 365},
		{"unknown", 30},
		{"", 30},
	}

	for _, tt := range tests {
		t.Run(tt.period, func(t *testing.T) {
			got := parsePeriodDays(tt.period)
			if got != tt.want {
				t.Errorf("parsePeriodDays(%q) = %d, want %d", tt.period, got, tt.want)
			}
		})
	}
}

func TestAnalyticsStructs(t *testing.T) {
	t.Run("PageView fields", func(t *testing.T) {
		userID := 42
		pv := PageView{
			IPAddress:   "1.2.3.4",
			Path:        "/blog/test",
			UserAgent:   "TestAgent/1.0",
			Referrer:    "https://google.com",
			UserID:      &userID,
			ContentType: "page",
		}
		if pv.IPAddress != "1.2.3.4" {
			t.Errorf("IPAddress = %q", pv.IPAddress)
		}
		if *pv.UserID != 42 {
			t.Errorf("UserID = %d", *pv.UserID)
		}
		if pv.ContentType != "page" {
			t.Errorf("ContentType = %q", pv.ContentType)
		}
	})

	t.Run("PageView nil UserID", func(t *testing.T) {
		pv := PageView{
			IPAddress:   "5.6.7.8",
			Path:        "/",
			ContentType: "other",
		}
		if pv.UserID != nil {
			t.Error("expected nil UserID for anonymous view")
		}
	})

	t.Run("AnalyticsSummary JSON fields", func(t *testing.T) {
		summary := AnalyticsSummary{
			TotalViews: 1000,
			UniqueIPs:  500,
			AvgDaily:   33.3,
		}
		if summary.TotalViews != 1000 {
			t.Errorf("TotalViews = %d", summary.TotalViews)
		}
		if summary.UniqueIPs != 500 {
			t.Errorf("UniqueIPs = %d", summary.UniqueIPs)
		}
	})

	t.Run("BrowserStat fields", func(t *testing.T) {
		stat := BrowserStat{Name: "Chrome", Count: 100, Percent: 45.5}
		if stat.Name != "Chrome" {
			t.Errorf("Name = %q", stat.Name)
		}
		if stat.Percent != 45.5 {
			t.Errorf("Percent = %f", stat.Percent)
		}
	})

	t.Run("LiveStats fields", func(t *testing.T) {
		ls := LiveStats{TodayViews: 50, TodayUniques: 30}
		if ls.TodayViews != 50 {
			t.Errorf("TodayViews = %d", ls.TodayViews)
		}
		if ls.TodayUniques != 30 {
			t.Errorf("TodayUniques = %d", ls.TodayUniques)
		}
	})

	t.Run("HourlyActivity fields", func(t *testing.T) {
		ha := HourlyActivity{Hour: 14, Views: 25}
		if ha.Hour != 14 {
			t.Errorf("Hour = %d", ha.Hour)
		}
	})

	t.Run("TopVisitor fields", func(t *testing.T) {
		tv := TopVisitor{
			IPAddress: "10.0.0.1",
			Views:     100,
			LastSeen:  "2026-03-01",
			TopPath:   "/blog/my-post",
			UserAgent: "Mozilla/5.0",
		}
		if tv.IPAddress != "10.0.0.1" {
			t.Errorf("IPAddress = %q", tv.IPAddress)
		}
		if tv.TopPath != "/blog/my-post" {
			t.Errorf("TopPath = %q", tv.TopPath)
		}
	})
}
