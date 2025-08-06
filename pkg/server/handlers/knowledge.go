package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jrossi/gismo/pkg/docset"
	"github.com/jrossi/gismo/pkg/external/exa"
	gismov1 "github.com/jrossi/gismo/pkg/generated/gismo/v1"
	"github.com/jrossi/gismo/pkg/knowledge"
	"github.com/jrossi/gismo/pkg/knowledge/cache"
)

// KnowledgeHandler implements the KnowledgeService gRPC service
type KnowledgeHandler struct {
	gismov1.UnimplementedKnowledgeServiceServer
	db *sql.DB
}

// NewKnowledgeHandler creates a new knowledge handler
func NewKnowledgeHandler(db *sql.DB) *KnowledgeHandler {
	return &KnowledgeHandler{
		db: db,
	}
}

// NewKnowledgeHandlerFromDB is an alias for backward compatibility
func NewKnowledgeHandlerFromDB(db *sql.DB) *KnowledgeHandler {
	return NewKnowledgeHandler(db)
}

// logProjectContext logs the project context from a request
func logProjectContext(method string, ctx *gismov1.ProjectContext) {
	if ctx == nil {
		log.Printf("[%s] No project context provided", method)
		return
	}
	if ctx.ProjectPath != "" || ctx.ProjectName != "" {
		log.Printf("[%s] Project: path=%s, name=%s, claude_id=%s",
			method, ctx.ProjectPath, ctx.ProjectName, ctx.ClaudeProjectId)
	}
}

// ImportDocset handles importing a docset from URL or local path
func (h *KnowledgeHandler) ImportDocset(req *gismov1.ImportDocsetRequest, stream gismov1.KnowledgeService_ImportDocsetServer) error {
	ctx := stream.Context()
	logProjectContext("ImportDocset", req.Context)

	// Extract URL from request
	var url string
	switch source := req.Source.(type) {
	case *gismov1.ImportDocsetRequest_Url:
		url = source.Url
	case *gismov1.ImportDocsetRequest_LocalPath:
		// TODO: Support local path imports
		return status.Error(codes.Unimplemented, "local path imports not yet implemented")
	default:
		return status.Error(codes.InvalidArgument, "source URL or path required")
	}

	// Send initial progress
	if err := stream.Send(&gismov1.ImportProgress{
		Stage:   gismov1.ImportProgress_DOWNLOADING,
		Message: "Starting docset download",
	}); err != nil {
		return status.Errorf(codes.Internal, "failed to send progress: %v", err)
	}

	// Create downloader
	downloader, err := docset.NewDownloader()
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create downloader: %v", err)
	}

	// Download docset with progress
	docsetPath, err := downloader.Download(ctx, url, func(percent int, message string) {
		// Ensure percent is within bounds for int32
		if percent < 0 {
			percent = 0
		} else if percent > 200 { // Max 200 since we divide by 2
			percent = 200
		}
		_ = stream.Send(&gismov1.ImportProgress{
			Stage:           gismov1.ImportProgress_DOWNLOADING,
			Message:         message,
			ProgressPercent: int32(percent / 2), // #nosec G115 - bounded above
		})
	})
	if err != nil {
		_ = stream.Send(&gismov1.ImportProgress{
			Stage:   gismov1.ImportProgress_ERROR,
			Message: fmt.Sprintf("Download failed: %v", err),
			Error:   err.Error(),
		})
		return status.Errorf(codes.Internal, "download failed: %v", err)
	}

	// Send parsing progress
	if err := stream.Send(&gismov1.ImportProgress{
		Stage:           gismov1.ImportProgress_PARSING,
		Message:         "Parsing docset",
		ProgressPercent: 50,
	}); err != nil {
		return status.Errorf(codes.Internal, "failed to send progress: %v", err)
	}

	// Create a knowledge store with our existing database connection
	store := knowledge.NewWithDB(h.db)

	// Create importer with the store
	importer := docset.NewImporter(store)

	// Import docset with progress
	sourceType := "official"
	if req.Metadata != nil && req.Metadata["source_type"] != "" {
		sourceType = req.Metadata["source_type"]
	}

	err = importer.Import(ctx, docsetPath, url, sourceType, func(p docset.ImportProgress) {
		stage := gismov1.ImportProgress_IMPORTING
		if strings.Contains(p.Message, "index") {
			stage = gismov1.ImportProgress_INDEXING
		}

		// Calculate percent with bounds checking
		percent := 50
		if p.Total > 0 {
			percent = 50 + (p.Current*50)/max(p.Total, 1)
			if percent > 100 {
				percent = 100
			}
		}

		// Ensure values fit in int32
		current := p.Current
		if current > 2147483647 {
			current = 2147483647
		}
		total := p.Total
		if total > 2147483647 {
			total = 2147483647
		}

		_ = stream.Send(&gismov1.ImportProgress{
			Stage:           stage,
			Message:         p.Message,
			ProgressPercent: int32(percent), // #nosec G115 - bounded above
			ItemsProcessed:  int32(current), // #nosec G115 - bounded above
			ItemsTotal:      int32(total),   // #nosec G115 - bounded above
		})
	})

	if err != nil {
		_ = stream.Send(&gismov1.ImportProgress{
			Stage:   gismov1.ImportProgress_ERROR,
			Message: fmt.Sprintf("Import failed: %v", err),
			Error:   err.Error(),
		})
		return status.Errorf(codes.Internal, "import failed: %v", err)
	}

	// Send completion
	if err := stream.Send(&gismov1.ImportProgress{
		Stage:           gismov1.ImportProgress_COMPLETE,
		Message:         "Import completed successfully",
		ProgressPercent: 100,
	}); err != nil {
		return status.Errorf(codes.Internal, "failed to send completion: %v", err)
	}

	return nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ListDocsets returns a list of imported docsets
