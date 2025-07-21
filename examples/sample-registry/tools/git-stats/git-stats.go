package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// GitStats represents repository statistics
type GitStats struct {
	TotalCommits    int
	Authors         map[string]int
	FilesChanged    map[string]int
	LinesAdded      int
	LinesDeleted    int
	FirstCommitDate time.Time
	LastCommitDate  time.Time
	ActiveDays      int
	WeeklyActivity  map[string]int
	FileExtensions  map[string]int
}

// AuthorStats represents statistics for a single author
type AuthorStats struct {
	Name    string
	Commits int
	Lines   int
}

// NewGitStats creates a new GitStats instance
func NewGitStats() *GitStats {
	return &GitStats{
		Authors:        make(map[string]int),
		FilesChanged:   make(map[string]int),
		WeeklyActivity: make(map[string]int),
		FileExtensions: make(map[string]int),
	}
}

// CollectStats gathers git repository statistics
func (gs *GitStats) CollectStats() error {
	// Check if we're in a git repository
	if err := gs.checkGitRepo(); err != nil {
		return err
	}

	// Collect basic commit stats
	if err := gs.collectCommitStats(); err != nil {
		return err
	}

	// Collect author statistics
	if err := gs.collectAuthorStats(); err != nil {
		return err
	}

	// Collect file change statistics
	if err := gs.collectFileStats(); err != nil {
		return err
	}

	// Collect line change statistics
	if err := gs.collectLineStats(); err != nil {
		return err
	}

	return nil
}

func (gs *GitStats) checkGitRepo() error {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	return cmd.Run()
}

func (gs *GitStats) collectCommitStats() error {
	// Get total commit count
	cmd := exec.Command("git", "rev-list", "--count", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get commit count: %w", err)
	}

	count, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return fmt.Errorf("failed to parse commit count: %w", err)
	}
	gs.TotalCommits = count

	// Get first and last commit dates
	cmd = exec.Command("git", "log", "--format=%at", "--reverse")
	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get commit dates: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) > 0 {
		// First commit
		if timestamp, err := strconv.ParseInt(lines[0], 10, 64); err == nil {
			gs.FirstCommitDate = time.Unix(timestamp, 0)
		}
		// Last commit
		if timestamp, err := strconv.ParseInt(lines[len(lines)-1], 10, 64); err == nil {
			gs.LastCommitDate = time.Unix(timestamp, 0)
		}
	}

	return nil
}

func (gs *GitStats) collectAuthorStats() error {
	cmd := exec.Command("git", "log", "--format=%an")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get authors: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		author := strings.TrimSpace(scanner.Text())
		if author != "" {
			gs.Authors[author]++
		}
	}

	return scanner.Err()
}

func (gs *GitStats) collectFileStats() error {
	cmd := exec.Command("git", "log", "--name-only", "--format=")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get file changes: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		filename := strings.TrimSpace(scanner.Text())
		if filename != "" {
			gs.FilesChanged[filename]++

			// Track file extensions
			if dotIndex := strings.LastIndex(filename, "."); dotIndex != -1 {
				ext := filename[dotIndex:]
				gs.FileExtensions[ext]++
			}
		}
	}

	return scanner.Err()
}

func (gs *GitStats) collectLineStats() error {
	cmd := exec.Command("git", "log", "--numstat", "--format=")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get line stats: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 {
			if added, err := strconv.Atoi(parts[0]); err == nil {
				gs.LinesAdded += added
			}
			if deleted, err := strconv.Atoi(parts[1]); err == nil {
				gs.LinesDeleted += deleted
			}
		}
	}

	return scanner.Err()
}

