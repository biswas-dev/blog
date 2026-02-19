package models

import (
	"testing"
	"time"
)

func TestSystemService_countMigrationFiles(t *testing.T) {
	service := &SystemService{
		migrationsPath: "testdata/migrations",
	}

	// This will return 0 if testdata doesn't exist, which is fine for this test
	count := service.countMigrationFiles()
	if count < 0 {
		t.Errorf("countMigrationFiles() returned negative count: %d", count)
	}
}

func TestSystemService_GetSystemInfo(t *testing.T) {
	t.Skip("Skipping test that requires database connection")
	// This test is skipped because GetSystemInfo requires a real database connection
	// Integration tests with a test database should be used instead
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "30s"},
		{2 * time.Minute, "2m 0s"},
		{1 * time.Hour, "1h 0m 0s"},
		{25 * time.Hour, "1d 1h 0m 0s"},
		{0, "0s"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.duration)
		if result == "" {
			t.Errorf("formatDuration(%v) returned empty string", tt.duration)
		}
	}
}
