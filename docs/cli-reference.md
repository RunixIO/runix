# CLI Reference

Complete reference for all Runix CLI commands, flags, and usage examples.

## Global Flags

| Flag         | Short | Default        | Description                                 |
| ------------ | ----- | -------------- | ------------------------------------------- |
| `--config`   |       | `./runix.yaml` | Path to configuration file                  |
| `--verbose`  | `-v`  | `false`        | Enable debug-level output                   |
| `--debug`    |       | `false`        | Enable trace-level logging with caller info |
| `--dry-run`  |       | `false`        | Show what would happen without executing    |
| `--no-color` |       | `false`        | Disable colored output                      |
| `--data-dir` |       | `~/.runix`     | Data directory for state, logs, and sockets |

Runix searches for config files in this order: `runix.yaml`, `runix.yml`, `runix.json`, `runix.toml`.

## Process Management

### `runix start`

Start a managed process. Runix auto-detects the runtime based on the entrypoint and project files.

```bash
runix start app.py
runix start server.go
runix start index.ts
runix start dev --runtime bun
```

**Flags:**

| Flag               | Short | Default                 | Description                                                              |
| ------------------ | ----- | ----------------------- | ------------------------------------------------------------------------ |
| `--only`           |       |                         | Start only specific processes by name (comma-separated, requires config) |
| `--name`           | `-n`  | entrypoint filename     | Process name                                                             |
| `--runtime`        | `-r`  | auto-detect             | Runtime: `go`, `python`, `node`, `bun`, `deno`, `ruby`, `php`, `auto`    |
| `--cwd`            | `-d`  | current directory       | Working directory                                                        |
| `--env`            | `-e`  |                         | Environment variable (`KEY=VAL`, repeatable)                             |
| `--watch`          | `-w`  | `false`                 | Watch for file changes and auto-restart                                  |
| `--watch-ignore`   |       |                         | Glob patterns to ignore when watching (repeatable)                       |
| `--restart-policy` |       | `on-failure`            | Restart policy: `always`, `on-failure`, `never`                          |
| `--max-restarts`   |       | `0` (uses default `10`) | Max restarts within the restart window                                   |
| `--max-retry`      |       |                         | Alias for `--max-restarts`                                               |
| `--instances`      |       | `1`                     | Number of instances to start                                             |
| `--namespace`      |       |                         | Process namespace                                                        |
| `--use-bundle`     |       | `false`                 | Wrap entrypoint with `bundle exec` (Ruby)                                |

**Examples:**

```bash
# Start a Go server with a custom name
runix start ./cmd/api --name api-server

# Start a Python worker with environment variables
runix start worker.py --name worker -e DB_HOST=localhost -e DB_PORT=5432

# Start a Node.js app in watch mode
runix start index.ts --watch --watch-ignore "dist/*" --watch-ignore "*.test.ts"

# Start with a specific runtime and restart policy
runix start main.go --runtime go --restart-policy always --max-restarts 5

# Start with a custom working directory
runix start app.py --cwd /path/to/project
```

### `runix stop`

Stop one or more managed processes. Accepts a process ID, name, or `all`.

```bash
runix stop api-server
runix stop abc123
runix stop all
```

**Flags:**

| Flag         | Short | Default | Description                       |
| ------------ | ----- | ------- | --------------------------------- |
| `--force`    |       | `false` | Force stop with SIGKILL           |
| `--timeout`  |       | `5s`    | Grace period before force killing |
| `--graceful` |       | `false` | Graceful stop                     |
| `--format`   | `-f`  | `text`  | Output format: `text`, `json`     |

**Examples:**

```bash
# Graceful stop (SIGTERM, waits 5 seconds)
runix stop api-server

# Force stop immediately
runix stop api-server --force

# Stop with a custom grace period
runix stop api-server --timeout 30s
```

### `runix restart`

Restart a managed process. Accepts a process ID, name, or `all`.

```bash
runix restart api-server
runix restart all
```

Restart applies exponential backoff if the process has accumulated prior restarts. Pre-restart and post-restart hooks are executed.

### `runix reload`

Graceful reload of a process. Similar to restart but fires reload-specific hooks instead of restart hooks. Does not apply backoff delay.

