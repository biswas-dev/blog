package models

import (
	"bufio"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// DatabaseBackupService handles database backup and restore operations
type DatabaseBackupService struct {
	db *sql.DB
}

// NewDatabaseBackupService creates a new DatabaseBackupService
func NewDatabaseBackupService(db *sql.DB) *DatabaseBackupService {
	return &DatabaseBackupService{
		db: db,
	}
}

// Export streams the database dump to the provided writer
func (s *DatabaseBackupService) Export(writer io.Writer) error {
	// Get database connection parameters from environment
	pgHost := os.Getenv("PG_HOST")
	pgPort := os.Getenv("PG_PORT")
	pgUser := os.Getenv("PG_USER")
	pgPassword := os.Getenv("PG_PASSWORD")
	pgDB := os.Getenv("PG_DB")

	// Set defaults if not provided
	if pgHost == "" {
		pgHost = "localhost"
	}
	if pgPort == "" {
		pgPort = "5432"
	}
	if pgUser == "" {
		pgUser = "blog"
	}
	if pgDB == "" {
		pgDB = "blog"
	}

	// Build pg_dump command
	cmd := exec.Command("pg_dump",
		"-h", pgHost,
		"-p", pgPort,
		"-U", pgUser,
		"-d", pgDB,
		"--no-password",
		"--clean",
		"--if-exists",
		"--create",
		"--inserts", // Use INSERT statements for better compatibility
	)

	// Set PGPASSWORD environment variable for authentication
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", pgPassword))

	// Stream output directly to writer
	cmd.Stdout = writer

	// Capture stderr for error reporting
	stderr := &strings.Builder{}
	cmd.Stderr = stderr

	// Execute the command
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("pg_dump failed: %w (stderr: %s)", err, stderr.String())
	}

	return nil
}

// Import restores the database from the provided reader
func (s *DatabaseBackupService) Import(reader io.Reader) error {
	// Get database connection parameters from environment
	pgHost := os.Getenv("PG_HOST")
	pgPort := os.Getenv("PG_PORT")
	pgUser := os.Getenv("PG_USER")
	pgPassword := os.Getenv("PG_PASSWORD")
	pgDB := os.Getenv("PG_DB")

	// Set defaults if not provided
	if pgHost == "" {
		pgHost = "localhost"
	}
	if pgPort == "" {
		pgPort = "5432"
	}
	if pgUser == "" {
		pgUser = "blog"
	}
	if pgDB == "" {
		pgDB = "blog"
	}

	// Build psql command
	cmd := exec.Command("psql",
		"-h", pgHost,
		"-p", pgPort,
		"-U", pgUser,
		"-d", pgDB,
		"--no-password",
		"-v", "ON_ERROR_STOP=1", // Stop on first error
	)

	// Set PGPASSWORD environment variable for authentication
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", pgPassword))

	// Stream input from reader
	cmd.Stdin = reader

	// Capture stdout and stderr for error reporting
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// Execute the command
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("psql failed: %w (stderr: %s)", err, stderr.String())
	}

	return nil
}

// ValidateImportFile performs basic validation on a SQL import file
func ValidateImportFile(reader io.Reader) error {
	scanner := bufio.NewScanner(reader)
	lineCount := 0
	hasSQL := false

	// Read first 100 lines to validate
	for scanner.Scan() && lineCount < 100 {
		lineCount++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}

		// Check for SQL keywords
		upperLine := strings.ToUpper(line)
		if strings.Contains(upperLine, "CREATE") ||
			strings.Contains(upperLine, "INSERT") ||
			strings.Contains(upperLine, "SELECT") ||
			strings.Contains(upperLine, "DROP") ||
			strings.Contains(upperLine, "ALTER") {
			hasSQL = true
		}

		// Check for potentially dangerous commands (basic safety check)
		if strings.Contains(upperLine, "DROP DATABASE") && !strings.Contains(upperLine, "--") {
			return errors.New("file contains DROP DATABASE command, which is not allowed")
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	if lineCount == 0 {
		return errors.New("file is empty")
	}

	if !hasSQL {
		return errors.New("file does not appear to contain valid SQL statements")
	}

	return nil
}
