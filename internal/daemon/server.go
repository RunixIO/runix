package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/runixio/runix/internal/auth"
	"github.com/runixio/runix/internal/config"
	"github.com/runixio/runix/internal/httputil"
	"github.com/runixio/runix/internal/logutil"
	"github.com/runixio/runix/internal/supervisor"
	"github.com/runixio/runix/internal/web"
	"github.com/runixio/runix/pkg/types"
)

const maxRequestBody = 1 << 20 // 1 MiB max request body

// Server is the daemon's Unix socket HTTP server.
type Server struct {
	supervisor *supervisor.Supervisor
	config     *types.RunixConfig
	listener   net.Listener
	httpServer *http.Server
	socketPath string
	pidFile    *PIDFile
	auth       auth.Authenticator

	webMu     sync.Mutex
	webCancel context.CancelFunc
	webAddr   string

	configPath string
}

// NewServer creates a new daemon server.
func NewServer(sup *supervisor.Supervisor, socketPath string, pidDir string, authenticator auth.Authenticator, cfg *types.RunixConfig, configPath string) *Server {
	if authenticator == nil {
		authenticator = &auth.NoAuth{}
	}
	return &Server{
		supervisor: sup,
		config:     cfg,
		socketPath: socketPath,
		pidFile:    NewPIDFile(pidDir),
		auth:       authenticator,
		configPath: configPath,
	}
}

// Start starts the daemon server. This blocks until the server is shut down.
func (s *Server) Start(ctx context.Context) error {
	// Remove any stale socket file.
	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Create parent dirs for socket.
	if err := os.MkdirAll(filepath.Dir(s.socketPath), 0o755); err != nil {
		return err
	}

	// Listen on Unix socket.
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return err
	}
	s.listener = listener

	// Restrict socket permissions.
	if err := os.Chmod(s.socketPath, 0o660); err != nil {
		log.Warn().Err(err).Msg("failed to set socket permissions")
	}

	// Write PID file.
	if err := s.pidFile.Write(); err != nil {
		log.Error().Err(err).Msg("failed to write PID file")
	}

	// Set up HTTP mux.
	mux := http.NewServeMux()

	// Public endpoints (no auth required).
	mux.HandleFunc("/api/ping", s.handlePing)

	// Authenticated endpoints.
	mux.HandleFunc("/api/start", s.authenticated(s.handleStart))
	mux.HandleFunc("/api/start_all", s.authenticated(s.handleStartAll))
	mux.HandleFunc("/api/stop", s.authenticated(s.handleStop))
	mux.HandleFunc("/api/restart", s.authenticated(s.handleRestart))
	mux.HandleFunc("/api/reload", s.authenticated(s.handleReload))
	mux.HandleFunc("/api/rolling_reload", s.authenticated(s.handleRollingReload))
	mux.HandleFunc("/api/delete", s.authenticated(s.handleDelete))
	mux.HandleFunc("/api/list", s.authenticated(s.handleList))
	mux.HandleFunc("/api/status", s.authenticated(s.handleStatus))
	mux.HandleFunc("/api/logs", s.authenticated(s.handleLogs))
	mux.HandleFunc("/api/save", s.authenticated(s.handleSave))
	mux.HandleFunc("/api/resurrect", s.authenticated(s.handleResurrect))
	mux.HandleFunc("/api/config_reload", s.authenticated(s.handleConfigReload))
	mux.HandleFunc("/api/web_start", s.authenticated(s.handleWebStart))

	// Log auth status.
	mode := s.auth.Mode()
	if mode == "disabled" {
		log.Warn().Msg("daemon running without authentication (unauthenticated access enabled)")
	} else {
		log.Info().Str("mode", mode).Msg("daemon authentication enabled")
	}

	s.httpServer = &http.Server{
		Handler: mux,
	}

	// Set up signal handling.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	serverCtx, serverCancel := context.WithCancel(ctx)
	defer serverCancel()

	// Serve HTTP in a goroutine.
	go func() {
		log.Info().Str("socket", s.socketPath).Msg("daemon listening")
		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("HTTP server error")
		}
	}()

	// Wait for signal or context cancellation.
	for {
		select {
		case sig := <-sigCh:
			switch sig {
			case syscall.SIGTERM, syscall.SIGINT:
				log.Info().Str("signal", sig.String()).Msg("received shutdown signal")
				return s.shutdown()
			case syscall.SIGHUP:
				log.Info().Msg("received SIGHUP, reloading config")
				_ = s.reloadConfig()
			}
		case <-serverCtx.Done():
			return s.shutdown()
		}
	}
}

