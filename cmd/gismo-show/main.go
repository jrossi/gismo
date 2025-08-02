package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jrossi/gismo/pkg/engine"
)

func main() {
	// Define global flags
	debug := flag.Bool("debug", false, "Enable debug output")
	configFile := flag.String("config", "", "Path to configuration file")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: gismo-show [options] <file>...\n\n")
		fmt.Fprintf(os.Stderr, "Show which configuration rules would apply to the given files\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Check for required arguments
	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Error: show-actions requires at least one file path\n")
		flag.Usage()
		os.Exit(1)
	}

	// Load configuration
	configLoader, err := engine.NewConfigLoader()
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Failed to create config loader: %v\n", err)
		}
		// Continue without config
		configLoader = nil
	}

	var appConfig *engine.AppConfig
	if configLoader != nil {
		if *configFile != "" {
			// Load specific config file
			appConfig, err = configLoader.LoadConfigWithPaths([]string{*configFile})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to load config file %s: %v\n", *configFile, err)
				os.Exit(1)
			}
		} else {
			// Load default config files
			appConfig, err = configLoader.LoadConfig()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
				os.Exit(1)
			}
		}
	}

	// Create linting config from app config
	lintingConfig := engine.LintingConfig{}
	if appConfig != nil {
		if appConfig.Parallel != nil {
			if appConfig.Parallel.MaxWorkers != nil {
				lintingConfig.MaxWorkers = *appConfig.Parallel.MaxWorkers
			}
			if appConfig.Parallel.DisableParallel != nil {
				lintingConfig.DisableParallel = *appConfig.Parallel.DisableParallel
			}
		}
	}

	// Create rule engine with linting capabilities
	ruleEngine := engine.NewLintingRuleEngineWithConfig(lintingConfig)

	// Set the app config if available
	if appConfig != nil {
		ruleEngine.SetAppConfig(appConfig)
	}

	// Process the file argument
	filePath := flag.Args()[0]
	if err := showFilter(filePath, ruleEngine, configLoader, *configFile, *debug); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// showConfig displays the current configuration
// Commented out - no longer used in simplified show-actions mode
/*
func showConfig(appConfig *gismo.AppConfig, debug bool) error {
	fmt.Println("=== Current Configuration ===")

	if appConfig == nil {
		fmt.Println("\nNo configuration loaded.")
		return nil
	}

	// Pretty print the configuration
	configJSON, err := json.MarshalIndent(appConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	fmt.Println()
	fmt.Println(string(configJSON))

	if debug {
		fmt.Println("\n--- Configuration Sources ---")
		fmt.Println("Configuration files are loaded in this order (later files override earlier):")
		fmt.Println("1. ~/.claude/gismo.json (global)")
		fmt.Println("2. .claude/gismo.json (project)")
		fmt.Println("3. .claude/gismo.local.json (local overrides)")
		fmt.Println("4. --config flag (if specified)")
	}

	return nil
}
*/

// ConfigPath represents a configuration file path with description
type ConfigPath struct {
	path string
	desc string
}

// getConfigPaths returns the configuration paths that would be loaded
func getConfigPaths(customConfigFile string, configLoader *engine.ConfigLoader) []ConfigPath {
	var paths []ConfigPath

	if customConfigFile != "" {
		paths = append(paths, ConfigPath{customConfigFile, "custom config"})
	} else if configLoader != nil {
		homeDir, _ := os.UserHomeDir()
		cwd, _ := os.Getwd()

		// Only include paths that actually exist
		potentialPaths := []ConfigPath{
			{filepath.Join(homeDir, ".claude", "gismo.json"), "global config"},
			{filepath.Join(cwd, ".claude", "gismo.json"), "project config"},
			{filepath.Join(cwd, ".claude", "gismo.local.json"), "local overrides"},
		}

		for _, cp := range potentialPaths {
			if _, err := os.Stat(cp.path); err == nil {
				paths = append(paths, cp)
			}
		}
	}

	return paths
}

