package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/runixio/runix/internal/metrics"
	"github.com/runixio/runix/internal/supervisor"
	"github.com/runixio/runix/pkg/types"
)

func setupTestMCPServer(t *testing.T) *MCPServer {
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
	col := metrics.NewCollector()
	return NewMCPServer(sup, col)
}

func getTextContent(result *mcp.CallToolResult) string {
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

// --- Tool tests ---

func TestListAppsEmpty(t *testing.T) {
	srv := setupTestMCPServer(t)
	req := mcp.CallToolRequest{Params: mcp.CallToolParams{Name: "list_apps"}}
	result, err := srv.handleListApps(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
}

func TestStartApp(t *testing.T) {
	srv := setupTestMCPServer(t)
	defer srv.supervisor.Shutdown()

	req := mcp.CallToolRequest{Params: mcp.CallToolParams{
		Name: "start_app",
		Arguments: map[string]interface{}{
			"entrypoint": "sleep",
			"args":       []interface{}{"60"},
			"name":       "test-proc",
		},
	}}
	result, err := srv.handleStartApp(context.Background(), req)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("start returned error: %v", result.Content)
	}
}

func TestStartAppWithEnv(t *testing.T) {
	srv := setupTestMCPServer(t)
	defer srv.supervisor.Shutdown()

	req := mcp.CallToolRequest{Params: mcp.CallToolParams{
		Name: "start_app",
		Arguments: map[string]interface{}{
			"entrypoint": "sleep",
			"args":       []interface{}{"60"},
			"name":       "env-proc",
			"env": map[string]interface{}{
				"FOO": "bar",
				"BAZ": "123",
			},
		},
	}}
	result, err := srv.handleStartApp(context.Background(), req)
	if err != nil {
		t.Fatalf("start with env failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("start with env returned error: %v", result.Content)
	}
}

func TestStartAndListApps(t *testing.T) {
	srv := setupTestMCPServer(t)
	defer srv.supervisor.Shutdown()

	// Start a process.
	startReq := mcp.CallToolRequest{Params: mcp.CallToolParams{
		Name: "start_app",
		Arguments: map[string]interface{}{
			"entrypoint": "sleep",
			"args":       []interface{}{"60"},
			"name":       "test-proc",
		},
	}}
	result, err := srv.handleStartApp(context.Background(), startReq)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("start returned error: %v", result.Content)
	}

	// List processes.
	listReq := mcp.CallToolRequest{Params: mcp.CallToolParams{Name: "list_apps"}}
	result2, err := srv.handleListApps(context.Background(), listReq)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	var procs []types.ProcessInfo
	content := getTextContent(result2)
	if err := json.Unmarshal([]byte(content), &procs); err != nil {
		t.Fatalf("failed to parse list: %v", err)
	}
	if len(procs) != 1 {
		t.Fatalf("expected 1 process, got %d", len(procs))
	}
	if procs[0].Name != "test-proc" {
		t.Errorf("expected name 'test-proc', got %q", procs[0].Name)
	}
}

func TestStopApp(t *testing.T) {
	srv := setupTestMCPServer(t)
	defer srv.supervisor.Shutdown()

	srv.supervisor.AddProcess(context.Background(), types.ProcessConfig{
		Name:          "stop-target",
		Entrypoint:    "sleep",
		Args:          []string{"60"},
		Runtime:       "unknown",
		RestartPolicy: types.RestartNever,
	})

	procs := srv.supervisor.List()
	if len(procs) == 0 {
		t.Fatal("expected process running")
	}

	req := mcp.CallToolRequest{Params: mcp.CallToolParams{
		Name: "stop_app",
		Arguments: map[string]interface{}{
			"target": procs[0].ID[:8],
		},
	}}
	result, err := srv.handleStopApp(context.Background(), req)
	if err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("stop returned error: %v", result.Content)
	}
}

