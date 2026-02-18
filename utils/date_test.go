package utils

import (
	"strings"
	"testing"
	"time"
)

func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		dateStr  string
		want     string
		contains string
	}{
		{
			name:     "empty string",
			dateStr:  "",
			want:     "",
		},
		{
			name:     "just now",
			dateStr:  now.Add(-5 * time.Second).Format(time.RFC3339),
			contains: "seconds ago",
		},
		{
			name:     "one second ago",
			dateStr:  now.Add(-1 * time.Second).Format(time.RFC3339),
			want:     "just now",
		},
		{
			name:     "minutes ago",
			dateStr:  now.Add(-5 * time.Minute).Format(time.RFC3339),
			contains: "minutes ago",
		},
		{
			name:     "one minute ago",
			dateStr:  now.Add(-1 * time.Minute).Format(time.RFC3339),
			want:     "1 minute ago",
		},
		{
			name:     "hours ago",
			dateStr:  now.Add(-3 * time.Hour).Format(time.RFC3339),
			contains: "hours ago",
		},
		{
			name:     "one hour ago",
			dateStr:  now.Add(-1 * time.Hour).Format(time.RFC3339),
			want:     "1 hour ago",
		},
		{
			name:     "days ago",
			dateStr:  now.Add(-3 * 24 * time.Hour).Format(time.RFC3339),
			contains: "days ago",
		},
		{
			name:     "one day ago",
			dateStr:  now.Add(-24 * time.Hour).Format(time.RFC3339),
			want:     "1 day ago",
		},
		{
			name:     "weeks ago",
			dateStr:  now.Add(-14 * 24 * time.Hour).Format(time.RFC3339),
			contains: "weeks ago",
		},
		{
			name:     "one week ago",
			dateStr:  now.Add(-7 * 24 * time.Hour).Format(time.RFC3339),
			want:     "1 week ago",
		},
		{
			name:     "months ago",
			dateStr:  now.Add(-60 * 24 * time.Hour).Format(time.RFC3339),
			contains: "months ago",
		},
		{
			name:     "one month ago",
			dateStr:  now.Add(-30 * 24 * time.Hour).Format(time.RFC3339),
			want:     "1 month ago",
		},
		{
			name:     "years ago",
			dateStr:  now.Add(-400 * 24 * time.Hour).Format(time.RFC3339),
			contains: "year",
		},
		{
			name:     "future date",
			dateStr:  now.Add(24 * time.Hour).Format(time.RFC3339),
			want:     "in the future",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatRelativeTime(tt.dateStr)

			if tt.want != "" && got != tt.want {
				t.Errorf("FormatRelativeTime() = %v, want %v", got, tt.want)
			}

			if tt.contains != "" && !strings.Contains(got, tt.contains) {
				t.Errorf("FormatRelativeTime() = %v, should contain %v", got, tt.contains)
			}
		})
	}
}

func TestFormatRelativeTime_DifferentFormats(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		dateStr string
		wantErr bool
	}{
		{
			name:    "RFC3339 format",
			dateStr: now.Add(-1 * time.Hour).Format(time.RFC3339),
			wantErr: false,
		},
		{
			name:    "RFC3339Nano format",
			dateStr: now.Add(-1 * time.Hour).Format(time.RFC3339Nano),
			wantErr: false,
		},
		{
			name:    "custom timezone format",
			dateStr: now.Add(-1 * time.Hour).Format("2006-01-02T15:04:05.999999Z07:00"),
			wantErr: false,
		},
		{
			name:    "invalid format returns original",
			dateStr: "invalid-date",
			wantErr: false, // Should return original string, not error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatRelativeTime(tt.dateStr)
			if got == "" && !tt.wantErr {
				t.Errorf("FormatRelativeTime() returned empty string for %v", tt.dateStr)
			}
		})
	}
}

func TestRelativeTimeFromTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		time     time.Time
		want     string
		contains string
	}{
		{
			name:     "just now",
			time:     now.Add(-1 * time.Second),
			want:     "just now",
		},
		{
			name:     "5 seconds ago",
			time:     now.Add(-5 * time.Second),
			contains: "seconds ago",
		},
		{
			name:     "1 minute ago",
			time:     now.Add(-1 * time.Minute),
			want:     "1 minute ago",
		},
		{
			name:     "5 minutes ago",
			time:     now.Add(-5 * time.Minute),
			contains: "minutes ago",
		},
		{
			name:     "1 hour ago",
			time:     now.Add(-1 * time.Hour),
			want:     "1 hour ago",
		},
		{
			name:     "1 day ago",
			time:     now.Add(-24 * time.Hour),
			want:     "1 day ago",
		},
		{
			name:     "future time",
			time:     now.Add(24 * time.Hour),
			want:     "in the future",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RelativeTimeFromTime(tt.time)

			if tt.want != "" && got != tt.want {
				t.Errorf("RelativeTimeFromTime() = %v, want %v", got, tt.want)
			}

			if tt.contains != "" && !strings.Contains(got, tt.contains) {
				t.Errorf("RelativeTimeFromTime() = %v, should contain %v", got, tt.contains)
			}
		})
	}
}