```bash
runix reload api-server
```

Use reload when you want a clean stop/start cycle without backoff penalties, typically after configuration changes.

### `runix delete`

Stop and remove a process from the process table. Accepts a process ID, name, or `all`.

```bash
runix delete api-server
runix delete all
```

**Flags:**

| Flag      | Default | Description                |
| --------- | ------- | -------------------------- |
| `--force` | `false` | Force stop before deleting |

### `runix list`

Display the process table. Alias: `ls`.

```bash
runix list
runix ls
```

**Flags:**

| Flag          | Short | Default | Description                                                             |
| ------------- | ----- | ------- | ----------------------------------------------------------------------- |
| `--format`    | `-f`  | `table` | Output format: `table`, `json`, `yaml`                                  |
| `--filter`    | `-F`  |         | Filter by state: `running`, `stopped`, `crashed`, `errored`, `all`      |
| `--namespace` | `-N`  |         | Filter by namespace                                                     |
| `--runtime`   | `-r`  |         | Filter by runtime: `go`, `python`, `node`, `bun`, `deno`, `ruby`, `php` |
| `--tag`       |       |         | Filter by tag                                                           |

**Examples:**

```bash
# Table format (default)
runix list

# JSON output for scripting
runix list --format json

# Show only crashed processes
runix list --filter crashed

# YAML output with filter
runix list -f yaml -F running
```

### `runix status`

Show detailed status for a specific process.

```bash
runix status api-server
```

**Flags:**

| Flag       | Short | Default | Description                    |
| ---------- | ----- | ------- | ------------------------------ |
| `--format` | `-f`  | `table` | Output format: `table`, `json` |

### `runix logs`

View or stream process logs. Follows by default (like `tail -f`). Alias: `log`.

```bash
runix logs api-server
runix logs api-server --nostream
```

**Flags:**

| Flag         | Short | Default | Description                        |
| ------------ | ----- | ------- | ---------------------------------- |
| `--nostream` |       | `false` | Print a snapshot without following |
| `--lines`    | `-n`  | `50`    | Number of recent lines to show     |
| `--err`      |       | `false` | Show only stderr output            |
| `--out`      |       | `false` | Show only stdout output            |

**Examples:**

```bash
# Follow live output (default)
runix logs api-server

# Print snapshot without following
runix logs api-server --nostream

# Show last 200 lines
runix logs api-server -n 200 --nostream

# Show only stderr
runix logs api-server --err

# Show only stdout
runix logs api-server --out
```

### `runix inspect`

Show full process details including configuration, state, recent logs, and resource metrics.

```bash
runix inspect api-server
```

**Flags:**

| Flag       | Short | Default | Description                                    |
| ---------- | ----- | ------- | ---------------------------------------------- |
| `--format` | `-f`  | `table` | Output format: `table`, `json`                 |
| `--logs`   |       | `20`    | Number of recent log lines to include          |
| `--tui`    |       | `false` | Open interactive TUI control panel for process |

## Persistence

### `runix save`

Persist the current process table to disk. Processes can be restored later with `resurrect`.

```bash
runix save
```

State is written to `{data-dir}/dump.json` using atomic write (write to temp file, then rename).

### `runix resurrect`

Restore previously saved processes. Reads from `dump.json` and starts each saved process.

```bash
runix resurrect
```

Typical workflow for persistence across restarts:

```bash
# Before shutdown
runix save

# After reboot or daemon restart
runix resurrect
```

## Daemon

### `runix daemon`

Manage the background daemon. The daemon provides persistent process management via HTTP-over-Unix-socket IPC.

**Subcommands:**

| Subcommand             | Description                                     |
| ---------------------- | ----------------------------------------------- |
| `runix daemon start`   | Start the daemon in the background              |
| `runix daemon stop`    | Stop the daemon (SIGTERM, 10s grace period)     |
| `runix daemon status`  | Show daemon status (PID, socket path)           |
| `runix daemon restart` | Restart the daemon                              |
| `runix daemon reload`  | Reload daemon config without stopping processes |
| `runix daemon run`     | Run daemon in foreground (used internally)      |

**Examples:**

