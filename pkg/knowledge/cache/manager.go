package cache

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/anush008/fastembed-go"
	json "github.com/goccy/go-json"
	"github.com/jrossi/gismo/pkg/external/exa"
)

// ExaCacheManager manages the Exa search cache with embeddings
type ExaCacheManager struct {
	db       *sql.DB
	embedder *fastembed.FlagEmbedding
}

// NewExaCacheManager creates a new cache manager
func NewExaCacheManager(db *sql.DB) (*ExaCacheManager, error) {
	// Initialize the embedder with same settings as search engine
	options := &fastembed.InitOptions{
		Model:     fastembed.BGESmallEN,
		CacheDir:  ".fastembed_cache",
		MaxLength: 512,
	}

	embedder, err := fastembed.NewFlagEmbedding(options)
	if err != nil {
		// Cache can work without embeddings, just won't have semantic search
		return &ExaCacheManager{
			db:       db,
			embedder: nil,
		}, nil
	}

	return &ExaCacheManager{
		db:       db,
		embedder: embedder,
	}, nil
}

// FindSimilarQuery finds a semantically similar cached query
func (m *ExaCacheManager) FindSimilarQuery(ctx context.Context, query string, projectContext string, threshold float32) (*exa.CachedSearch, error) {
	if m.embedder == nil {
		// Fallback to exact match
		return m.findExactQuery(ctx, query, projectContext)
	}

	// Generate embedding for the query
	embeddings, err := m.embedder.Embed([]string{query}, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Convert embedding to DuckDB array format
	parts := make([]string, len(embeddings[0]))
	for i, v := range embeddings[0] {
		parts[i] = fmt.Sprintf("%f", v)
	}
	embeddingStr := "[" + strings.Join(parts, ", ") + "]"

	// Search for similar queries using cosine similarity
	query = `
		SELECT 
			id, query, search_type, results, created_at, 
			last_accessed, access_count, ttl_days,
			array_cosine_similarity(query_embedding, $1::REAL[]) as similarity
		FROM exa_search_cache
		WHERE project_context = $2
			AND query_embedding IS NOT NULL
			AND array_cosine_similarity(query_embedding, $1::REAL[]) >= $3
		ORDER BY similarity DESC
		LIMIT 1`

	var cached exa.CachedSearch
	var resultsJSON interface{} // DuckDB returns JSON as interface{}
	var similarity float32

	err = m.db.QueryRowContext(ctx, query, embeddingStr, projectContext, threshold).Scan(
		&cached.ID,
		&cached.Query,
		&cached.SearchType,
		&resultsJSON,
		&cached.CreatedAt,
		&cached.LastAccessed,
		&cached.AccessCount,
		&cached.TTLDays,
		&similarity,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find similar query: %w", err)
	}

	// Parse results JSON - handle both string and []interface{} types
	var resultsBytes []byte
	switch v := resultsJSON.(type) {
	case string:
		resultsBytes = []byte(v)
	case []byte:
		resultsBytes = v
	default:
		// DuckDB returns JSON as already parsed interface{}, so re-marshal it
		var err error
		resultsBytes, err = json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal cached results: %w", err)
		}
	}

	if err := json.Unmarshal(resultsBytes, &cached.Results); err != nil {
		return nil, fmt.Errorf("failed to parse cached results: %w", err)
	}

	// Update access count and timestamp
	m.updateAccessInfo(ctx, cached.ID)

	return &cached, nil
}

// StoreSearch stores a search with embeddings for query and results
func (m *ExaCacheManager) StoreSearch(ctx context.Context, query string, searchType string, results []exa.SearchResult, projectContext string) error {
	// Generate unique ID for this search
	hash := sha256.Sum256([]byte(query + projectContext + time.Now().String()))
	searchID := fmt.Sprintf("%x", hash[:8])

	// Generate embedding for the query
	var embeddingStr string
	if m.embedder != nil {
		embeddings, err := m.embedder.Embed([]string{query}, 1)
		if err == nil && len(embeddings) > 0 {
			parts := make([]string, len(embeddings[0]))
			for i, v := range embeddings[0] {
				parts[i] = fmt.Sprintf("%f", v)
			}
			embeddingStr = "[" + strings.Join(parts, ", ") + "]"
		}
	}

	// Convert results to JSON
	resultsJSON, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	// Insert into cache
	var insertQuery string
	var args []interface{}

	if embeddingStr != "" {
		insertQuery = `
			INSERT INTO exa_search_cache 
			(id, query, query_embedding, search_type, results, project_context, created_at, last_accessed, access_count, ttl_days)
			VALUES ($1, $2, $3::REAL[], $4, $5, $6, NOW(), NOW(), 1, 7)`
		args = []interface{}{searchID, query, embeddingStr, searchType, string(resultsJSON), projectContext}
	} else {
		insertQuery = `
			INSERT INTO exa_search_cache 
			(id, query, search_type, results, project_context, created_at, last_accessed, access_count, ttl_days)
			VALUES ($1, $2, $3, $4, $5, NOW(), NOW(), 1, 7)`
		args = []interface{}{searchID, query, searchType, string(resultsJSON), projectContext}
	}

	_, err = m.db.ExecContext(ctx, insertQuery, args...)
	if err != nil {
		return fmt.Errorf("failed to store search: %w", err)
	}

	// Store individual results with embeddings
	if m.embedder != nil {
		for _, result := range results {
			m.storeResultWithEmbedding(ctx, searchID, result)
		}
	}

	return nil
}

