package handlers

import (
	"context"
	"runtime"
	"time"

	gismov1 "github.com/jrossi/gismo/pkg/generated/gismo/v1"
	"github.com/jrossi/gismo/pkg/version"
)

// SystemHandler implements the SystemService gRPC interface
type SystemHandler struct {
	gismov1.UnimplementedSystemServiceServer
	startTime time.Time
	features  map[string]string
}

// NewSystemHandler creates a new system handler
func NewSystemHandler() *SystemHandler {
	return &SystemHandler{
		startTime: time.Now(),
		features: map[string]string{
			"reflection":   "enabled",
			"knowledge":    "enabled",
			"codesitter":   "enabled",
			"mcp":          "enabled",
			"docker":       "supported",
			"grpc_version": "1.58.0",
		},
	}
}

// GetVersion returns version information
func (h *SystemHandler) GetVersion(ctx context.Context, req *gismov1.GetVersionRequest) (*gismov1.GetVersionResponse, error) {
	return &gismov1.GetVersionResponse{
		Version:   version.BuildVersion,
		Commit:    version.BuildCommit,
		BuildDate: version.BuildDate,
		BuiltBy:   version.BuildBy,
		GoVersion: runtime.Version(),
		Features:  h.features,
	}, nil
}

// HealthCheck returns health status
func (h *SystemHandler) HealthCheck(ctx context.Context, req *gismov1.HealthCheckRequest) (*gismov1.HealthCheckResponse, error) {
	uptime := int64(time.Since(h.startTime).Seconds())

	services := map[string]bool{
		"grpc":       true,
		"knowledge":  true,
		"codesitter": true,
		"reflection": true,
	}

	return &gismov1.HealthCheckResponse{
		Healthy:       true,
		UptimeSeconds: uptime,
		Services:      services,
	}, nil
}