// shutdown performs graceful shutdown of the server.
func (s *Server) shutdown() error {
	log.Info().Msg("shutting down daemon")

	// Stop the web server if running.
	s.webMu.Lock()
	if s.webCancel != nil {
		s.webCancel()
	}
	s.webMu.Unlock()

	// Close the HTTP server.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("HTTP server shutdown error")
	}

	// Shutdown the supervisor.
	if err := s.supervisor.Shutdown(); err != nil {
		log.Error().Err(err).Msg("supervisor shutdown error")
	}

	// Remove PID file and socket.
	_ = s.pidFile.Remove()
	_ = os.Remove(s.socketPath)

	log.Info().Msg("daemon stopped")
	return nil
}

// handleAll applies fn to every managed process and returns a partial-failure error if any fail.
func (s *Server) handleAll(fn func(string) error) (interface{}, error) {
	procs := s.supervisor.List()
	var errs []string
	for _, p := range procs {
		if err := fn(p.ID); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", p.Name, err))
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("partial failure: %s", strings.Join(errs, "; "))
	}
	return map[string]string{"status": "ok"}, nil
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := limitedBody(w, r).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, ErrorResponse(err))
		return
	}

	var payload StartPayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, ErrorResponse(err))
		return
	}

	cfg := types.ProcessConfig{
		Name:          payload.Name,
		Runtime:       payload.Runtime,
		Entrypoint:    payload.Entrypoint,
		Args:          payload.Args,
		Cwd:           payload.Cwd,
		Env:           payload.Env,
		RestartPolicy: types.RestartPolicy(payload.RestartPolicy),
		MaxRestarts:   payload.MaxRestarts,
	}

	proc, err := s.supervisor.AddProcess(context.Background(), cfg)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, ErrorResponse(err))
		return
	}

	info := proc.Info()
	resp, err := DataResponse(info)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, ErrorResponse(err))
		return
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

// handleStartAll starts all processes from the config file.
func (s *Server) handleStartAll(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := limitedBody(w, r).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, ErrorResponse(err))
		return
	}

	var payload StartAllPayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, ErrorResponse(err))
		return
	}

	// Resolve config: load from payload path if provided, otherwise use daemon's config.
	cfg := s.config
	if payload.Config != "" {
		loaded, err := config.Load(payload.Config)
		if err != nil {
			httputil.WriteJSON(w, http.StatusBadRequest, ErrorResponse(fmt.Errorf("failed to load config: %w", err)))
			return
		}
		cfg = loaded
	}

	if cfg == nil || len(cfg.Processes) == 0 {
		resp, _ := DataResponse(map[string]string{"status": "no processes defined in config"})
		httputil.WriteJSON(w, http.StatusOK, resp)
		return
	}

	// Filter: only autostart processes, optional name filter.
	only := make(map[string]bool)
	if payload.Only != "" {
		for _, name := range strings.Split(payload.Only, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				only[name] = true
			}
		}
	}

	var toStart []types.ProcessConfig
	for _, p := range cfg.Processes {
		if !p.Autostart && len(only) == 0 {
			continue
		}
		if len(only) > 0 && !only[p.Name] {
			continue
		}
		toStart = append(toStart, p)
	}

	if len(toStart) == 0 {
		resp, _ := DataResponse(map[string]string{"status": "no processes to start"})
		httputil.WriteJSON(w, http.StatusOK, resp)
		return
	}

	// Sort by priority (lower first), then topological order for depends_on.
	SortStartOrder(toStart)
	toStart = ExpandProcessInstances(toStart)

	var started []types.ProcessInfo
	var errs []string
	for _, cfg := range toStart {
		proc, err := s.supervisor.AddProcess(context.Background(), cfg)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %s", cfg.Name, err.Error()))
			continue
		}
		started = append(started, proc.Info())
	}

	resp, err := DataResponse(map[string]interface{}{
		"started": started,
		"errors":  errs,
	})
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, ErrorResponse(err))
		return
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

