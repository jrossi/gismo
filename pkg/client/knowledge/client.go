package knowledge

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/jrossi/gismo/pkg/database/project"
	gismov1 "github.com/jrossi/gismo/pkg/generated/gismo/v1"
)

// Client provides access to the knowledge service
type Client struct {
	conn   *grpc.ClientConn
	client gismov1.KnowledgeServiceClient
}

// getProjectContext returns the current project context
func getProjectContext() *gismov1.ProjectContext {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		// If we can't get cwd, return empty context
		return &gismov1.ProjectContext{}
	}

	// Check if we're in a Claude project directory
	claudeProjectDir := os.Getenv("CLAUDE_PROJECT_DIR")
	if claudeProjectDir != "" {
		cwd = claudeProjectDir
	}

	// Convert to Claude project name format
	projectName := project.PathToProjectName(cwd)

	// Check if we have a Claude project ID in environment
	claudeProjectID := os.Getenv("CLAUDE_PROJECT_ID")

	// Also check if ~/.claude/projects/<project-name> exists
	// This would confirm we're in a Claude Code context
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		claudeProjectPath := filepath.Join(homeDir, ".claude", "projects", projectName)
		// Just checking existence, no action needed if it exists
		_, _ = os.Stat(claudeProjectPath)
	}

	return &gismov1.ProjectContext{
		ProjectPath:     cwd,
		ProjectName:     projectName,
		ClaudeProjectId: claudeProjectID,
	}
}

// New creates a new knowledge client
func New() (*Client, error) {
	// Get runtime directory
	runtimeDir, err := getRuntimeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime directory: %w", err)
	}

	socketPath := filepath.Join(runtimeDir, "gismo.sock")

	// Check if socket exists
	if _, err := os.Stat(socketPath); err != nil {
		return nil, fmt.Errorf("gismo server not running (socket not found at %s)", socketPath)
	}

	// Connect to server
	conn, err := grpc.NewClient(
		"unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	return &Client{
		conn:   conn,
		client: gismov1.NewKnowledgeServiceClient(conn),
	}, nil
}

