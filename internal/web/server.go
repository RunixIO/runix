package web

import (
	"context"
	"embed"
	"io/fs"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"github.com/runixio/runix/internal/auth"
	"github.com/runixio/runix/internal/supervisor"
	"github.com/runixio/runix/pkg/types"
)

//go:embed frontend
var frontendFS embed.FS

// Server is the web UI + API server.
type Server struct {
	supervisor *supervisor.Supervisor
	listen     string
	auth       auth.Authenticator
	sessions   *auth.SessionStore
	hub        *WSHub

	loginMu       sync.Mutex
	loginFailures map[string][]time.Time
}

// NewServer creates a new web server with an Authenticator.
func NewServerWithAuth(sup *supervisor.Supervisor, listen string, authenticator auth.Authenticator) *Server {
	if authenticator == nil {
		authenticator = &auth.NoAuth{}
	}

	var sessions *auth.SessionStore
	if authenticator.Mode() != "disabled" {
		sessions = auth.NewSessionStore(auth.DefaultSessionTTL)
	}

	return &Server{
		supervisor:    sup,
		listen:        listen,
		auth:          authenticator,
		sessions:      sessions,
		hub:           NewWSHub(),
		loginFailures: make(map[string][]time.Time),
	}
}

// NewServer creates a new web server (legacy, uses AuthConfig).
// Deprecated: Use NewServerWithAuth for new code.
func NewServer(sup *supervisor.Supervisor, listen string, legacyAuth types.AuthConfig) *Server {
	var authenticator auth.Authenticator
	if legacyAuth.Enabled {
		a, err := auth.NewAuthenticator(types.AuthSettings{
			Enabled:  true,
			Mode:     types.AuthModeBasic,
			Username: legacyAuth.Username,
			Password: legacyAuth.Password,
		})
		if err != nil {
			log.Warn().Err(err).Msg("failed to create web authenticator, falling back to NoAuth")
			authenticator = &auth.NoAuth{}
		} else {
			authenticator = a
		}
	} else {
		authenticator = &auth.NoAuth{}
	}
	return NewServerWithAuth(sup, listen, authenticator)
}

// Start starts the HTTP server. Blocks until the context is cancelled.
func (s *Server) Start(ctx context.Context) error {
	r := chi.NewRouter()

	// Middleware (must be registered BEFORE routes in chi).
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	if s.auth.Mode() != "disabled" {
		r.Use(WebAuthMiddleware(s.sessions))
	}

	// Auth routes (public paths — WebAuthMiddleware lets these through).
	r.Get("/login", s.handleLoginPage)
	r.Post("/api/auth/login", s.handleLoginAPI)
	r.Post("/api/auth/logout", s.handleLogoutAPI)
	r.Get("/api/auth/status", s.handleAuthStatusAPI)

	// API routes (protected by WebAuthMiddleware).
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/processes", s.handleListProcesses)
		r.Get("/processes/{id}", s.handleGetProcess)
		r.Post("/processes", s.handleStartProcess)
		r.Post("/processes/{id}/stop", s.handleStopProcess)
		r.Post("/processes/{id}/restart", s.handleRestartProcess)
		r.Post("/processes/{id}/reload", s.handleReloadProcess)
		r.Delete("/processes/{id}", s.handleDeleteProcess)
		r.Get("/processes/{id}/logs", s.handleGetLogs)
		r.Get("/system/metrics", s.handleSystemMetrics)
	})

	// WebSocket (protected by WebAuthMiddleware).
	r.Get("/ws", s.handleWebSocket)

	// Static frontend (protected by WebAuthMiddleware).
	staticFS, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		return err
	}
	r.Handle("/*", http.FileServer(http.FS(staticFS)))

	// Log auth status.
	mode := s.auth.Mode()
	if mode == "disabled" {
		log.Warn().Msg("web server running without authentication")
	} else {
		log.Info().Str("mode", mode).Msg("web server authentication enabled")
	}

	// Start the WebSocket hub.
	go s.hub.Run()

	// Start HTTP server.
	srv := &http.Server{
		Addr:    s.listen,
		Handler: r,
	}

	listener, err := net.Listen("tcp", s.listen)
	if err != nil {
		return err
	}

	go func() {
		log.Info().Str("addr", s.listen).Msg("web server starting")
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("web server error")
		}
	}()

	// Start broadcasting process updates.
	go s.broadcastUpdates(ctx)

	// Wait for context cancellation.
	<-ctx.Done()

	// Stop the session store and WebSocket hub.
	if s.sessions != nil {
		s.sessions.Stop()
	}
	s.hub.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// broadcastUpdates periodically sends process list updates to WebSocket clients.
func (s *Server) broadcastUpdates(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.hub.ClientCount() == 0 {
				continue
			}
			procs := s.supervisor.List()
			s.hub.Broadcast(ProcessListMessage{
				Type:      "process_list",
				Processes: summarizeProcesses(procs),
				Timestamp: time.Now().Unix(),
			})
		}
	}
}

// ProcessListMessage is sent via WebSocket.
type ProcessListMessage struct {
	Type      string             `json:"type"`
	Processes []wsProcessSummary `json:"processes"`
	Timestamp int64              `json:"timestamp"`
}

type wsProcessSummary struct {
	ID            string             `json:"id"`
	NumericID     int                `json:"numeric_id"`
	Name          string             `json:"name"`
	Namespace     string             `json:"namespace,omitempty"`
	InstanceIndex int                `json:"instance_index,omitempty"`
	Runtime       string             `json:"runtime"`
	State         types.ProcessState `json:"state"`
	PID           int                `json:"pid"`
	ExitCode      int                `json:"exit_code,omitempty"`
	Restarts      int                `json:"restarts"`
	CPUPercent    float64            `json:"cpu_percent"`
	MemBytes      int64              `json:"memory_bytes"`
	MemPercent    float64            `json:"mem_percent"`
	Threads       int                `json:"threads"`
	FDs           int                `json:"fds"`
	Uptime        time.Duration      `json:"uptime,omitempty"`
}

func summarizeProcesses(procs []types.ProcessInfo) []wsProcessSummary {
	result := make([]wsProcessSummary, 0, len(procs))
	for _, p := range procs {
		result = append(result, wsProcessSummary{
			ID:            p.ID,
			NumericID:     p.NumericID,
			Name:          p.Name,
			Namespace:     p.Namespace,
			InstanceIndex: p.InstanceIndex,
			Runtime:       p.Runtime,
			State:         p.State,
			PID:           p.PID,
			ExitCode:      p.ExitCode,
			Restarts:      p.Restarts,
			CPUPercent:    p.CPUPercent,
			MemBytes:      p.MemBytes,
			MemPercent:    p.MemPercent,
			Threads:       p.Threads,
			FDs:           p.FDs,
			Uptime:        p.Uptime,
		})
	}
	return result
}

// Ensure websocket import is used.
var _ = websocket.CloseGoingAway
