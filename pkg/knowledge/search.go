package knowledge

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// SearchType represents the type of search to perform
type SearchType string

const (
	SearchTypeKeyword  SearchType = "keyword"
	SearchTypeSemantic SearchType = "semantic"
	SearchTypeHybrid   SearchType = "hybrid"
)

// SearchResult represents a single search result
type SearchResult struct {
	DocsetID   string  `json:"docset_id"`
	DocsetName string  `json:"docset_name"`
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	Path       string  `json:"path"`
	Summary    string  `json:"summary"`
	Score      float64 `json:"score"`
}

// SearchOptions contains options for search queries
type SearchOptions struct {
	Type        SearchType
	DocsetIDs   []string
	ContentType []string
	Limit       int
	Offset      int
}

// Search performs a search across docsets
func (s *Store) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 50
	}

	switch opts.Type {
	case SearchTypeKeyword:
		return s.keywordSearch(ctx, query, opts)
	case SearchTypeSemantic:
		return s.semanticSearch(ctx, query, opts)
	case SearchTypeHybrid:
		return s.hybridSearch(ctx, query, opts)
	default:
		return s.keywordSearch(ctx, query, opts)
	}
}

// keywordSearch performs keyword-based search
func (s *Store) keywordSearch(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	// Build the base query
	baseQuery := `
		SELECT 
			dc.docset_id,
			d.name as docset_name,
			dc.name,
			dc.type,
			dc.path,
			dc.summary,
			0.0 as score
		FROM docset_content dc
		JOIN docsets d ON dc.docset_id = d.id
		WHERE 1=1
	`

	args := []interface{}{}
	conditions := []string{}

	// Add search condition
	searchTerms := strings.Fields(strings.ToLower(query))
	if len(searchTerms) > 0 {
		searchConditions := []string{}
		for _, term := range searchTerms {
			searchConditions = append(searchConditions, "(LOWER(dc.name) LIKE ? OR LOWER(dc.content) LIKE ? OR LOWER(dc.summary) LIKE ?)")
			pattern := "%" + term + "%"
			args = append(args, pattern, pattern, pattern)
		}
		conditions = append(conditions, "("+strings.Join(searchConditions, " AND ")+")")
	}

	// Filter by docset IDs if specified
	if len(opts.DocsetIDs) > 0 {
		placeholders := make([]string, len(opts.DocsetIDs))
		for i, id := range opts.DocsetIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		conditions = append(conditions, "dc.docset_id IN ("+strings.Join(placeholders, ",")+")")
	}

	// Filter by content type if specified
	if len(opts.ContentType) > 0 {
		placeholders := make([]string, len(opts.ContentType))
		for i, ct := range opts.ContentType {
			placeholders[i] = "?"
			args = append(args, ct)
		}
		conditions = append(conditions, "dc.type IN ("+strings.Join(placeholders, ",")+")")
	}

	// Build final query
	if len(conditions) > 0 {
		baseQuery += " AND " + strings.Join(conditions, " AND ")
	}

	// Add ordering and pagination
	baseQuery += fmt.Sprintf(" ORDER BY dc.name LIMIT %d OFFSET %d", opts.Limit, opts.Offset)

	// Execute query
	rows, err := s.db.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer rows.Close()

	// Collect results
	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		err := rows.Scan(
			&result.DocsetID,
			&result.DocsetName,
			&result.Name,
			&result.Type,
			&result.Path,
			&result.Summary,
			&result.Score,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		results = append(results, result)
	}

	return results, rows.Err()
}

// semanticSearch performs vector similarity search
func (s *Store) semanticSearch(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	// For now, return an error as we need embedding generation
	// In production, this would:
	// 1. Generate embedding for the query
	// 2. Use DuckDB's vector similarity search
	// 3. Return ranked results
	return nil, fmt.Errorf("semantic search not yet implemented")
}

// hybridSearch combines keyword and semantic search
func (s *Store) hybridSearch(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	// For now, fall back to keyword search
	// In production, this would combine both approaches
	return s.keywordSearch(ctx, query, opts)
}

// GetContent retrieves the full content of a specific entry
func (s *Store) GetContent(ctx context.Context, docsetID, path string) (string, error) {
	var content sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT content 
		FROM docset_content 
		WHERE docset_id = ? AND path = ?
	`, docsetID, path).Scan(&content)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("content not found")
		}
		return "", fmt.Errorf("failed to get content: %w", err)
	}

	return content.String, nil
}

// CountSearchResults returns the total count for a search query
func (s *Store) CountSearchResults(ctx context.Context, query string, opts SearchOptions) (int, error) {
	// Similar to keywordSearch but with COUNT(*)
	baseQuery := `
		SELECT COUNT(*)
		FROM docset_content dc
		JOIN docsets d ON dc.docset_id = d.id
		WHERE 1=1
	`

	args := []interface{}{}
	conditions := []string{}

	// Add search condition
	searchTerms := strings.Fields(strings.ToLower(query))
	if len(searchTerms) > 0 {
		searchConditions := []string{}
		for _, term := range searchTerms {
			searchConditions = append(searchConditions, "(LOWER(dc.name) LIKE ? OR LOWER(dc.content) LIKE ? OR LOWER(dc.summary) LIKE ?)")
			pattern := "%" + term + "%"
			args = append(args, pattern, pattern, pattern)
		}
		conditions = append(conditions, "("+strings.Join(searchConditions, " AND ")+")")
	}

	// Filter by docset IDs if specified
	if len(opts.DocsetIDs) > 0 {
		placeholders := make([]string, len(opts.DocsetIDs))
		for i, id := range opts.DocsetIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		conditions = append(conditions, "dc.docset_id IN ("+strings.Join(placeholders, ",")+")")
	}

	// Filter by content type if specified
	if len(opts.ContentType) > 0 {
		placeholders := make([]string, len(opts.ContentType))
		for i, ct := range opts.ContentType {
			placeholders[i] = "?"
			args = append(args, ct)
		}
		conditions = append(conditions, "dc.type IN ("+strings.Join(placeholders, ",")+")")
	}

	// Build final query
	if len(conditions) > 0 {
		baseQuery += " AND " + strings.Join(conditions, " AND ")
	}

	var count int
	err := s.db.QueryRowContext(ctx, baseQuery, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count results: %w", err)
	}

	return count, nil
}
