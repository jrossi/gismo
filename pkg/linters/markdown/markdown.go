package markdown

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jrossi/gismo/pkg/linters"

	"github.com/kaptinlin/jsonschema"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"go.abhg.dev/goldmark/frontmatter"

	markdown "github.com/teekennedy/goldmark-markdown"
)

// MarkdownLinter handles markdown file linting, formatting, and front matter validation
type MarkdownLinter struct {
	parser  goldmark.Markdown
	rules   []MarkdownRule
	schemas map[string]*jsonschema.Schema
	config  *MarkdownConfig
}

// MarkdownConfig represents markdown linter specific configuration
type MarkdownConfig struct {
	MaxLineLength      *int             `json:"maxLineLength,omitempty"`
	RequireFrontmatter *bool            `json:"requireFrontmatter,omitempty"`
	FrontmatterSchema  *json.RawMessage `json:"frontmatterSchema,omitempty"`
	DisabledRules      []string         `json:"disabledRules,omitempty"`
	MaxBlankLines      *int             `json:"maxBlankLines,omitempty"`
	ListIndentSize     *int             `json:"listIndentSize,omitempty"`
}

// MarkdownRule defines the interface for markdown linting rules
type MarkdownRule interface {
	Check(doc ast.Node, source []byte, filePath string) []linters.Issue
	Name() string
}

// NewMarkdownLinter creates a new markdown linter with default configuration
func NewMarkdownLinter() *MarkdownLinter {
	return NewMarkdownLinterWithConfig(nil)
}

// NewMarkdownLinterWithConfig creates a new markdown linter with the given configuration
func NewMarkdownLinterWithConfig(config *MarkdownConfig) *MarkdownLinter {
	// Create parser with front matter support
	frontmatterExtension := &frontmatter.Extender{}
	md := goldmark.New(
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithExtensions(
			frontmatterExtension,
		),
	)

	// Apply default configuration if not provided
	if config == nil {
		config = &MarkdownConfig{}
	}

	// Set defaults
	if config.MaxLineLength == nil {
		defaultLength := 120
		config.MaxLineLength = &defaultLength
	}
	if config.MaxBlankLines == nil {
		defaultBlankLines := 2
		config.MaxBlankLines = &defaultBlankLines
	}
	if config.ListIndentSize == nil {
		defaultIndentSize := 2
		config.ListIndentSize = &defaultIndentSize
	}
	if config.RequireFrontmatter == nil {
		defaultRequireFrontmatter := false
		config.RequireFrontmatter = &defaultRequireFrontmatter
	}

	// Initialize all linting rules
	rules := []MarkdownRule{
		&HeadingHierarchyRule{},
		&ListIndentationRule{IndentSize: *config.ListIndentSize},
		&CodeBlockRule{},
		&LineLengthRule{MaxLength: *config.MaxLineLength},
		&TrailingWhitespaceRule{},
		&EmphasisConsistencyRule{},
		&BlankLineRule{MaxBlankLines: *config.MaxBlankLines},
	}

	// Filter out disabled rules
	enabledRules := []MarkdownRule{}
	for _, rule := range rules {
		disabled := false
		for _, disabledRule := range config.DisabledRules {
			if rule.Name() == disabledRule {
				disabled = true
				break
			}
		}
		if !disabled {
			enabledRules = append(enabledRules, rule)
		}
	}

	return &MarkdownLinter{
		parser:  md,
		rules:   enabledRules,
		schemas: make(map[string]*jsonschema.Schema),
		config:  config,
	}
}

