package utils

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// StripHTML
// ---------------------------------------------------------------------------

func BenchmarkStripHTML_Short(b *testing.B) {
	input := "<p>Hello <strong>world</strong></p>"
	for i := 0; i < b.N; i++ {
		StripHTML(input)
	}
}

func BenchmarkStripHTML_Medium(b *testing.B) {
	input := strings.Repeat(`<div class="post"><h2>Title</h2><p>Paragraph with <a href="#">link</a> and <em>emphasis</em>.</p></div>`, 10)
	for i := 0; i < b.N; i++ {
		StripHTML(input)
	}
}

func BenchmarkStripHTML_Long(b *testing.B) {
	input := strings.Repeat(`<div class="post"><h2>Title</h2><p>Paragraph with <a href="#">link</a> and <em>emphasis</em>.</p></div>`, 100)
	for i := 0; i < b.N; i++ {
		StripHTML(input)
	}
}

func BenchmarkStripHTML_NoTags(b *testing.B) {
	input := strings.Repeat("This is plain text without any HTML tags. ", 50)
	for i := 0; i < b.N; i++ {
		StripHTML(input)
	}
}

// ---------------------------------------------------------------------------
// FormatFriendlyDate
// ---------------------------------------------------------------------------

func BenchmarkFormatFriendlyDate_RFC3339(b *testing.B) {
	dateStr := time.Now().Format(time.RFC3339)
	for i := 0; i < b.N; i++ {
		FormatFriendlyDate(dateStr)
	}
}

func BenchmarkFormatFriendlyDate_AlreadyFormatted(b *testing.B) {
	// Non-RFC3339 string just returns as-is
	dateStr := "January 2, 2006"
	for i := 0; i < b.N; i++ {
		FormatFriendlyDate(dateStr)
	}
}

func BenchmarkFormatFriendlyDate_Empty(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FormatFriendlyDate("")
	}
}

// ---------------------------------------------------------------------------
// FormatRelativeTime
// ---------------------------------------------------------------------------

func BenchmarkFormatRelativeTime_Recent(b *testing.B) {
	dateStr := time.Now().Add(-5 * time.Minute).Format(time.RFC3339)
	for i := 0; i < b.N; i++ {
		FormatRelativeTime(dateStr)
	}
}

func BenchmarkFormatRelativeTime_Old(b *testing.B) {
	dateStr := time.Now().Add(-365 * 24 * time.Hour).Format(time.RFC3339)
	for i := 0; i < b.N; i++ {
		FormatRelativeTime(dateStr)
	}
}

// ---------------------------------------------------------------------------
// CalculateReadingTime
// ---------------------------------------------------------------------------

func BenchmarkCalculateReadingTime_Short(b *testing.B) {
	content := "Short article content."
	for i := 0; i < b.N; i++ {
		CalculateReadingTime(content)
	}
}

func BenchmarkCalculateReadingTime_Long(b *testing.B) {
	content := strings.Repeat("word ", 2000)
	for i := 0; i < b.N; i++ {
		CalculateReadingTime(content)
	}
}