// showConfigSources displays which configuration files were loaded
func showConfigSources(customConfigFile string, configLoader *engine.ConfigLoader) {
	fmt.Printf("=== Configuration Sources ===\n")

	if customConfigFile != "" {
		// Custom config file specified
		fmt.Printf("Using custom config: %s\n", customConfigFile)
		if _, err := os.Stat(customConfigFile); err == nil {
			fmt.Printf("  ✓ File exists\n")
		} else {
			fmt.Printf("  ✗ File not found\n")
		}
	} else if configLoader != nil {
		// Show standard config hierarchy
		homeDir, _ := os.UserHomeDir()
		cwd, _ := os.Getwd()

		configPaths := []ConfigPath{
			{filepath.Join(homeDir, ".claude", "gismo.json"), "global config"},
			{filepath.Join(cwd, ".claude", "gismo.json"), "project config"},
			{filepath.Join(cwd, ".claude", "gismo.local.json"), "local overrides"},
		}

		fmt.Printf("Configuration files (in order of precedence):\n")
		for _, cp := range configPaths {
			if _, err := os.Stat(cp.path); err == nil {
				fmt.Printf("  ✓ %s (%s)\n", cp.path, cp.desc)
			} else {
				fmt.Printf("  ✗ %s (%s) - not found\n", cp.path, cp.desc)
			}
		}
		fmt.Printf("\nLater files override settings from earlier files.\n")
	} else {
		fmt.Printf("No configuration loaded.\n")
	}
}

// showFilter shows which rules and linters apply to a specific file
func showFilter(filePath string, ruleEngine *engine.LintingRuleEngine, configLoader *engine.ConfigLoader, customConfigFile string, debug bool) error {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", filePath)
	}

	// Get the absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Show configuration sources first
	showConfigSources(customConfigFile, configLoader)

	fmt.Printf("\n=== Configuration Analysis for: %s ===\n", absPath)

	// Get the app config from the rule engine
	appConfig := ruleEngine.GetAppConfig()
	if appConfig == nil {
		fmt.Printf("\nNo configuration loaded.\n")
		return nil
	}

	// Determine which linters would handle this file
	fmt.Printf("\n--- Applicable Linters ---\n")
	ext := filepath.Ext(filePath)
	applicableLinters := []string{}

	// Check all possible linters
	linterMap := map[string][]string{
		".go":       {"golang"},
		".md":       {"markdown"},
		".markdown": {"markdown"},
		".js":       {"javascript"},
		".jsx":      {"javascript"},
		".ts":       {"javascript"},
		".tsx":      {"javascript"},
		".py":       {"python"},
		".rs":       {"rust"},
		".proto":    {"protobuf"},
		".json":     {"json"},
		".jsonc":    {"json"},
		".json5":    {"json"},
	}

	if linters, ok := linterMap[ext]; ok {
		for _, linter := range linters {
			applicableLinters = append(applicableLinters, linter)
			fmt.Printf("✓ %s linter (handles %s files)\n", linter, ext)
		}
	} else {
		fmt.Printf("ℹ️  No linters configured for %s files\n", ext)
	}

	// Show base configuration for each applicable linter
	for _, linterName := range applicableLinters {
		fmt.Printf("\n--- Base Configuration for %s ---\n", linterName)

		if linterConfig, exists := appConfig.GetLinterConfig(linterName); exists {
			// Pretty print the linter config
			var configMap map[string]interface{}
			if err := json.Unmarshal(linterConfig, &configMap); err == nil {
				for key, value := range configMap {
					fmt.Printf("  %s: %v\n", key, value)
				}
			} else {
				fmt.Printf("  Raw config: %s\n", string(linterConfig))
			}
		} else {
			fmt.Printf("  (default configuration)\n")
		}

		// Check if linter is enabled
		if appConfig.IsLinterEnabled(linterName) {
			fmt.Printf("  ✓ Linter is enabled\n")
		} else {
			fmt.Printf("  ✗ Linter is disabled\n")
		}
	}

	// Show which rules would apply with config source info
	fmt.Printf("\n--- Rule Hierarchy ---\n")
	fmt.Printf("Rules are applied in order. Later rules override earlier ones.\n")

	// Try to determine which config file rules come from based on their position
	// This is a heuristic since we don't track sources during merge
	configPaths := getConfigPaths(customConfigFile, configLoader)

	fmt.Printf("\n")

	matchedRules := false
	for i, rule := range appConfig.Rules {
		// Check if this rule matches the file
		matched := MatchesPattern(rule.Pattern, absPath)

		if debug && !matched {
			fmt.Printf("   Pattern '%s' did not match '%s'\n", rule.Pattern, absPath)
		}

		if matched {
			matchedRules = true
			fmt.Printf("%d. Pattern: %s", i+1, rule.Pattern)
			if rule.Linter == "*" {
				fmt.Printf(" (applies to ALL linters)")
			} else {
				fmt.Printf(" (applies to %s linter)", rule.Linter)
			}

			// Try to indicate which config file this likely came from
			// This is a heuristic based on rule order
			if len(configPaths) > 0 {
				configIndex := min(i/max(1, len(appConfig.Rules)/len(configPaths)), len(configPaths)-1)
				fmt.Printf(" [likely from: %s]", configPaths[configIndex].desc)
			}
			fmt.Printf("\n")

			// Show what this rule would override
			var overrideMap map[string]interface{}
			if err := json.Unmarshal(rule.Rules, &overrideMap); err == nil {
				for key, value := range overrideMap {
					fmt.Printf("   → %s: %v\n", key, value)
				}
			}
			fmt.Printf("\n")
		}
	}

	if !matchedRules {
		fmt.Printf("ℹ️  No pattern-based rules match this file.\n")
		fmt.Printf("   Base linter configuration will be used.\n")
	}

	// Show the final merged configuration for each linter
	for _, linterName := range applicableLinters {
		fmt.Printf("\n--- Final Configuration for %s ---\n", linterName)
		fmt.Printf("(After applying all matching rules)\n")

		// Get all overrides that would apply
		overrides := appConfig.GetRuleOverrides(absPath, linterName)

		// Start with base config
		finalConfig := make(map[string]interface{})
		if baseConfig, exists := appConfig.GetLinterConfig(linterName); exists {
			_ = json.Unmarshal(baseConfig, &finalConfig)
		}

		// Apply each override in order
		for _, override := range overrides {
			var overrideMap map[string]interface{}
			if err := json.Unmarshal(override, &overrideMap); err == nil {
				for k, v := range overrideMap {
					finalConfig[k] = v
				}
			}
		}

		// Display final config
		if len(finalConfig) > 0 {
			for key, value := range finalConfig {
				fmt.Printf("  %s: %v\n", key, value)
			}
		} else {
			fmt.Printf("  (default configuration)\n")
		}
	}

	// Show Claude Code integration information with visual tree
	fmt.Printf("\n--- Claude Code Hook Execution Flow ---\n\n")

	if len(applicableLinters) > 0 {
		// Show the execution tree for different operations
		showExecutionTree(filePath, applicableLinters, appConfig, ruleEngine, customConfigFile)
	} else {
		fmt.Printf("ℹ️  This file type is not monitored by gismo.\n")
		fmt.Printf("   Claude Code operations on this file will not trigger linting.\n")
	}

	return nil
}

