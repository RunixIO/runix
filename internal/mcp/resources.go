package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// registerResources adds MCP resource handlers for process info, logs, and metrics.
func (s *MCPServer) registerResources() {
	// Process list resource.
	s.mcpServer.AddResource(
		mcp.NewResource("apps://list", "app_list",
			mcp.WithResourceDescription("List of all managed processes with their current state"),
			mcp.WithMIMEType("application/json"),
		),
		s.handleAppListResource,
	)

	// Single app status resource template.
	s.mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate("apps://{name}", "app_status",
			mcp.WithTemplateDescription("Detailed status of a specific process by ID or name"),
			mcp.WithTemplateMIMEType("application/json"),
		),
		s.handleAppResource,
	)

	// Process logs resource template.
	s.mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate("logs://{name}", "app_logs",
			mcp.WithTemplateDescription("Recent log output from a process"),
			mcp.WithTemplateMIMEType("text/plain"),
		),
		s.handleLogsResource,
	)

	// Process metrics resource template.
	s.mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate("metrics://{name}", "app_metrics",
			mcp.WithTemplateDescription("Resource usage metrics for a specific process"),
			mcp.WithTemplateMIMEType("application/json"),
		),
		s.handleMetricsResource,
	)
}

func (s *MCPServer) handleAppListResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	procs := s.supervisor.List()
	data, err := json.MarshalIndent(procs, "", "  ")
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "apps://list",
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func (s *MCPServer) handleAppResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	name := extractURISegment(request.Params.URI, "apps://")

	proc, err := s.supervisor.Get(name)
	if err != nil {
		return nil, fmt.Errorf("process %q not found: %w", name, err)
	}

	info := proc.Info()
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func (s *MCPServer) handleLogsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	name := extractURISegment(request.Params.URI, "logs://")

	proc, err := s.supervisor.Get(name)
	if err != nil {
		return nil, fmt.Errorf("process %q not found: %w", name, err)
	}

	info := proc.Info()
	logPath := s.supervisor.LogPath(info.Name)

	data, err := readResourceFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      request.Params.URI,
					MIMEType: "text/plain",
					Text:     "(no logs available)",
				},
			}, nil
		}
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "text/plain",
			Text:     string(data),
		},
	}, nil
}

func (s *MCPServer) handleMetricsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	name := extractURISegment(request.Params.URI, "metrics://")

	proc, err := s.supervisor.Get(name)
	if err != nil {
		return nil, fmt.Errorf("process %q not found: %w", name, err)
	}

	info := proc.Info()

	// Collect metrics from the collector if available.
	var metricsData interface{}
	if s.metrics != nil && info.PID > 0 {
		if m, ok := s.metrics.Get(info.PID); ok {
			metricsData = m
		} else {
			metricsData = map[string]string{"status": "no metrics collected yet"}
		}
	} else {
		metricsData = map[string]string{"status": "metrics collection not enabled"}
	}

	data, err := json.MarshalIndent(metricsData, "", "  ")
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

// extractURISegment extracts the segment after the given prefix from a URI.
// For example, extractURISegment("apps://my-app", "apps://") returns "my-app".
func extractURISegment(uri, prefix string) string {
	if strings.HasPrefix(uri, prefix) {
		return strings.TrimPrefix(uri, prefix)
	}
	return uri
}

// readResourceFile reads a file up to 1 MiB. For larger files, only the last
// 1 MiB is returned.
func readResourceFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	const maxBytes = 1 << 20 // 1 MiB
	if fi.Size() <= maxBytes {
		return io.ReadAll(f)
	}

	if _, err := f.Seek(-maxBytes, io.SeekEnd); err != nil {
		return nil, err
	}
	return io.ReadAll(f)
}