// Close closes the client connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ImportDocset imports a docset from a URL with progress callbacks
func (c *Client) ImportDocset(ctx context.Context, url string, progress func(stage, message string, percent int)) error {
	stream, err := c.client.ImportDocset(ctx, &gismov1.ImportDocsetRequest{
		Context: getProjectContext(),
		Source: &gismov1.ImportDocsetRequest_Url{
			Url: url,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to start import: %w", err)
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("import failed: %w", err)
		}

		if progress != nil {
			stage := resp.Stage.String()
			progress(stage, resp.Message, int(resp.ProgressPercent))
		}
	}

	return nil
}

// ListDocsets returns all imported docsets
func (c *Client) ListDocsets(ctx context.Context) ([]*gismov1.Docset, error) {
	resp, err := c.client.ListDocsets(ctx, &gismov1.ListDocsetsRequest{
		Context: getProjectContext(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list docsets: %w", err)
	}
	return resp.Docsets, nil
}

// RemoveDocset removes a docset by ID
func (c *Client) RemoveDocset(ctx context.Context, docsetID string) error {
	resp, err := c.client.RemoveDocset(ctx, &gismov1.RemoveDocsetRequest{
		Context:  getProjectContext(),
		DocsetId: docsetID,
	})
	if err != nil {
		return fmt.Errorf("failed to remove docset: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("removal failed: %s", resp.Message)
	}
	return nil
}

// Search performs a search across docsets
func (c *Client) Search(ctx context.Context, query string, opts SearchOptions) ([]*gismov1.SearchResult, error) {
	// Ensure limit doesn't overflow int32
	limit := opts.Limit
	if limit > 2147483647 {
		limit = 2147483647
	}

	req := &gismov1.SearchRequest{
		Context: getProjectContext(),
		Query:   query,
		Type:    convertSearchType(opts.Type),
		Limit:   int32(limit), // #nosec G115 -- bounded above
	}

	if len(opts.DocsetIDs) > 0 {
		req.DocsetIds = opts.DocsetIDs
	}

	resp, err := c.client.Search(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return resp.Results, nil
}

// GetContent retrieves content by ID
func (c *Client) GetContent(ctx context.Context, contentID int32) (*gismov1.GetContentResponse, error) {
	return c.client.GetContent(ctx, &gismov1.GetContentRequest{
		Context:   getProjectContext(),
		ContentId: contentID,
	})
}

// ExecuteQuery executes a raw SQL query
func (c *Client) ExecuteQuery(ctx context.Context, sql string, maxRows int32) (*gismov1.QueryResponse, error) {
	return c.client.ExecuteQuery(ctx, &gismov1.QueryRequest{
		Context: getProjectContext(),
		Sql:     sql,
		MaxRows: maxRows,
	})
}

// ExecuteQueryStream executes a SQL query with streaming results
func (c *Client) ExecuteQueryStream(ctx context.Context, sql string, handler func(*gismov1.QueryResult) error) error {
	stream, err := c.client.ExecuteQueryStream(ctx, &gismov1.QueryRequest{
		Context: getProjectContext(),
		Sql:     sql,
	})
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	for {
		result, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("stream error: %w", err)
		}

		if err := handler(result); err != nil {
			return err
		}
	}

	return nil
}

// ExaSearch performs an Exa.ai search with caching
func (c *Client) ExaSearch(ctx context.Context, query string, options *ExaSearchOptions) (*gismov1.ExaSearchResponse, error) {
	projCtx := getProjectContext()

	req := &gismov1.ExaSearchRequest{
		Context:             projCtx,
		Query:               query,
		UseCache:            true,
		SimilarityThreshold: 0.8,
	}

	if options != nil {
		// Safe conversion with bounds check
		numResults := options.NumResults
		if numResults > 2147483647 {
			numResults = 2147483647
		}
		req.Options = &gismov1.ExaSearchOptions{
			NumResults:         int32(numResults), // #nosec G115 - bounded above
			SearchType:         options.SearchType,
			UseAutoprompt:      options.UseAutoprompt,
			IncludeDomains:     options.IncludeDomains,
			ExcludeDomains:     options.ExcludeDomains,
			StartPublishedDate: options.StartPublishedDate,
			EndPublishedDate:   options.EndPublishedDate,
			Category:           options.Category,
		}
		if options.UseCache != nil {
			req.UseCache = *options.UseCache
		}
		if options.SimilarityThreshold > 0 {
			req.SimilarityThreshold = options.SimilarityThreshold
		}
	}

	return c.client.ExaSearch(ctx, req)
}

// ProvideFeedback provides feedback on search usefulness
func (c *Client) ProvideFeedback(ctx context.Context, searchID string, score float32, usefulURLs []string) (*gismov1.SearchFeedbackResponse, error) {
	projCtx := getProjectContext()

	req := &gismov1.SearchFeedbackRequest{
		Context:         projCtx,
		SearchId:        searchID,
		UsefulnessScore: score,
		UsefulUrls:      usefulURLs,
		FeedbackType:    "explicit",
	}

	return c.client.ProvideFeedback(ctx, req)
}

// GetCachedSearches retrieves cached searches for the current project
func (c *Client) GetCachedSearches(ctx context.Context, limit int, queryFilter string) (*gismov1.GetCachedSearchesResponse, error) {
	projCtx := getProjectContext()

	// Safe conversion with bounds check
	safeLimit := limit
	if safeLimit > 2147483647 {
		safeLimit = 2147483647
	}
	if safeLimit < 0 {
		safeLimit = 0
	}

	req := &gismov1.GetCachedSearchesRequest{
		Context:     projCtx,
		Limit:       int32(safeLimit), // #nosec G115 - bounded above
		QueryFilter: queryFilter,
	}

	return c.client.GetCachedSearches(ctx, req)
}

// ExaSearchOptions contains options for Exa search
type ExaSearchOptions struct {
	NumResults          int
	SearchType          string // "neural", "keyword", "auto"
	UseAutoprompt       bool
	IncludeDomains      []string
	ExcludeDomains      []string
	StartPublishedDate  string
	EndPublishedDate    string
	Category            string
	UseCache            *bool
	SimilarityThreshold float32
}

// SearchOptions contains options for search queries
type SearchOptions struct {
	Type      SearchType
	DocsetIDs []string
	Limit     int
	Offset    int
}

// SearchType represents the type of search to perform
type SearchType string

const (
	SearchTypeKeyword  SearchType = "keyword"
	SearchTypeSemantic SearchType = "semantic"
	SearchTypeHybrid   SearchType = "hybrid"
)

// convertSearchType converts local search type to proto
func convertSearchType(t SearchType) gismov1.SearchRequest_SearchType {
	switch t {
	case SearchTypeSemantic:
		return gismov1.SearchRequest_SEMANTIC
	case SearchTypeHybrid:
		return gismov1.SearchRequest_HYBRID
	default:
		return gismov1.SearchRequest_KEYWORD
	}
}

// getRuntimeDir returns the appropriate runtime directory for the platform
func getRuntimeDir() (string, error) {
	// Try XDG_RUNTIME_DIR first
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "gismo"), nil
	}

	// Fall back to temp directory
	return filepath.Join(os.TempDir(), fmt.Sprintf("gismo-%d", os.Getuid())), nil
}
