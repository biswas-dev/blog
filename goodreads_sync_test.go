package main

import (
	"testing"
	"time"
)

func TestNextGoodreadsSync(t *testing.T) {
	loc, err := time.LoadLocation("America/Toronto")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct{ now, want string }{
		{"2026-10-08T14:00:00-04:00", "2026-10-09T03:00:00-04:00"},
		{"2026-10-08T02:59:00-04:00", "2026-10-08T03:00:00-04:00"},
		{"2026-10-08T03:00:00-04:00", "2026-10-09T03:00:00-04:00"},
		{"2026-11-01T01:30:00-04:00", "2026-11-01T03:00:00-05:00"}, // DST ends
	}
	for _, tt := range tests {
		now, _ := time.Parse(time.RFC3339, tt.now)
		want, _ := time.Parse(time.RFC3339, tt.want)
		if got := nextGoodreadsSync(now, loc); !got.Equal(want) {
			t.Errorf("nextGoodreadsSync(%s) = %s, want %s", tt.now, got, want)
		}
	}
}