// SetConfig updates the linter configuration
func (l *MarkdownLinter) SetConfig(configData json.RawMessage) error {
	var config MarkdownConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return fmt.Errorf("failed to parse markdown config: %w", err)
	}

	// Update config and reinitialize rules
	l.config = &config

	// Set defaults if not provided
	if l.config.MaxLineLength == nil {
		defaultLength := 120
		l.config.MaxLineLength = &defaultLength
	}
	if l.config.MaxBlankLines == nil {
		defaultBlankLines := 2
		l.config.MaxBlankLines = &defaultBlankLines
	}
	if l.config.ListIndentSize == nil {
		defaultIndentSize := 2
		l.config.ListIndentSize = &defaultIndentSize
	}
	if l.config.RequireFrontmatter == nil {
		defaultRequireFrontmatter := false
		l.config.RequireFrontmatter = &defaultRequireFrontmatter
	}

	// Reinitialize rules with new config
	rules := []MarkdownRule{
		&HeadingHierarchyRule{},
		&ListIndentationRule{IndentSize: *l.config.ListIndentSize},
		&CodeBlockRule{},
		&LineLengthRule{MaxLength: *l.config.MaxLineLength},
		&TrailingWhitespaceRule{},
		&EmphasisConsistencyRule{},
		&BlankLineRule{MaxBlankLines: *l.config.MaxBlankLines},
	}

	// Filter out disabled rules
	l.rules = []MarkdownRule{}
	for _, rule := range rules {
		disabled := false
		for _, disabledRule := range l.config.DisabledRules {
			if rule.Name() == disabledRule {
				disabled = true
				break
			}
		}
		if !disabled {
			l.rules = append(l.rules, rule)
		}
	}

	return nil
}

// Name returns the linter name
func (l *MarkdownLinter) Name() string {
	return "markdown"
}

// CanHandle returns true for markdown files
func (l *MarkdownLinter) CanHandle(filePath string) bool {
	return strings.HasSuffix(filePath, ".md") || strings.HasSuffix(filePath, ".markdown")
}

// Lint performs comprehensive linting on a markdown file
func (l *MarkdownLinter) Lint(_ context.Context, filePath string, content []byte) (*linters.LintResult, error) {
	result := &linters.LintResult{
		Success: true,
		Issues:  []linters.Issue{},
	}

	// Parse markdown with front matter
	reader := text.NewReader(content)
	parserCtx := parser.NewContext()
	document := l.parser.Parser().Parse(reader, parser.WithContext(parserCtx))

	// Extract front matter if present
	frontMatterData := frontmatter.Get(parserCtx)
	if frontMatterData != nil {
		// Convert to our concrete FrontMatter type
		var fm FrontMatter
		if err := frontMatterData.Decode(&fm); err != nil {
			// If decode fails, just skip validation
			// This could happen with non-standard front matter
		} else {
			// Validate front matter against schema
			if schemaIssues := l.validateFrontMatter(filePath, &fm); len(schemaIssues) > 0 {
				result.Issues = append(result.Issues, schemaIssues...)
			}
		}
	} else if l.config != nil && l.config.RequireFrontmatter != nil && *l.config.RequireFrontmatter {
		// Front matter is required but not present
		result.Issues = append(result.Issues, linters.Issue{
			File:     filePath,
			Line:     1,
			Column:   1,
			Severity: "error",
			Message:  "Front matter is required but not present",
			Rule:     "require-frontmatter",
		})
	}

	// Apply all linting rules
	for _, rule := range l.rules {
		issues := rule.Check(document, content, filePath)
		result.Issues = append(result.Issues, issues...)
	}

	// Generate formatted output using a new renderer instance for thread safety
	var formatted bytes.Buffer
	formatter := markdown.NewRenderer()
	if err := formatter.Render(&formatted, content, document); err != nil {
		return nil, fmt.Errorf("failed to format markdown: %w", err)
	}
	result.Formatted = formatted.Bytes()

	// Check if formatting changed (indicates formatting issues)
	if !bytes.Equal(content, result.Formatted) {
		result.Issues = append(result.Issues, linters.Issue{
			File:     filePath,
			Line:     1,
			Column:   1,
			Severity: "warning",
			Message:  "File requires formatting to meet standards",
			Rule:     "formatting",
		})
	}

	// Determine overall success
	for _, issue := range result.Issues {
		if issue.Severity == "error" {
			result.Success = false
			break
		}
	}

	return result, nil
}

