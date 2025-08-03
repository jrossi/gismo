//go:build mage
// +build mage

package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/magefile/mage/target"
)

const (
	buildDir    = "build"
	binDir      = "build/bin"
	coverageDir = "build/coverage"
	distDir     = "build/dist"
)

var (
	// Build information
	goFlags = []string{"-trimpath"}

	// Binaries to build
	binaries = map[string]string{
		"gismo":          "./cmd/gismo",
		"gismo-init":     "./cmd/gismo-init",
		"gismo-show":     "./cmd/gismo-show",
		"gismo-registry": "./cmd/gismo-registry",
		"gismo-package":  "./cmd/gismo-package",
		"gismo-server":   "./cmd/gismo-server",
	}
)

// getBuildFlags returns the build flags with version information
func getBuildFlags() ([]string, error) {
	version, err := sh.Output("git", "describe", "--tags", "--always", "--dirty")
	if err != nil {
		version = "dev"
	}

	commit, err := sh.Output("git", "rev-parse", "--short", "HEAD")
	if err != nil {
		commit = "none"
	}

	date := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	ldflags := fmt.Sprintf("-s -w -X main.version=%s -X main.commit=%s -X main.date=%s -X main.builtBy=mage",
		version, commit, date)

	return []string{"-ldflags", ldflags}, nil
}

// cleanupServers kills any running gismo-server processes and removes stale lock files
func cleanupServers() error {
	// Kill any running gismo-server processes
	_ = sh.Run("pkill", "-f", "gismo-server")

	// Get runtime directory and clean up lock files
	runtimeDir := getRuntimeDir()
	if runtimeDir != "" {
		lockFile := filepath.Join(runtimeDir, "gismo.lock")
		socketFile := filepath.Join(runtimeDir, "gismo.sock")

		// Remove lock and socket files if they exist
		_ = os.Remove(lockFile)
		_ = os.Remove(socketFile)
	}

	return nil
}

// getRuntimeDir returns the gismo runtime directory
func getRuntimeDir() string {
	// Try XDG_RUNTIME_DIR first
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "gismo")
	}

	// Fall back to temp directory with UID
	var uid string
	if u, err := user.Current(); err == nil {
		uid = u.Uid
	} else {
		uid = "unknown"
	}

	return filepath.Join(os.TempDir(), fmt.Sprintf("gismo-%s", uid))
}

// ensureDirs creates necessary build directories
func ensureDirs() error {
	dirs := []string{buildDir, binDir, coverageDir, distDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// All runs format, lint, test, and build
func All() error {
	mg.Deps(Fmt, Lint, Test, Build)
	return nil
}

// getSourceFiles returns all Go source files and go.mod/go.sum for dependency tracking
func getSourceFiles() ([]string, error) {
	var sources []string

	// Add go.mod and go.sum as dependencies
	sources = append(sources, "go.mod", "go.sum")

	// Find all .go files
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip build directory and vendor
		if strings.Contains(path, "build/") || strings.Contains(path, "vendor/") {
			return nil
		}

		if strings.HasSuffix(path, ".go") && !strings.Contains(path, "_test.go") {
			sources = append(sources, path)
		}

		return nil
	})

	return sources, err
}

// buildBinary builds a single binary if needed
func buildBinary(name, cmdPath string) error {
	mg.Deps(ensureDirs)

	outputPath := filepath.Join(binDir, name)

	// Get source files for dependency tracking
	sources, err := getSourceFiles()
	if err != nil {
		return fmt.Errorf("failed to get source files: %w", err)
	}

	// Check if rebuild is needed
	rebuild, err := target.Path(outputPath, sources...)
	if err != nil {
		return fmt.Errorf("failed to check target dependencies: %w", err)
	}

	if !rebuild {
		fmt.Printf("✅ %s is up to date\n", name)
		return nil
	}

	fmt.Printf("Building %s...\n", name)

	buildFlags, err := getBuildFlags()
	if err != nil {
		return err
	}

	args := append([]string{"build"}, goFlags...)
	args = append(args, buildFlags...)
	args = append(args, "-o", outputPath, cmdPath)

	if err := sh.Run("go", args...); err != nil {
		return fmt.Errorf("failed to build %s: %w", name, err)
	}

	return nil
}

// Build builds all binaries
func Build() error {
	fmt.Println("Building binaries...")

	for name, path := range binaries {
		if err := buildBinary(name, path); err != nil {
			return err
		}
	}

	fmt.Println("✅ All binaries built successfully")
	return nil
}

// Test runs all tests with coverage
func Test() error {
	mg.Deps(ensureDirs)

	// Clean up any stale server processes before running tests
	if err := cleanupServers(); err != nil {
		fmt.Printf("Warning: failed to cleanup servers: %v\n", err)
	}

	fmt.Println("Running tests...")
	coverageFile := filepath.Join(coverageDir, "coverage.out")
	return sh.Run("go", "test", "-v", "-race", "-coverprofile="+coverageFile, "./...")
}

// Bench runs benchmarks
func Bench() error {
	fmt.Println("Running benchmarks...")
	return sh.Run("go", "test", "-bench=.", "-benchmem", "./...")
}

// Fmt formats all Go code
func Fmt() error {
	fmt.Println("Formatting code...")

	if err := sh.Run("go", "fmt", "./..."); err != nil {
		return err
	}

	return sh.Run("gofmt", "-s", "-w", ".")
}

