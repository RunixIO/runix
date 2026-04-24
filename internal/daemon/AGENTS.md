# internal/daemon/ — IPC Layer

HTTP-over-Unix-socket communication between CLI client and background daemon.

## Architecture

```
CLI (client.go) ──HTTP over Unix socket──> Daemon (server.go) ──> Supervisor
```

## Files

| File | Purpose |
|------|---------|
| `protocol.go` | Action constants, Request/Response types, payload structs |
| `server.go` | Chi HTTP server on Unix socket, action → handler routing |
| `client.go` | HTTP client dialing Unix socket, `IsAlive()` with 2s timeout |
| `daemon.go` | `StartDaemon()` (fork subprocess), `RunDaemon()` (server lifecycle) |
| `pidfile.go` | PID file management for daemon process tracking |
| `server_test.go` | Integration tests with real Unix socket |

## Adding a New IPC Action

1. `protocol.go`: add `ActionFoo = "foo"` constant + `FooPayload` struct
2. `server.go`: add `mux.HandleFunc("/api/foo", s.handleFoo)` + handler method
3. `client.go`: add `Foo(payload) (daemon.Response, error)` method
4. `cmd/runix/<command>.go`: call `sendIPC(daemon.ActionFoo, payload)` in dual-mode pattern

## Daemon Lifecycle

- `StartDaemon()`: forks `runix daemon run` with `Setpgid: true`, polls `IsAlive()` up to 5s at 100ms intervals
- `RunDaemon()`: creates data dirs (`~/.runix/{apps,state,tmp}`), supervisor, server. Optionally starts MCP HTTP server
- Server shutdown: 10s graceful timeout on SIGINT/SIGTERM
- SIGHUP handler exists but is not yet implemented (placeholder for config reload)

## Protocol

Request: `{"action": "start", "payload": {...}}`
Response: `{"success": true, "data": {...}}` or `{"success": false, "error": "msg"}`

Helper functions: `ErrorResponse(err)`, `DataResponse(data)`.

## Permissions

- Socket: `0o660` (owner+group read-write)
- PID file: `0o644` (owner rw, group/other read)
