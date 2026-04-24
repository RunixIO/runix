package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/server"
	"github.com/rs/zerolog/log"

	"github.com/runixio/runix/internal/auth"
	"github.com/runixio/runix/internal/metrics"
	"github.com/runixio/runix/internal/supervisor"
	"github.com/runixio/runix/pkg/types"
)

// MCPServer wraps an mcp-go server with Runix process management tools.
type MCPServer struct {
	mcpServer  *server.MCPServer
	supervisor *supervisor.Supervisor
	metrics    *metrics.Collector
	auth       auth.Authenticator
}

// NewMCPServer creates a new MCP server with all Runix tools and resources registered.
func NewMCPServer(sup *supervisor.Supervisor, col *metrics.Collector) *MCPServer {
	s := &MCPServer{
		mcpServer:  server.NewMCPServer("runix", "1.0.0"),
		supervisor: sup,
		metrics:    col,
		auth:       &auth.NoAuth{},
	}

	s.registerTools()
	s.registerResources()

	log.Info().Msg("MCP server initialized with tools and resources")
	return s
}

// SetAuth sets the authenticator for the MCP server (used for HTTP transport).
func (s *MCPServer) SetAuth(a auth.Authenticator) {
	if a == nil {
		a = &auth.NoAuth{}
	}
	s.auth = a
}

// Start begins serving the MCP server using the transport specified in cfg.
// It blocks until the context is cancelled or a fatal error occurs.
func (s *MCPServer) Start(ctx context.Context, cfg types.MCPConfig) error {
	transport := cfg.Transport
	if transport == "" {
		transport = "stdio"
	}

	switch transport {
	case "stdio":
		return s.serveStdio(ctx)
	case "http":
		listen := cfg.Listen
		if listen == "" {
			listen = "localhost:8090"
		}
		return s.serveHTTP(ctx, listen)
	default:
		return fmt.Errorf("unsupported MCP transport: %q", transport)
	}
}

// Server returns the underlying mcp-go server for advanced use.
func (s *MCPServer) Server() *server.MCPServer {
	return s.mcpServer
}