```bash
# Start daemon
runix daemon start

# Check status
runix daemon status

# Restart daemon
runix daemon restart

# Stop daemon
runix daemon stop
```

The CLI automatically starts the daemon when needed. If the daemon is not running, commands fall back to direct mode (in-process supervisor).

## Dashboards

### `runix tui`

Launch the interactive terminal UI dashboard built with BubbleTea.

```bash
runix tui
```

**Keybindings:**

| Key            | Action                   |
| -------------- | ------------------------ |
| `Up`/`Down`    | Navigate process list    |
| `Enter`        | View process details     |
| `l`            | View process logs        |
| `r`            | Restart selected process |
| `s`            | Stop selected process    |
| `d`            | Delete selected process  |
| `?`            | Show help                |
| `Ctrl+R`       | Refresh process list     |
| `q` / `Ctrl+C` | Quit                     |

The TUI auto-refreshes every 2 seconds.

### `runix web`

Launch the web-based dashboard with live WebSocket updates.

```bash
runix web
```

**Flags:**

| Flag       | Default          | Description                |
| ---------- | ---------------- | -------------------------- |
| `--listen` | `localhost:9615` | Address to listen on       |
| `--open`   | `false`          | Open browser automatically |

**Examples:**

```bash
# Launch on default address
runix web

# Custom port
runix web --listen 0.0.0.0:8080

# Launch and open browser
runix web --open
```

## Cron

### `runix cron`

Manage cron jobs defined in the configuration file.

**Subcommands:**

| Subcommand                | Description                   |
| ------------------------- | ----------------------------- |
| `runix cron list`         | List all configured cron jobs |
| `runix cron start <name>` | Enable a cron job             |
| `runix cron stop <name>`  | Disable a cron job            |
| `runix cron run <name>`   | Manually trigger a cron job   |

**`runix cron list` flags:**

| Flag       | Short | Default | Description                    |
| ---------- | ----- | ------- | ------------------------------ |
| `--format` | `-f`  | `table` | Output format: `table`, `json` |

**Examples:**

```bash
# List all cron jobs
runix cron list

# Manually trigger a job
runix cron run cleanup

# Disable a job
runix cron stop cleanup

# Re-enable a job
runix cron start cleanup
```

## Watch

### `runix watch`

Enable file watching on an already-running process. When files change, the process is automatically restarted.

```bash
runix watch api-server
```

**Flags:**

| Flag         | Default           | Description                          |
| ------------ | ----------------- | ------------------------------------ |
| `--paths`    | process cwd       | Paths to watch (repeatable)          |
| `--ignore`   | built-in defaults | Glob patterns to ignore (repeatable) |
| `--debounce` | `100ms`           | Debounce duration                    |

**Default ignore patterns:** `.git`, `node_modules`, `__pycache__`, `*.pyc`, `.DS_Store`, `vendor`, `dist`, `build`, `bin`

**Examples:**

```bash
# Watch with default settings
runix watch api-server

# Watch specific paths
runix watch api-server --paths ./src --paths ./internal

# Custom debounce and ignore patterns
runix watch api-server --debounce 500ms --ignore "*.test.go" --ignore "docs/*"
```

## Diagnostics

### `runix doctor`

Run diagnostic checks on your environment.

```bash
runix doctor
```

**Flags:**

| Flag       | Short | Default | Description                   |
| ---------- | ----- | ------- | ----------------------------- |
| `--format` | `-f`  | `text`  | Output format: `text`, `json` |

Checks:

- Available runtimes (Go, Python, Node.js, Bun, Deno, Ruby, PHP)
- Data and runtime directories
- Daemon status and socket accessibility
- File permissions

### `runix startup`

Install or uninstall Runix as a system service for boot-time startup.

```bash
runix startup
```

**Flags:**

| Flag          | Default     | Description                       |
| ------------- | ----------- | --------------------------------- |
| `--platform`  | auto-detect | Init system: `systemd`, `launchd` |
| `--uninstall` | `false`     | Uninstall the startup service     |

**Examples:**

```bash
# Install as system service (auto-detects systemd or launchd)
runix startup

# Install for systemd explicitly
runix startup --platform systemd

# Uninstall
runix startup --uninstall
```

## Updates

### `runix update`

