package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// MarkdownLinter provides markdown linting functionality
type MarkdownLinter struct {
	rules    map[string]bool
	warnings []LintIssue
	errors   []LintIssue
}

// LintIssue represents a linting issue
type LintIssue struct {
	File       string
	Line       int
	Column     int
	Rule       string
	Severity   string
	Message    string
	Suggestion string
}

// NewMarkdownLinter creates a new markdown linter
func NewMarkdownLinter() *MarkdownLinter {
	return &MarkdownLinter{
		rules: map[string]bool{
			"heading-style":      true,
			"line-length":        true,
			"no-trailing-spaces": true,
			"no-multiple-blanks": true,
			"link-check":         true,
			"code-block-style":   true,
			"list-style":         true,
			"emphasis-style":     true,
		},
		warnings: []LintIssue{},
		errors:   []LintIssue{},
	}
}

// LintFile lints a single markdown file
func (ml *MarkdownLinter) LintFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return ml.LintReader(file, filename)
}

// LintReader lints markdown content from a reader
func (ml *MarkdownLinter) LintReader(reader io.Reader, filename string) error {
	scanner := bufio.NewScanner(reader)
	lineNum := 0

	var prevLine string
	inCodeBlock := false
	codeBlockType := ""

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Check for code block boundaries
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if !inCodeBlock {
				inCodeBlock = true
				codeBlockType = strings.TrimSpace(line)
			} else {
				inCodeBlock = false
				codeBlockType = ""
			}
		}

		// Skip linting inside code blocks (except for code block rules)
		if !inCodeBlock {
			ml.checkHeadingStyle(filename, lineNum, line)
			ml.checkLineLength(filename, lineNum, line)
			ml.checkListStyle(filename, lineNum, line)
			ml.checkEmphasisStyle(filename, lineNum, line)
			ml.checkLinks(filename, lineNum, line)
		}

		ml.checkTrailingSpaces(filename, lineNum, line)
		ml.checkMultipleBlanks(filename, lineNum, line, prevLine)
		ml.checkCodeBlockStyle(filename, lineNum, line, codeBlockType)

		prevLine = line
	}

	return scanner.Err()
}

// checkHeadingStyle validates heading formatting
func (ml *MarkdownLinter) checkHeadingStyle(filename string, lineNum int, line string) {
	if !ml.rules["heading-style"] {
		return
	}

	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "#") {
		// Check for space after #
		hashCount := 0
		for _, r := range trimmed {
			if r == '#' {
				hashCount++
			} else {
				break
			}
		}

		if hashCount > 6 {
			ml.addError(filename, lineNum, 1, "heading-style",
				"Heading level too deep (max 6)", "Use h1-h6 headings only")
		}

		if len(trimmed) > hashCount && trimmed[hashCount] != ' ' {
			ml.addError(filename, lineNum, hashCount+1, "heading-style",
				"Missing space after heading markers", "Add space after # symbols")
		}

		// Check for trailing #
		if strings.HasSuffix(trimmed, "#") && !strings.HasSuffix(trimmed, " #") {
			ml.addWarning(filename, lineNum, len(trimmed), "heading-style",
				"Avoid trailing # in headings", "Remove trailing # symbols")
		}
	}
}

// checkLineLength validates line length
func (ml *MarkdownLinter) checkLineLength(filename string, lineNum int, line string) {
	if !ml.rules["line-length"] {
		return
	}

	maxLength := 100
	if len(line) > maxLength {
		ml.addWarning(filename, lineNum, maxLength+1, "line-length",
			fmt.Sprintf("Line too long (%d > %d characters)", len(line), maxLength),
			"Break long lines for better readability")
	}
}

// checkTrailingSpaces validates trailing whitespace
func (ml *MarkdownLinter) checkTrailingSpaces(filename string, lineNum int, line string) {
	if !ml.rules["no-trailing-spaces"] {
		return
	}

	if len(line) > 0 && line[len(line)-1] == ' ' {
		// Count trailing spaces
		spaceCount := 0
		for i := len(line) - 1; i >= 0 && line[i] == ' '; i-- {
			spaceCount++
		}

		// Allow exactly 2 trailing spaces for line breaks
		if spaceCount != 2 {
			ml.addError(filename, lineNum, len(line)-spaceCount+1, "no-trailing-spaces",
				"Trailing whitespace found", "Remove trailing spaces")
		}
	}
}

