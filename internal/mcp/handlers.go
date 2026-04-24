package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/runixio/runix/pkg/types"
)

// handleListApps lists all managed processes.
func (s *MCPServer) handleListApps(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	procs := s.supervisor.List()
	if len(procs) == 0 {
		return mcp.NewToolResultText("No processes running"), nil
	}

	data, err := json.MarshalIndent(procs, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal processes: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// handleStartApp starts a new managed process.
func (s *MCPServer) handleStartApp(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	entrypoint := mcp.ParseString(request, "entrypoint", "")
	if entrypoint == "" {
		return mcp.NewToolResultError("entrypoint is required"), nil
	}

	cfg := types.ProcessConfig{
		Name:          mcp.ParseString(request, "name", ""),
		Entrypoint:    entrypoint,
		Runtime:       mcp.ParseString(request, "runtime", ""),
		Cwd:           mcp.ParseString(request, "cwd", ""),
		RestartPolicy: types.RestartPolicy(mcp.ParseString(request, "restart_policy", "")),
	}

	if cfg.Name == "" {
		cfg.Name = entrypoint
	}

	// Extract env map from arguments.
	if args := request.GetArguments(); args != nil {
		if raw, ok := args["env"]; ok {
			if envMap, ok := raw.(map[string]interface{}); ok {
				cfg.Env = make(map[string]string, len(envMap))
				for k, v := range envMap {
					cfg.Env[k] = fmt.Sprintf("%v", v)
				}
			}
		}
	}

	proc, err := s.supervisor.AddProcess(ctx, cfg)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to start process: %v", err)), nil
	}

	// Track metrics for the new process.
	info := proc.Info()
	if info.PID > 0 && s.metrics != nil {
		s.metrics.Track(info.PID)
	}

	return mcp.NewToolResultText(fmt.Sprintf("Process %q started (id: %s, pid: %d)", info.Name, info.ID[:8], info.PID)), nil
}

// handleStopApp stops a managed process.
func (s *MCPServer) handleStopApp(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	target := mcp.ParseString(request, "target", "")
	if target == "" {
		return mcp.NewToolResultError("target is required"), nil
	}

	force := mcp.ParseBoolean(request, "force", false)
	timeout := 5 * time.Second

	if err := s.supervisor.StopProcess(target, force, timeout); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to stop %q: %v", target, err)), nil
	}

	// Untrack metrics.
	if proc, err := s.supervisor.Get(target); err == nil {
		if info := proc.Info(); info.PID > 0 && s.metrics != nil {
			s.metrics.Untrack(info.PID)
		}
	}

	return mcp.NewToolResultText(fmt.Sprintf("Process %q stopped", target)), nil
}

// handleRestartApp restarts a managed process.
func (s *MCPServer) handleRestartApp(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	target := mcp.ParseString(request, "target", "")
	if target == "" {
		return mcp.NewToolResultError("target is required"), nil
	}

	if err := s.supervisor.RestartProcess(ctx, target); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to restart %q: %v", target, err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Process %q restarted", target)), nil
}

// handleReloadApp performs a graceful reload of a managed process.
func (s *MCPServer) handleReloadApp(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	target := mcp.ParseString(request, "target", "")
	if target == "" {
		return mcp.NewToolResultError("target is required"), nil
	}

	if err := s.supervisor.ReloadProcess(ctx, target); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to reload %q: %v", target, err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Process %q reloaded", target)), nil
}

// handleDeleteApp stops and removes a process from the process table.
func (s *MCPServer) handleDeleteApp(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	target := mcp.ParseString(request, "target", "")
	if target == "" {
		return mcp.NewToolResultError("target is required"), nil
	}

	if err := s.supervisor.RemoveProcess(target); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to delete %q: %v", target, err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Process %q deleted", target)), nil
}

// handleGetStatus returns detailed status of a specific process.
func (s *MCPServer) handleGetStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	target := mcp.ParseString(request, "target", "")
	if target == "" {
		return mcp.NewToolResultError("target is required"), nil
	}

	proc, err := s.supervisor.Get(target)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("process not found: %v", err)), nil
	}

	info := proc.Info()
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal: %v", err)), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}

// handleGetLogs returns recent log output from a process.
func (s *MCPServer) handleGetLogs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	target := mcp.ParseString(request, "target", "")
	if target == "" {
		return mcp.NewToolResultError("target is required"), nil
	}

	proc, err := s.supervisor.Get(target)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("process not found: %v", err)), nil
	}

	info := proc.Info()
	logPath := s.supervisor.LogPath(info.Name)

	data, err := readLogLines(logPath, mcp.ParseInt(request, "lines", 50))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to read logs: %v", err)), nil
	}

	if data == "" {
		return mcp.NewToolResultText("(no logs available)"), nil
	}
	return mcp.NewToolResultText(data), nil
}

// handleSaveState persists the current process table to disk.
func (s *MCPServer) handleSaveState(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := s.supervisor.Save(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to save: %v", err)), nil
	}
	procs := s.supervisor.List()
	return mcp.NewToolResultText(fmt.Sprintf("Saved state for %d process(es)", len(procs))), nil
}

// handleResurrect restores and starts previously saved processes.
func (s *MCPServer) handleResurrect(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := s.supervisor.Resurrect(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resurrect: %v", err)), nil
	}
	procs := s.supervisor.List()
	return mcp.NewToolResultText(fmt.Sprintf("Resurrected %d process(es)", len(procs))), nil
}

// readLogLines reads the last N lines from a log file.
func readLogLines(path string, numLines int) (string, error) {
	data, err := readFile(path)
	if err != nil {
		return "", err
	}
	if data == "" {
		return "", nil
	}

	content := data
	count := 0
	idx := len(content)
	for i := len(content) - 1; i >= 0; i-- {
		if content[i] == '\n' {
			count++
			if count == numLines {
				idx = i + 1
				break
			}
		}
	}
	if idx < len(content) {
		return content[idx:], nil
	}
	return content, nil
}

// readFile is a testable file reader, bounded to maxLogBytes.
var readFile = func(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return "", err
	}

	const maxLogBytes = 1 << 20 // 1 MiB
	if fi.Size() <= maxLogBytes {
		d, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(d), nil
	}

	// Large file: seek to tail.
	if _, err := f.Seek(-maxLogBytes, 2); err != nil { // io.SeekEnd = 2
		return "", err
	}
	d, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(d), nil
}