// Lint runs golangci-lint
func Lint() error {
	fmt.Println("Running linter...")

	// Check if golangci-lint is available
	if err := sh.Run("which", "golangci-lint"); err != nil {
		fmt.Println("golangci-lint not found, installing...")
		if err := sh.Run("go", "install", "github.com/golangci/golangci-lint/cmd/golangci-lint@latest"); err != nil {
			return fmt.Errorf("failed to install golangci-lint: %w", err)
		}
	}

	return sh.Run("golangci-lint", "run", "./...")
}

// Install installs all binaries to GOPATH
func Install() error {
	buildFlags, err := getBuildFlags()
	if err != nil {
		return err
	}

	fmt.Println("Installing binaries...")
	for name, path := range binaries {
		fmt.Printf("Installing %s...\n", name)

		args := append([]string{"install"}, goFlags...)
		args = append(args, buildFlags...)
		args = append(args, path)

		if err := sh.Run("go", args...); err != nil {
			return fmt.Errorf("failed to install %s: %w", name, err)
		}
	}

	fmt.Println("✅ All binaries installed successfully")
	return nil
}

// Clean removes all build artifacts
func Clean() error {
	fmt.Println("Cleaning build artifacts...")

	// Remove build directory
	if err := sh.Rm(buildDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove build directory: %w", err)
	}

	// Clean Hugo/docs generated files
	hugoArtifacts := []string{
		"docs/public",
		"docs/resources",
		"docs/.hugo_build.lock",
		"docs/node_modules",
		"docs/themes/docsy/assets/_vendor",
	}

	for _, artifact := range hugoArtifacts {
		if err := sh.Rm(artifact); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s: %w", artifact, err)
		}
	}

	// Clean go cache
	if err := sh.Run("go", "clean", "-cache"); err != nil {
		return fmt.Errorf("failed to clean go cache: %w", err)
	}

	fmt.Println("✅ Cleaned successfully")
	return nil
}

// Deps downloads and tidies dependencies
func Deps() error {
	fmt.Println("Managing dependencies...")

	if err := sh.Run("go", "mod", "download"); err != nil {
		return err
	}

	return sh.Run("go", "mod", "tidy")
}

// Generate runs code generation (sqlc, etc.)
func Generate() error {
	fmt.Println("Running code generation...")

	// Check if sqlc is available
	if err := sh.Run("which", "sqlc"); err != nil {
		fmt.Println("sqlc not found, installing...")
		if err := sh.Run("go", "install", "github.com/sqlc-dev/sqlc/cmd/sqlc@latest"); err != nil {
			return fmt.Errorf("failed to install sqlc: %w", err)
		}
	}

	// Run sqlc generate
	fmt.Println("Generating database code with sqlc...")
	if err := sh.Run("sqlc", "generate"); err != nil {
		return fmt.Errorf("failed to run sqlc generate: %w", err)
	}

	fmt.Println("✅ Code generation completed successfully")
	return nil
}

// Coverage generates HTML coverage report
func Coverage() error {
	mg.Deps(Test)

	fmt.Println("Generating coverage report...")
	coverageFile := filepath.Join(coverageDir, "coverage.out")
	htmlFile := filepath.Join(coverageDir, "coverage.html")

	return sh.Run("go", "tool", "cover", "-html="+coverageFile, "-o", htmlFile)
}

// TestDocs runs documentation tests
func TestDocs() error {
	fmt.Println("Testing documentation examples...")

	docsDir := "docs/testable"
	if _, err := os.Stat(docsDir); os.IsNotExist(err) {
		fmt.Println("Documentation test directory not found, skipping...")
		return nil
	}

	return sh.RunWith(map[string]string{"PWD": docsDir}, "go", "test", "-v", ".")
}

// Snapshot creates a snapshot release with goreleaser
func Snapshot() error {
	fmt.Println("Creating snapshot release...")

	// Check if goreleaser is available
	if err := sh.Run("which", "goreleaser"); err != nil {
		return fmt.Errorf("goreleaser not found. Install from https://goreleaser.com")
	}

	return sh.Run("goreleaser", "release", "--snapshot", "--clean")
}

// Release creates a release with goreleaser
func Release() error {
	fmt.Println("Creating release...")

	// Check if goreleaser is available
	if err := sh.Run("which", "goreleaser"); err != nil {
		return fmt.Errorf("goreleaser not found. Install from https://goreleaser.com")
	}

	return sh.Run("goreleaser", "release", "--clean")
}

// Dev builds and runs the main gismo binary for development
func Dev() error {
	mg.Deps(Build)

	gismoPath := filepath.Join(binDir, "gismo")
	args := os.Args[2:] // Skip "mage dev"

	fmt.Printf("Running: %s %s\n", gismoPath, strings.Join(args, " "))
	return sh.Run(gismoPath, args...)
}

// Check runs all quality checks (fmt, lint, test)
func Check() error {
	mg.Deps(Fmt, Lint, Test)
	fmt.Println("✅ All checks passed")
	return nil
}

// Info shows build information
func Info() error {
	fmt.Printf("Go version: %s\n", runtime.Version())
	fmt.Printf("Go OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	if version, err := sh.Output("git", "describe", "--tags", "--always", "--dirty"); err == nil {
		fmt.Printf("Version: %s\n", version)
	}

	if commit, err := sh.Output("git", "rev-parse", "--short", "HEAD"); err == nil {
		fmt.Printf("Commit: %s\n", commit)
	}

	fmt.Printf("Build directory: %s\n", buildDir)
	fmt.Printf("Binary directory: %s\n", binDir)

	return nil
}
