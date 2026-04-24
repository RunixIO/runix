package mcp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/rs/zerolog/log"

	"github.com/runixio/runix/internal/auth"
)

// serveHTTP starts the MCP server using StreamableHTTP transport.
// This is suitable for Web UI and remote integrations.
func (s *MCPServer) serveHTTP(ctx context.Context, listen string) error {
	log.Info().Str("addr", listen).Msg("MCP server starting (HTTP transport)")

	httpServer := mcpserver.NewStreamableHTTPServer(s.mcpServer)

	// Build the handler, optionally wrapping with auth middleware.
	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpServer.ServeHTTP(w, r)
	})

	if s.auth.Mode() != "disabled" {
		log.Info().Str("mode", s.auth.Mode()).Msg("MCP HTTP server authentication enabled")
		handler = auth.Middleware(s.auth)(handler)
	} else {
		log.Warn().Msg("MCP HTTP server running without authentication")
	}

	// Create a standard HTTP server for graceful shutdown.
	srv := &http.Server{
		Addr:    listen,
		Handler: handler,
	}

	// Start serving in a goroutine.
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("HTTP server error: %w", err)
		}
		close(errCh)
	}()

	log.Info().Str("addr", listen).Str("endpoint", "/mcp").Msg("MCP HTTP server listening")

	// Wait for context cancellation or server error.
	select {
	case <-ctx.Done():
		log.Info().Msg("MCP HTTP server shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