// PrintStats displays the collected statistics
func (gs *GitStats) PrintStats() {
	fmt.Println("📊 Git Repository Statistics")
	fmt.Println("═══════════════════════════")

	// Basic stats
	fmt.Printf("📝 Total Commits: %d\n", gs.TotalCommits)
	if !gs.FirstCommitDate.IsZero() && !gs.LastCommitDate.IsZero() {
		duration := gs.LastCommitDate.Sub(gs.FirstCommitDate)
		fmt.Printf("📅 Repository Age: %.0f days\n", duration.Hours()/24)
		fmt.Printf("🗓️  First Commit: %s\n", gs.FirstCommitDate.Format("2006-01-02"))
		fmt.Printf("🗓️  Last Commit: %s\n", gs.LastCommitDate.Format("2006-01-02"))
	}

	// Line stats
	fmt.Printf("➕ Lines Added: %d\n", gs.LinesAdded)
	fmt.Printf("➖ Lines Deleted: %d\n", gs.LinesDeleted)
	fmt.Printf("📈 Net Lines: %d\n", gs.LinesAdded-gs.LinesDeleted)

	// Top authors
	fmt.Println("\n👥 Top Contributors:")
	authors := make([]AuthorStats, 0, len(gs.Authors))
	for name, commits := range gs.Authors {
		authors = append(authors, AuthorStats{Name: name, Commits: commits})
	}
	sort.Slice(authors, func(i, j int) bool {
		return authors[i].Commits > authors[j].Commits
	})

	for i, author := range authors {
		if i >= 10 { // Top 10 authors
			break
		}
		percentage := float64(author.Commits) / float64(gs.TotalCommits) * 100
		fmt.Printf("   %2d. %-20s %4d commits (%.1f%%)\n",
			i+1, author.Name, author.Commits, percentage)
	}

	// Top file extensions
	fmt.Println("\n📁 File Types:")
	extensions := make([]struct {
		Ext   string
		Count int
	}, 0, len(gs.FileExtensions))

	for ext, count := range gs.FileExtensions {
		extensions = append(extensions, struct {
			Ext   string
			Count int
		}{ext, count})
	}

	sort.Slice(extensions, func(i, j int) bool {
		return extensions[i].Count > extensions[j].Count
	})

	for i, ext := range extensions {
		if i >= 10 { // Top 10 extensions
			break
		}
		fmt.Printf("   %2d. %-10s %4d changes\n", i+1, ext.Ext, ext.Count)
	}

	// Most changed files
	fmt.Println("\n🔥 Most Changed Files:")
	files := make([]struct {
		Name  string
		Count int
	}, 0, len(gs.FilesChanged))

	for name, count := range gs.FilesChanged {
		files = append(files, struct {
			Name  string
			Count int
		}{name, count})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Count > files[j].Count
	})

	for i, file := range files {
		if i >= 10 { // Top 10 files
			break
		}
		fmt.Printf("   %2d. %-30s %4d changes\n", i+1, file.Name, file.Count)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Git Statistics - Git repository analysis tool

Usage: git-stats [options]

Options:
  -h, --help     Show this help message
  --json         Output in JSON format
  --csv          Output in CSV format

Examples:
  git-stats                    # Show formatted statistics
  git-stats --json            # Output JSON format
  git-stats --csv             # Output CSV format

Note: Must be run from within a git repository.

`)
}

func main() {
	jsonOutput := false
	csvOutput := false

	// Parse arguments
	for _, arg := range os.Args[1:] {
		switch arg {
		case "-h", "--help":
			printUsage()
			os.Exit(0)
		case "--json":
			jsonOutput = true
		case "--csv":
			csvOutput = true
		}
	}

	// Collect statistics
	stats := NewGitStats()
	if err := stats.CollectStats(); err != nil {
		fmt.Fprintf(os.Stderr, "Error collecting git statistics: %v\n", err)
		fmt.Fprintf(os.Stderr, "Make sure you're running this from within a git repository.\n")
		os.Exit(1)
	}

	// Output in requested format
	if jsonOutput {
		// TODO: Implement JSON output
		fmt.Println("JSON output not yet implemented")
	} else if csvOutput {
		// TODO: Implement CSV output
		fmt.Println("CSV output not yet implemented")
	} else {
		stats.PrintStats()
	}
}