// FrontMatter represents structured front matter data
type FrontMatter struct {
	Title       string            `yaml:"title" json:"title"`
	Date        string            `yaml:"date" json:"date"`
	Tags        []string          `yaml:"tags" json:"tags"`
	Author      string            `yaml:"author" json:"author"`
	Description string            `yaml:"description" json:"description"`
	Metadata    map[string]string `yaml:",inline" json:"-"`
}

// loadSchema loads and compiles a JSON schema from either inline JSON or file path
func (l *MarkdownLinter) loadSchema(schemaData *json.RawMessage) (*jsonschema.Schema, error) {
	if schemaData == nil {
		return nil, fmt.Errorf("schema data is nil")
	}

	compiler := jsonschema.NewCompiler()

	// Try to detect if this is a file path (string) or inline schema (object)
	var schemaPath string
	if err := json.Unmarshal(*schemaData, &schemaPath); err == nil {
		// It's a string, treat as file path
		if !filepath.IsAbs(schemaPath) {
			// Make relative paths relative to current working directory
			wd, err := os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("failed to get working directory: %w", err)
			}
			schemaPath = filepath.Join(wd, schemaPath)
		}

		// Load schema from file
		schemaBytes, err := os.ReadFile(schemaPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
		}

		return compiler.Compile(schemaBytes)
	}

	// Not a string, treat as inline schema object
	return compiler.Compile([]byte(*schemaData))
}

// validateFrontMatter validates front matter against JSON schemas
func (l *MarkdownLinter) validateFrontMatter(format string, data *FrontMatter) []linters.Issue {
	if l.config == nil || l.config.FrontmatterSchema == nil {
		return []linters.Issue{}
	}

	schema, err := l.loadSchema(l.config.FrontmatterSchema)
	if err != nil {
		return []linters.Issue{{
			File:     "",
			Line:     1,
			Column:   1,
			Severity: "error",
			Message:  fmt.Sprintf("Failed to load frontmatter schema: %v", err),
			Rule:     "frontmatter-schema",
		}}
	}

	dataJSON, err := json.Marshal(data)
	if err != nil {
		return []linters.Issue{{
			File:     "",
			Line:     1,
			Column:   1,
			Severity: "error",
			Message:  fmt.Sprintf("Failed to marshal frontmatter data: %v", err),
			Rule:     "frontmatter-schema",
		}}
	}

	var frontmatterData map[string]interface{}
	if err := json.Unmarshal(dataJSON, &frontmatterData); err != nil {
		return []linters.Issue{{
			File:     "",
			Line:     1,
			Column:   1,
			Severity: "error",
			Message:  fmt.Sprintf("Failed to unmarshal frontmatter data: %v", err),
			Rule:     "frontmatter-schema",
		}}
	}

	if err := schema.Validate(frontmatterData); err != nil {
		return []linters.Issue{{
			File:     "",
			Line:     1,
			Column:   1,
			Severity: "error",
			Message:  fmt.Sprintf("Frontmatter validation failed: %v", err),
			Rule:     "frontmatter-schema",
		}}
	}

	return []linters.Issue{}
}

// HeadingHierarchyRule ensures proper heading level progression (H1 → H2 → H3)
type HeadingHierarchyRule struct{}

func (r *HeadingHierarchyRule) Name() string {
	return "heading-hierarchy"
}

