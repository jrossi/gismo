package docset

import (
	"context"
	"fmt"
	"strings"
)

// Indexer handles generating embeddings and indexing docset content
type Indexer struct {
	// In a real implementation, this would contain an embedding model
}

// NewIndexer creates a new indexer
func NewIndexer() *Indexer {
	return &Indexer{}
}

// GenerateEmbedding generates an embedding for the given text
func (i *Indexer) GenerateEmbedding(text string) ([]float32, error) {
	// Placeholder implementation
	// In production, this would use fastembed or similar

	// For now, return a simple hash-based "embedding"
	// This is just for demonstration - real embeddings would be 384-1536 dimensions
	embedding := make([]float32, 384)

	// Simple hash function to generate deterministic values
	hash := 0
	for _, ch := range text {
		hash = (hash*31 + int(ch)) % 1000000
	}

	// Fill embedding with pseudo-random values based on hash
	for i := range embedding {
		embedding[i] = float32((hash+i)%1000) / 1000.0
	}

	return embedding, nil
}

// ExtractKeywords extracts keywords from text for keyword search
func (i *Indexer) ExtractKeywords(text string) []string {
	// Simple keyword extraction
	// In production, use proper NLP techniques

	// Convert to lowercase
	text = strings.ToLower(text)

	// Remove common stop words
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "from": true,
		"is": true, "was": true, "are": true, "were": true, "been": true,
		"be": true, "have": true, "has": true, "had": true, "do": true,
		"does": true, "did": true, "will": true, "would": true, "should": true,
		"could": true, "may": true, "might": true, "must": true, "shall": true,
		"can": true, "need": true, "this": true, "that": true, "these": true,
		"those": true, "i": true, "you": true, "he": true, "she": true,
		"it": true, "we": true, "they": true, "them": true, "their": true,
	}

	// Split into words
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
	})

	// Filter and deduplicate
	keywordMap := make(map[string]bool)
	for _, word := range words {
		if len(word) > 2 && !stopWords[word] {
			keywordMap[word] = true
		}
	}

	// Convert to slice
	keywords := make([]string, 0, len(keywordMap))
	for keyword := range keywordMap {
		keywords = append(keywords, keyword)
	}

	return keywords
}

// ProcessContent processes content for indexing
func (i *Indexer) ProcessContent(ctx context.Context, content string, contentType string) (ProcessedContent, error) {
	// Strip HTML if needed
	processedText := content
	if strings.Contains(contentType, "html") || strings.Contains(content, "<html") {
		processedText = stripHTML(content)
	}

	// Generate embedding
	embedding, err := i.GenerateEmbedding(processedText)
	if err != nil {
		return ProcessedContent{}, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Extract keywords
	keywords := i.ExtractKeywords(processedText)

	// Generate summary
	summary := processedText
	if len(summary) > 200 {
		summary = summary[:200] + "..."
	}

	return ProcessedContent{
		CleanText: processedText,
		Summary:   summary,
		Keywords:  keywords,
		Embedding: embedding,
	}, nil
}

// ProcessedContent represents the result of content processing
type ProcessedContent struct {
	CleanText string
	Summary   string
	Keywords  []string
	Embedding []float32
}