// ProvideFeedback updates the usefulness score for a search
func (m *ExaCacheManager) ProvideFeedback(ctx context.Context, searchID string, score float32, feedbackType string) error {
	// Insert feedback
	insertQuery := `
		INSERT INTO exa_feedback (search_id, usefulness_score, feedback_type, created_at)
		VALUES ($1, $2, $3, NOW())`

	_, err := m.db.ExecContext(ctx, insertQuery, searchID, score, feedbackType)
	if err != nil {
		return fmt.Errorf("failed to store feedback: %w", err)
	}

	// Update TTL based on feedback
	if score > 0.7 {
		// Extend TTL for highly useful searches
		updateTTL := `
			UPDATE exa_search_cache 
			SET ttl_days = LEAST(ttl_days * 2, 30)
			WHERE id = $1`
		_, _ = m.db.ExecContext(ctx, updateTTL, searchID)
	}

	return nil
}

// RunEviction removes expired cache entries
func (m *ExaCacheManager) RunEviction(ctx context.Context) error {
	// Delete searches that have exceeded their TTL
	query := `
		DELETE FROM exa_search_cache
		WHERE created_at + INTERVAL '1 day' * ttl_days < NOW()`

	result, err := m.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to run eviction: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected > 0 {
		// Also clean up related tables
		_, _ = m.db.ExecContext(ctx, `DELETE FROM exa_feedback WHERE search_id NOT IN (SELECT id FROM exa_search_cache)`)
		_, _ = m.db.ExecContext(ctx, `DELETE FROM exa_search_results WHERE search_id NOT IN (SELECT id FROM exa_search_cache)`)
	}

	return nil
}

// GetCachedSearches retrieves recent cached searches for a project
func (m *ExaCacheManager) GetCachedSearches(ctx context.Context, projectContext string, limit int) ([]exa.CachedSearch, error) {
	query := `
		SELECT id, query, search_type, results, created_at, last_accessed, access_count, ttl_days
		FROM exa_search_cache
		WHERE project_context = $1
		ORDER BY last_accessed DESC
		LIMIT $2`

	rows, err := m.db.QueryContext(ctx, query, projectContext, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get cached searches: %w", err)
	}
	defer rows.Close()

	var searches []exa.CachedSearch
	for rows.Next() {
		var cached exa.CachedSearch
		var resultsJSON interface{}

		err := rows.Scan(
			&cached.ID,
			&cached.Query,
			&cached.SearchType,
			&resultsJSON,
			&cached.CreatedAt,
			&cached.LastAccessed,
			&cached.AccessCount,
			&cached.TTLDays,
		)
		if err != nil {
			continue
		}

		// Parse results JSON - handle both string and []interface{} types
		var resultsBytes []byte
		switch v := resultsJSON.(type) {
		case string:
			resultsBytes = []byte(v)
		case []byte:
			resultsBytes = v
		default:
			// DuckDB returns JSON as already parsed interface{}, so re-marshal it
			resultsBytes, _ = json.Marshal(v)
		}

		if err := json.Unmarshal(resultsBytes, &cached.Results); err == nil {
			cached.ProjectContext = projectContext
			searches = append(searches, cached)
		}
	}

	return searches, nil
}

// Helper methods

func (m *ExaCacheManager) findExactQuery(ctx context.Context, query string, projectContext string) (*exa.CachedSearch, error) {
	sqlQuery := `
		SELECT id, query, search_type, results, created_at, last_accessed, access_count, ttl_days
		FROM exa_search_cache
		WHERE query = $1 AND project_context = $2
		LIMIT 1`

	var cached exa.CachedSearch
	var resultsJSON interface{}

	err := m.db.QueryRowContext(ctx, sqlQuery, query, projectContext).Scan(
		&cached.ID,
		&cached.Query,
		&cached.SearchType,
		&resultsJSON,
		&cached.CreatedAt,
		&cached.LastAccessed,
		&cached.AccessCount,
		&cached.TTLDays,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Parse results JSON - handle both string and []interface{} types
	var resultsBytes []byte
	switch v := resultsJSON.(type) {
	case string:
		resultsBytes = []byte(v)
	case []byte:
		resultsBytes = v
	default:
		// DuckDB returns JSON as already parsed interface{}, so re-marshal it
		resultsBytes, _ = json.Marshal(v)
	}

	if err := json.Unmarshal(resultsBytes, &cached.Results); err != nil {
		return nil, err
	}

	m.updateAccessInfo(ctx, cached.ID)
	return &cached, nil
}

func (m *ExaCacheManager) updateAccessInfo(ctx context.Context, searchID string) {
	query := `
		UPDATE exa_search_cache 
		SET last_accessed = NOW(), access_count = access_count + 1
		WHERE id = $1`
	_, _ = m.db.ExecContext(ctx, query, searchID)
}

func (m *ExaCacheManager) storeResultWithEmbedding(ctx context.Context, searchID string, result exa.SearchResult) {
	// Combine title and snippet for embedding
	text := result.Title + " " + result.Summary
	if text == "" {
		text = result.Text
	}

	var embeddingStr string
	if m.embedder != nil && text != "" {
		embeddings, err := m.embedder.Embed([]string{text}, 1)
		if err == nil && len(embeddings) > 0 {
			parts := make([]string, len(embeddings[0]))
			for i, v := range embeddings[0] {
				parts[i] = fmt.Sprintf("%f", v)
			}
			embeddingStr = "[" + strings.Join(parts, ", ") + "]"
		}
	}

	metadata, _ := json.Marshal(map[string]interface{}{
		"author":        result.Author,
		"publishedDate": result.PublishedDate,
		"score":         result.Score,
	})

	query := `
		INSERT INTO exa_search_results 
		(search_id, url, title, snippet, content, content_embedding, metadata)
		VALUES ($1, $2, $3, $4, $5, $6::REAL[], $7)`

	_, _ = m.db.ExecContext(ctx, query,
		searchID,
		result.URL,
		result.Title,
		result.Summary,
		result.Text,
		embeddingStr,
		string(metadata),
	)
}
