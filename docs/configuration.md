# Runix Configuration Reference

Complete reference for the Runix process manager configuration system.

---

## Table of Contents

- [1. Overview](#1-overview)
- [2. Config File Format](#2-config-file-format)
- [3. Top-Level Structure](#3-top-level-structure)
- [4. Process Configuration](#4-process-configuration)
- [5. Namespace and Instances](#5-namespace-and-instances)
- [6. Runtime-Specific Behavior](#6-runtime-specific-behavior)
- [7. Hooks](#7-hooks)
- [8. Watch and Auto-Reload](#8-watch-and-auto-reload)
- [9. Health Checks](#9-health-checks)
- [10. Logging](#10-logging)
- [11. Cron Jobs](#11-cron-jobs)
- [12. Secrets](#12-secrets)
- [13. Profiles](#13-profiles)
- [14. Config Inheritance (extends)](#14-config-inheritance-extends)
- [15. Persistence and Storage](#15-persistence-and-storage)
- [16. Validation Rules](#16-validation-rules)
- [17. Path Resolution](#17-path-resolution)
- [18. Complete Examples](#18-complete-examples)
- [19. Best Practices](#19-best-practices)
- [20. Troubleshooting](#20-troubleshooting)

---

## 1. Overview

### What is the Runix config file?

The Runix configuration file defines the set of processes to manage, their runtime environments, restart policies, health checks, lifecycle hooks, and operational defaults. It is the central declaration of your application infrastructure at the process level.

### Why it exists

Without a config file, you can start individual processes via the CLI (`runix start app.py`). The config file enables:

- **Declarative process management** — define all processes in one place
- **Reproducible environments** — the same config works across machines
- **Lifecycle automation** — hooks, health checks, restart policies, watch/reload
- **Multi-process orchestration** — dependencies, priority ordering, namespaces
- **Persistence** — save/resurrect restores processes after restarts

### How it relates to CLI commands and the daemon

Runix operates in two modes:

1. **Daemon mode** — `runix daemon` starts a background process manager. The config is loaded when the daemon starts. All CLI commands (`runix start`, `runix stop`, `runix list`, etc.) communicate with the daemon via IPC over a Unix socket.

2. **Direct mode** — CLI commands like `runix start app.py` operate without a daemon, creating an in-process supervisor. Config-based commands (`runix start --config runix.yaml`) can also work in direct mode.

The `--config` flag or auto-detected `runix.yaml` provides the config in both modes.

---

## 2. Config File Format

### Supported formats

| Format | File names                | Extension       |
| ------ | ------------------------- | --------------- |
| YAML   | `runix.yaml`, `runix.yml` | `.yaml`, `.yml` |
| JSON   | `runix.json`              | `.json`         |
| TOML   | `runix.toml`              | `.toml`         |

YAML is the recommended and most commonly used format. All examples in this reference use YAML.

### File discovery

When no `--config` flag is provided, Runix searches for config files in the current directory in this order:

1. `runix.yaml`
2. `runix.yml`
3. `runix.json`
4. `runix.toml`

If no file is found, Runix operates with built-in defaults (no processes defined). This is not an error — you can use the CLI to start processes ad-hoc.

### Path resolution

- Config file paths specified via `--config` are resolved to absolute paths.
- Relative paths in the config (e.g., `cwd: "./web"`) are resolved **relative to the directory containing the config file**, not the current working directory of the `runix` command.
- If a process has no `cwd` set, it defaults to the directory containing the config file.

### Duration format

Duration fields accept Go-style duration strings:

| Value     | Meaning                  |
| --------- | ------------------------ |
| `"100ms"` | 100 milliseconds         |
| `"5s"`    | 5 seconds                |
| `"30s"`   | 30 seconds               |
| `"1m"`    | 1 minute                 |
| `"5m"`    | 5 minutes                |
| `"1h"`    | 1 hour                   |
| `"168h"`  | 168 hours (7 days)       |
| `"7d"`    | 7 days (parsed by Viper) |

---

## 3. Top-Level Structure

```yaml
daemon:
  # Daemon settings

defaults:
  # Default values for all processes

processes:
  # List of managed processes

cron:
  # Scheduled jobs

web:
  # Web dashboard settings

mcp:
  # MCP server settings

security:
  # Authentication and access control

metrics:
  # Metrics collection settings

secrets:
  # Secret references

profiles:
  # Per-environment overrides
```

### `daemon`

Controls the background daemon process manager.

| Field         | Type   | Default                     | Description                                  |
| ------------- | ------ | --------------------------- | -------------------------------------------- |
| `data_dir`    | string | `~/.runix`                  | Root data directory for state, logs, sockets |
| `socket_path` | string | `<data_dir>/tmp/runix.sock` | Unix domain socket path for IPC              |
| `pid_dir`     | string | `<data_dir>/tmp/`           | Directory for PID files                      |
| `log_level`   | string | `"info"`                    | Log level: `debug`, `info`, `warn`, `error`  |

**Default data directory:** `~/.runix/` with subdirectories `apps/`, `state/`, `tmp/`. Legacy path: `~/.local/share/runix/`.

Override with `--data-dir` flag or `daemon.data_dir` in config.

```yaml
daemon:
  data_dir: "~/.runix"
  socket_path: "~/.runix/tmp/runix.sock"
  log_level: "debug"
```

### `defaults`

Default values applied to every process unless overridden at the process level.

| Field            | Type     | Default            | Description                                      |
| ---------------- | -------- | ------------------ | ------------------------------------------------ |
| `restart_policy` | string   | `"on-failure"`     | When to restart: `always`, `on-failure`, `never` |
| `max_restarts`   | int      | `10`               | Maximum restarts within the restart window       |
| `restart_window` | duration | `"60s"`            | Time window for counting restarts                |
| `backoff_base`   | duration | `"1s"`             | Initial delay for exponential backoff            |
| `backoff_max`    | duration | `"60s"`            | Maximum backoff delay cap                        |
| `log_max_size`   | int64    | `10485760` (10 MB) | Max log file size before rotation (bytes)        |
| `log_max_age`    | duration | `"168h"` (7 days)  | Max log file age before rotation                 |
| `watch_debounce` | duration | `"100ms"`          | Debounce window for file watch events            |

```yaml
defaults:
  restart_policy: "on-failure"
  max_restarts: 10
  backoff_base: "1s"
  backoff_max: "60s"
  log_max_size: "10MB"
  log_max_age: "7d"
  watch_debounce: "100ms"
```

### `processes`

List of process definitions. Each entry is a complete process configuration. See [Section 4](#4-process-configuration).

### `cron`

List of scheduled jobs. See [Section 11](#11-cron-jobs).

### `web`

Web dashboard configuration.

| Field           | Type   | Default            | Description                                                        |
| --------------- | ------ | ------------------ | ------------------------------------------------------------------ |
| `enabled`       | bool   | `false`            | Enable the web dashboard                                           |
| `listen`        | string | `"localhost:9615"` | Address to listen on                                               |
| `auth.enabled`  | bool   | `false`            | Enable basic authentication (legacy — use `security.auth` instead) |
| `auth.username` | string |                    | Username for basic auth                                            |
| `auth.password` | string |                    | Password for basic auth                                            |

> **Note:** The `web.auth` section is legacy. For full auth options including token mode, bcrypt hashes, and `local_only` bypass, use the top-level `security.auth` section instead.

```yaml
web:
  enabled: true
  listen: "localhost:9615"
  auth:
    enabled: true
    username: "admin"
    password: "secret"
```

### `mcp`

MCP (Model Context Protocol) server for AI agent integration.

| Field       | Type   | Default   | Description                       |
| ----------- | ------ | --------- | --------------------------------- |
| `enabled`   | bool   | `false`   | Enable the MCP server             |
| `transport` | string | `"stdio"` | Transport: `stdio` or `http`      |
| `listen`    | string |           | Listen address for HTTP transport |

```yaml
mcp:
  enabled: false
  transport: "stdio"
```

### `security`

Authentication and access control for the web dashboard, daemon IPC, and other interfaces.

| Field                | Type   | Default   | Description                                            |
| -------------------- | ------ | --------- | ------------------------------------------------------ |
| `auth.enabled`       | bool   | `false`   | Enable authentication                                  |
| `auth.mode`          | string | `"basic"` | Auth mode: `disabled`, `basic`, or `token`             |
| `auth.username`      | string |           | Username for basic auth                                |
| `auth.password`      | string |           | Password for basic auth (plain text, development only) |
| `auth.password_hash` | string |           | Bcrypt hash of password (preferred over plain text)    |
| `auth.token`         | string |           | Bearer token for token auth (minimum 16 characters)    |
| `auth.local_only`    | bool   | `false`   | Allow unauthenticated access from localhost            |

```yaml
security:
  auth:
    enabled: true
    mode: "basic"
    username: "admin"
    password_hash: "$2a$10$..."
    local_only: true
```

**Validation rules:**

- Basic auth requires `username` and either `password` or `password_hash` (mutually exclusive)
- Token auth requires `token` with at least 16 characters
- `local_only: true` allows unauthenticated access from `127.0.0.1` and `::1`

### `metrics`

Resource metrics collection settings.

| Field      | Type     | Default | Description                        |
| ---------- | -------- | ------- | ---------------------------------- |
| `enabled`  | bool     | `false` | Enable periodic metrics collection |
| `interval` | duration | `"5s"`  | Collection polling interval        |

```yaml
metrics:
  enabled: true
  interval: "10s"
```

Metrics collection reads `/proc` and is Linux-only. On other OSes, metrics return empty values.

### `secrets`

Named secret references that processes can use via environment variables. See [Section 12](#12-secrets).

### `profiles`

Per-environment overrides for process environment variables. See [Section 13](#13-profiles).

---

## 4. Process Configuration

Each entry in the `processes` list defines a managed process.

### Complete Field Reference

| Field             | Type        | Required | Default         | Description                                                           |
| ----------------- | ----------- | -------- | --------------- | --------------------------------------------------------------------- |
| `name`            | string      | **Yes**  |                 | Unique process name                                                   |
| `entrypoint`      | string      | **Yes**  |                 | File path or command to execute                                       |
| `runtime`         | string      | No       | auto-detect     | Runtime: `go`, `python`, `node`, `bun`, `deno`, `ruby`, `php`, `auto` |
| `args`            | string list | No       | `[]`            | Arguments passed after the entrypoint                                 |
| `cwd`             | string      | No       | config file dir | Working directory for the process                                     |
| `env`             | map         | No       | `{}`            | Environment variables (key-value)                                     |
| `interpreter`     | string      | No       | auto            | Override the runtime interpreter binary                               |
| `use_bundle`      | bool        | No       | `false`         | Wrap with `bundle exec` (Ruby)                                        |
| `namespace`       | string      | No       |                 | Logical grouping prefix                                               |
| `instances`       | int         | No       | `1`             | Number of process instances to run                                    |
| `restart_policy`  | string      | No       | from `defaults` | `always`, `on-failure`, `never`                                       |
| `max_restarts`    | int         | No       | from `defaults` | Max restarts within window                                            |
| `restart_window`  | duration    | No       | from `defaults` | Window for counting restarts                                          |
| `autostart`       | bool        | No       | `false`         | Start when config is loaded                                           |
| `stop_signal`     | string      | No       | `"SIGTERM"`     | Signal sent to stop                                                   |
| `stop_timeout`    | duration    | No       | `"5s"`          | Grace period before SIGKILL                                           |
| `priority`        | int         | No       | `0`             | Startup order (lower = starts first)                                  |
| `depends_on`      | string list | No       | `[]`            | Process names this depends on                                         |
| `tags`            | string list | No       | `[]`            | Freeform tags for filtering                                           |
| `labels`          | map         | No       | `{}`            | Key-value labels for organization                                     |
| `watch`           | object      | No       |                 | File watch configuration                                              |
| `hooks`           | object      | No       |                 | Lifecycle hook commands                                               |
| `healthcheck`     | object      | No       |                 | Health check configuration                                            |
| `healthcheck_url` | string      | No       |                 | Shorthand HTTP health check URL                                       |
| `cron_restart`    | string      | No       |                 | Cron expression for scheduled restarts                                |
| `extends`         | string      | No       |                 | Inherit config from another named process                             |
| `cpu_quota`       | string      | No       |                 | CPU limit (e.g., `"50%"`)                                             |
| `memory_limit`    | string      | No       |                 | Memory limit (e.g., `"512MB"`)                                        |
| `log_max_files`   | int         | No       |                 | Max rotated log files to keep                                         |

### Field Details

#### `name` (required)

Unique identifier for the process. Must be unique across all processes in the config. Used for CLI commands (`runix stop api`), filtering, and identification in logs.

```yaml
name: "api"
```

**Validation:** Must be non-empty. Must not duplicate another process name.

#### `entrypoint` (required)

The file path or command to execute. How it's interpreted depends on the runtime:

| Runtime               | Entrypoint       | Resulting Command               |
| --------------------- | ---------------- | ------------------------------- |
| `go`                  | `"."`            | `go run .`                      |
| `go`                  | `"main.go"`      | `go run main.go`                |
| `go`                  | `"./cmd/api"`    | `go run ./cmd/api`              |
| `go`                  | `"./bin/server"` | `./bin/server` (runs as binary) |
| `python`              | `"app.py"`       | `python app.py`                 |
| `node`                | `"server.js"`    | `node server.js`                |
| `node`                | `"app.ts"`       | `npx tsx app.ts`                |
| `bun`                 | `"dev"`          | `bun run dev`                   |
| `deno`                | `"main.ts"`      | `deno run main.ts`              |
| `ruby`                | `"app.rb"`       | `ruby app.rb`                   |
| `ruby` + `use_bundle` | `"config.ru"`    | `bundle exec ruby config.ru`    |
| `php`                 | `"artisan"`      | `php artisan`                   |
| `php`                 | `"app.php"`      | `php app.php`                   |

```yaml
entrypoint: "worker.py"
```

**Validation:** Must be non-empty.

#### `runtime`

Declares which runtime adapter to use. When omitted, Runix auto-detects based on project files.

| Value    | Detection files (auto-detect)                                       |
| -------- | ------------------------------------------------------------------- |
| `go`     | `go.mod`                                                            |
| `python` | `requirements.txt`, `pyproject.toml`, `setup.py`, `Pipfile`, `*.py` |
| `node`   | `package.json`                                                      |
| `bun`    | `bun.lockb`, `bunfig.toml`, `bun.lock`                              |
| `deno`   | `deno.json`, `deno.jsonc`                                           |
| `ruby`   | `Gemfile`, `Gemfile.lock`                                           |
| `php`    | `composer.json`, `artisan`, `*.php`                                 |
| `auto`   | Detect using order above (first match wins)                         |

Detection order: **Go → Python → Node → Bun → Deno → Ruby → PHP**.

When `runtime` is set explicitly, detection is skipped entirely.

```yaml
runtime: "python"
```

#### `args`

Arguments passed after the entrypoint. These are runtime-dependent:

```yaml
# Deno permissions
args: ["--allow-net", "--allow-env", "--allow-read"]

# Laravel artisan subcommand
args: ["serve", "--host=0.0.0.0", "--port=8000"]

# Ruby bundler args
args: ["-p", "3000", "-o", "0.0.0.0"]

# Go flags
args: ["--port", "8080"]
```

#### `cwd`

Working directory for the process. The process is started with this directory as its current working directory. If omitted, defaults to the directory containing the config file.

```yaml
cwd: "./web"      # Relative to config file
cwd: "/app"       # Absolute path
```

#### `env`

Environment variables overlaid on the current process environment. Keys replace existing values; new keys are appended. All values must be strings.

```yaml
env:
  PORT: "8080"
  HOST: "0.0.0.0"
  DATABASE_URL: "postgres://localhost/myapp"
  RAILS_ENV: "development"
```

**Common mistake:** Using numeric values without quotes. YAML will parse `PORT: 8080` as an integer, not a string. Always quote values: `PORT: "8080"`.

#### `interpreter`

Overrides the runtime interpreter binary. Useful for specifying custom versions or paths:

```yaml
# Custom Python version
interpreter: "/usr/bin/python3.11"

# Ruby via rbenv
interpreter: "/home/user/.rbenv/shims/ruby"

# Custom PHP version
interpreter: "/usr/bin/php8.2"

# Custom Deno path
interpreter: "/usr/local/bin/deno"
```

When `interpreter` is set, the supervisor's `buildArgs()` constructs: `[interpreter, entrypoint, args...]`.

#### `use_bundle` (Ruby only)

When `true`, wraps the command with `bundle exec`. The effective command becomes `bundle exec <interpreter> <entrypoint> [args...]`.

```yaml
runtime: "ruby"
entrypoint: "config.ru"
use_bundle: true
args: ["-p", "3000"]
# Produces: bundle exec ruby config.ru -p 3000
```

Only meaningful for Ruby processes. Defaults to `false`.

#### `restart_policy`

Controls automatic restart behavior:

| Policy       | Behavior                                 |
| ------------ | ---------------------------------------- |
| `always`     | Restart on any exit — success or failure |
| `on-failure` | Restart only on non-zero exit code       |
| `never`      | Never restart automatically              |

When a process exceeds `max_restarts` within `restart_window`, it stops restarting and remains in its terminal state.

#### `stop_signal`

Signal sent to the process group to initiate a graceful stop.

| Value     | Signal              | Typical use                              |
| --------- | ------------------- | ---------------------------------------- |
| `SIGTERM` | Terminate (default) | Standard graceful shutdown               |
| `SIGINT`  | Interrupt           | Ctrl+C equivalent                        |
| `SIGQUIT` | Quit                | Core dump + terminate                    |
| `SIGUSR1` | User signal 1       | Custom handler                           |
| `SIGUSR2` | User signal 2       | Custom handler                           |
| `SIGKILL` | Kill                | Immediate termination (cannot be caught) |

Signals are sent to the entire process group (`Setpgid: true`), so child processes spawned by your application also receive the signal.

#### `stop_timeout`

Grace period after sending `stop_signal` before Runix force-kills with `SIGKILL`.

```yaml
stop_timeout: "10s" # Wait 10 seconds, then SIGKILL
```

**Default:** `5s`.

#### `priority`

Controls startup ordering. Lower values start first. Processes with the same priority start in config order.

```yaml
priority: 5    # Starts early (database)
priority: 10   # Starts second (api)
priority: 20   # Starts last (workers)
```

#### `depends_on`

List of process names that must be running before this process starts. Runix checks that all dependencies are in `running` state before starting the dependent process.

```yaml
depends_on: ["db", "redis"]
```

#### `tags` and `labels`

Freeform metadata for filtering and organization:

```yaml
tags: ["backend", "http", "critical"]
labels:
  team: "platform"
  tier: "production"
```

Filter with CLI: `runix list --tag backend`, `runix list --runtime python`.

#### `autostart`

When `true`, the process starts automatically when the config is loaded (e.g., when the daemon starts or when running `runix start --config runix.yaml`).

```yaml
autostart: true
```

**Default:** `false`.

#### `cpu_quota` and `memory_limit`

Resource limits applied via cgroups (Linux only):

```yaml
cpu_quota: "50%"
memory_limit: "512MB"
```

#### `cron_restart`

A cron expression that triggers an automatic process restart:

```yaml
cron_restart: "0 3 * * *" # Restart daily at 3 AM
```

#### `extends`

Inherit configuration from another named process and override specific fields. See [Section 14](#14-config-inheritance-extends).

---

## 5. Namespace and Instances

### Namespaces

Namespaces provide a logical grouping prefix for processes. They affect the full process name used in CLI commands and display output.

```yaml
processes:
  - name: "api"
    namespace: "backend"
    # Full name: backend/api

  - name: "db"
    namespace: "infra"
    # Full name: infra/db
```

CLI usage:

```bash
runix stop backend/api
runix list --namespace backend
```

### Multi-instance processes

Set `instances: N` where `N > 1` to run multiple copies of a process. Each instance gets a zero-based index suffix.

```yaml
- name: "api"
  namespace: "backend"
  instances: 3
  entrypoint: "./cmd/api"
```

This creates three processes:

| Instance | Full Name       | Instance Index |
| -------- | --------------- | -------------- |
| 1        | `backend/api:0` | 0              |
| 2        | `backend/api:1` | 1              |
| 3        | `backend/api:2` | 2              |

### Naming rules

The `FullName()` format follows these rules:

```
name
namespace/name
name:index        (when instances > 1)
namespace/name:index
```

- If `namespace` is set, it's prefixed with `/`
- If `instances > 1`, the zero-based `:index` suffix is appended
- Process names must be unique (checked at the `name` level, not `FullName`)

---

## 6. Runtime-Specific Behavior

### Go

**Detection:** `go.mod`

**Entrypoint behavior:**

| Entrypoint       | Command                                     |
| ---------------- | ------------------------------------------- |
| `"."`            | `go run .`                                  |
| `"main.go"`      | `go run main.go`                            |
| `"./cmd/api"`    | `go run ./cmd/api`                          |
| `"./bin/server"` | `./bin/server` (binary, if no `.go` suffix) |

**Interpreter:** Overrides `go` binary (e.g., `interpreter: "/usr/local/go/bin/go"`).

```yaml
- name: "api"
  runtime: "go"
  entrypoint: "./cmd/api"
  args: ["--port", "8080"]
```

### Python

**Detection:** `requirements.txt`, `pyproject.toml`, `setup.py`, `Pipfile`, or any `*.py` file in the directory.

**Interpreter resolution:**

1. `interpreter` field if set
2. `venv/bin/python` or `.venv/bin/python` if a virtual environment exists
3. `python3` via PATH lookup
4. `python` via PATH lookup
5. Fallback: `"python3"`

```yaml
- name: "worker"
  runtime: "python"
  entrypoint: "worker.py"
  cwd: "./worker"
  interpreter: "/usr/bin/python3.12" # optional override
```

### Node.js

**Detection:** `package.json`

**TypeScript handling:** If the entrypoint ends in `.ts` or `.tsx`, Runix automatically uses `npx tsx` to run it.

| Entrypoint               | Command                |
| ------------------------ | ---------------------- |
| `"server.js"`            | `node server.js`       |
| `"app.ts"`               | `npx tsx app.ts`       |
| `"app.ts"` + interpreter | `<interpreter> app.ts` |

```yaml
- name: "backend"
  runtime: "node"
  entrypoint: "server.ts"
  args: ["--inspect"]
```

### Bun

**Detection:** `bun.lockb`, `bunfig.toml`, `bun.lock`

**Command:** `bun run <entrypoint> [args...]`

```yaml
- name: "frontend"
  runtime: "bun"
  entrypoint: "dev"
  cwd: "./web"
```

### Deno

**Detection:** `deno.json`, `deno.jsonc`

**Command:** `deno run [args...] <entrypoint>`

Deno permissions are passed as `args` (placed between `run` and the entrypoint):

```yaml
- name: "deno-api"
  runtime: "deno"
  entrypoint: "main.ts"
  args: ["--allow-net", "--allow-env", "--allow-read"]
  # Produces: deno run --allow-net --allow-env --allow-read main.ts
```

**Interpreter:** Overrides the `deno` binary path.

### Ruby

**Detection:** `Gemfile`, `Gemfile.lock`

**Interpreter resolution:**

1. `interpreter` field if set
2. `ruby` via PATH lookup (`exec.LookPath`)
3. Fallback: `"ruby"`

This handles rbenv, rvm, and other version managers automatically since their shims are on PATH.

**Commands:**

| `use_bundle`         | Entrypoint    | Command                               |
| -------------------- | ------------- | ------------------------------------- |
| `false`              | `"app.rb"`    | `ruby app.rb`                         |
| `true`               | `"config.ru"` | `bundle exec ruby config.ru`          |
| `true` + interpreter | `"config.ru"` | `bundle exec <interpreter> config.ru` |

```yaml
# Rails with Bundler
- name: "rails-api"
  runtime: "ruby"
  entrypoint: "config.ru"
  use_bundle: true
  args: ["-p", "3000", "-o", "0.0.0.0"]

# Simple Ruby script
- name: "ruby-worker"
  runtime: "ruby"
  entrypoint: "worker.rb"
```

### PHP

**Detection:** `composer.json`, `artisan`, or any `*.php` file in the directory.

**Interpreter resolution:**

1. `interpreter` field if set
2. `php` via PATH lookup (`exec.LookPath`)
3. Fallback: `"php"`

**Command:** `php <entrypoint> [args...]`

```yaml
# Laravel artisan serve
- name: "laravel-api"
  runtime: "php"
  entrypoint: "artisan"
  args: ["serve", "--host=0.0.0.0", "--port=8000"]
  # Produces: php artisan serve --host=0.0.0.0 --port=8000

# Laravel queue worker
- name: "php-worker"
  runtime: "php"
  entrypoint: "artisan"
  args: ["queue:work"]
  # Produces: php artisan queue:work

# Simple PHP script
- name: "script"
  runtime: "php"
  entrypoint: "app.php"
  # Produces: php app.php
```

---

## 7. Hooks

Hooks are shell commands executed at specific points in the process lifecycle. All hooks run via `sh -c <command>` in the process's working directory with the process's environment plus `RUNIX=true` and `RUNIX_HOOK=true`.

### Available Hooks

| Hook               | Timing                               | Blocking?                          | Typical Use                     |
| ------------------ | ------------------------------------ | ---------------------------------- | ------------------------------- |
| `pre_start`        | Before process starts                | **Yes** — failure prevents start   | Build, install dependencies     |
| `post_start`       | After process enters `running` state | No — failure logged only           | Health check ping, notification |
| `pre_stop`         | Before stop signal sent              | No — failure logged only           | Drain connections               |
| `post_stop`        | After process fully stopped          | No — failure logged only           | Cleanup, remove temp files      |
| `pre_restart`      | Before restart sequence              | **Yes** — failure prevents restart | Save state                      |
| `post_restart`     | After restart completes              | No — failure logged only           | Verify health                   |
| `pre_reload`       | Before config reload                 | **Yes** — failure prevents reload  | Backup config                   |
| `post_reload`      | After config reloaded                | No — failure logged only           | Verify state                    |
| `pre_healthcheck`  | Before health check runs             | No — failure logged only           | Log diagnostic info             |
| `post_healthcheck` | After health check completes         | No — failure logged only           | Log results                     |

### Hook Configuration

Each hook is an object with three fields:

| Field            | Type     | Required | Default | Description                                |
| ---------------- | -------- | -------- | ------- | ------------------------------------------ |
| `command`        | string   | **Yes**  |         | Shell command to execute                   |
| `timeout`        | duration | No       | `"30s"` | Maximum execution time                     |
| `ignore_failure` | bool     | No       | `false` | If true, errors are logged but don't block |

```yaml
hooks:
  pre_start:
    command: "make build"
    timeout: "30s"
  post_start:
    command: "curl -sf http://localhost:8080/health || true"
    timeout: "5s"
    ignore_failure: true
  pre_stop:
    command: "echo 'Draining connections'"
  post_stop:
    command: "make clean"
  pre_healthcheck:
    command: "echo 'Checking health'"
    ignore_failure: true
  post_healthcheck:
    command: "echo 'Health check done'"
    ignore_failure: true
```

### Hook Behavior Details

**Pre-hooks** (`pre_start`, `pre_restart`, `pre_reload`) are **blocking**:

- If the hook fails and `ignore_failure` is `false`, the lifecycle action is aborted.
- The process remains in its current state.
- The error is returned to the caller.

**Post-hooks** (`post_start`, `post_stop`, `post_restart`, `post_reload`, `pre_healthcheck`, `post_healthcheck`) are **non-blocking**:

- Failures are logged but never abort the operation.
- The lifecycle action has already completed; the hook is informational.

**Timeout:** If a hook exceeds its `timeout`, it is cancelled via context and treated as a failure.

**Environment:** Hooks inherit the process's `env` overlay plus `RUNIX=true` and `RUNIX_HOOK=true`.

---

## 8. Watch and Auto-Reload

File watching enables automatic process restart when source files change. Useful during development.

### Configuration

```yaml
watch:
  enabled: true
  paths: ["./src", "./app"]
  ignore: ["node_modules/", ".git/", "dist/"]
  debounce: "200ms"
```

| Field      | Type        | Required | Default                        | Description                       |
| ---------- | ----------- | -------- | ------------------------------ | --------------------------------- |
| `enabled`  | bool        | **Yes**  | `false`                        | Enable file watching              |
| `paths`    | string list | No       | `[cwd]`                        | Directories to watch              |
| `ignore`   | string list | No       | built-in patterns              | Glob patterns to skip             |
| `debounce` | string      | No       | from `defaults.watch_debounce` | Minimum interval between restarts |

**Built-in ignore patterns:** `.git`, `node_modules`, `__pycache__`, `*.pyc`, `.DS_Store`, `vendor`, `dist`, `build`, `bin`.

### How it works

1. Runix uses `fsnotify` to watch the specified paths recursively.
2. File change events are debounced (multiple rapid changes → single restart).
3. The process is stopped and restarted via the standard lifecycle (respecting `stop_signal`, `stop_timeout`).
4. Pre/post hooks run during the restart.

### Graceful vs forced stop

When a restart is triggered:

1. Runix sends `stop_signal` (default: `SIGTERM`) to the process group.
2. Waits up to `stop_timeout` (default: `5s`) for the process to exit.
3. If the process hasn't exited, sends `SIGKILL` to force-terminate.
4. After exit, starts the process again.

---

## 9. Health Checks

### Configuration

```yaml
healthcheck:
  type: http # http, tcp, or command
  url: "http://localhost:8080/health"
  interval: "10s"
  timeout: "5s"
  retries: 3
  grace_period: "30s"
```

| Field          | Type   | Required      | Default | Description                                   |
| -------------- | ------ | ------------- | ------- | --------------------------------------------- |
| `type`         | string | **Yes**       |         | `http`, `tcp`, or `command`                   |
| `url`          | string | for `http`    |         | HTTP URL to check                             |
| `tcp_endpoint` | string | for `tcp`     |         | Host:port to check                            |
| `command`      | string | for `command` |         | Shell command to run                          |
| `interval`     | string | No            |         | Time between checks                           |
| `timeout`      | string | No            |         | Per-check timeout                             |
| `retries`      | int    | No            |         | Consecutive failures before marking unhealthy |
| `grace_period` | string | No            |         | Initial period before checks start            |

### Types

**HTTP:** Sends a GET request. Status 200-399 = healthy.

```yaml
healthcheck:
  type: http
  url: "http://localhost:8080/health"
  interval: "10s"
  timeout: "5s"
  retries: 3
  grace_period: "30s"
```

**TCP:** Attempts a TCP connection. Successful connection = healthy.

```yaml
healthcheck:
  type: tcp
  tcp_endpoint: "localhost:5432"
  interval: "10s"
  timeout: "3s"
  retries: 5
  grace_period: "15s"
```

**Command:** Runs a shell command. Exit code 0 = healthy.

```yaml
healthcheck:
  type: command
  command: "test -f /tmp/worker_healthy"
  interval: "30s"
  timeout: "5s"
  retries: 2
```

### Shorthand

For simple HTTP checks, use `healthcheck_url` instead:

```yaml
healthcheck_url: "http://localhost:8080/health"
```

---

## 10. Logging

### Log file structure

Each process gets separate stdout and stderr log files:

```
~/.runix/apps/
├── <process-id>/
│   ├── stdout.log
│   └── stderr.log
```

Legacy: A combined `app.log` is also maintained for backward compatibility.

### Log rotation settings

Controlled at two levels — `defaults` (global) and per-process:

| Field           | Type     | Description                                |
| --------------- | -------- | ------------------------------------------ |
| `log_max_size`  | int64    | Max bytes before rotation (default: 10 MB) |
| `log_max_age`   | duration | Max age before rotation (default: 7 days)  |
| `log_max_files` | int      | Max rotated files to keep                  |

Per-process overrides:

```yaml
- name: "api"
  log_max_files: 10
```

---

## 11. Cron Jobs

Scheduled shell commands that run independently of managed processes.

```yaml
cron:
  - name: "cleanup"
    schedule: "0 2 * * *"
    command: "scripts/cleanup.sh"
    cwd: "."
    env:
      CLEANUP_MODE: "full"
    timeout: "5m"
    enabled: true
```

| Field      | Type     | Required | Default | Description                    |
| ---------- | -------- | -------- | ------- | ------------------------------ |
| `name`     | string   | **Yes**  |         | Unique job name                |
| `schedule` | string   | **Yes**  |         | Cron expression (5 or 6 field) |
| `command`  | string   | **Yes**  |         | Shell command to execute       |
| `runtime`  | string   | No       |         | Runtime to use                 |
| `cwd`      | string   | No       |         | Working directory              |
| `env`      | map      | No       | `{}`    | Environment variables          |
| `timeout`  | duration | No       |         | Maximum execution time         |
| `enabled`  | bool     | No       | `true`  | Whether the job is active      |

### Cron expression format

```
# 6-field (with seconds)
┌───────── second (0-59)
│ ┌───────── minute (0-59)
│ │ ┌───────── hour (0-23)
│ │ │ ┌───────── day of month (1-31)
│ │ │ │ ┌───────── month (1-12)
│ │ │ │ │ ┌───────── day of week (0-6, Sun=0)
* * * * * *

# 5-field (standard, seconds default to 0)
┌───────── minute (0-59)
│ ┌───────── hour (0-23)
│ │ ┌───────── day of month (1-31)
│ │ │ ┌───────── month (1-12)
│ │ │ │ ┌───────── day of week (0-6, Sun=0)
* * * * *
```

**Examples:**

| Expression         | Meaning                              |
| ------------------ | ------------------------------------ |
| `"0 2 * * *"`      | Every day at 2:00 AM                 |
| `"*/30 * * * * *"` | Every 30 seconds                     |
| `"0 */6 * * *"`    | Every 6 hours                        |
| `"0 0 1 * *"`      | First day of every month at midnight |
| `"30 9 * * 1-5"`   | Weekdays at 9:30 AM                  |

---

## 12. Secrets

Named secret references that abstract how sensitive values are obtained.

```yaml
secrets:
  db_password:
    type: env
    value: "DB_PASSWORD"
  api_key:
    type: file
    value: "/run/secrets/api_key"
  vault_token:
    type: vault
    value: "secret/path#key"
```

| Field   | Type   | Required | Description               |
| ------- | ------ | -------- | ------------------------- |
| `type`  | string | **Yes**  | `env`, `file`, or `vault` |
| `value` | string | **Yes**  | Source identifier         |

**Types:**

| Type    | Behavior                                               |
| ------- | ------------------------------------------------------ |
| `env`   | Reads from the environment variable named by `value`   |
| `file`  | Reads the file at the path given by `value`            |
| `vault` | Reads from a Vault secret at the path given by `value` |

For Vault secrets, Runix reads `VAULT_ADDR` and `VAULT_TOKEN` from the environment. The `value` field uses `path#key` syntax, for example `secret/myapp#password`. Runix tries KV v2 first and falls back to KV v1.

---

## 13. Profiles

Per-environment overrides for process environment variables. Activate with `--profile <name>`.

```yaml
profiles:
  staging:
    api:
      PORT: "8081"
      LOG_LEVEL: "debug"
  production:
    api:
      LOG_LEVEL: "warn"
      DATABASE_URL: "postgres://prod-db/myapp"
```

Structure: `profiles.<profile-name>.<process-name>.<env-key>: <value>`

When activated, the specified env vars override the process's `env` section for matching process names.

```bash
runix start --profile staging
```

---

## 14. Config Inheritance (`extends`)

A process can inherit all configuration from another named process and override specific fields.

```yaml
processes:
  - name: "api"
    runtime: "go"
    entrypoint: "./cmd/api"
    args: ["--port", "8080"]
    env:
      PORT: "8080"
      HOST: "0.0.0.0"
    tags: ["backend", "http"]

  - name: "shared-api"
    extends: "api"
    entrypoint: "./cmd/shared-api"
    args: ["--port", "9090"]
    tags: ["backend", "shared"]
    env:
      PORT: "9090"
      MODE: "shared"
```

### Merge behavior

- Child values override parent values.
- `env` maps are merged (child keys override parent keys; parent keys not in child are preserved).
- Slices (`args`, `tags`, `depends_on`) are **replaced**, not appended.
- Complex objects (`watch`, `hooks`, `healthcheck`) are replaced entirely.

### Validation

- Circular extends are detected and rejected: `A extends B extends A`.
- Extending a non-existent process is an error.
- Chain extends are supported: `A extends B extends C` resolves correctly.

---

## 15. Persistence and Storage

### Data directory

```
~/.runix/
├── apps/           # Per-process log directories
│   └── <id>/
│       ├── stdout.log
│       └── stderr.log
├── state/          # Process state persistence
│   └── state.json
├── tmp/
│   ├── runix.sock  # Unix domain socket for IPC
│   └── runix.pid   # Daemon PID file
└── dump.json       # Full state dump for save/resurrect
```

### Save and Resurrect

- **`runix save`** — Writes all running process configurations to `~/.runix/dump.json`.
- **`runix resurrect`** — Reads `dump.json` (falls back to `state.json` for legacy compat) and restores all saved processes.

Socket permissions: `0o660` (owner + group read/write). PID file: `0o644`.

---

## 16. Validation Rules

Runix validates the config on load. The following rules are enforced:

### Process validation

| Rule                                | Error                                                 |
| ----------------------------------- | ----------------------------------------------------- |
| `name` is empty                     | `"process name is required"`                          |
| `entrypoint` is empty               | `"entrypoint is required for process <name>"`         |
| `restart_policy` is invalid         | `"invalid restart_policy <value> for process <name>"` |
| `runtime` is not a recognized value | `"invalid runtime <value> for process <name>"`        |
| Duplicate process `name`            | `"duplicate process name: <name>"`                    |

### Valid runtimes

`go`, `python`, `node`, `bun`, `deno`, `ruby`, `php`, `auto`, `unknown`

### Valid restart policies

`always`, `on-failure`, `never`, `""` (empty = use default)

### Cron validation

| Rule                | Error                                                                            |
| ------------------- | -------------------------------------------------------------------------------- |
| `name` is empty     | `"cron[<index>]: name is required"`                                              |
| `schedule` is empty | `"cron[<index>]: schedule is required"`                                          |
| `command` is empty  | `"command is required for cron job <name>"` (validated at `CronJobConfig` level) |

### Valid stop signals

`SIGTERM`, `TERM`, `SIGINT`, `INT`, `SIGQUIT`, `QUIT`, `SIGUSR1`, `USR1`, `SIGUSR2`, `USR2`, `SIGKILL`, `KILL`. Unrecognized values fall back to `SIGTERM`.

---

## 17. Path Resolution

### Resolution rules

| Field                         | Resolution                                                                                                                                      |
| ----------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| Config file path (`--config`) | Resolved to absolute via `filepath.Abs()`                                                                                                       |
| `cwd`                         | If empty: config file directory. If relative: resolved relative to config file directory. If absolute: used as-is.                              |
| `entrypoint`                  | Used as provided — passed directly to the runtime adapter or `exec.Command`. Relative paths resolve against `cwd` at execution time.            |
| `interpreter`                 | Passed to `exec.Command`. If just a name (e.g., `"ruby"`), the OS resolves it via `PATH` at execution time. If an absolute path, used directly. |
| Hook `command`                | Executed via `sh -c` in `cwd`. Shell handles path resolution.                                                                                   |
| Log files                     | Created under `<data_dir>/apps/<process-id>/`. Managed internally.                                                                              |
| Socket/PID                    | Created under `<data_dir>/tmp/`. Managed internally.                                                                                            |

### Important notes

- Runtime adapters (Python, Ruby, PHP, Deno) use `exec.LookPath()` to resolve interpreter names to absolute paths. This means the resolved path depends on the `PATH` at the time of process start.
- Paths in `env` values are **not** resolved by Runix — they're passed as-is.
- `~` in paths is handled by the shell, not by Runix. For `daemon.data_dir`, use quotes: `"~/.runix"`.

---

## 18. Complete Examples

### Basic single process

```yaml
processes:
  - name: "api"
    entrypoint: "./cmd/api"
```

### Go API with health checks and hooks

```yaml
processes:
  - name: "api"
    namespace: "backend"
    runtime: "go"
    entrypoint: "./cmd/api"
    args: ["--port", "8080"]
    cwd: "."
    instances: 2
    tags: ["backend", "http"]
    priority: 10
    depends_on: ["db"]
    env:
      PORT: "8080"
      HOST: "0.0.0.0"
    restart_policy: "always"
    max_restarts: 5
    stop_signal: "SIGTERM"
    stop_timeout: "10s"
    healthcheck:
      type: http
      url: "http://localhost:8080/health"
      interval: "10s"
      timeout: "5s"
      retries: 3
      grace_period: "30s"
    hooks:
      pre_start:
        command: "make build"
        timeout: "30s"
      post_stop:
        command: "make clean"
```

### Python worker with venv

```yaml
processes:
  - name: "worker"
    namespace: "backend"
    runtime: "python"
    entrypoint: "worker.py"
    cwd: "./worker"
    tags: ["backend", "jobs"]
    restart_policy: "on-failure"
    hooks:
      pre_start:
        command: "pip install -q -r requirements.txt"
        timeout: "30s"
        ignore_failure: true
```

### Node.js TypeScript app

```yaml
processes:
  - name: "backend"
    runtime: "node"
    entrypoint: "server.ts"
    cwd: "./backend"
    args: ["--inspect"]
    watch:
      enabled: true
      paths: ["./src"]
      ignore: ["node_modules/", "dist/"]
      debounce: "200ms"
```

### Bun frontend with watch

```yaml
processes:
  - name: "frontend"
    runtime: "bun"
    entrypoint: "dev"
    cwd: "./web"
    tags: ["frontend", "dev"]
    watch:
      enabled: true
      paths: ["./src", "./pages"]
      ignore: ["node_modules/", ".next/"]
      debounce: "100ms"
    hooks:
      pre_start:
        command: "bun install"
        timeout: "60s"
```

### Deno API with permissions

```yaml
processes:
  - name: "deno-api"
    runtime: "deno"
    entrypoint: "main.ts"
    args: ["--allow-net", "--allow-env", "--allow-read"]
    cwd: "./deno-api"
    tags: ["backend", "deno"]
    restart_policy: "on-failure"
    watch:
      enabled: true
      paths: ["./src"]
      ignore: ["node_modules/"]
      debounce: "200ms"
```

### Ruby on Rails with Bundler

```yaml
processes:
  - name: "rails-api"
    runtime: "ruby"
    entrypoint: "config.ru"
    use_bundle: true
    args: ["-p", "3000", "-o", "0.0.0.0"]
    cwd: "./rails-api"
    tags: ["backend", "ruby", "rails"]
    restart_policy: "on-failure"
    env:
      RAILS_ENV: "development"
    watch:
      enabled: true
      paths: ["./app", "./config", "./lib"]
      ignore: ["tmp/", "log/", "node_modules/"]
      debounce: "200ms"
```

### PHP / Laravel application

```yaml
processes:
  - name: "laravel-api"
    runtime: "php"
    entrypoint: "artisan"
    args: ["serve", "--host=0.0.0.0", "--port=8000"]
    cwd: "./laravel-api"
    tags: ["backend", "php", "laravel"]
    env:
      APP_ENV: "development"
    watch:
      enabled: true
      paths: ["./app", "./routes", "./config"]
      ignore: ["storage/", "vendor/"]
      debounce: "200ms"

  - name: "php-worker"
    runtime: "php"
    entrypoint: "artisan"
    args: ["queue:work"]
    cwd: "./laravel-api"
    tags: ["backend", "php", "worker"]
    restart_policy: "always"
```

### Multi-instance namespaced app

```yaml
processes:
  - name: "api"
    namespace: "backend"
    runtime: "go"
    entrypoint: "./cmd/api"
    instances: 3
    args: ["--port", "8080"]
    env:
      PORT: "8080"
```

Creates: `backend/api:0`, `backend/api:1`, `backend/api:2`.

### Config inheritance

```yaml
processes:
  - name: "api"
    runtime: "go"
    entrypoint: "./cmd/api"
    args: ["--port", "8080"]
    env:
      PORT: "8080"
    tags: ["backend"]

  - name: "shared-api"
    extends: "api"
    entrypoint: "./cmd/shared-api"
    args: ["--port", "9090"]
    env:
      PORT: "9090"
      MODE: "shared"
```

### Cron job

```yaml
cron:
  - name: "cleanup"
    schedule: "0 2 * * *"
    command: "scripts/cleanup.sh"
    cwd: "."
    env:
      CLEANUP_MODE: "full"
    timeout: "5m"
    enabled: true
```

### Full production config

```yaml
daemon:
  data_dir: "~/.runix"
  log_level: "info"

defaults:
  restart_policy: "on-failure"
  max_restarts: 10
  backoff_base: "1s"
  backoff_max: "60s"
  log_max_size: "10MB"
  log_max_age: "7d"
  watch_debounce: "100ms"

secrets:
  db_password:
    type: env
    value: "DB_PASSWORD"
  api_key:
    type: file
    value: "/run/secrets/api_key"

profiles:
  staging:
    api:
      PORT: "8081"
      LOG_LEVEL: "debug"
  production:
    api:
      LOG_LEVEL: "warn"

processes:
  - name: "api"
    namespace: "backend"
    runtime: "go"
    entrypoint: "./cmd/api"
    args: ["--port", "8080"]
    cwd: "."
    instances: 2
    tags: ["backend", "http"]
    priority: 10
    depends_on: ["db"]
    env:
      PORT: "8080"
      HOST: "0.0.0.0"
    restart_policy: "always"
    max_restarts: 5
    stop_signal: "SIGTERM"
    stop_timeout: "10s"
    cpu_quota: "50%"
    memory_limit: "512MB"
    log_max_files: 10
    healthcheck:
      type: http
      url: "http://localhost:8080/health"
      interval: "10s"
      timeout: "5s"
      retries: 3
      grace_period: "30s"
    hooks:
      pre_start:
        command: "make build"
        timeout: "30s"
      post_start:
        command: "curl -sf http://localhost:8080/health || true"
        timeout: "5s"
        ignore_failure: true
      post_stop:
        command: "make clean"

  - name: "worker"
    namespace: "backend"
    runtime: "python"
    entrypoint: "worker.py"
    cwd: "./worker"
    priority: 20
    depends_on: ["api"]
    restart_policy: "on-failure"

  - name: "frontend"
    runtime: "bun"
    entrypoint: "dev"
    cwd: "./web"
    priority: 30
    watch:
      enabled: true
      paths: ["./src", "./pages"]
      ignore: ["node_modules/", ".next/"]

  - name: "db"
    namespace: "infra"
    runtime: "go"
    entrypoint: "./db-proxy"
    priority: 5
    healthcheck:
      type: tcp
      tcp_endpoint: "localhost:5432"
      interval: "10s"
      timeout: "3s"
      retries: 5
      grace_period: "15s"

cron:
  - name: "cleanup"
    schedule: "0 2 * * *"
    command: "scripts/cleanup.sh"
    enabled: true

web:
  enabled: false
  listen: "localhost:9615"

mcp:
  enabled: false
  transport: "stdio"
```

---

## 19. Best Practices

### Organizing large configs

- **Split by concern:** Group processes by namespace (`backend/`, `infra/`, `frontend/`).
- **Use `extends`:** Define a base process and create variants instead of duplicating config.
- **Use `defaults`:** Set common policies once instead of repeating them per-process.
- **Use `profiles`:** Keep environment-specific values in profiles rather than separate config files.

### Naming conventions

- Use lowercase, hyphen-separated names: `api-server`, `queue-worker`, `db-migrator`.
- Keep names short but descriptive. Avoid generic names like `app` or `process1`.
- Use namespaces for multi-service projects: `backend/api`, `frontend/web`, `infra/redis`.

### Instance strategy

- Start with `instances: 1`. Scale up only when needed.
- Each instance gets its own log files, PID, and restart counter.
- Use load balancing upstream (nginx, caddy) to distribute traffic across instances.

### Restart safety

- Set `max_restarts` with a `restart_window` to prevent restart loops.
- Use `backoff_base` and `backoff_max` to avoid thundering herd on shared infrastructure.
- Use `on-failure` as the default policy. Reserve `always` for critical services.

### Hook usage

- Keep hook commands fast. Long-running hooks delay lifecycle transitions.
- Set explicit `timeout` values. The default 30s may be too long or too short.
- Use `ignore_failure: true` for non-critical post-hooks (notifications, logging).
- Never use `ignore_failure: true` on `pre_start` hooks that perform essential setup (builds, migrations).

### Watch configuration

- Be specific with `paths`. Watching `.` is slower than watching `["./src"]`.
- Always exclude `vendor/`, `node_modules/`, `.git/`, and build output directories.
- Use a `debounce` of 100–300ms to batch rapid file saves.

---

## 20. Troubleshooting

### Wrong runtime detected

**Problem:** Runix detects Node instead of Deno, or Python instead of Ruby.

**Fix:** Set `runtime` explicitly:

```yaml
runtime: "deno"
```

Or use the CLI flag: `runix start main.ts --runtime deno`.

### Missing interpreter

**Problem:** `python3: not found` or similar.

**Fix:** Set `interpreter` to the full path:

```yaml
interpreter: "/usr/bin/python3.11"
```

Or ensure the binary is on your `PATH`.

### Relative path issues

**Problem:** Entrypoint not found.

**Fix:** Paths in `cwd` resolve relative to the config file directory, not your shell's cwd. Either:

- Use absolute paths: `cwd: "/home/user/project/web"`
- Or ensure the relative path is correct from the config file's location

### Duplicate process names

**Problem:** `"duplicate process name: \"api\""`

**Fix:** Every `name` in `processes` must be unique, even across namespaces. Rename one:

```yaml
- name: "api-v1"
- name: "api-v2"
```

### Bad hook commands

**Problem:** Hook fails with "command not found".

**Fix:** Hook commands run via `sh -c`. Ensure the command is valid shell syntax and that binaries are on `PATH`. For complex commands, wrap in a script:

```yaml
hooks:
  pre_start:
    command: "bash scripts/setup.sh"
    timeout: "60s"
```

### Invalid YAML

**Problem:** Config fails to parse.

**Common causes:**

- Unquoted special characters: `PORT: 8080` should be `PORT: "8080"`
- Inconsistent indentation (YAML requires consistent spaces, not tabs)
- Missing colon after key: `name "api"` should be `name: "api"`
- Trailing content after a multiline value

**Fix:** Validate with: `runix validate --config runix.yaml`

### Processes not starting on daemon boot

**Problem:** After `runix resurrect`, processes don't start.

**Fix:** Check that `entrypoint` paths still exist. `dump.json` stores the original config, including relative paths that depend on the working directory at save time. Run `runix doctor` to diagnose.

### Process immediately crashes

**Problem:** Process enters `crashed` state immediately after starting.

**Diagnosis:** Check logs: `runix logs <name>`. Common causes:

- Missing dependencies (run install hooks)
- Port already in use
- Invalid command-line arguments
- Missing environment variables

### Process won't stop

**Problem:** `runix stop` hangs.

**Cause:** Process ignores `SIGTERM`. The `stop_timeout` (default: 5s) determines how long before Runix sends `SIGKILL`.

**Fix:** Increase `stop_timeout` or change `stop_signal`:

```yaml
stop_signal: "SIGINT"
stop_timeout: "30s"
```