Update the Runix binary from GitHub releases. Verifies SHA256 checksums.

```bash
runix update
```

**Flags:**

| Flag        | Default | Description                               |
| ----------- | ------- | ----------------------------------------- |
| `--check`   | `false` | Only check for updates without installing |
| `--version` |         | Install a specific version                |

**Examples:**

```bash
# Check for updates
runix update --check

# Update to latest
runix update

# Install a specific version
runix update --version v0.2.0
```

### `runix version`

Print the current version.

```bash
runix version
```

**Flags:**

| Flag        | Default | Description                                          |
| ----------- | ------- | ---------------------------------------------------- |
| `--verbose` | `false` | Show build details (build time, Go version, OS/Arch) |

## MCP

### `runix mcp`

Start the MCP (Model Context Protocol) server for AI agent integration.

```bash
runix mcp
```

**Flags:**

| Flag          | Default          | Description                       |
| ------------- | ---------------- | --------------------------------- |
| `--transport` | `stdio`          | Transport type: `stdio` or `http` |
| `--listen`    | `localhost:8090` | Listen address for HTTP transport |

See the [MCP documentation](mcp.md) for integration details.

## Configuration

### `runix validate`

Validate a Runix configuration file.

```bash
runix validate
runix validate /path/to/runix.yaml
```

Checks for duplicate process names, invalid runtimes, missing required fields, and auth configuration errors.

### `runix config reload`

Reload the daemon's configuration from disk.

```bash
runix config reload
```

Signals the running daemon to re-read its configuration file.

## Utilities

### `runix flush`

Flush (clear) log files for a process.

```bash
runix flush api-server
runix flush all
```

**Flags:**

| Flag       | Short | Default | Description                   |
| ---------- | ----- | ------- | ----------------------------- |
| `--format` | `-f`  | `text`  | Output format: `text`, `json` |

### `runix migrate`

Run data migration between Runix versions.

**Subcommands:**

| Subcommand                 | Description                          |
| -------------------------- | ------------------------------------ |
| `runix migrate pm2 [file]` | Migrate a PM2 configuration to Runix |

**`runix migrate pm2` flags:**

| Flag       | Short | Default  | Description                     |
| ---------- | ----- | -------- | ------------------------------- |
| `--output` | `-o`  |          | Write output to file            |
| `--format` | `-f`  | `"yaml"` | Output format: `yaml` or `json` |

```bash
# Convert a PM2 ecosystem file
runix migrate pm2 ecosystem.config.js

# Convert and write to file
runix migrate pm2 ecosystem.config.js -o runix.yaml

# Output as JSON
runix migrate pm2 ecosystem.config.js -f json
```

### `runix events`

Stream real-time process events.

```bash
runix events
```

**Flags:**

| Flag       | Short | Default | Description                                    |
| ---------- | ----- | ------- | ---------------------------------------------- |
| `--follow` | `-f`  | `false` | Follow the event stream in real time           |
| `--since`  |       | `"1h"`  | Show events since duration (e.g. `10m`, `24h`) |
| `--type`   | `-t`  |         | Filter by event type (e.g. `process.crashed`)  |

### `runix ready`

Check if a process is ready (healthy and running).

```bash
runix ready api-server
```

**Flags:**

| Flag        | Default | Description                    |
| ----------- | ------- | ------------------------------ |
| `--timeout` | `60s`   | Maximum time to wait for ready |

## Process Lookup

Commands that accept a process target (`<id|name>`) resolve in this order:

1. **Exact ID match** — the full process UUID
2. **Name match** — the process name
3. **ID prefix match** — a unique prefix of the process ID

If a prefix matches multiple processes, the command fails with an ambiguity error.

## Environment Variables

| Variable       | Description                                                                           |
| -------------- | ------------------------------------------------------------------------------------- |
| `RUNIX_*`      | Any config field can be set via `RUNIX_` prefix (e.g., `RUNIX_DAEMON_LOGLEVEL=debug`) |
| `RUNIX_CONFIG` | Path to config file (set by CLI when forking daemon)                                  |

## Exit Codes

| Code | Meaning             |
| ---- | ------------------- |
| `0`  | Success             |
| `1`  | General error       |
| `2`  | Command usage error |
