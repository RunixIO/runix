<div align="center">

# Runix

A modern process manager and application supervisor written in Go.

Runix manages your applications — Go, Python, Node.js/TypeScript, Bun, Deno, Ruby, and PHP — with a unified CLI, TUI, Web UI, and MCP interface. Inspired by PM2, but faster, lighter, and built for polyglot developers.

</div>

## Features

- **Multi-runtime support** — Go, Python, Node.js/TypeScript, Bun, Deno, Ruby, PHP
- **Auto-detection** — Runix detects your project's runtime automatically
- **Process lifecycle** — start, stop, restart, reload, with configurable restart policies
- **Exponential backoff** — crash-loop protection with configurable max restarts
- **Log management** — per-process log capture, rotation, and streaming
- **Persistence** — save and resurrect process state across restarts
- **Watch mode** — auto-restart on file changes with configurable debounce
- **Cron scheduling** — scheduled restarts and task execution
- **TUI dashboard** — terminal UI with process table, log viewer, and keybindings
- **Web UI** — browser-based dashboard with WebSocket live updates
- **MCP support** — AI agent integration via Model Context Protocol
- **Daemon mode** — background daemon with HTTP-over-Unix-socket IPC
- **Self-update** — built-in binary update from GitHub releases
- **System service** — systemd and launchd integration for boot-time startup
- **Lifecycle hooks** — run commands before/after start, stop, restart, and reload
- **Resource metrics** — per-process CPU, memory, thread, and FD tracking

## Install

### One-line install (Linux/macOS)

```bash
curl -sfL https://raw.githubusercontent.com/runixio/runix/main/scripts/install.sh | bash
```

Or install a specific version:

```bash
curl -sfL https://raw.githubusercontent.com/runixio/runix/main/scripts/install.sh | bash -s v0.1.0
```

### Go install

```bash
go install github.com/runixio/runix/cmd/runix@latest
```

### Build from source

```bash
git clone https://github.com/runixio/runix.git
cd runix
make build
sudo make install
```

## Quick Start

```bash
# Start a process (auto-detects runtime)
runix start app.py
runix start server.go
runix start index.ts
runix start dev --runtime bun

# List running processes
runix list

# View logs
runix logs app.py
runix logs app.py -f       # follow mode

# Stop, restart, reload
runix stop app.py
runix restart app.py
runix reload app.py

# Save and restore
runix save
runix resurrect

# Launch TUI dashboard
runix tui

# Launch Web dashboard
runix web

# Check environment
runix doctor
```

## Daemon

Runix can run as a background daemon for persistent process management:

```bash
# Start daemon
runix daemon start

# Check daemon status
runix daemon status

# Stop daemon
runix daemon stop
```

The CLI automatically starts the daemon when needed. All commands work with or without a running daemon — without it, the CLI operates in direct mode.

## Configuration

Create a `runix.yaml` in your project root:

```yaml
processes:
  - name: "api"
    runtime: "go"
    entrypoint: "./cmd/api"
    args: ["--port", "8080"]
    env:
      PORT: "8080"
    restart_policy: "always"
    max_restarts: 5

  - name: "worker"
    runtime: "python"
    entrypoint: "worker.py"
    restart_policy: "on-failure"

  - name: "frontend"
    runtime: "bun"
    entrypoint: "dev"
    watch:
      enabled: true
      paths: ["./src"]
```

See `configs/runix.example.yaml` for the full configuration reference.

### Lifecycle Hooks

Run commands at defined points in the process lifecycle:

```yaml
processes:
  - name: "api"
    entrypoint: "./api"
    hooks:
      pre_start:
        command: "echo 'Starting API'"
        timeout: "5s"
      post_start:
        command: "curl -sf http://localhost:8080/health || true"
        ignore_failure: true
      pre_stop:
        command: "echo 'Draining connections'"
      post_stop:
        command: "echo 'Stopped'"
      pre_restart:
        command: "echo 'Restarting'"
      post_restart:
        command: "curl -sf http://localhost:8080/health"
```

**Behavior:**