func (h *KnowledgeHandler) ListDocsets(ctx context.Context, req *gismov1.ListDocsetsRequest) (*gismov1.ListDocsetsResponse, error) {
	logProjectContext("ListDocsets", req.Context)
	query := `
		SELECT id, name, version, language, source_url, source_type, imported_at, metadata,
		       (SELECT COUNT(*) FROM docset_content WHERE docset_id = docsets.id) as content_count
		FROM docsets
		ORDER BY imported_at DESC
	`

	rows, err := h.db.QueryContext(ctx, query)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to query docsets: %v", err)
	}
	defer rows.Close()

	var docsets []*gismov1.Docset
	for rows.Next() {
		var d gismov1.Docset
		var importedAt time.Time
		var metadataRaw interface{}

		err := rows.Scan(
			&d.Id,
			&d.Name,
			&d.Version,
			&d.Language,
			&d.SourceUrl,
			&d.SourceType,
			&importedAt,
			&metadataRaw,
			&d.ContentCount,
		)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to scan docset: %v", err)
		}

		d.ImportedAt = timestamppb.New(importedAt)

		// Handle metadata - DuckDB returns it as a map[string]interface{}
		if metadataRaw != nil {
			switch m := metadataRaw.(type) {
			case map[string]interface{}:
				d.Metadata = make(map[string]string)
				for k, v := range m {
					if str, ok := v.(string); ok {
						d.Metadata[k] = str
					} else if v != nil {
						d.Metadata[k] = fmt.Sprintf("%v", v)
					}
				}
			case string:
				// Try to parse as JSON string
				var metadata map[string]string
				if err := json.Unmarshal([]byte(m), &metadata); err == nil {
					d.Metadata = metadata
				}
			}
		}

		docsets = append(docsets, &d)
	}

	if err := rows.Err(); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to iterate docsets: %v", err)
	}

	return &gismov1.ListDocsetsResponse{
		Docsets: docsets,
	}, nil
}

// RemoveDocset removes an imported docset
func (h *KnowledgeHandler) RemoveDocset(ctx context.Context, req *gismov1.RemoveDocsetRequest) (*gismov1.RemoveDocsetResponse, error) {
	logProjectContext("RemoveDocset", req.Context)
	if req.DocsetId == "" {
		return nil, status.Error(codes.InvalidArgument, "docset_id is required")
	}

	// TODO: Implement actual removal from database
	return &gismov1.RemoveDocsetResponse{
		Success: true,
		Message: "Docset removed (stub)",
	}, nil
}

