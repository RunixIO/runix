# internal/web/ — Web Dashboard

Chi HTTP server + WebSocket hub for real-time process monitoring. Frontend embedded via `//go:embed`.

## Files

| File | Purpose |
|------|---------|
| `server.go` | Server setup, static file serving from embedded `frontend/`, 5s shutdown |
| `api.go` | REST endpoints: GET/POST for list, start, stop, restart, status, logs |
| `websocket.go` | `WSHub` broadcasts `ProcessInfo` updates to connected clients |
| `frontend/index.html` | Single-page dashboard, embedded at build time |
| `api_test.go` | httptest-based tests for REST endpoints |

## REST API

- `GET /api/processes` — list all processes
- `GET /api/processes/{id}` — single process status
- `POST /api/processes/{id}/start` — start process
- `POST /api/processes/{id}/stop` — stop process
- `POST /api/processes/{id}/restart` — restart process
- `GET /api/processes/{id}/logs` — tail logs (query: `?lines=N`)

## WebSocket

`WSHub` runs a goroutine that: accepts client registrations, broadcasts `ProcessInfo` JSON on state changes. Thread-safe via `sync.RWMutex` on clients map.

## Embedding

`//go:embed frontend` at `server.go:19` — the `frontend/` directory is baked into the binary. No external file serving needed.