// SortStartOrder sorts processes by priority then topologically by depends_on.
func SortStartOrder(processes []types.ProcessConfig) {
	sort.SliceStable(processes, func(i, j int) bool {
		return processes[i].Priority < processes[j].Priority
	})

	index := make(map[string]int)
	for i, p := range processes {
		index[p.Name] = i
	}

	inDegree := make(map[int]int)
	for _, p := range processes {
		for _, dep := range p.DependsOn {
			if _, ok := index[dep]; ok {
				inDegree[index[p.Name]]++
			}
		}
	}

	var queue []int
	for i := range processes {
		if inDegree[i] == 0 {
			queue = append(queue, i)
		}
	}

	var ordered []types.ProcessConfig
	visited := make(map[int]bool)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if visited[cur] {
			continue
		}
		visited[cur] = true
		ordered = append(ordered, processes[cur])

		for i, p := range processes {
			if visited[i] {
				continue
			}
			for _, dep := range p.DependsOn {
				if dep == processes[cur].Name {
					inDegree[i]--
					if inDegree[i] == 0 {
						queue = append(queue, i)
					}
				}
			}
		}
	}

	for i, p := range processes {
		if !visited[i] {
			ordered = append(ordered, p)
		}
	}

	copy(processes, ordered)
}

func (s *Server) handleGroupTarget(target string, fn func(string) error) (interface{}, error) {
	procs, err := s.supervisor.GetGroup(target)
	if err != nil {
		return nil, err
	}
	results := make(map[string]string, len(procs))
	for _, proc := range procs {
		if err := fn(proc.ID); err != nil {
			return nil, err
		}
		results[proc.Config.Name] = "ok"
	}
	return results, nil
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := limitedBody(w, r).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, ErrorResponse(err))
		return
	}

	var payload StopPayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, ErrorResponse(err))
		return
	}

	timeout := 5 * time.Second
	if payload.Timeout != "" {
		if d, err := time.ParseDuration(payload.Timeout); err == nil {
			timeout = d
		}
	}

	if payload.Target == "all" {
		result, err := s.handleAll(func(id string) error {
			return s.supervisor.StopProcess(id, payload.Force, timeout)
		})
		if err != nil {
			httputil.WriteJSON(w, http.StatusInternalServerError, ErrorResponse(err))
			return
		}
		resp, _ := DataResponse(result)
		httputil.WriteJSON(w, http.StatusOK, resp)
		return
	}

	result, err := s.handleGroupTarget(payload.Target, func(id string) error {
		return s.supervisor.StopProcess(id, payload.Force, timeout)
	})
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, ErrorResponse(err))
		return
	}
	if results, ok := result.(map[string]string); ok && len(results) == 1 {
		resp, _ := DataResponse(map[string]string{"status": "stopped"})
		httputil.WriteJSON(w, http.StatusOK, resp)
		return
	}

	resp, _ := DataResponse(result)
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := limitedBody(w, r).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, ErrorResponse(err))
		return
	}

	var payload StopPayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, ErrorResponse(err))
		return
	}

	if payload.Target == "all" {
		ctx := r.Context()
		result, err := s.handleAll(func(id string) error {
			return s.supervisor.RestartProcess(ctx, id)
		})
		if err != nil {
			httputil.WriteJSON(w, http.StatusInternalServerError, ErrorResponse(err))
			return
		}
		resp, _ := DataResponse(result)
		httputil.WriteJSON(w, http.StatusOK, resp)
		return
	}

	result, err := s.handleGroupTarget(payload.Target, func(id string) error {
		return s.supervisor.RestartProcess(r.Context(), id)
	})
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, ErrorResponse(err))
		return
	}
	if results, ok := result.(map[string]string); ok && len(results) == 1 {
		resp, _ := DataResponse(map[string]string{"status": "restarted"})
		httputil.WriteJSON(w, http.StatusOK, resp)
		return
	}

	resp, _ := DataResponse(result)
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := limitedBody(w, r).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, ErrorResponse(err))
		return
	}

	var payload StopPayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, ErrorResponse(err))
		return
	}

	if payload.Target == "all" {
		ctx := r.Context()
		result, err := s.handleAll(func(id string) error {
			return s.supervisor.ReloadProcess(ctx, id)
		})
		if err != nil {
			httputil.WriteJSON(w, http.StatusInternalServerError, ErrorResponse(err))
			return
		}
		resp, _ := DataResponse(result)
		httputil.WriteJSON(w, http.StatusOK, resp)
		return
	}

	result, err := s.handleGroupTarget(payload.Target, func(id string) error {
		return s.supervisor.ReloadProcess(r.Context(), id)
	})
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, ErrorResponse(err))
		return
	}
	if results, ok := result.(map[string]string); ok && len(results) == 1 {
		resp, _ := DataResponse(map[string]string{"status": "reloaded"})
		httputil.WriteJSON(w, http.StatusOK, resp)
		return
	}

	resp, _ := DataResponse(result)
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRollingReload(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := limitedBody(w, r).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, ErrorResponse(err))
		return
	}

	var payload RollingReloadPayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, ErrorResponse(err))
		return
	}

	var names []string
	if payload.Target == "" || payload.Target == "all" {
		procs := s.supervisor.List()
		names = make([]string, len(procs))
		for i, p := range procs {
			names[i] = p.Name
		}
	} else {
		resolvedNames, err := s.supervisor.GetGroupNames(payload.Target)
		if err != nil {
			httputil.WriteJSON(w, http.StatusInternalServerError, ErrorResponse(err))
			return
		}
		names = resolvedNames
	}

	if err := s.supervisor.RollingReload(r.Context(), names, supervisor.RollingReloadOptions{
		BatchSize:         payload.BatchSize,
		WaitReady:         payload.WaitReady,
		RollbackOnFailure: payload.RollbackOnFailure,
	}); err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, ErrorResponse(err))
		return
	}

	resp, _ := DataResponse(map[string]string{"status": "rolling_reloaded"})
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := limitedBody(w, r).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, ErrorResponse(err))
		return
	}

	var payload StopPayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, ErrorResponse(err))
		return
	}

	if payload.Target == "all" {
		result, err := s.handleAll(func(id string) error {
			return s.supervisor.RemoveProcess(id)
		})
		if err != nil {
			httputil.WriteJSON(w, http.StatusInternalServerError, ErrorResponse(err))
			return
		}
		resp, _ := DataResponse(result)
		httputil.WriteJSON(w, http.StatusOK, resp)
		return
	}

	result, err := s.handleGroupTarget(payload.Target, func(id string) error {
		return s.supervisor.RemoveProcess(id)
	})
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, ErrorResponse(err))
		return
	}
	if results, ok := result.(map[string]string); ok && len(results) == 1 {
		resp, _ := DataResponse(map[string]string{"status": "deleted"})
		httputil.WriteJSON(w, http.StatusOK, resp)
		return
	}

	resp, _ := DataResponse(result)
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	infos := s.supervisor.List()
	resp, err := DataResponse(infos)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, ErrorResponse(err))
		return
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := limitedBody(w, r).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, ErrorResponse(err))
		return
	}

	var payload StopPayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, ErrorResponse(err))
		return
	}

	proc, err := s.supervisor.Get(payload.Target)
	if err != nil {
		httputil.WriteJSON(w, http.StatusNotFound, ErrorResponse(err))
		return
	}

	info := proc.Info()
	resp, err := DataResponse(info)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, ErrorResponse(err))
		return
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := limitedBody(w, r).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, ErrorResponse(err))
		return
	}

	var payload LogsPayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, ErrorResponse(err))
		return
	}

	proc, err := s.supervisor.Get(payload.Target)
	if err != nil {
		httputil.WriteJSON(w, http.StatusNotFound, ErrorResponse(err))
		return
	}

	logPath := s.supervisor.LogPath(proc.Config.Name)
	data, err := logutil.ReadFileBounded(logPath, 1<<20) // 1 MiB max
	if err != nil {
		if os.IsNotExist(err) {
			resp, _ := DataResponse(map[string]string{"logs": ""})
			httputil.WriteJSON(w, http.StatusOK, resp)
			return
		}
		httputil.WriteJSON(w, http.StatusInternalServerError, ErrorResponse(err))
		return
	}

	lines := string(data)
	if payload.Lines > 0 {
		lines = logutil.TailLines(lines, payload.Lines)
	}

	resp, _ := DataResponse(map[string]string{"logs": lines})
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	if err := s.supervisor.Save(); err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, ErrorResponse(err))
		return
	}
	resp, _ := DataResponse(map[string]string{"status": "saved"})
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (s *Server) handleResurrect(w http.ResponseWriter, r *http.Request) {
	if err := s.supervisor.Resurrect(); err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, ErrorResponse(err))
		return
	}
	resp, _ := DataResponse(map[string]string{"status": "resurrected"})
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (s *Server) handleWebStart(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := limitedBody(w, r).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, ErrorResponse(err))
		return
	}

	var payload WebStartPayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, ErrorResponse(err))
		return
	}

	listen := payload.Listen
	if listen == "" {
		listen = "localhost:9615"
	}

	// Preflight the bind so we fail fast instead of reporting success and then
	// discovering the address conflict inside the background web server goroutine.
	probe, err := net.Listen("tcp", listen)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, ErrorResponse(fmt.Errorf("failed to bind web listener: %w", err)))
		return
	}
	_ = probe.Close()

	s.webMu.Lock()
	defer s.webMu.Unlock()

	// Already running — return existing address.
	if s.webCancel != nil {
		resp, _ := DataResponse(map[string]string{
			"status": "already_running",
			"addr":   s.webAddr,
		})
		httputil.WriteJSON(w, http.StatusOK, resp)
		return
	}

	// Start the web server in a goroutine.
	webSrv := web.NewServerWithAuth(s.supervisor, listen, s.auth)
	ctx, cancel := context.WithCancel(context.Background())
	s.webCancel = cancel
	s.webAddr = listen

	go func() {
		log.Info().Str("addr", listen).Msg("web server starting (via IPC)")
		if err := webSrv.Start(ctx); err != nil && ctx.Err() == nil {
			log.Error().Err(err).Msg("web server error")
		}
		s.webMu.Lock()
		s.webCancel = nil
		s.webAddr = ""
		s.webMu.Unlock()
	}()

	resp, _ := DataResponse(map[string]string{
		"status": "started",
		"addr":   listen,
	})
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	resp, _ := DataResponse(map[string]string{"status": "ok"})
	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (s *Server) handleConfigReload(w http.ResponseWriter, r *http.Request) {
	// Accept optional config path from payload.
	path := s.configPath
	var req Request
	if err := limitedBody(w, r).Decode(&req); err == nil {
		var payload ConfigReloadPayload
		if err := json.Unmarshal(req.Payload, &payload); err == nil && payload.ConfigPath != "" {
			path = payload.ConfigPath
			s.configPath = path // remember for future reloads
		}
	}

	if err := s.reloadConfigFrom(path); err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, ErrorResponse(err))
		return
	}
	resp, _ := DataResponse(map[string]string{"status": "reloaded"})
	httputil.WriteJSON(w, http.StatusOK, resp)
}