// Search performs a search across docsets
func (h *KnowledgeHandler) Search(ctx context.Context, req *gismov1.SearchRequest) (*gismov1.SearchResponse, error) {
	logProjectContext("Search", req.Context)
	if req.Query == "" {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}

	// Build the base query based on search type
	var query string
	var args []interface{}

	// For now, implement keyword search (semantic search would require embeddings)
	switch req.Type {
	case gismov1.SearchRequest_SEMANTIC, gismov1.SearchRequest_HYBRID:
		// For semantic search, we'd need to generate embeddings and use vector similarity
		// For now, fall back to keyword search
		fallthrough
	default: // KEYWORD search
		query = `
			SELECT 
				dc.id,
				dc.docset_id,
				d.name as docset_name,
				dc.name as item_name,
				dc.type as item_type,
				dc.path,
				COALESCE(dc.summary, SUBSTRING(dc.content, 1, 200)) as content_preview,
				CAST(0.0 AS REAL) as relevance_score
			FROM docset_content dc
			JOIN docsets d ON dc.docset_id = d.id
			WHERE 1=1
		`

		// Add search condition
		searchPattern := "%" + strings.ToLower(req.Query) + "%"
		query += ` AND (
			LOWER(dc.name) LIKE ? OR 
			LOWER(dc.type) LIKE ? OR 
			LOWER(dc.content) LIKE ? OR
			LOWER(dc.summary) LIKE ?
		)`
		args = append(args, searchPattern, searchPattern, searchPattern, searchPattern)

		// Filter by docsets if specified
		if len(req.DocsetIds) > 0 {
			placeholders := make([]string, len(req.DocsetIds))
			for i, id := range req.DocsetIds {
				placeholders[i] = "?"
				args = append(args, id)
			}
			query += fmt.Sprintf(" AND dc.docset_id IN (%s)", strings.Join(placeholders, ","))
		}

		// Apply limit
		limit := req.Limit
		if limit <= 0 || limit > 100 {
			limit = 20
		}
		query += " LIMIT ?"
		args = append(args, limit)
	}

	// Execute query
	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search failed: %v", err)
	}
	defer rows.Close()

	var results []*gismov1.SearchResult
	for rows.Next() {
		var r gismov1.SearchResult
		var contentID int32
		err := rows.Scan(
			&contentID,
			&r.DocsetId,
			&r.DocsetName,
			&r.ItemName,
			&r.ItemType,
			&r.Path,
			&r.ContentPreview,
			&r.RelevanceScore,
		)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to scan result: %v", err)
		}
		r.ContentId = contentID
		results = append(results, &r)
	}

	if err := rows.Err(); err != nil {
		return nil, status.Errorf(codes.Internal, "search iteration failed: %v", err)
	}

	// Ensure count fits in int32
	totalCount := len(results)
	if totalCount > 2147483647 {
		totalCount = 2147483647
	}

	return &gismov1.SearchResponse{
		Results:         results,
		TotalCount:      int32(totalCount), // #nosec G115 - bounded above
		ExecutionTimeMs: 0,                 // Could track actual execution time
	}, nil
}

// GetContent retrieves content by ID
func (h *KnowledgeHandler) GetContent(ctx context.Context, req *gismov1.GetContentRequest) (*gismov1.GetContentResponse, error) {
	logProjectContext("GetContent", req.Context)
	if req.ContentId == 0 {
		return nil, status.Error(codes.InvalidArgument, "content_id is required")
	}

	query := `
		SELECT 
			dc.docset_id,
			dc.name,
			dc.type,
			dc.path,
			dc.content,
			dc.summary,
			dc.metadata
		FROM docset_content dc
		WHERE dc.id = ?
	`

	var resp gismov1.GetContentResponse
	var metadataRaw interface{}
	var content, summary sql.NullString

	err := h.db.QueryRowContext(ctx, query, req.ContentId).Scan(
		&resp.DocsetId,
		&resp.Name,
		&resp.Type,
		&resp.Path,
		&content,
		&summary,
		&metadataRaw,
	)

	if err == sql.ErrNoRows {
		return nil, status.Error(codes.NotFound, "content not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retrieve content: %v", err)
	}

	// Handle nullable fields
	if content.Valid {
		resp.Content = content.String
	}
	if summary.Valid {
		resp.Summary = summary.String
	}

	// Handle metadata - DuckDB returns it as a map[string]interface{}
	if metadataRaw != nil {
		switch m := metadataRaw.(type) {
		case map[string]interface{}:
			resp.Metadata = make(map[string]string)
			for k, v := range m {
				if str, ok := v.(string); ok {
					resp.Metadata[k] = str
				} else if v != nil {
					resp.Metadata[k] = fmt.Sprintf("%v", v)
				}
			}
		}
	}

	return &resp, nil
}