func TestFormatFriendlyDate(t *testing.T) {
	tests := []struct {
		name     string
		dateStr  string
		want     string
		contains string
	}{
		{
			name:    "empty string",
			dateStr: "",
			want:    "",
		},
		{
			name:     "RFC3339 format",
			dateStr:  "2024-01-15T10:30:00Z",
			contains: "January",
		},
		{
			name:     "RFC3339Nano format",
			dateStr:  "2024-06-20T14:45:30.123456Z",
			contains: "June",
		},
		{
			name:     "custom timezone format",
			dateStr:  "2024-12-25T00:00:00.000000+00:00",
			contains: "December",
		},
		{
			name:    "invalid format returns original",
			dateStr: "invalid-date-string",
			want:    "invalid-date-string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatFriendlyDate(tt.dateStr)

			if tt.want != "" && got != tt.want {
				t.Errorf("FormatFriendlyDate() = %v, want %v", got, tt.want)
			}

			if tt.contains != "" && !strings.Contains(got, tt.contains) {
				t.Errorf("FormatFriendlyDate() = %v, should contain %v", got, tt.contains)
			}
		})
	}
}

func TestFormatFriendlyDate_Format(t *testing.T) {
	// Test that the format is exactly "January 2, 2006"
	dateStr := "2024-03-15T12:00:00Z"
	got := FormatFriendlyDate(dateStr)

	// Should contain month name
	if !strings.Contains(got, "March") {
		t.Errorf("Expected month name 'March', got %v", got)
	}

	// Should contain day
	if !strings.Contains(got, "15") {
		t.Errorf("Expected day '15', got %v", got)
	}

	// Should contain year
	if !strings.Contains(got, "2024") {
		t.Errorf("Expected year '2024', got %v", got)
	}

	// Should have comma after day
	if !strings.Contains(got, ",") {
		t.Errorf("Expected comma in date format, got %v", got)
	}
}

func TestCalculateReadingTime(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{
			name:    "empty content",
			content: "",
			want:    1, // Minimum 1 minute
		},
		{
			name:    "very short content",
			content: "Hello world",
			want:    1, // Less than 200 words, rounds up to 1
		},
		{
			name:    "exactly 200 words",
			content: strings.Repeat("word ", 200),
			want:    1,
		},
		{
			name:    "201 words",
			content: strings.Repeat("word ", 201),
			want:    2, // Should round up
		},
		{
			name:    "400 words",
			content: strings.Repeat("word ", 400),
			want:    2,
		},
		{
			name:    "600 words",
			content: strings.Repeat("word ", 600),
			want:    3,
		},
		{
			name:    "1000 words",
			content: strings.Repeat("word ", 1000),
			want:    5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateReadingTime(tt.content)
			if got != tt.want {
				t.Errorf("CalculateReadingTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateReadingTime_WithPunctuation(t *testing.T) {
	// Content with punctuation should still count words correctly
	content := "This is a test. It has punctuation, doesn't it? Yes!"
	got := CalculateReadingTime(content)

	// Should count as at least 1 minute (it's 11 words)
	if got < 1 {
		t.Errorf("Expected at least 1 minute, got %d", got)
	}
}

func TestCalculateReadingTime_WithNewlines(t *testing.T) {
	// Content with newlines should still count words correctly
	content := `This is the first line.

	This is the second line.

	This is the third line.`

	got := CalculateReadingTime(content)

	// Should count all words regardless of newlines
	if got < 1 {
		t.Errorf("Expected at least 1 minute, got %d", got)
	}
}

func TestCalculateReadingTime_LongArticle(t *testing.T) {
	// Simulate a 2000-word article
	words := make([]string, 2000)
	for i := range words {
		words[i] = "word"
	}
	content := strings.Join(words, " ")

	got := CalculateReadingTime(content)

	// 2000 words / 200 words per minute = 10 minutes
	if got != 10 {
		t.Errorf("Expected 10 minutes for 2000 words, got %d", got)
	}
}

func TestRelativeTimeFromTime_EdgeCases(t *testing.T) {
	now := time.Now()

	t.Run("exactly zero duration", func(t *testing.T) {
		got := RelativeTimeFromTime(now)
		if got != "just now" {
			t.Errorf("Expected 'just now' for zero duration, got %v", got)
		}
	})

	t.Run("very far future", func(t *testing.T) {
		future := now.Add(365 * 24 * time.Hour)
		got := RelativeTimeFromTime(future)
		if got != "in the future" {
			t.Errorf("Expected 'in the future', got %v", got)
		}
	})

	t.Run("very far past", func(t *testing.T) {
		past := now.Add(-10 * 365 * 24 * time.Hour)
		got := RelativeTimeFromTime(past)
		if !strings.Contains(got, "years ago") {
			t.Errorf("Expected 'years ago', got %v", got)
		}
	})
}