// checkMultipleBlanks validates multiple blank lines
func (ml *MarkdownLinter) checkMultipleBlanks(filename string, lineNum int, line, prevLine string) {
	if !ml.rules["no-multiple-blanks"] {
		return
	}

	if strings.TrimSpace(line) == "" && strings.TrimSpace(prevLine) == "" {
		ml.addWarning(filename, lineNum, 1, "no-multiple-blanks",
			"Multiple consecutive blank lines", "Use single blank line for separation")
	}
}

// checkLinks validates markdown links
func (ml *MarkdownLinter) checkLinks(filename string, lineNum int, line string) {
	if !ml.rules["link-check"] {
		return
	}

	// Find markdown links [text](url)
	linkRegex := regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)
	matches := linkRegex.FindAllStringSubmatch(line, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			linkText := match[1]
			linkURL := match[2]

			// Check for empty link text
			if strings.TrimSpace(linkText) == "" {
				ml.addWarning(filename, lineNum, strings.Index(line, match[0]), "link-check",
					"Empty link text", "Provide descriptive link text")
			}

			// Check for invalid URLs (basic validation)
			ml.validateURL(filename, lineNum, line, linkURL, match[0])
		}
	}

	// Check reference-style links [text][ref]
	refLinkRegex := regexp.MustCompile(`\[([^\]]*)\]\[([^\]]*)\]`)
	refMatches := refLinkRegex.FindAllStringSubmatch(line, -1)

	for _, match := range refMatches {
		if len(match) >= 3 && strings.TrimSpace(match[1]) == "" {
			ml.addWarning(filename, lineNum, strings.Index(line, match[0]), "link-check",
				"Empty reference link text", "Provide descriptive link text")
		}
	}
}

// validateURL performs basic URL validation
func (ml *MarkdownLinter) validateURL(filename string, lineNum int, line, url, fullMatch string) {
	// Skip anchor links and email addresses
	if strings.HasPrefix(url, "#") || strings.HasPrefix(url, "mailto:") {
		return
	}

	// Check for common URL issues
	if strings.Contains(url, " ") {
		ml.addError(filename, lineNum, strings.Index(line, fullMatch), "link-check",
			"URL contains spaces", "Encode spaces as %20 or remove them")
		return
	}

	// Check for HTTP links (suggest HTTPS)
	if strings.HasPrefix(url, "http://") {
		ml.addWarning(filename, lineNum, strings.Index(line, fullMatch), "link-check",
			"HTTP link found", "Consider using HTTPS for security")
	}

	// For local files, check if they exist
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		// Remove fragment identifier
		localPath := strings.Split(url, "#")[0]
		if localPath != "" {
			// Make path relative to markdown file
			dir := filepath.Dir(filename)
			fullPath := filepath.Join(dir, localPath)

			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				ml.addError(filename, lineNum, strings.Index(line, fullMatch), "link-check",
					fmt.Sprintf("Local file not found: %s", localPath), "Check file path and spelling")
			}
		}
	}
}

// checkCodeBlockStyle validates code block formatting
func (ml *MarkdownLinter) checkCodeBlockStyle(filename string, lineNum int, line, codeBlockType string) {
	if !ml.rules["code-block-style"] {
		return
	}

	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "```") {
		// Check for language specification
		if len(trimmed) == 3 {
			ml.addWarning(filename, lineNum, 1, "code-block-style",
				"Missing language specification in code block", "Specify language for syntax highlighting")
		} else if codeBlockType != "" {
			// Validate language specification
			lang := strings.TrimSpace(trimmed[3:])
			if strings.Contains(lang, " ") {
				ml.addWarning(filename, lineNum, 4, "code-block-style",
					"Invalid language specification", "Use single word language identifier")
			}
		}
	}
}

// checkListStyle validates list formatting
func (ml *MarkdownLinter) checkListStyle(filename string, lineNum int, line string) {
	if !ml.rules["list-style"] {
		return
	}

	trimmed := strings.TrimSpace(line)

	// Check unordered lists
	if matched, _ := regexp.MatchString(`^[*+-]\s`, trimmed); matched {
		// Check for consistent bullet style within document
		bulletChar := rune(trimmed[0])
		if bulletChar != '*' && bulletChar != '-' && bulletChar != '+' {
			ml.addWarning(filename, lineNum, 1, "list-style",
				"Inconsistent bullet style", "Use consistent bullet characters (* or - or +)")
		}
	}

	// Check ordered lists
	if matched, _ := regexp.MatchString(`^\d+\.\s`, trimmed); matched {
		// Extract number
		parts := strings.SplitN(trimmed, ".", 2)
		if len(parts) >= 1 {
			if num, err := strconv.Atoi(parts[0]); err == nil && num == 0 {
				ml.addWarning(filename, lineNum, 1, "list-style",
					"List item starts with 0", "List items should start with 1")
			}
		}
	}
}

