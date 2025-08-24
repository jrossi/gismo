package version

// These variables are set at build time via ldflags
var (
	// BuildVersion is the semantic version
	BuildVersion = "dev"
	// BuildCommit is the git commit hash
	BuildCommit = "unknown"
	// BuildDate is the build date
	BuildDate = "unknown"
	// BuildBy is the build system
	BuildBy = "unknown"
)

// Info returns version information as a map
func Info() map[string]string {
	return map[string]string{
		"version": BuildVersion,
		"commit":  BuildCommit,
		"date":    BuildDate,
		"builtBy": BuildBy,
	}
}