func (r *HeadingHierarchyRule) Check(doc ast.Node, source []byte, filePath string) []linters.Issue {
	var issues []linters.Issue
	var lastLevel int
	var hasH1 bool

	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if heading, ok := node.(*ast.Heading); ok && entering {
			if heading.Level == 1 {
				if hasH1 {
					issues = append(issues, linters.Issue{
						File:     filePath,
						Line:     getLineNumber(source, heading),
						Column:   1,
						Severity: "warning",
						Message:  "Multiple H1 headings found, consider using H2 for subsequent sections",
						Rule:     r.Name(),
					})
				}
				hasH1 = true
			}

			if lastLevel > 0 && heading.Level > lastLevel+1 {
				issues = append(issues, linters.Issue{
					File:     filePath,
					Line:     getLineNumber(source, heading),
					Column:   1,
					Severity: "error",
					Message:  fmt.Sprintf("Heading level %d skips level %d (should not skip levels)", heading.Level, lastLevel+1),
					Rule:     r.Name(),
				})
			}
			lastLevel = heading.Level
		}
		return ast.WalkContinue, nil
	})

	return issues
}

// ListIndentationRule ensures consistent indentation for nested lists
type ListIndentationRule struct {
	IndentSize int
}

func (r *ListIndentationRule) Name() string {
	return "list-indentation"
}

func (r *ListIndentationRule) Check(_ ast.Node, source []byte, filePath string) []linters.Issue {
	var issues []linters.Issue
	lines := strings.Split(string(source), "\n")

	// Use configured indent size, default to 2 if not set
	indentSize := r.IndentSize
	if indentSize <= 0 {
		indentSize = 2
	}

	// Check list indentation using simple pattern matching
	listPattern := regexp.MustCompile(`^(\s*)[-*+]|\d+\.\s`)

	for i, line := range lines {
		if matches := listPattern.FindStringSubmatch(line); matches != nil {
			indent := len(matches[1])
			if indent%indentSize != 0 {
				issues = append(issues, linters.Issue{
					File:     filePath,
					Line:     i + 1,
					Column:   1,
					Severity: "warning",
					Message:  fmt.Sprintf("List items should use %d-space indentation for nesting", indentSize),
					Rule:     r.Name(),
				})
			}
		}
	}

	return issues
}

// CodeBlockRule ensures code blocks have language specifications
type CodeBlockRule struct{}

func (r *CodeBlockRule) Name() string {
	return "code-block-language"
}

func (r *CodeBlockRule) Check(doc ast.Node, source []byte, filePath string) []linters.Issue {
	var issues []linters.Issue

	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if codeBlock, ok := node.(*ast.FencedCodeBlock); ok && entering {
			if codeBlock.Info == nil || codeBlock.Info.Value(source) == nil || len(codeBlock.Info.Value(source)) == 0 {
				issues = append(issues, linters.Issue{
					File:     filePath,
					Line:     getLineNumber(source, codeBlock),
					Column:   1,
					Severity: "warning",
					Message:  "Code blocks should specify a language for syntax highlighting",
					Rule:     r.Name(),
				})
			}
		}
		return ast.WalkContinue, nil
	})

	return issues
}

// LineLengthRule checks for lines exceeding maximum length
type LineLengthRule struct {
	MaxLength int
}

func (r *LineLengthRule) Name() string {
	return "line-length"
}

func (r *LineLengthRule) Check(_ ast.Node, source []byte, filePath string) []linters.Issue {
	var issues []linters.Issue
	lines := strings.Split(string(source), "\n")

	for i, line := range lines {
		if len(line) > r.MaxLength {
			issues = append(issues, linters.Issue{
				File:     filePath,
				Line:     i + 1,
				Column:   r.MaxLength + 1,
				Severity: "warning",
				Message:  fmt.Sprintf("Line exceeds maximum length of %d characters (%d)", r.MaxLength, len(line)),
				Rule:     r.Name(),
			})
		}
	}

	return issues
}

// TrailingWhitespaceRule checks for trailing whitespace
type TrailingWhitespaceRule struct{}

func (r *TrailingWhitespaceRule) Name() string {
	return "trailing-whitespace"
}

