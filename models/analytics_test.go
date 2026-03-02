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
