package docset

import "strings"

// stripHTML removes HTML tags from text (basic implementation)
func stripHTML(html string) string {
	// Very basic HTML stripping
	// In production, use a proper HTML parser
	inTag := false
	var result strings.Builder

	for _, ch := range html {
		if ch == '<' {
			inTag = true
		} else if ch == '>' {
			inTag = false
		} else if !inTag {
			result.WriteRune(ch)
		}
	}

	// Clean up whitespace
	text := result.String()
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	text = strings.ReplaceAll(text, "  ", " ")

	return text
}
