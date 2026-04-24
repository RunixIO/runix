package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// registerTools adds all Runix tools to the MCP server.
func (s *MCPServer) registerTools() {
	s.mcpServer.AddTool(mcp.NewTool("list_apps",
		mcp.WithDescription("List all managed processes with their status, PID, runtime, and resource usage"),
		mcp.WithReadOnlyHintAnnotation(true),
	), s.handleListApps)

	s.mcpServer.AddTool(mcp.NewTool("start_app",
		mcp.WithDescription("Start a new managed process. Auto-detects runtime if not specified."),
		mcp.WithString("entrypoint",
			mcp.Required(),
			mcp.Description("The command or script to run (e.g. 'server.go', 'app.py', 'index.js')"),
		),
		mcp.WithString("name",
			mcp.Description("Process name (default: entrypoint filename)"),
		),
		mcp.WithString("runtime",
			mcp.Description("Runtime to use: go, python, node, bun, auto (default: auto-detect)"),
		),
		mcp.WithString("cwd",
			mcp.Description("Working directory"),
		),
		mcp.WithString("restart_policy",
			mcp.Description("Restart policy: always, on-failure, never"),
		),
		mcp.WithObject("env",
			mcp.Description("Environment variables as key-value pairs"),
		),
	), s.handleStartApp)

	s.mcpServer.AddTool(mcp.NewTool("stop_app",
		mcp.WithDescription("Stop a managed process by ID or name. Use 'all' to stop everything."),
		mcp.WithString("target",
			mcp.Required(),
			mcp.Description("Process ID, name, or 'all'"),
		),
		mcp.WithBoolean("force",
			mcp.Description("Force stop with SIGKILL"),
		),
	), s.handleStopApp)

	s.mcpServer.AddTool(mcp.NewTool("restart_app",
		mcp.WithDescription("Restart a managed process by ID or name"),
		mcp.WithString("target",
			mcp.Required(),
			mcp.Description("Process ID or name"),
		),
	), s.handleRestartApp)

	s.mcpServer.AddTool(mcp.NewTool("reload_app",
		mcp.WithDescription("Gracefully reload a managed process by ID or name (fires reload hooks)"),
		mcp.WithString("target",
			mcp.Required(),
			mcp.Description("Process ID or name"),
		),
	), s.handleReloadApp)

	s.mcpServer.AddTool(mcp.NewTool("delete_app",
		mcp.WithDescription("Stop and remove a process from the process table"),
		mcp.WithString("target",
			mcp.Required(),
			mcp.Description("Process ID or name"),
		),
	), s.handleDeleteApp)

	s.mcpServer.AddTool(mcp.NewTool("get_status",
		mcp.WithDescription("Get detailed status of a specific process including config and resource usage"),
		mcp.WithString("target",
			mcp.Required(),
			mcp.Description("Process ID or name"),
		),
		mcp.WithReadOnlyHintAnnotation(true),
	), s.handleGetStatus)

	s.mcpServer.AddTool(mcp.NewTool("get_logs",
		mcp.WithDescription("Get recent log output from a process"),
		mcp.WithString("target",
			mcp.Required(),
			mcp.Description("Process ID or name"),
		),
		mcp.WithNumber("lines",
			mcp.Description("Number of log lines to return (default: 50)"),
		),
		mcp.WithReadOnlyHintAnnotation(true),
	), s.handleGetLogs)

	s.mcpServer.AddTool(mcp.NewTool("save_state",
		mcp.WithDescription("Save the current process table to disk for later resurrection"),
	), s.handleSaveState)

	s.mcpServer.AddTool(mcp.NewTool("resurrect_processes",
		mcp.WithDescription("Restore previously saved processes and start them"),
	), s.handleResurrect)
}