// ExecuteQuery executes a raw SQL query
func (h *KnowledgeHandler) ExecuteQuery(ctx context.Context, req *gismov1.QueryRequest) (*gismov1.QueryResponse, error) {
	logProjectContext("ExecuteQuery", req.Context)
	if req.Sql == "" {
		return nil, status.Error(codes.InvalidArgument, "sql is required")
	}

	// Apply timeout if specified
	if req.TimeoutMs > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeMilliseconds(req.TimeoutMs))
		defer cancel()
	}

	// Execute query
	rows, err := h.db.QueryContext(ctx, req.Sql)
	if err != nil {
		return &gismov1.QueryResponse{
			Error: err.Error(),
		}, nil
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return &gismov1.QueryResponse{
			Error: fmt.Sprintf("failed to get columns: %v", err),
		}, nil
	}

	// Collect results
	var results []*structpb.Struct
	resultCount := int32(0)
	for rows.Next() && (req.MaxRows == 0 || resultCount < req.MaxRows) {
		// Create value holders for scanning
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return &gismov1.QueryResponse{
				Error: fmt.Sprintf("failed to scan row: %v", err),
			}, nil
		}

		// Convert to structpb
		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = convertValueForProtobuf(values[i])
		}

		rowStruct, err := structpb.NewStruct(row)
		if err != nil {
			// Log the error for debugging but continue
			log.Printf("Warning: failed to convert row to structpb: %v", err)
			continue
		}
		results = append(results, rowStruct)
		resultCount++
	}

	return &gismov1.QueryResponse{
		Columns: columns,
		Rows:    results,
	}, nil
}

// ExecuteQueryStream executes a raw SQL query with streaming results
func (h *KnowledgeHandler) ExecuteQueryStream(req *gismov1.QueryRequest, stream gismov1.KnowledgeService_ExecuteQueryStreamServer) error {
	logProjectContext("ExecuteQueryStream", req.Context)
	if req.Sql == "" {
		return status.Error(codes.InvalidArgument, "sql is required")
	}

	ctx := stream.Context()

	// Apply timeout if specified
	if req.TimeoutMs > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeMilliseconds(req.TimeoutMs))
		defer cancel()
	}

	// Execute query
	rows, err := h.db.QueryContext(ctx, req.Sql)
	if err != nil {
		return stream.Send(&gismov1.QueryResult{
			Result: &gismov1.QueryResult_Error{
				Error: err.Error(),
			},
		})
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return stream.Send(&gismov1.QueryResult{
			Result: &gismov1.QueryResult_Error{
				Error: fmt.Sprintf("failed to get columns: %v", err),
			},
		})
	}

	// Send metadata
	err = stream.Send(&gismov1.QueryResult{
		Result: &gismov1.QueryResult_Metadata{
			Metadata: &gismov1.QueryMetadata{
				Columns: columns,
			},
		},
	})
	if err != nil {
		return err
	}

	// Stream rows
	rowCount := int32(0)
	for rows.Next() && (req.MaxRows == 0 || rowCount < req.MaxRows) {
		// Create value holders for scanning
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return stream.Send(&gismov1.QueryResult{
				Result: &gismov1.QueryResult_Error{
					Error: fmt.Sprintf("failed to scan row: %v", err),
				},
			})
		}

		// Convert to structpb
		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = convertValueForProtobuf(values[i])
		}

		rowStruct, err := structpb.NewStruct(row)
		if err != nil {
			// Log the error for debugging but continue
			log.Printf("Warning: failed to convert row to structpb: %v", err)
			continue
		}

		err = stream.Send(&gismov1.QueryResult{
			Result: &gismov1.QueryResult_Row{
				Row: rowStruct,
			},
		})
		if err != nil {
			return err
		}
		rowCount++
	}

	// Send completion
	return stream.Send(&gismov1.QueryResult{
		Result: &gismov1.QueryResult_Complete{
			Complete: &gismov1.QueryComplete{
				TotalRows: rowCount,
			},
		},
	})
}

// timeMilliseconds converts milliseconds to time.Duration
// safeIntToInt32 safely converts int to int32 with bounds checking
func safeIntToInt32(n int) int32 {
	const maxInt32 = 2147483647
	if n > maxInt32 {
		return maxInt32
	}
	if n < -maxInt32-1 {
		return -maxInt32 - 1
	}
	return int32(n)
}