// checkEmphasisStyle validates emphasis formatting
func (ml *MarkdownLinter) checkEmphasisStyle(filename string, lineNum int, line string) {
	if !ml.rules["emphasis-style"] {
		return
	}

	// Check for mixed emphasis styles
	if strings.Contains(line, "*") && strings.Contains(line, "_") {
		// This is a simple check - a more sophisticated implementation
		// would parse the emphasis properly
		ml.addWarning(filename, lineNum, 1, "emphasis-style",
			"Mixed emphasis styles", "Use consistent emphasis markers (* or _)")
	}
}

// addError adds an error to the linting results
func (ml *MarkdownLinter) addError(filename string, line, column int, rule, message, suggestion string) {
	ml.errors = append(ml.errors, LintIssue{
		File:       filename,
		Line:       line,
		Column:     column,
		Rule:       rule,
		Severity:   "error",
		Message:    message,
		Suggestion: suggestion,
	})
}

// addWarning adds a warning to the linting results
func (ml *MarkdownLinter) addWarning(filename string, line, column int, rule, message, suggestion string) {
	ml.warnings = append(ml.warnings, LintIssue{
		File:       filename,
		Line:       line,
		Column:     column,
		Rule:       rule,
		Severity:   "warning",
		Message:    message,
		Suggestion: suggestion,
	})
}

// PrintResults prints linting results
func (ml *MarkdownLinter) PrintResults() {
	if len(ml.errors) == 0 && len(ml.warnings) == 0 {
		fmt.Println("✅ No issues found")
		return
	}

	// Print errors
	for _, issue := range ml.errors {
		fmt.Printf("❌ %s:%d:%d [%s] %s\n",
			issue.File, issue.Line, issue.Column, issue.Rule, issue.Message)
		if issue.Suggestion != "" {
			fmt.Printf("   💡 %s\n", issue.Suggestion)
		}
	}

	// Print warnings
	for _, issue := range ml.warnings {
		fmt.Printf("⚠️  %s:%d:%d [%s] %s\n",
			issue.File, issue.Line, issue.Column, issue.Rule, issue.Message)
		if issue.Suggestion != "" {
			fmt.Printf("   💡 %s\n", issue.Suggestion)
		}
	}

	// Summary
	fmt.Printf("\n📊 Summary: %d error(s), %d warning(s)\n",
		len(ml.errors), len(ml.warnings))
}

// GetExitCode returns appropriate exit code based on results
func (ml *MarkdownLinter) GetExitCode() int {
	if len(ml.errors) > 0 {
		return 2 // Errors found
	}
	return 0 // No errors
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Markdown Linter - Markdown style and link checking

Usage: markdown-lint [options] <file1> [file2] ...

Options:
  -h, --help          Show this help message
  --disable RULE      Disable specific rule
  --list-rules        List available rules
  --config FILE       Use configuration file

Rules:
  heading-style       Check heading formatting
  line-length         Check line length (max 100 chars)
  no-trailing-spaces  Check for trailing whitespace
  no-multiple-blanks  Check for multiple blank lines
  link-check          Validate links and references
  code-block-style    Check code block formatting
  list-style          Check list formatting
  emphasis-style      Check emphasis consistency

Examples:
  markdown-lint README.md
  markdown-lint docs/*.md
  markdown-lint --disable line-length *.md

`)
}

func main() {
	linter := NewMarkdownLinter()
	var files []string

	// Parse command line arguments
	for i, arg := range os.Args[1:] {
		switch arg {
		case "-h", "--help":
			printUsage()
			os.Exit(0)
		case "--list-rules":
			fmt.Println("Available rules:")
			for rule := range linter.rules {
				fmt.Printf("  %s\n", rule)
			}
			os.Exit(0)
		case "--disable":
			if i+1 < len(os.Args[1:]) {
				rule := os.Args[i+2]
				if _, exists := linter.rules[rule]; exists {
					linter.rules[rule] = false
				} else {
					fmt.Fprintf(os.Stderr, "Unknown rule: %s\n", rule)
					os.Exit(1)
				}
			}
		default:
			if !strings.HasPrefix(arg, "--") && !strings.HasPrefix(arg, "-") {
				files = append(files, arg)
			}
		}
	}

	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "No files specified\n")
		printUsage()
		os.Exit(1)
	}

	// Lint each file
	for _, filename := range files {
		if err := linter.LintFile(filename); err != nil {
			fmt.Fprintf(os.Stderr, "Error linting %s: %v\n", filename, err)
			continue
		}
	}

	// Print results and exit
	linter.PrintResults()
	os.Exit(linter.GetExitCode())
}