- `pre_*` hooks can **block** the lifecycle action on failure (unless `ignore_failure: true`)
- `post_*` hooks are **informational** — failures are logged but don't undo the action
- Each hook has a configurable timeout (default: 30s)
- Hook output is captured in structured logs

## CLI Commands

| Command                         | Description                            |
| ------------------------------- | -------------------------------------- |
| `runix start <entrypoint>`      | Start a managed process                |
| `runix stop <id\|name\|all>`    | Stop process(es)                       |
| `runix restart <id\|name\|all>` | Restart process(es)                    |
| `runix reload <id\|name>`       | Graceful reload                        |
| `runix delete <id\|name\|all>`  | Stop and remove from table             |
| `runix list`                    | Show process table                     |
| `runix status <id\|name>`       | Detailed process info                  |
| `runix inspect <id\|name>`      | Full process details + logs + TUI      |
| `runix logs <id\|name>`         | View/stream logs                       |
| `runix save`                    | Persist process state                  |
| `runix resurrect`               | Restore saved processes                |
| `runix startup`                 | Install boot service (systemd/launchd) |
| `runix update`                  | Update Runix binary                    |
| `runix version`                 | Print version                          |
| `runix tui`                     | Launch terminal UI                     |
| `runix web`                     | Launch web dashboard                   |
| `runix cron`                    | Manage cron jobs                       |
| `runix watch`                   | Enable file watching                   |
| `runix doctor`                  | Run diagnostics                        |
| `runix daemon`                  | Manage background daemon               |
| `runix mcp`                     | Start MCP server for AI agents         |
| `runix validate`                | Validate a configuration file          |
| `runix flush`                   | Flush process logs                     |
| `runix migrate`                 | Migrate data between versions          |
| `runix events`                  | Stream process events                  |
| `runix ready`                   | Check if process is ready              |
| `runix config reload`           | Reload daemon configuration            |

## MCP Integration

Runix provides an MCP server for AI agent integration. Configure it in your AI tool's settings:

```json
{
  "mcpServers": {
    "runix": {
      "command": "runix",
      "args": ["mcp"]
    }
  }
}
```

Available MCP tools: `list_apps`, `start_app`, `stop_app`, `restart_app`, `reload_app`, `delete_app`, `get_status`, `get_logs`, `save_state`, `resurrect_processes`.

MCP resources: `apps://list`, `apps://{name}`, `logs://{name}`, `metrics://{name}`.

## Architecture

```
 ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐
 │   CLI   │  │   TUI   │  │  Web UI │  │   MCP   │
 └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘
      │            │            │            │
      └────────────┴────────────┴────────────┘
                         │
                ┌────────┴────────┐
                │   IPC / Direct  │
                └────────┬────────┘
                         │
                ┌────────┴────────┐
                │   Supervisor    │
                │   Engine        │
                └────────┬────────┘
                         │
     ┌─────────┬─────────┼─────────┬─────────┐
     │         │         │         │         │
 ┌───┴───┐ ┌───┴───┐ ┌───┴───┐ ┌───┴───┐ ┌──┴──┐
 │  Go   │ │Python │ │ Node  │ │  Bun  │ │ ... │
 └───────┘ └───────┘ └───────┘ └───────┘ └─────┘
```

## Documentation

Full documentation is available in the [`docs/`](docs/) directory:

- [CLI Reference](docs/cli-reference.md) — all commands, flags, and examples
- [Configuration](docs/configuration.md) — YAML schema, fields, and validation
- [Architecture](docs/architecture.md) — system design and module breakdown
- [Hooks](docs/hooks.md) — lifecycle hooks and execution behavior
- [Reload, Watch, and Cron](docs/reload-watch-cron.md) — automated process management
- [Logging](docs/logging.md) — log storage, rotation, and reading
- [MCP Integration](docs/mcp.md) — AI agent tools and resources
- [Troubleshooting](docs/troubleshooting.md) — common issues and debugging
- [Developer Guide](docs/developer-guide.md) — contributing and extending Runix

## Development

```bash
make build    # Build binary
make test     # Run tests
make vet      # Run go vet
make lint     # Run linter (requires golangci-lint)
make clean    # Remove build artifacts
```

## License

MIT
