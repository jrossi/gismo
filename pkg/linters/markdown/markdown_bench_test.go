package markdown

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func BenchmarkMarkdownLinter_SmallDocument(b *testing.B) {
	linter := NewMarkdownLinter()
	content := []byte(`# Small Document

This is a small test document.

## Section

- Item 1
- Item 2

` + "```go" + `
fmt.Println("Hello")
` + "```")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := linter.Lint(context.Background(), "small.md", content)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarkdownLinter_MediumDocument(b *testing.B) {
	linter := NewMarkdownLinter()

	// Create medium-sized document (around 5KB)
	var builder strings.Builder
	builder.WriteString("# Medium Document\n\n")

	for i := 0; i < 50; i++ {
		builder.WriteString(fmt.Sprintf("## Section %d\n\n", i+1))
		builder.WriteString("This is some content for the section.\n\n")
		builder.WriteString("- List item 1\n")
		builder.WriteString("  - Nested item\n")
		builder.WriteString("- List item 2\n\n")
		builder.WriteString("```go\n")
		builder.WriteString(fmt.Sprintf("// Example code %d\n", i+1))
		builder.WriteString("fmt.Printf(\"Section %d\\n\")\n")
		builder.WriteString("```\n\n")
	}

	content := []byte(builder.String())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := linter.Lint(context.Background(), "medium.md", content)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarkdownLinter_LargeDocument(b *testing.B) {
	linter := NewMarkdownLinter()

	// Create large document (around 50KB)
	var builder strings.Builder
	builder.WriteString("# Large Document\n\n")

	for i := 0; i < 500; i++ {
		builder.WriteString(fmt.Sprintf("## Section %d\n\n", i+1))
		builder.WriteString("This is some content for the section with more detailed text.\n\n")
		builder.WriteString("### Subsection\n\n")
		builder.WriteString("- List item 1 with longer description\n")
		builder.WriteString("  - Nested item with details\n")
		builder.WriteString("  - Another nested item\n")
		builder.WriteString("- List item 2 with more content\n\n")
		builder.WriteString("```go\n")
		builder.WriteString(fmt.Sprintf("// Example code %d\n", i+1))
		builder.WriteString("package main\n\n")
		builder.WriteString("func main() {\n")
		builder.WriteString(fmt.Sprintf("    fmt.Printf(\"Section %d\\n\", %d)\n", i+1, i+1))
		builder.WriteString("}\n")
		builder.WriteString("```\n\n")
		builder.WriteString("Some additional content with **bold** and *italic* text.\n\n")
	}

	content := []byte(builder.String())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := linter.Lint(context.Background(), "large.md", content)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarkdownLinter_DocumentWithManyIssues(b *testing.B) {
	linter := NewMarkdownLinter()

	// Create document with many linting issues
	var builder strings.Builder
	builder.WriteString("# Document With Issues  \n\n") // trailing space

	for i := 0; i < 100; i++ {
		builder.WriteString(fmt.Sprintf("##### Section %d\n\n", i+1)) // skipped levels
		builder.WriteString("Content with trailing spaces.   \n\n")   // trailing spaces
		builder.WriteString("- Item 1\n")
		builder.WriteString("   - Bad indentation\n") // 3 spaces
		builder.WriteString("- Item 2\n\n")
		builder.WriteString("```\n") // no language
		builder.WriteString("code without language specification\n")
		builder.WriteString("```\n\n")
		builder.WriteString("This line is extremely long and exceeds our 120 character limit by being way too verbose and continuing past what is reasonable.\n\n") // long line
	}

	content := []byte(builder.String())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := linter.Lint(context.Background(), "issues.md", content)
		if err != nil {
			b.Fatal(err)
		}
		// Verify we found issues
		if len(result.Issues) == 0 {
			b.Fatal("Expected to find issues")
		}
	}
}

func BenchmarkMarkdownLinter_ConcurrentSmall(b *testing.B) {
	linter := NewMarkdownLinter()
	content := []byte(`# Concurrent Test

Small document for concurrent testing.

## Section

- Item 1
- Item 2`)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := linter.Lint(context.Background(), "concurrent.md", content)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkMarkdownLinter_Rules_HeadingHierarchy(b *testing.B) {
	linter := NewMarkdownLinter()
	content := []byte(`# Title
## Section
### Subsection
#### Details
##### Deep
###### Deeper
# Another Title
## Another Section
### Another Sub`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := linter.Lint(context.Background(), "headings.md", content)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarkdownLinter_Rules_ListIndentation(b *testing.B) {
	linter := NewMarkdownLinter()

	var builder strings.Builder
	for i := 0; i < 200; i++ {
		builder.WriteString("- Item 1\n")
		builder.WriteString("  - Nested item\n")
		builder.WriteString("    - Double nested\n")
		builder.WriteString("- Item 2\n")
		builder.WriteString("   - Wrong indentation\n") // 3 spaces
		builder.WriteString("- Item 3\n\n")
	}

	content := []byte(builder.String())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := linter.Lint(context.Background(), "lists.md", content)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarkdownLinter_Rules_CodeBlocks(b *testing.B) {
	linter := NewMarkdownLinter()

	var builder strings.Builder
	for i := 0; i < 100; i++ {
		builder.WriteString("```go\n")
		builder.WriteString("fmt.Println(\"with language\")\n")
		builder.WriteString("```\n\n")
		builder.WriteString("```\n") // without language
		builder.WriteString("code without language\n")
		builder.WriteString("```\n\n")
	}

	content := []byte(builder.String())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := linter.Lint(context.Background(), "code.md", content)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarkdownLinter_Rules_LineLength(b *testing.B) {
	linter := NewMarkdownLinter()

	var builder strings.Builder
	for i := 0; i < 100; i++ {
		builder.WriteString("Short line\n")
		builder.WriteString("This is a medium length line that should be fine.\n")
		builder.WriteString("This is an extremely long line that definitely exceeds our 120 character limit and should trigger a line length warning from our linter.\n")
		builder.WriteString("Another short line\n\n")
	}

	content := []byte(builder.String())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := linter.Lint(context.Background(), "length.md", content)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarkdownLinter_Rules_TrailingWhitespace(b *testing.B) {
	linter := NewMarkdownLinter()

	var builder strings.Builder
	for i := 0; i < 500; i++ {
		builder.WriteString("Line without trailing spaces\n")
		builder.WriteString("Line with trailing spaces   \n")
		builder.WriteString("Line with tab\t\n")
		builder.WriteString("Clean line\n")
	}

	content := []byte(builder.String())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := linter.Lint(context.Background(), "whitespace.md", content)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarkdownLinter_NewLinter(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewMarkdownLinter()
	}
}

func BenchmarkMarkdownLinter_CanHandle(b *testing.B) {
	linter := NewMarkdownLinter()

	testPaths := []string{
		"test.md",
		"README.md",
		"docs/api.md",
		"test.go",
		"config.json",
		"script.sh",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, path := range testPaths {
			_ = linter.CanHandle(path)
		}
	}
}

func BenchmarkMarkdownLinter_Memory(b *testing.B) {
	// Test memory allocation patterns
	linter := NewMarkdownLinter()
	content := []byte(`# Memory Test

This is a test for memory allocation patterns.

## Section

- Item 1
  - Nested
- Item 2

` + "```go" + `
fmt.Println("test")
` + "```")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		result, err := linter.Lint(context.Background(), "memory.md", content)
		if err != nil {
			b.Fatal(err)
		}
		// Access result to prevent optimization
		_ = len(result.Issues)
	}
}