// showExecutionTree displays a visual tree of how Claude Code hooks execute
func showExecutionTree(filePath string, applicableLinters []string, appConfig *engine.AppConfig, ruleEngine *engine.LintingRuleEngine, customConfigFile string) {
	ext := filepath.Ext(filePath)

	// ANSI color codes
	const (
		reset  = "\033[0m"
		bold   = "\033[1m"
		dim    = "\033[2m"
		red    = "\033[31m"
		green  = "\033[32m"
		yellow = "\033[33m"
		blue   = "\033[34m"
		cyan   = "\033[36m"
		white  = "\033[37m"
	)

	// Tree drawing characters
	const (
		vertical   = "│"
		horizontal = "─"
		corner     = "└"
		branch     = "├"
		space      = " "
	)

	// First show which settings.json file configures the hooks
	fmt.Printf("%sHook Configuration Source:%s\n", bold, reset)

	// Check for Claude Code settings.json files
	homeDir, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()

	settingsPaths := []ConfigPath{
		{filepath.Join(homeDir, ".claude", "settings.json"), "global hooks"},
		{filepath.Join(cwd, ".claude", "settings.json"), "project hooks"},
	}

	foundSettings := false
	for _, sp := range settingsPaths {
		if _, err := os.Stat(sp.path); err == nil {
			fmt.Printf("%s✓ %s (%s)%s\n", green, sp.path, sp.desc, reset)
			foundSettings = true
		}
	}

	if !foundSettings {
		fmt.Printf("%s⚠️  No Claude Code settings.json found%s\n", yellow, reset)
		fmt.Printf("   Run 'gismo init' to configure hooks\n")
	}

	fmt.Printf("\n%sWhen Claude Code operates on this file:%s\n\n", bold, reset)

	// PreToolUse Hook - only for Write
	fmt.Printf("%s%sPreToolUse Hook%s %s(Write operation only)%s\n", green, bold, reset, dim, reset)
	fmt.Printf("%s%s%s%s %sTriggered BEFORE content is written%s\n", green, vertical, horizontal, horizontal, dim, reset)
	fmt.Printf("%s%s\n", green, vertical)

	for i, linterName := range applicableLinters {
		isLast := i == len(applicableLinters)-1
		connector := branch
		if isLast {
			connector = corner
		}

		if appConfig.IsLinterEnabled(linterName) {
			fmt.Printf("%s%s%s%s %s%s linter%s", green, connector, horizontal, horizontal, cyan, linterName, reset)

			// Show specific checks for golang
			if linterName == "golang" {
				fmt.Printf(" %s(pre-lint content)%s\n", dim, reset)
				if !isLast {
					fmt.Printf("%s%s   %s%s%s Syntax validation%s\n", green, vertical, dim, branch, horizontal, reset)
					fmt.Printf("%s%s   %s%s%s Format checking (gofmt)%s\n", green, vertical, dim, corner, horizontal, reset)
				} else {
					fmt.Printf("%s    %s%s%s Syntax validation%s\n", space, dim, branch, horizontal, reset)
					fmt.Printf("%s    %s%s%s Format checking (gofmt)%s\n", space, dim, corner, horizontal, reset)
				}
			} else {
				fmt.Printf(" %s(validate content)%s\n", dim, reset)
			}
		} else {
			fmt.Printf("%s%s%s%s %s%s linter%s %s[DISABLED]%s\n", green, connector, horizontal, horizontal, dim, linterName, reset, yellow, reset)
		}
	}

	fmt.Printf("\n%s  ↓%s %sIf any errors found → %s%sBLOCK operation%s\n", green, reset, dim, red, bold, reset)
	fmt.Printf("%s  ↓%s %sIf all pass → %s%sPROCEED with write%s\n\n", green, reset, dim, green, bold, reset)

	// PostToolUse Hook - for Write, Edit, MultiEdit
	fmt.Printf("%s%sPostToolUse Hook%s %s(Write, Edit, MultiEdit operations)%s\n", blue, bold, reset, dim, reset)
	fmt.Printf("%s%s%s%s %sTriggered AFTER file is modified on disk%s\n", blue, vertical, horizontal, horizontal, dim, reset)
	fmt.Printf("%s%s\n", blue, vertical)

	// Show parallel execution
	if appConfig.Parallel != nil && appConfig.Parallel.MaxWorkers != nil && *appConfig.Parallel.MaxWorkers > 1 {
		fmt.Printf("%s%s%s%s %s%sParallel execution%s %s(up to %d workers)%s\n", blue, branch, horizontal, horizontal, yellow, bold, reset, dim, *appConfig.Parallel.MaxWorkers, reset)
	} else {
		fmt.Printf("%s%s%s%s %s%sParallel execution%s\n", blue, branch, horizontal, horizontal, yellow, bold, reset)
	}
	fmt.Printf("%s%s\n", blue, vertical)

	for i, linterName := range applicableLinters {
		isLast := i == len(applicableLinters)-1
		connector := branch
		verticalPrefix := vertical
		if isLast {
			connector = corner
			verticalPrefix = space
		}

		if appConfig.IsLinterEnabled(linterName) {
			fmt.Printf("%s%s%s%s %s%s linter%s", blue, connector, horizontal, horizontal, cyan, linterName, reset)

			// Show specific checks based on linter type
			if linterName == "golang" {
				fmt.Printf(" %s(full analysis)%s\n", dim, reset)

				// Get linter config to show specific checks
				golangChecks := []string{
					"gofmt - Format validation",
					"go vet - Static analysis",
					"golangci-lint - Multiple checks",
					"staticcheck - Advanced analysis",
				}

				// Special handling for test files
				if strings.HasSuffix(filePath, "_test.go") {
					golangChecks = append(golangChecks, "go test - Run tests")
				}

				for j, check := range golangChecks {
					checkIsLast := j == len(golangChecks)-1
					checkConnector := branch
					if checkIsLast {
						checkConnector = corner
					}

					if isLast {
						fmt.Printf("%s    %s%s%s %s%s\n", space, dim, checkConnector, horizontal, check, reset)
					} else {
						fmt.Printf("%s%s   %s%s%s %s%s\n", blue, verticalPrefix, dim, checkConnector, horizontal, check, reset)
					}
				}

				// Check associated test file
				if !strings.HasSuffix(filePath, "_test.go") && ext == ".go" {
					testFile := strings.TrimSuffix(filepath.Base(filePath), ext) + "_test" + ext
					if isLast {
						fmt.Printf("%s    %s%s%s Also check: %s%s\n", space, dim, corner, horizontal, testFile, reset)
					} else {
						fmt.Printf("%s%s   %s%s%s Also check: %s%s\n", blue, verticalPrefix, dim, corner, horizontal, testFile, reset)
					}
				}
			} else if linterName == "javascript" {
				fmt.Printf(" %s(ESLint + format)%s\n", dim, reset)
			} else if linterName == "python" {
				fmt.Printf(" %s(ruff + mypy)%s\n", dim, reset)
			} else if linterName == "markdown" {
				fmt.Printf(" %s(markdownlint)%s\n", dim, reset)
			} else {
				fmt.Printf("\n")
			}
		} else {
			fmt.Printf("%s%s%s%s %s%s linter%s %s[DISABLED]%s\n", blue, connector, horizontal, horizontal, dim, linterName, reset, yellow, reset)
		}
	}

	fmt.Printf("\n%s  ↓%s %sResults aggregated%s\n", blue, reset, dim, reset)
	fmt.Printf("%s  ↓%s\n", blue, reset)
	fmt.Printf("%s%s%s%s %sExit Codes:%s\n", white, branch, horizontal, horizontal, bold, reset)
	fmt.Printf("%s%s   %s%s%s %s0%s = Success %s(logged to transcript)%s\n", white, vertical, dim, branch, horizontal, green, reset, dim, reset)
	fmt.Printf("%s%s   %s%s%s %s2%s = Errors found %s(shown to Claude via stderr)%s\n", white, corner, dim, corner, horizontal, red, reset, dim, reset)

	// Show which linters are disabled
	disabledCount := 0
	for _, linterName := range applicableLinters {
		if !appConfig.IsLinterEnabled(linterName) {
			disabledCount++
		}
	}

	if disabledCount > 0 {
		fmt.Printf("\n%s⚠️  Note:%s %d of %d linters are currently disabled\n", yellow, reset, disabledCount, len(applicableLinters))
		fmt.Printf("   Enable them in your configuration for comprehensive checking.\n")
	}
}