func TestRestartApp(t *testing.T) {
	srv := setupTestMCPServer(t)
	defer srv.supervisor.Shutdown()

	srv.supervisor.AddProcess(context.Background(), types.ProcessConfig{
		Name:          "restart-target",
		Entrypoint:    "sleep",
		Args:          []string{"60"},
		Runtime:       "unknown",
		RestartPolicy: types.RestartNever,
	})

	procs := srv.supervisor.List()
	if len(procs) == 0 {
		t.Fatal("expected process running")
	}

	req := mcp.CallToolRequest{Params: mcp.CallToolParams{
		Name: "restart_app",
		Arguments: map[string]interface{}{
			"target": procs[0].ID[:8],
		},
	}}
	result, err := srv.handleRestartApp(context.Background(), req)
	if err != nil {
		t.Fatalf("restart failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("restart returned error: %v", result.Content)
	}
}

func TestReloadApp(t *testing.T) {
	srv := setupTestMCPServer(t)
	defer srv.supervisor.Shutdown()

	srv.supervisor.AddProcess(context.Background(), types.ProcessConfig{
		Name:          "reload-target",
		Entrypoint:    "sleep",
		Args:          []string{"60"},
		Runtime:       "unknown",
		RestartPolicy: types.RestartNever,
	})

	procs := srv.supervisor.List()
	if len(procs) == 0 {
		t.Fatal("expected process running")
	}

	req := mcp.CallToolRequest{Params: mcp.CallToolParams{
		Name: "reload_app",
		Arguments: map[string]interface{}{
			"target": procs[0].ID[:8],
		},
	}}
	result, err := srv.handleReloadApp(context.Background(), req)
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("reload returned error: %v", result.Content)
	}
}

func TestDeleteApp(t *testing.T) {
	srv := setupTestMCPServer(t)
	defer srv.supervisor.Shutdown()

	srv.supervisor.AddProcess(context.Background(), types.ProcessConfig{
		Name:          "delete-target",
		Entrypoint:    "sleep",
		Args:          []string{"60"},
		Runtime:       "unknown",
		RestartPolicy: types.RestartNever,
	})

	procs := srv.supervisor.List()
	if len(procs) == 0 {
		t.Fatal("expected process running")
	}

	req := mcp.CallToolRequest{Params: mcp.CallToolParams{
		Name: "delete_app",
		Arguments: map[string]interface{}{
			"target": procs[0].ID[:8],
		},
	}}
	result, err := srv.handleDeleteApp(context.Background(), req)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("delete returned error: %v", result.Content)
	}

	// Verify it's gone.
	procs = srv.supervisor.List()
	if len(procs) != 0 {
		t.Fatalf("expected 0 processes after delete, got %d", len(procs))
	}
}

func TestGetStatusNotFound(t *testing.T) {
	srv := setupTestMCPServer(t)
	req := mcp.CallToolRequest{Params: mcp.CallToolParams{
		Name: "get_status",
		Arguments: map[string]interface{}{
			"target": "nonexistent",
		},
	}}
	result, err := srv.handleGetStatus(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Error("expected error for nonexistent process")
	}
}

func TestGetStatus(t *testing.T) {
	srv := setupTestMCPServer(t)
	defer srv.supervisor.Shutdown()

	srv.supervisor.AddProcess(context.Background(), types.ProcessConfig{
		Name:          "status-check",
		Entrypoint:    "sleep",
		Args:          []string{"60"},
		Runtime:       "unknown",
		RestartPolicy: types.RestartNever,
	})

	procs := srv.supervisor.List()
	if len(procs) == 0 {
		t.Fatal("expected process running")
	}

	req := mcp.CallToolRequest{Params: mcp.CallToolParams{
		Name: "get_status",
		Arguments: map[string]interface{}{
			"target": procs[0].ID[:8],
		},
	}}
	result, err := srv.handleGetStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("get_status failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("get_status returned error: %v", result.Content)
	}

	var info types.ProcessInfo
	if err := json.Unmarshal([]byte(getTextContent(result)), &info); err != nil {
		t.Fatalf("failed to parse status: %v", err)
	}
	if info.Name != "status-check" {
		t.Errorf("expected name 'status-check', got %q", info.Name)
	}
}

