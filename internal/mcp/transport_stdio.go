package mcp

import (
	"context"
	"os"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/rs/zerolog/log"
)

// serveStdio starts the MCP server using stdio transport.
// This is the primary transport for AI agent and CLI integrations.
// It respects ctx cancellation by closing stdin to unblock the stdio server.
func (s *MCPServer) serveStdio(ctx context.Context) error {
	log.Info().Msg("MCP server starting (stdio transport)")

	done := make(chan error, 1)
	go func() {
		done <- mcpserver.ServeStdio(s.mcpServer)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		// Close stdin to unblock ServeStdio's read loop.
		os.Stdin.Close()
		return <-done
	}
}