// Helper functions

// MatchesPattern checks if a file path matches a glob pattern
// It supports ** for matching any number of directories
func MatchesPattern(pattern, path string) bool {
	// For absolute paths, also try relative matching from current directory
	relPath := path
	if filepath.IsAbs(path) {
		if cwd, err := os.Getwd(); err == nil {
			if rel, err := filepath.Rel(cwd, path); err == nil {
				relPath = rel
			}
		}
	}

	// Try both absolute and relative paths
	for _, p := range []string{path, relPath} {
		// First try direct match
		if matched, _ := filepath.Match(pattern, p); matched {
			return true
		}

		// Try matching against just the filename
		if matched, _ := filepath.Match(pattern, filepath.Base(p)); matched {
			return true
		}

		// Handle ** patterns
		if strings.Contains(pattern, "**") {
			if MatchesDoubleStarPattern(pattern, p) {
				return true
			}
		}
	}

	return false
}

// MatchesDoubleStarPattern handles patterns with ** for directory wildcards
func MatchesDoubleStarPattern(pattern, path string) bool {
	// Convert ** to a regex-like pattern
	// e.g., "internal/**/*.go" should match "internal/foo/bar.go"
	parts := strings.Split(pattern, "**")
	if len(parts) == 2 {
		prefix := strings.TrimSuffix(parts[0], "/")
		suffix := strings.TrimPrefix(parts[1], "/")

		// For patterns starting with **, match anywhere in path
		if prefix == "" && suffix != "" {
			// Pattern like "**/*.go" should match any .go file at any depth
			pathParts := strings.Split(path, "/")
			for i := range pathParts {
				subPath := strings.Join(pathParts[i:], "/")
				if matched, _ := filepath.Match(suffix, subPath); matched {
					return true
				}
			}
			// Also check just the filename
			return MatchesSimplePattern(suffix, filepath.Base(path))
		}

		// Check if path starts with prefix
		if prefix != "" && !strings.HasPrefix(path, prefix+"/") && path != prefix {
			return false
		}

		// Get the part after the prefix
		remainder := strings.TrimPrefix(path, prefix)
		remainder = strings.TrimPrefix(remainder, "/")

		// Check if the remainder matches the suffix pattern
		if suffix != "" {
			// For patterns like "*.go", we need to check the end of the path
			if strings.HasPrefix(suffix, "*") && !strings.Contains(suffix, "/") {
				return strings.HasSuffix(remainder, strings.TrimPrefix(suffix, "*"))
			}
			// For other patterns, try to match against the remainder
			if matched, _ := filepath.Match(suffix, remainder); matched {
				return true
			}
			// Also try matching just the filename part
			if matched, _ := filepath.Match(suffix, filepath.Base(remainder)); matched {
				return true
			}
		} else {
			// Pattern ends with **, matches everything under prefix
			return true
		}
	}

	return false
}

// MatchesSimplePattern is a helper for simple pattern matching
func MatchesSimplePattern(pattern, name string) bool {
	matched, _ := filepath.Match(pattern, name)
	return matched
}