func TestGetLogs(t *testing.T) {
	srv := setupTestMCPServer(t)
	defer srv.supervisor.Shutdown()

	srv.supervisor.AddProcess(context.Background(), types.ProcessConfig{
		Name:          "log-check",
		Entrypoint:    "sleep",
		Args:          []string{"60"},
		Runtime:       "unknown",
		RestartPolicy: types.RestartNever,
	})

	procs := srv.supervisor.List()
	if len(procs) == 0 {
		t.Fatal("expected process running")
	}

	req := mcp.CallToolRequest{Params: mcp.CallToolParams{
		Name: "get_logs",
		Arguments: map[string]interface{}{
			"target": procs[0].ID[:8],
			"lines":  10,
		},
	}}
	result, err := srv.handleGetLogs(context.Background(), req)
	if err != nil {
		t.Fatalf("get_logs failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("get_logs returned error: %v", result.Content)
	}
}

func TestSaveAndResurrect(t *testing.T) {
	srv := setupTestMCPServer(t)
	defer srv.supervisor.Shutdown()

	saveReq := mcp.CallToolRequest{Params: mcp.CallToolParams{Name: "save_state"}}
	result, err := srv.handleSaveState(context.Background(), saveReq)
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	_ = result

	resReq := mcp.CallToolRequest{Params: mcp.CallToolParams{Name: "resurrect_processes"}}
	result2, err := srv.handleResurrect(context.Background(), resReq)
	if err != nil {
		t.Fatalf("resurrect failed: %v", err)
	}
	_ = result2
}

func TestMissingRequiredParam(t *testing.T) {
	srv := setupTestMCPServer(t)

	req := mcp.CallToolRequest{Params: mcp.CallToolParams{
		Name: "start_app",
		Arguments: map[string]interface{}{
			"name": "no-entrypoint",
		},
	}}
	result, err := srv.handleStartApp(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Error("expected error for missing entrypoint")
	}
}

func TestMissingTargetParam(t *testing.T) {
	srv := setupTestMCPServer(t)

	tools := []string{"stop_app", "restart_app", "reload_app", "delete_app", "get_status", "get_logs"}
	for _, toolName := range tools {
		req := mcp.CallToolRequest{Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: map[string]interface{}{},
		}}

		var result *mcp.CallToolResult
		var err error
		switch toolName {
		case "stop_app":
			result, err = srv.handleStopApp(context.Background(), req)
		case "restart_app":
			result, err = srv.handleRestartApp(context.Background(), req)
		case "reload_app":
			result, err = srv.handleReloadApp(context.Background(), req)
		case "delete_app":
			result, err = srv.handleDeleteApp(context.Background(), req)
		case "get_status":
			result, err = srv.handleGetStatus(context.Background(), req)
		case "get_logs":
			result, err = srv.handleGetLogs(context.Background(), req)
		}

		if err != nil {
			t.Fatalf("%s: unexpected error: %v", toolName, err)
		}
		if !result.IsError {
			t.Errorf("%s: expected error for missing target", toolName)
		}
	}
}

// --- Resource tests ---

func TestAppListResource(t *testing.T) {
	srv := setupTestMCPServer(t)
	defer srv.supervisor.Shutdown()

	req := mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "apps://list"}}
	result, err := srv.handleAppListResource(context.Background(), req)
	if err != nil {
		t.Fatalf("resource error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("expected resource content")
	}
}

func TestAppResourceNotFound(t *testing.T) {
	srv := setupTestMCPServer(t)

	req := mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "apps://nonexistent"}}
	_, err := srv.handleAppResource(context.Background(), req)
	if err == nil {
		t.Error("expected error for nonexistent process")
	}
}

func TestLogsResourceNotFound(t *testing.T) {
	srv := setupTestMCPServer(t)

	req := mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "logs://nonexistent"}}
	_, err := srv.handleLogsResource(context.Background(), req)
	if err == nil {
		t.Error("expected error for nonexistent process")
	}
}

func TestMetricsResourceNotFound(t *testing.T) {
	srv := setupTestMCPServer(t)

	req := mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "metrics://nonexistent"}}
	_, err := srv.handleMetricsResource(context.Background(), req)
	if err == nil {
		t.Error("expected error for nonexistent process")
	}
}

func TestExtractURISegment(t *testing.T) {
	tests := []struct {
		uri    string
		prefix string
		want   string
	}{
		{"apps://my-app", "apps://", "my-app"},
		{"logs://sleep-1", "logs://", "sleep-1"},
		{"metrics://abc123", "metrics://", "abc123"},
		{"other://thing", "apps://", "other://thing"},
	}
	for _, tt := range tests {
		got := extractURISegment(tt.uri, tt.prefix)
		if got != tt.want {
			t.Errorf("extractURISegment(%q, %q) = %q, want %q", tt.uri, tt.prefix, got, tt.want)
		}
	}
}

// --- Transport tests ---

func TestNewMCPServerRegistration(t *testing.T) {
	srv := setupTestMCPServer(t)
	if srv.mcpServer == nil {
		t.Fatal("expected mcpServer to be initialized")
	}
	if srv.supervisor == nil {
		t.Fatal("expected supervisor to be set")
	}
	if srv.metrics == nil {
		t.Fatal("expected metrics to be set")
	}
}

func TestStartInvalidTransport(t *testing.T) {
	srv := setupTestMCPServer(t)
	cfg := types.MCPConfig{Transport: "invalid"}
	err := srv.Start(context.Background(), cfg)
	if err == nil {
		t.Error("expected error for invalid transport")
	}
}
