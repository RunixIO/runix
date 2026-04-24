package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/runixio/runix/internal/httputil"
	"github.com/runixio/runix/internal/logutil"
	"github.com/runixio/runix/internal/metrics"
	"github.com/runixio/runix/pkg/types"
)

const maxWebBody = 1 << 20 // 1 MiB

// handleListProcesses returns all managed processes.
func (s *Server) handleListProcesses(w http.ResponseWriter, r *http.Request) {
	procs := s.supervisor.List()
	httputil.WriteJSON(w, http.StatusOK, procs)
}

// handleGetProcess returns a single process by ID.
func (s *Server) handleGetProcess(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	proc, err := s.supervisor.Get(id)
	if err != nil {
		httputil.WriteJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, proc.Info())
}

// handleStartProcess creates and starts a new process.
func (s *Server) handleStartProcess(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxWebBody)
	var cfg types.ProcessConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	proc, err := s.supervisor.AddProcess(r.Context(), cfg)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, proc.Info())
}

// handleStopProcess stops a running process.
func (s *Server) handleStopProcess(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	force := r.URL.Query().Get("force") == "true"
	timeout := 5 * time.Second
	if t := r.URL.Query().Get("timeout"); t != "" {
		if d, err := time.ParseDuration(t); err == nil {
			timeout = d
		}
	}

	if err := s.supervisor.StopProcess(id, force, timeout); err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// handleRestartProcess restarts a process.
func (s *Server) handleRestartProcess(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.supervisor.RestartProcess(r.Context(), id); err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "restarted"})
}

// handleReloadProcess reloads a process.
func (s *Server) handleReloadProcess(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.supervisor.ReloadProcess(r.Context(), id); err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "reloaded"})
}

// handleDeleteProcess removes a process.
func (s *Server) handleDeleteProcess(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.supervisor.RemoveProcess(id); err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleGetLogs returns the last N lines of a process log.
func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	proc, err := s.supervisor.Get(id)
	if err != nil {
		httputil.WriteJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	info := proc.Info()
	logPath := s.supervisor.LogPath(info.Name)

	lines := 50
	if l := r.URL.Query().Get("lines"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			lines = n
		}
	}

	data, err := logutil.ReadFileBounded(logPath, 1<<20) // 1 MiB max
	if err != nil {
		if os.IsNotExist(err) {
			httputil.WriteJSON(w, http.StatusOK, map[string]string{"logs": ""})
			return
		}
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	content := logutil.TailLines(string(data), lines)
	httputil.WriteJSON(w, http.StatusOK, map[string]string{
		"logs":  content,
		"path":  filepath.Base(logPath),
		"lines": strconv.Itoa(lines),
	})
}

// handleSystemMetrics returns system-wide resource usage.
func (s *Server) handleSystemMetrics(w http.ResponseWriter, r *http.Request) {
	sm := metrics.GetSystemMetrics()
	httputil.WriteJSON(w, http.StatusOK, sm)
}
