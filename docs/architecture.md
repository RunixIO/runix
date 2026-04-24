# Architecture

Runix is structured as a layered system with clear separation between the interface layer, the IPC layer, the supervisor engine, and the runtime adapters.

## System Overview

```
┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
│   CLI    │  │   TUI    │  │  Web UI  │  │   MCP    │
│ (cobra)  │  │(BubbleTea)│  │ (chi+WS) │  │ (mcp-go) │
└────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘
     │              │              │              │
     └──────────────┴──────────────┴──────────────┘
                         │
                ┌────────┴────────┐
                │  IPC / Direct   │
                │ (Unix Socket)   │
                └────────┬────────┘
                         │
                ┌────────┴────────┐
                │   Supervisor    │
                │   Engine        │
                └────────┬────────┘
                         │
     ┌───────────┬───────┼───────┬───────────┐
     │           │       │       │           │
 ┌───┴───┐  ┌───┴──┐ ┌──┴──┐ ┌─┴──┐  ┌────┴────┐
 │  Go   │  │Python│ │Node │ │Bun │  │Watcher  │
 │Adapter│  │Adapter│ │Adapter│ │Adapter│ │Scheduler│
 └───────┘  └──────┘ └─────┘ └────┘  │ Logger  │
                                      │ Metrics │
                                      └─────────┘
```

All interfaces (CLI, TUI, Web UI, MCP) communicate with the supervisor through the same IPC protocol or instantiate it directly.

## Module Breakdown

### `cmd/runix/` — CLI Commands

Each file implements one cobra command. Commands follow a consistent pattern:

1. Attempt to connect to the daemon via IPC client
2. If the daemon is alive, send the request over Unix socket
3. If not, create an in-process supervisor (direct mode) and execute locally

This dual-mode design means every command works with or without a running daemon.

### `internal/supervisor/` — Supervisor Engine

The supervisor is the core of Runix. It manages the full lifecycle of all processes.

**Key types:**

- `Supervisor` — holds a map of `ManagedProcess` by ID, a name-to-ID index, and monitor cancel functions
- `ManagedProcess` — wraps `os/exec.Cmd` with state management, restart tracking, and log writers

**Process state machine:**

```
           start()
 stopped ─────────> starting ─────────> running
    ^                                    │  │
    │                                    │  │ exit (error)
    │                          stop()    │  v
    │                             │      v crashed
    │                     stopping│      │ │
    │                             v      │ │ auto-restart
    │                          stopped<──┘ │ (with backoff)
    │                                      v
    └────────────────────────────────── errored
```

State transitions use lock-free atomic compare-and-swap. The `ValidTransitions` map enforces which transitions are legal.

**Process lookup** resolves targets in priority order:

1. Exact ID match
2. Exact name match
3. Unique ID prefix (ambiguous prefixes are rejected)

### `internal/runtime/` — Runtime Adapters

Each adapter implements the `Runtime` interface:

```go
type Runtime interface {
    Name() string
    Detect(dir string) bool
    StartCmd(opts StartOptions) (*exec.Cmd, error)
}
```

| Adapter     | Detection                                                                | Start Command                                           |
| ----------- | ------------------------------------------------------------------------ | ------------------------------------------------------- |
| **Go**      | `go.mod` exists                                                          | `go run <entrypoint>` or `go run .`                     |
| **Python**  | `requirements.txt`, `pyproject.toml`, `setup.py`, `Pipfile`, `.py` files | `venv/bin/python` (if venv exists) or `python3`         |
| **Node.js** | `package.json` exists                                                    | `node <entrypoint>` or `npx tsx` for `.ts`/`.tsx` files |
| **Bun**     | `bun.lockb`, `bunfig.toml`, `bun.lock` exists                            | `bun run <entrypoint>`                                  |
| **Deno**    | `deno.json`, `deno.jsonc` exists                                         | `deno run [args] <entrypoint>`                          |
| **Ruby**    | `Gemfile`, `Gemfile.lock` exists                                         | `ruby <entrypoint>` or `bundle exec ruby <entrypoint>`  |
| **PHP**     | `composer.json`, `artisan`, `.php` files                                 | `php <entrypoint> [args]`                               |

The `Detector` iterates adapters in order (Go, Python, Node.js, Bun, Deno, Ruby, PHP) and returns the first match. When `--runtime` is specified explicitly, detection is skipped.

### `internal/daemon/` — Daemon and IPC

The daemon provides persistent background process management via HTTP-over-Unix-socket IPC.

**Components:**

- **Server** — listens on a Unix socket, routes HTTP requests to supervisor methods
- **Client** — sends HTTP requests to the Unix socket with a 30-second timeout
- **Protocol** — defines action constants and typed request/response payloads
- **PID file** — tracks the daemon process ID for lifecycle management

**Protocol format:**

```
Request:  POST /api/{action}
          Body: { "action": "<action>", "payload": { ... } }

Response: { "success": true|false, "data": { ... }, "error": "" }
```

**Supported actions:**