// ExaSearch performs a search using Exa.ai with semantic caching
func (h *KnowledgeHandler) ExaSearch(ctx context.Context, req *gismov1.ExaSearchRequest) (*gismov1.ExaSearchResponse, error) {
	if req.Context == nil {
		return nil, status.Error(codes.InvalidArgument, "project context is required")
	}

	logProjectContext("ExaSearch", req.Context)

	// Initialize cache manager
	cacheManager, err := cache.NewExaCacheManager(h.db)
	if err != nil {
		log.Printf("Warning: Failed to initialize cache manager: %v", err)
	}

	// Default similarity threshold
	similarityThreshold := float32(0.8)
	if req.SimilarityThreshold > 0 {
		similarityThreshold = req.SimilarityThreshold
	}

	// Check cache first if enabled
	if req.UseCache && cacheManager != nil {
		cached, err := cacheManager.FindSimilarQuery(ctx, req.Query, req.Context.ProjectName, similarityThreshold)
		if err != nil {
			log.Printf("Cache lookup error: %v", err)
		} else if cached != nil {
			log.Printf("Cache hit for query: %s", req.Query)
			return &gismov1.ExaSearchResponse{
				SearchId:        cached.ID,
				Results:         convertExaResultsToProto(cached.Results),
				FromCache:       true,
				CacheSimilarity: similarityThreshold,
				ExecutionTimeMs: 0,
			}, nil
		}
	}

	// Get API key from environment
	apiKey := os.Getenv("EXA_API_KEY")
	if apiKey == "" {
		return nil, status.Error(codes.FailedPrecondition, "EXA_API_KEY environment variable not set")
	}

	// Create Exa client
	client := exa.NewClient(apiKey)

	// Build search request
	searchReq := &exa.SearchRequest{
		Query:      req.Query,
		NumResults: 10,
		Type:       "neural",
		Contents: exa.Contents{
			Text:    true,
			Summary: true,
		},
	}

	if req.Options != nil {
		if req.Options.NumResults > 0 {
			searchReq.NumResults = int(req.Options.NumResults)
		}
		if req.Options.SearchType != "" {
			searchReq.Type = req.Options.SearchType
		}
		searchReq.UseAutoprompt = req.Options.UseAutoprompt
		searchReq.IncludeDomains = req.Options.IncludeDomains
		searchReq.ExcludeDomains = req.Options.ExcludeDomains
		searchReq.StartPublishedDate = req.Options.StartPublishedDate
		searchReq.EndPublishedDate = req.Options.EndPublishedDate
		searchReq.Category = req.Options.Category
	}

	// Execute search
	start := time.Now()
	searchResp, err := client.Search(ctx, searchReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Exa search failed: %v", err)
	}
	execTime := time.Since(start).Milliseconds()

	// Store in cache
	if cacheManager != nil {
		searchType := "neural"
		if req.Options != nil && req.Options.SearchType != "" {
			searchType = req.Options.SearchType
		}
		err = cacheManager.StoreSearch(ctx, req.Query, searchType, searchResp.Results, req.Context.ProjectName)
		if err != nil {
			log.Printf("Failed to cache search results: %v", err)
		}
	}

	// Get the search ID from cache (it was just stored)
	searchID := ""
	if cacheManager != nil {
		cached, _ := cacheManager.FindSimilarQuery(ctx, req.Query, req.Context.ProjectName, 0.99)
		if cached != nil {
			searchID = cached.ID
		}
	}

	return &gismov1.ExaSearchResponse{
		SearchId:         searchID,
		Results:          convertExaResultsToProto(searchResp.Results),
		FromCache:        false,
		CacheSimilarity:  0,
		ExecutionTimeMs:  execTime,
		AutopromptString: searchResp.AutoPrompt,
	}, nil
}

// ProvideFeedback records feedback about search usefulness
func (h *KnowledgeHandler) ProvideFeedback(ctx context.Context, req *gismov1.SearchFeedbackRequest) (*gismov1.SearchFeedbackResponse, error) {
	if req.Context == nil {
		return nil, status.Error(codes.InvalidArgument, "project context is required")
	}

	logProjectContext("ProvideFeedback", req.Context)

	// Initialize cache manager
	cacheManager, err := cache.NewExaCacheManager(h.db)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to initialize cache manager: %v", err)
	}

	// Store feedback
	feedbackType := "explicit"
	if req.FeedbackType != "" {
		feedbackType = req.FeedbackType
	}

	err = cacheManager.ProvideFeedback(ctx, req.SearchId, req.UsefulnessScore, feedbackType)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to store feedback: %v", err)
	}

	// Get updated TTL
	var updatedTTL int32
	query := `SELECT ttl_days FROM exa_search_cache WHERE id = $1`
	err = h.db.QueryRowContext(ctx, query, req.SearchId).Scan(&updatedTTL)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Failed to get updated TTL: %v", err)
	}

	return &gismov1.SearchFeedbackResponse{
		Success:        true,
		Message:        fmt.Sprintf("Feedback recorded for search %s", req.SearchId),
		UpdatedTtlDays: updatedTTL,
	}, nil
}

