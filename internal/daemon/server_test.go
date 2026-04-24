package daemon

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/runixio/runix/internal/logutil"
	"github.com/runixio/runix/internal/supervisor"
	"github.com/runixio/runix/pkg/types"
)

// newTestServer creates a Server + Client pair backed by a temp directory.
// The returned cleanup function shuts down the HTTP server and supervisor.
func newTestServer(t *testing.T) (*Server, *Client, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "runix.sock")
	logDir := filepath.Join(tmpDir, "logs")

	sup := supervisor.New(supervisor.Options{
		LogDir: logDir,
	})

	srv := NewServer(sup, socketPath, tmpDir, nil, nil, "")

	// Set up the listener and HTTP server manually so we control the lifecycle
	// without depending on the blocking Start method.
	_ = os.Remove(socketPath)
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		t.Fatalf("mkdir socket dir: %v", err)
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv.listener = listener

	mux := http.NewServeMux()
	mux.HandleFunc("/api/start", srv.handleStart)
	mux.HandleFunc("/api/start_all", srv.handleStartAll)
	mux.HandleFunc("/api/stop", srv.handleStop)
	mux.HandleFunc("/api/restart", srv.handleRestart)
	mux.HandleFunc("/api/reload", srv.handleReload)
	mux.HandleFunc("/api/delete", srv.handleDelete)
	mux.HandleFunc("/api/list", srv.handleList)
	mux.HandleFunc("/api/status", srv.handleStatus)
	mux.HandleFunc("/api/logs", srv.handleLogs)
	mux.HandleFunc("/api/save", srv.handleSave)
	mux.HandleFunc("/api/resurrect", srv.handleResurrect)
	mux.HandleFunc("/api/ping", srv.handlePing)

	srv.httpServer = &http.Server{Handler: mux}

	go func() {
		if err := srv.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			t.Logf("http serve: %v", err)
		}
	}()

	client := NewClient(socketPath)

	// Wait until the server is reachable.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if client.IsAlive() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !client.IsAlive() {
		t.Fatal("server did not become ready")
	}

	cleanup := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.httpServer.Shutdown(shutdownCtx)
		_ = sup.Shutdown()
	}

	return srv, client, cleanup
}

// addTestProcess starts a long-running process directly through the supervisor
// using context.Background(), bypassing the IPC layer's request-scoped context.
// This keeps the process alive for stop/list/status tests.
func addTestProcess(t *testing.T, srv *Server, name string) string {
	t.Helper()

	proc, err := srv.supervisor.AddProcess(context.Background(), types.ProcessConfig{
		Name:          name,
		Entrypoint:    "sleep",
		Args:          []string{"60"},
		RestartPolicy: types.RestartNever,
	})
	if err != nil {
		t.Fatalf("add process %q: %v", name, err)
	}
	return proc.ID
}

func TestServerStopServiceGroup(t *testing.T) {
	srv, client, cleanup := newTestServer(t)
	defer cleanup()

	addTestProcess(t, srv, "api:0")
	addTestProcess(t, srv, "api:1")

	payload, _ := json.Marshal(StopPayload{Target: "api"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Send(ctx, Request{Action: ActionStop, Payload: payload})
	if err != nil {
		t.Fatalf("stop group: %v", err)
	}
	if !resp.Success {
		t.Fatalf("stop group failed: %s", resp.Error)
	}

	for _, name := range []string{"api:0", "api:1"} {
		proc, err := srv.supervisor.Get(name)
		if err != nil {
			t.Fatalf("Get(%q): %v", name, err)
		}
		if state := proc.GetState(); state != types.StateStopped {
			t.Fatalf("expected %q to be stopped, got %s", name, state)
		}
	}
}

func TestServerPing(t *testing.T) {
	_, client, cleanup := newTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req := Request{Action: ActionPing}
	resp, err := client.Send(ctx, req)
	if err != nil {
		t.Fatalf("ping: %v", err)
	}
	if !resp.Success {
		t.Fatalf("ping failed: %s", resp.Error)
	}

	var data map[string]string
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("unmarshal ping response: %v", err)
	}
	if data["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", data["status"])
	}
}

func TestServerStartStop(t *testing.T) {
	srv, client, cleanup := newTestServer(t)
	defer cleanup()

	// Start a long-running process directly through the supervisor so it stays
	// alive (the IPC handleStart uses the request context which is cancelled
	// when the client disconnects, killing the child process).
	procID := addTestProcess(t, srv, "test-sleep")

	// Stop the process via IPC.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stopPayload, _ := json.Marshal(StopPayload{Target: procID})
	stopReq := Request{Action: ActionStop, Payload: stopPayload}

	resp, err := client.Send(ctx, stopReq)
	if err != nil {
		t.Fatalf("stop process: %v", err)
	}
	if !resp.Success {
		t.Fatalf("stop process failed: %s", resp.Error)
	}

	var stopResult map[string]string
	if err := json.Unmarshal(resp.Data, &stopResult); err != nil {
		t.Fatalf("unmarshal stop response: %v", err)
	}
	if stopResult["status"] != "stopped" {
		t.Fatalf("expected status stopped, got %q", stopResult["status"])
	}
}

