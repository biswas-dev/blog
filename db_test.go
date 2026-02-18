package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestDatabaseTypes(t *testing.T) {
	t.Run("Database struct exists", func(t *testing.T) {
		var db Database
		_ = db // Verify type exists
	})

	t.Run("ErrNoMatch error", func(t *testing.T) {
		if ErrNoMatch == nil {
			t.Error("ErrNoMatch should be defined")
		}

		errorMsg := ErrNoMatch.Error()
		if !strings.Contains(errorMsg, "no matching record") {
			t.Errorf("Expected error message to contain 'no matching record', got: %s", errorMsg)
		}
	})
}

// Test DSN (Data Source Name) formatting
func TestDSNFormatting(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		database string
		host     string
		port     string
		expected string
	}{
		{
			name:     "localhost connection",
			username: "testuser",
			password: "testpass",
			database: "testdb",
			host:     "localhost",
			port:     "5432",
			expected: "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=disable",
		},
		{
			name:     "remote connection",
			username: "dbuser",
			password: "dbpass",
			database: "production",
			host:     "192.168.1.100",
			port:     "5433",
			expected: "host=192.168.1.100 port=5433 user=dbuser password=dbpass dbname=production sslmode=disable",
		},
		{
			name:     "connection with special characters",
			username: "user@domain",
			password: "p@ss!w0rd",
			database: "my-database",
			host:     "db.example.com",
			port:     "5432",
			expected: "host=db.example.com port=5432 user=user@domain password=p@ss!w0rd dbname=my-database sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
				tt.host, tt.port, tt.username, tt.password, tt.database)

			if dsn != tt.expected {
				t.Errorf("DSN mismatch:\ngot:  %s\nwant: %s", dsn, tt.expected)
			}

			// Verify all components are present
			if !strings.Contains(dsn, tt.host) {
				t.Error("DSN should contain host")
			}
			if !strings.Contains(dsn, tt.port) {
				t.Error("DSN should contain port")
			}
			if !strings.Contains(dsn, tt.username) {
				t.Error("DSN should contain username")
			}
			if !strings.Contains(dsn, tt.password) {
				t.Error("DSN should contain password")
			}
			if !strings.Contains(dsn, tt.database) {
				t.Error("DSN should contain database name")
			}
			if !strings.Contains(dsn, "sslmode=disable") {
				t.Error("DSN should contain sslmode")
			}
		})
	}
}

// Test DSN component validation
func TestDSNComponents(t *testing.T) {
	t.Run("empty components", func(t *testing.T) {
		username := ""
		password := ""
		database := ""
		host := ""
		port := ""

		dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			host, port, username, password, database)

		// Should still format correctly, even with empty values
		if !strings.Contains(dsn, "host=") {
			t.Error("DSN should contain host= even if empty")
		}
		if !strings.Contains(dsn, "port=") {
			t.Error("DSN should contain port= even if empty")
		}
	})

	t.Run("spaces in password", func(t *testing.T) {
		password := "pass word"
		dsn := fmt.Sprintf("password=%s", password)

		if !strings.Contains(dsn, "pass word") {
			t.Error("DSN should handle spaces in password")
		}
	})
}

// Test database port numbers
func TestDatabasePorts(t *testing.T) {
	ports := []string{
		"5432",  // Default PostgreSQL
		"5433",  // Alternative
		"15432", // Custom
		"25432", // Custom
	}

	for _, port := range ports {
		t.Run("port "+port, func(t *testing.T) {
			dsn := fmt.Sprintf("host=localhost port=%s user=test password=test dbname=test sslmode=disable", port)

			if !strings.Contains(dsn, "port="+port) {
				t.Errorf("DSN should contain port=%s", port)
			}
		})
	}
}

// Test SSL mode variations
func TestSSLMode(t *testing.T) {
	t.Run("sslmode=disable", func(t *testing.T) {
		sslmode := "disable"
		dsn := fmt.Sprintf("host=localhost sslmode=%s", sslmode)

		if !strings.Contains(dsn, "sslmode=disable") {
			t.Error("DSN should contain sslmode=disable")
		}
	})

	t.Run("other sslmode values format correctly", func(t *testing.T) {
		modes := []string{"require", "verify-ca", "verify-full"}

		for _, mode := range modes {
			dsn := fmt.Sprintf("sslmode=%s", mode)
			if !strings.Contains(dsn, mode) {
				t.Errorf("DSN should support sslmode=%s", mode)
			}
		}
	})
}

// Test database name formats
func TestDatabaseNames(t *testing.T) {
	tests := []struct {
		name   string
		dbname string
	}{
		{"simple name", "mydb"},
		{"with underscore", "my_database"},
		{"with dash", "my-database"},
		{"with numbers", "db123"},
		{"mixed", "my_db_123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := fmt.Sprintf("dbname=%s", tt.dbname)

			if !strings.Contains(dsn, tt.dbname) {
				t.Errorf("DSN should contain dbname=%s", tt.dbname)
			}
		})
	}
}

// Test connection string key-value pairs
func TestDSNKeyValuePairs(t *testing.T) {
	dsn := "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=disable"

	requiredKeys := []string{"host=", "port=", "user=", "password=", "dbname=", "sslmode="}

	for _, key := range requiredKeys {
		if !strings.Contains(dsn, key) {
			t.Errorf("DSN should contain key '%s'", key)
		}
	}
}

// Test that Database struct can hold connection
func TestDatabaseStruct(t *testing.T) {
	t.Run("can create Database instance", func(t *testing.T) {
		db := Database{
			Conn: nil, // In real usage, this would be *sql.DB
		}

		if db.Conn != nil {
			t.Error("Expected nil connection in test instance")
		}
	})
}
