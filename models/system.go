package models

import (
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"anshumanbiswas.com/blog/version"
)

// SystemInfo contains all system information
type SystemInfo struct {
	Application ApplicationInfo `json:"application"`
	Database    DatabaseInfo    `json:"database"`
	Environment EnvironmentInfo `json:"environment"`
	Deployment  DeploymentInfo  `json:"deployment"`
}

// ApplicationInfo contains application-level information
type ApplicationInfo struct {
	Version    string `json:"version"`
	GitCommit  string `json:"git_commit"`
	BuildTime  string `json:"build_time"`
	GoVersion  string `json:"go_version"`
	Platform   string `json:"platform"`
	Executable string `json:"executable"`
}

// DatabaseInfo contains database-related information
type DatabaseInfo struct {
	Type               string `json:"type"`
	TotalMigrations    int    `json:"total_migrations"`
	AppliedMigrations  int    `json:"applied_migrations"`
	PendingMigrations  int    `json:"pending_migrations"`
	CurrentVersion     int    `json:"current_version"`
	Dirty              bool   `json:"dirty"`
	Connected          bool   `json:"connected"`
	ConnectionError    string `json:"connection_error,omitempty"`
	ServerTimeUTC      string `json:"server_time_utc"`
}

// EnvironmentInfo contains environment-level information
type EnvironmentInfo struct {
	Name       string        `json:"name"`
	ServerTime string        `json:"server_time"`
	Uptime     time.Duration `json:"uptime"`
	UptimeStr  string        `json:"uptime_str"`
}

// DeploymentInfo contains deployment-related information
type DeploymentInfo struct {
	Method         string `json:"method"`
	LastDeployTime string `json:"last_deploy_time"`
	Branch         string `json:"branch"`
}

// SystemService provides system information
type SystemService struct {
	db             *sql.DB
	migrationsPath string
	startTime      time.Time
}

// NewSystemService creates a new SystemService
func NewSystemService(db *sql.DB, migrationsPath string, startTime time.Time) *SystemService {
	return &SystemService{
		db:             db,
		migrationsPath: migrationsPath,
		startTime:      startTime,
	}
}

// GetSystemInfo returns comprehensive system information
func (s *SystemService) GetSystemInfo() (*SystemInfo, error) {
	// Get application info
	appInfo := s.getApplicationInfo()

	// Get database info
	dbInfo := s.getDatabaseInfo()

	// Get environment info
	envInfo := s.getEnvironmentInfo()

	// Get deployment info
	deployInfo := s.getDeploymentInfo()

	return &SystemInfo{
		Application: appInfo,
		Database:    dbInfo,
		Environment: envInfo,
		Deployment:  deployInfo,
	}, nil
}

func (s *SystemService) getApplicationInfo() ApplicationInfo {
	executable, _ := os.Executable()

	return ApplicationInfo{
		Version:    version.Version,
		GitCommit:  version.GitCommit,
		BuildTime:  version.BuildTime,
		GoVersion:  version.GoVersion,
		Platform:   fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		Executable: executable,
	}
}

func (s *SystemService) getDatabaseInfo() DatabaseInfo {
	info := DatabaseInfo{
		Type:      "PostgreSQL",
		Connected: false,
	}

	// Test database connection
	err := s.db.Ping()
	if err != nil {
		info.ConnectionError = err.Error()
		return info
	}
	info.Connected = true

	// Get database server time
	var serverTime time.Time
	err = s.db.QueryRow("SELECT NOW()").Scan(&serverTime)
	if err == nil {
		info.ServerTimeUTC = serverTime.UTC().Format(time.RFC3339)
	}

	// Count total migrations (*.up.sql files)
	info.TotalMigrations = s.countMigrationFiles()

	// Get current migration version and dirty state from schema_migrations
	info.CurrentVersion, info.Dirty = s.getMigrationState()
	info.AppliedMigrations = info.CurrentVersion

	// Calculate pending migrations
	info.PendingMigrations = info.TotalMigrations - info.AppliedMigrations
	if info.PendingMigrations < 0 {
		info.PendingMigrations = 0
	}

	return info
}

func (s *SystemService) getEnvironmentInfo() EnvironmentInfo {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	uptime := time.Since(s.startTime)

	return EnvironmentInfo{
		Name:       env,
		ServerTime: time.Now().Format(time.RFC3339),
		Uptime:     uptime,
		UptimeStr:  formatDuration(uptime),
	}
}

func (s *SystemService) getDeploymentInfo() DeploymentInfo {
	// Try to determine deployment method and last deploy time
	method := "Unknown"
	lastDeployTime := ""
	branch := ""

	// Check if running in Docker
	if _, err := os.Stat("/.dockerenv"); err == nil {
		method = "Docker"
	}

	// Use build time as deployment time
	if version.BuildTime != "unknown" {
		lastDeployTime = version.BuildTime
	}

	// Try to extract branch from git commit (if available in git metadata)
	if version.GitCommit != "unknown" {
		// In a real deployment, this could read from a deployment metadata file
		branch = os.Getenv("DEPLOY_BRANCH")
		if branch == "" {
			branch = "unknown"
		}
	}

	return DeploymentInfo{
		Method:         method,
		LastDeployTime: lastDeployTime,
		Branch:         branch,
	}
}

// countMigrationFiles counts .up.sql files in the migrations directory
func (s *SystemService) countMigrationFiles() int {
	count := 0

	if s.migrationsPath == "" {
		return count
	}

	err := filepath.WalkDir(s.migrationsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".up.sql") {
			count++
		}
		return nil
	})

	if err != nil {
		return 0
	}

	return count
}

// getMigrationState reads the current version and dirty flag from schema_migrations.
// golang-migrate stores exactly one row: (version bigint, dirty boolean).
func (s *SystemService) getMigrationState() (version int, dirty bool) {
	err := s.db.QueryRow("SELECT version, dirty FROM schema_migrations LIMIT 1").Scan(&version, &dirty)
	if err != nil {
		// Table might not exist yet
		return 0, false
	}
	return version, dirty
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	parts := []string{}
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if seconds > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}

	return strings.Join(parts, " ")
}
