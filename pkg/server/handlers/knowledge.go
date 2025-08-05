package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	gismov1 "github.com/jrossi/gismo/pkg/generated/gismo/v1"
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

// ImportDocset handles importing a docset from URL or local path
func (h *KnowledgeHandler) ImportDocset(req *gismov1.ImportDocsetRequest, stream gismov1.KnowledgeService_ImportDocsetServer) error {
	// Send initial progress
	err := stream.Send(&gismov1.ImportProgress{
		Stage:   gismov1.ImportProgress_DOWNLOADING,
		Message: "Starting docset import",
	})
	if err != nil {
		return status.Errorf(codes.Internal, "failed to send progress: %v", err)
	}

	// TODO: Implement actual import logic
	// For now, return a stub implementation
	err = stream.Send(&gismov1.ImportProgress{
		Stage:           gismov1.ImportProgress_COMPLETE,
		Message:         "Import complete (stub)",
		DocsetId:        "stub-docset-id",
		ProgressPercent: 100,
	})
	if err != nil {
		return status.Errorf(codes.Internal, "failed to send progress: %v", err)
	}

	return nil
}

// ListDocsets returns a list of imported docsets
func (h *KnowledgeHandler) ListDocsets(ctx context.Context, req *gismov1.ListDocsetsRequest) (*gismov1.ListDocsetsResponse, error) {
	// TODO: Implement actual listing from database
	return &gismov1.ListDocsetsResponse{
		Docsets: []*gismov1.Docset{},
	}, nil
}

// RemoveDocset removes an imported docset
func (h *KnowledgeHandler) RemoveDocset(ctx context.Context, req *gismov1.RemoveDocsetRequest) (*gismov1.RemoveDocsetResponse, error) {
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
	if req.Query == "" {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}

	// TODO: Implement actual search
	return &gismov1.SearchResponse{
		Results:    []*gismov1.SearchResult{},
		TotalCount: 0,
	}, nil
}

// GetContent retrieves content by ID
func (h *KnowledgeHandler) GetContent(ctx context.Context, req *gismov1.GetContentRequest) (*gismov1.GetContentResponse, error) {
	if req.ContentId == 0 {
		return nil, status.Error(codes.InvalidArgument, "content_id is required")
	}

	// TODO: Implement actual content retrieval
	return nil, status.Error(codes.NotFound, "content not found")
}

// ExecuteQuery executes a raw SQL query
func (h *KnowledgeHandler) ExecuteQuery(ctx context.Context, req *gismov1.QueryRequest) (*gismov1.QueryResponse, error) {
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
			row[col] = values[i]
		}

		rowStruct, err := structpb.NewStruct(row)
		if err != nil {
			continue // Skip rows that can't be converted
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
			row[col] = values[i]
		}

		rowStruct, err := structpb.NewStruct(row)
		if err != nil {
			continue // Skip rows that can't be converted
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
func timeMilliseconds(ms int32) time.Duration {
	return time.Duration(ms) * time.Millisecond
}