func TestServerList(t *testing.T) {
	srv, client, cleanup := newTestServer(t)
	defer cleanup()

	// Start a long-running process directly through the supervisor.
	procID := addTestProcess(t, srv, "test-list-proc")

	// List processes via IPC.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	listReq := Request{Action: ActionList}
	resp, err := client.Send(ctx, listReq)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !resp.Success {
		t.Fatalf("list failed: %s", resp.Error)
	}

	var infos []types.ProcessInfo
	if err := json.Unmarshal(resp.Data, &infos); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}

	found := false
	for _, info := range infos {
		if info.ID == procID && info.Name == "test-list-proc" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("process %q not found in list (got %d entries)", procID, len(infos))
	}
}

func TestClientIsAlive(t *testing.T) {
	// With no server, IsAlive should return false.
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "nonexistent.sock")
	client := NewClient(socketPath)

	if client.IsAlive() {
		t.Fatal("expected IsAlive=false when no server is running")
	}

	// Start a real server.
	_, aliveClient, cleanup := newTestServer(t)
	defer cleanup()

	if !aliveClient.IsAlive() {
		t.Fatal("expected IsAlive=true when server is running")
	}
}

func TestPIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	pf := NewPIDFile(tmpDir)

	// Initially, no file exists.
	pid, err := pf.Read()
	if err != nil {
		t.Fatalf("read empty pid file: %v", err)
	}
	if pid != 0 {
		t.Fatalf("expected pid 0 for missing file, got %d", pid)
	}

	// Write the PID file.
	if err := pf.Write(); err != nil {
		t.Fatalf("write pid file: %v", err)
	}

	// Read it back.
	pid, err = pf.Read()
	if err != nil {
		t.Fatalf("read pid file: %v", err)
	}
	if pid != os.Getpid() {
		t.Fatalf("expected pid %d, got %d", os.Getpid(), pid)
	}

	// IsRunning should return true for our own process.
	if !pf.IsRunning() {
		t.Fatal("expected IsRunning=true for current process")
	}

	// Verify the file exists on disk.
	if _, err := os.Stat(pf.Path); os.IsNotExist(err) {
		t.Fatal("pid file should exist after Write")
	}

	// Remove the PID file.
	if err := pf.Remove(); err != nil {
		t.Fatalf("remove pid file: %v", err)
	}

	// File should be gone.
	if _, err := os.Stat(pf.Path); !os.IsNotExist(err) {
		t.Fatal("pid file should be removed after Remove")
	}

	// Remove again (idempotent).
	if err := pf.Remove(); err != nil {
		t.Fatalf("remove pid file (already gone): %v", err)
	}

	// Write a bogus PID to test IsRunning returns false.
	if err := os.WriteFile(pf.Path, []byte("999999999"), 0o644); err != nil {
		t.Fatalf("write bogus pid: %v", err)
	}
	if pf.IsRunning() {
		t.Fatal("expected IsRunning=false for non-existent PID")
	}
}

func TestTailLines(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		n      int
		expect string
	}{
		{
			name:   "empty string",
			input:  "",
			n:      5,
			expect: "",
		},
		{
			name:   "fewer newlines than n",
			input:  "line1\nline2\n",
			n:      10,
			expect: "line1\nline2\n",
		},
		{
			name:   "n equals number of newlines",
			input:  "a\nb\nc\n",
			n:      3,
			expect: "b\nc\n",
		},
		{
			name:   "more newlines than n",
			input:  "a\nb\nc\nd\ne\nf\n",
			n:      3,
			expect: "e\nf\n",
		},
		{
			name:   "single trailing newline n=1",
			input:  "only\n",
			n:      1,
			expect: "only\n",
		},
		{
			name:   "zero n returns full string",
			input:  "a\nb\nc\n",
			n:      0,
			expect: "a\nb\nc\n",
		},
		{
			name:   "negative n returns full string",
			input:  "a\nb\nc\n",
			n:      -1,
			expect: "a\nb\nc\n",
		},
		{
			name:   "no trailing newline",
			input:  "a\nb\nc",
			n:      1,
			expect: "c",
		},
		{
			name:   "two lines n=1",
			input:  "a\nb\nc\n",
			n:      1,
			expect: "a\nb\nc\n",
		},
		{
			name:   "two lines n=2",
			input:  "a\nb\nc\n",
			n:      2,
			expect: "c\n",
		},
		{
			name:   "multi line n=2",
			input:  "a\nb\nc\nd\ne\nf\n",
			n:      2,
			expect: "f\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := logutil.TailLines(tt.input, tt.n)
			if got != tt.expect {
				t.Errorf("TailLines(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.expect)
			}
		})
	}
}

func TestServerStartAllExpandsInstances(t *testing.T) {
	srv, client, cleanup := newTestServer(t)
	defer cleanup()

	srv.config = &types.RunixConfig{
		Processes: []types.ProcessConfig{
			{Name: "api", Entrypoint: "sleep", Args: []string{"60"}, Autostart: true, Instances: 2, RestartPolicy: types.RestartNever},
		},
	}

	payload, _ := json.Marshal(StartAllPayload{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Send(ctx, Request{Action: ActionStartAll, Payload: payload})
	if err != nil {
		t.Fatalf("start_all: %v", err)
	}
	if !resp.Success {
		t.Fatalf("start_all failed: %s", resp.Error)
	}

	procs := srv.supervisor.List()
	if len(procs) != 2 {
		t.Fatalf("expected 2 started processes, got %d", len(procs))
	}
	if procs[0].Config.Name != "api:0" || procs[1].Config.Name != "api:1" {
		t.Fatalf("unexpected started names: %q, %q", procs[0].Config.Name, procs[1].Config.Name)
	}
}
