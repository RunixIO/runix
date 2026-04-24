package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/runixio/runix/internal/supervisor"
	"github.com/runixio/runix/pkg/types"
)

func setupTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sup := supervisor.New(supervisor.Options{
		LogDir: logDir,
		Defaults: types.DefaultsConfig{
			RestartPolicy: types.RestartNever,
			MaxRestarts:   0,
		},
	})
	return NewServer(sup, "127.0.0.1:0", types.AuthConfig{})
}

func TestListProcessesEmpty(t *testing.T) {
	srv := setupTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/processes", nil)
	w := httptest.NewRecorder()
	srv.handleListProcesses(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var procs []types.ProcessInfo
	if err := json.NewDecoder(w.Body).Decode(&procs); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(procs) != 0 {
		t.Errorf("expected empty, got %d", len(procs))
	}
}

func TestGetProcessNotFound(t *testing.T) {
	srv := setupTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/processes/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()
	srv.handleGetProcess(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestStartAndListProcess(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.supervisor.Shutdown()

	body := `{"name":"test-sleep","entrypoint":"sleep","args":["60"],"runtime":"unknown"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/processes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleStartProcess(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var info types.ProcessInfo
	if err := json.NewDecoder(w.Body).Decode(&info); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if info.Name != "test-sleep" {
		t.Errorf("expected 'test-sleep', got %q", info.Name)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/processes", nil)
	w2 := httptest.NewRecorder()
	srv.handleListProcesses(w2, req2)
	var procs []types.ProcessInfo
	json.NewDecoder(w2.Body).Decode(&procs)
	if len(procs) != 1 {
		t.Errorf("expected 1 process, got %d", len(procs))
	}
}

func TestStopProcess(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.supervisor.Shutdown()

	srv.supervisor.AddProcess(context.Background(), types.ProcessConfig{
		Name:          "stop-test",
		Entrypoint:    "sleep",
		Args:          []string{"60"},
		Runtime:       "unknown",
		RestartPolicy: types.RestartNever,
	})
	time.Sleep(100 * time.Millisecond)

	procs := srv.supervisor.List()
	if len(procs) == 0 {
		t.Fatal("expected process running")
	}
	id := procs[0].ID[:8]

	req := httptest.NewRequest(http.MethodPost, "/api/v1/processes/"+id+"/stop", nil)
	req.SetPathValue("id", id)
	w := httptest.NewRecorder()
	srv.handleStopProcess(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
