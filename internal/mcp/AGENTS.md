# internal/mcp/ — MCP Server

Model Context Protocol server for AI agent integration. Exposes process management via stdio or HTTP transports.

## Files

| File                 | Purpose                                                      |
| -------------------- | ------------------------------------------------------------ |
| `server.go`          | MCP server setup, tool/resource registration                 |
| `tools.go`           | Tool definitions: start, stop, restart, list, status, logs   |
| `handlers.go`        | Tool implementation, maps MCP calls to supervisor operations |
| `resources.go`       | MCP resources: process list, individual process state        |
| `transport_stdio.go` | Stdio transport (default)                                    |
| `transport_http.go`  | HTTP/SSE transport for remote access                         |
| `server_test.go`     | Integration tests (531 lines)                                |

## Transport Modes

- **stdio** (default): reads JSON-RPC from stdin, writes to stdout
- **http**: SSE-based streaming on configurable address, 5s shutdown timeout

## Tool Surface

Each tool maps to a supervisor operation: `list_apps`, `start_app`, `stop_app`, `restart_app`, `reload_app`, `delete_app`, `get_status`, `get_logs`, `save_state`, `resurrect_processes`. Handlers use the supervisor directly (not daemon IPC).

## Testing

Testable file reader via function variable: `var readFile = func(path string) (string, error)` in `handlers.go:234` — allows mocking in tests without filesystem dependency.