// reloadConfig reloads using the stored config path (for SIGHUP).
func (s *Server) reloadConfig() error {
	return s.reloadConfigFrom(s.configPath)
}

// reloadConfigFrom reloads the config file and updates auth + web server.
func (s *Server) reloadConfigFrom(path string) error {
	if path == "" {
		return fmt.Errorf("no config file to reload")
	}

	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("failed to reload config: %w", err)
	}

	newAuth, authErr := auth.NewAuthenticator(cfg.Security.Auth)
	if authErr != nil {
		return fmt.Errorf("failed to create authenticator: %w", authErr)
	}

	s.config = cfg
	s.auth = newAuth

	// Restart web server if running so it picks up the new auth.
	s.webMu.Lock()
	if s.webCancel != nil {
		addr := s.webAddr
		s.webCancel()
		time.Sleep(500 * time.Millisecond)
		webSrv := web.NewServerWithAuth(s.supervisor, addr, s.auth)
		ctx, cancel := context.WithCancel(context.Background())
		s.webCancel = cancel
		go func() {
			log.Info().Str("addr", addr).Msg("web server restarting after config reload")
			if err := webSrv.Start(ctx); err != nil && ctx.Err() == nil {
				log.Error().Err(err).Msg("web server error")
			}
			s.webMu.Lock()
			s.webCancel = nil
			s.webAddr = ""
			s.webMu.Unlock()
		}()
	}
	s.webMu.Unlock()

	log.Info().Str("path", path).Msg("config reloaded")
	return nil
}

// authenticated wraps a handler with authentication checking.
func (s *Server) authenticated(handler http.HandlerFunc) http.HandlerFunc {
	return auth.MiddlewareFunc(s.auth, handler)
}

// limitedBody wraps r.Body with a MaxBytesReader and returns a JSON decoder.
func limitedBody(w http.ResponseWriter, r *http.Request) *json.Decoder {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	return json.NewDecoder(r.Body)
}