func (r *TrailingWhitespaceRule) Check(_ ast.Node, source []byte, filePath string) []linters.Issue {
	var issues []linters.Issue
	lines := strings.Split(string(source), "\n")

	for i, line := range lines {
		if len(line) > 0 && (line[len(line)-1] == ' ' || line[len(line)-1] == '\t') {
			issues = append(issues, linters.Issue{
				File:     filePath,
				Line:     i + 1,
				Column:   len(line),
				Severity: "error",
				Message:  "Line has trailing whitespace",
				Rule:     r.Name(),
			})
		}
	}

	return issues
}

// EmphasisConsistencyRule ensures consistent emphasis markers
type EmphasisConsistencyRule struct{}

func (r *EmphasisConsistencyRule) Name() string {
	return "emphasis-consistency"
}

func (r *EmphasisConsistencyRule) Check(doc ast.Node, source []byte, filePath string) []linters.Issue {
	var issues []linters.Issue

	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if emphasis, ok := node.(*ast.Emphasis); ok && entering {
			// Check if using * for italic (preferred)
			line := getLineNumber(source, emphasis)
			if line > 0 {
				lines := strings.Split(string(source), "\n")
				if line <= len(lines) {
					lineText := lines[line-1]
					if strings.Contains(lineText, "_") && !strings.Contains(lineText, "*") {
						issues = append(issues, linters.Issue{
							File:     filePath,
							Line:     line,
							Column:   1,
							Severity: "info",
							Message:  "Prefer * for italic emphasis over _",
							Rule:     r.Name(),
						})
					}
				}
			}
		}
		return ast.WalkContinue, nil
	})

	return issues
}

// BlankLineRule ensures proper spacing around elements
type BlankLineRule struct {
	MaxBlankLines int
}

func (r *BlankLineRule) Name() string {
	return "blank-line-spacing"
}

func (r *BlankLineRule) Check(_ ast.Node, source []byte, filePath string) []linters.Issue {
	var issues []linters.Issue
	lines := strings.Split(string(source), "\n")

	// Use configured max blank lines, default to 2 if not set
	maxBlankLines := r.MaxBlankLines
	if maxBlankLines <= 0 {
		maxBlankLines = 2
	}

	// Check for excessive blank lines
	consecutiveBlank := 0
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			consecutiveBlank++
			if consecutiveBlank > maxBlankLines {
				issues = append(issues, linters.Issue{
					File:     filePath,
					Line:     i + 1,
					Column:   1,
					Severity: "warning",
					Message:  fmt.Sprintf("More than %d consecutive blank lines", maxBlankLines),
					Rule:     r.Name(),
				})
			}
		} else {
			consecutiveBlank = 0
		}
	}

	return issues
}

// getLineNumber calculates the line number for an AST node
func getLineNumber(source []byte, node ast.Node) int {
	// Try to get line information from different node types
	switch n := node.(type) {
	case *ast.Heading:
		if n.Lines().Len() > 0 {
			segment := n.Lines().At(0)
			lineCount := 1
			for i := 0; i < segment.Start && i < len(source); i++ {
				if source[i] == '\n' {
					lineCount++
				}
			}
			return lineCount
		}
		// Fallback: search for heading text
		lines := strings.Split(string(source), "\n")
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), strings.Repeat("#", n.Level)) {
				return i + 1
			}
		}
	case *ast.FencedCodeBlock:
		if n.Lines().Len() > 0 {
			segment := n.Lines().At(0)
			lineCount := 1
			for i := 0; i < segment.Start && i < len(source); i++ {
				if source[i] == '\n' {
					lineCount++
				}
			}
			return lineCount
		}
	case *ast.List:
		if n.Lines().Len() > 0 {
			segment := n.Lines().At(0)
			lineCount := 1
			for i := 0; i < segment.Start && i < len(source); i++ {
				if source[i] == '\n' {
					lineCount++
				}
			}
			return lineCount
		}
	}

	// For other nodes, try to find a parent with line info
	parent := node.Parent()
	if parent != nil {
		return getLineNumber(source, parent)
	}

	return 1
}