// GetCachedSearches retrieves recent cached searches
func (h *KnowledgeHandler) GetCachedSearches(ctx context.Context, req *gismov1.GetCachedSearchesRequest) (*gismov1.GetCachedSearchesResponse, error) {
	if req.Context == nil {
		return nil, status.Error(codes.InvalidArgument, "project context is required")
	}

	logProjectContext("GetCachedSearches", req.Context)

	// Initialize cache manager
	cacheManager, err := cache.NewExaCacheManager(h.db)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to initialize cache manager: %v", err)
	}

	// Default limit
	limit := 10
	if req.Limit > 0 {
		limit = int(req.Limit)
	}

	// Get cached searches
	searches, err := cacheManager.GetCachedSearches(ctx, req.Context.ProjectName, limit)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get cached searches: %v", err)
	}

	// Convert to proto
	pbSearches := make([]*gismov1.CachedSearch, 0, len(searches))
	for _, search := range searches {
		// Calculate average usefulness
		var avgUsefulness float32
		query := `SELECT AVG(usefulness_score) FROM exa_feedback WHERE search_id = $1`
		_ = h.db.QueryRowContext(ctx, query, search.ID).Scan(&avgUsefulness)

		// Safe int to int32 conversions with bounds checking
		accessCount := safeIntToInt32(search.AccessCount)
		ttlDays := safeIntToInt32(search.TTLDays)
		resultCount := safeIntToInt32(len(search.Results))

		pbSearch := &gismov1.CachedSearch{
			Id:                search.ID,
			Query:             search.Query,
			SearchType:        search.SearchType,
			CreatedAt:         timestamppb.New(search.CreatedAt),
			LastAccessed:      timestamppb.New(search.LastAccessed),
			AccessCount:       accessCount,
			TtlDays:           ttlDays,
			ResultCount:       resultCount,
			AverageUsefulness: avgUsefulness,
		}

		// Apply query filter if provided
		if req.QueryFilter != "" {
			// Simple substring match
			if !strings.Contains(strings.ToLower(search.Query), strings.ToLower(req.QueryFilter)) {
				continue
			}
		}

		pbSearches = append(pbSearches, pbSearch)
	}

	return &gismov1.GetCachedSearchesResponse{
		Searches:   pbSearches,
		TotalCount: safeIntToInt32(len(pbSearches)),
	}, nil
}

// convertExaResultsToProto converts Exa search results to proto format
func convertExaResultsToProto(results []exa.SearchResult) []*gismov1.ExaResult {
	pbResults := make([]*gismov1.ExaResult, 0, len(results))
	for _, result := range results {
		pbResult := &gismov1.ExaResult{
			Id:            result.ID,
			Url:           result.URL,
			Title:         result.Title,
			Snippet:       result.Summary,
			Text:          result.Text,
			PublishedDate: result.PublishedDate,
			Author:        result.Author,
			Score:         float32(result.Score),
			Highlights:    result.Highlights,
			Metadata:      make(map[string]string),
		}
		pbResults = append(pbResults, pbResult)
	}
	return pbResults
}

// convertValueForProtobuf converts database values to types that can be stored in structpb
func convertValueForProtobuf(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case time.Time:
		// Convert time.Time to RFC3339 string
		return val.Format(time.RFC3339)
	case *time.Time:
		if val != nil {
			return val.Format(time.RFC3339)
		}
		return nil
	case []byte:
		// Convert byte arrays to string
		return string(val)
	case []interface{}:
		// Handle arrays by converting each element
		result := make([]interface{}, len(val))
		for i, elem := range val {
			result[i] = convertValueForProtobuf(elem)
		}
		return result
	case map[string]interface{}:
		// Handle nested maps
		result := make(map[string]interface{})
		for k, v := range val {
			result[k] = convertValueForProtobuf(v)
		}
		return result
	case []float64:
		// Convert float arrays to interface arrays
		result := make([]interface{}, len(val))
		for i, f := range val {
			result[i] = f
		}
		return result
	case []float32:
		// Convert float32 arrays to interface arrays
		result := make([]interface{}, len(val))
		for i, f := range val {
			result[i] = float64(f)
		}
		return result
	default:
		// Return as-is for primitive types
		return v
	}
}

func timeMilliseconds(ms int32) time.Duration {
	return time.Duration(ms) * time.Millisecond
}
