package version

import "runtime"

// These variables are populated at build time via -ldflags
var (
	// Version is the semantic version of the application (e.g., v1.0.0)
	Version = "dev"

	// GitCommit is the git commit hash of the build
	GitCommit = "unknown"

	// BuildTime is the UTC timestamp when the binary was built
	BuildTime = "unknown"

	// GoVersion is the Go version used to build the binary
	GoVersion = runtime.Version()
)

// Info returns all version information as a map
func Info() map[string]string {
	return map[string]string{
		"version":    Version,
		"git_commit": GitCommit,
		"build_time": BuildTime,
		"go_version": GoVersion,
	}
}