| Action           | Description                     |
| ---------------- | ------------------------------- |
| `start`          | Start a new process             |
| `start_all`      | Start all processes from config |
| `stop`           | Stop a process                  |
| `restart`        | Restart a process               |
| `reload`         | Reload a process                |
| `rolling_reload` | Rolling reload across processes |
| `delete`         | Delete a process                |
| `list`           | List all processes              |
| `status`         | Get process status              |
| `logs`           | Get process logs                |
| `save`           | Persist state to disk           |
| `resurrect`      | Restore saved processes         |
| `ping`           | Health check                    |
| `config_reload`  | Reload daemon configuration     |
| `web_start`      | Start the web dashboard         |
| `cron_list`      | List cron jobs                  |
| `cron_start`     | Enable a cron job               |
| `cron_stop`      | Disable a cron job              |
| `cron_run`       | Manually trigger a cron job     |

**Daemon lifecycle:**

1. `StartDaemon()` forks `runix daemon run` as a background child process
2. Polls the Unix socket until it responds (5-second deadline)
3. `RunDaemon()` creates a Supervisor, starts the Server, and optionally the MCP server
4. Handles `SIGTERM`, `SIGINT` for graceful shutdown (10-second timeout)
5. Handles `SIGHUP` for config reload (reserved)

**Auto-start:** The CLI calls `ensureDaemon()` before most commands, which starts the daemon if it is not already running.

### `internal/scheduler/` — Cron Scheduler

Built on `robfig/cron` with seconds-level precision. The scheduler manages named jobs that can be enabled and disabled at runtime.

If a 5-field cron expression is provided, `"0 "` is prepended automatically for the seconds field.

### `internal/metrics/` — Resource Metrics

Linux-only metrics collection from `/proc`:

| Metric           | Source                                      |
| ---------------- | ------------------------------------------- |
| CPU              | `/proc/[pid]/stat` (tick deltas)            |
| Memory (RSS)     | `/proc/[pid]/stat` (page count × page size) |
| Threads          | `/proc/[pid]/stat`                          |
| File descriptors | `/proc/[pid]/fd` count                      |
| System memory    | `/proc/meminfo`                             |
| Load average     | `/proc/loadavg`                             |
| Uptime           | `/proc/uptime`                              |

Metrics are collected periodically (default: every 5 seconds).

### `internal/updater/` — Self-Update

Queries the GitHub releases API, downloads the platform-specific binary, verifies SHA256 against `checksums.txt`, and replaces the current binary atomically (with copy fallback for cross-device renames).

### `internal/web/` — Web UI

Chi-based HTTP server with:

- REST API at `/api/*` for process management
- WebSocket hub at `/ws` for live updates (2-second push interval)
- Embedded single-page frontend (Tokyo Night theme)

### `internal/tui/` — Terminal UI

BubbleTea application with:

- Process table view (Bubbles table component)
- Log viewer (Viewport component)
- Help overlay
- Status bar with process counts

### `internal/logrot/` — Log Rotation

Per-process log file rotation based on size and age. See the [Logging documentation](logging.md).

### `internal/secrets/` — Secret Resolvers

Resolves secret references from environment variables, files, or Vault.

### `internal/hooks/` — Hook Execution

Runs lifecycle hook commands via `sh -c` in the process working directory. See the [Hooks documentation](hooks.md).

## Data Flow

### Starting a Process (Daemon Mode)

```
CLI                          Daemon                       Supervisor
 │                              │                            │
 │  POST /api/start             │                            │
 │  ──────────────────────────> │                            │
 │                              │  AddProcess(config)        │
 │                              │  ────────────────────────> │
 │                              │                            │  Detect runtime
 │                              │                            │  Build exec.Cmd
 │                              │                            │  Run pre_start hook
 │                              │                            │  Start process
 │                              │                            │  Run post_start hook
 │                              │                            │  Start exit monitor
 │                              │  <──────────────────────── │
 │  { success: true, data }     │                            │
 │  <────────────────────────── │                            │
```

### Starting a Process (Direct Mode)

```
CLI
 │
 │  Create in-process Supervisor
 │  AddProcess(config)
 │  Detect runtime
 │  Build exec.Cmd
 │  Run pre_start hook
 │  Start process
 │  Run post_start hook
 │  Start exit monitor
 │
```

### Auto-Restart on Crash

```
Process exits
     │
     v
Exit monitor goroutine fires
     │
     v
Check restart policy (always/on-failure/never)
     │
     v
Check max_restarts within restart_window
     │
     ├── exceeded ──> set state to "errored", stop restarting
     │
     v (within limits)
Calculate backoff delay (exponential with jitter)
     │
     v
Wait for backoff delay
     │
     v
Set state to "starting"
     │
     v
Start process again
```

## Directory Layout

```
~/.runix/                       # Data directory (default)
├── apps/
│   └── {process-id}/
│       ├── stdout.log          # Standard output
│       ├── stderr.log          # Standard error
│       └── metadata.json       # Process metadata
├── state/
│   └── state.json              # Process state persistence
├── tmp/
│   ├── runix.sock              # Unix domain socket
│   └── runix.pid               # Daemon PID file
└── dump.json                   # Saved process state (save/resurrect)
```

## Concurrency Model

- The supervisor uses `sync.RWMutex` for process registry access
- Process state uses lock-free `atomic.Value` for compare-and-swap transitions
- Each process has a dedicated exit monitor goroutine
- The log writer uses a mutex for thread-safe line buffering
- The WebSocket hub uses channels for broadcast distribution
- Metrics collection runs in a separate goroutine per collection interval
