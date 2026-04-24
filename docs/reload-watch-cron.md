# Reload, Watch, and Cron

Runix provides three mechanisms for automated process management: reload for graceful reconfiguration, file watching for development workflows, and cron scheduling for recurring tasks.

## Reload

Reload performs a clean stop-start cycle on a process without the exponential backoff penalty of a restart. It fires reload-specific hooks (`pre_reload`, `post_reload`) instead of restart hooks.

### When to Use Reload vs Restart

| Action      | Backoff                         | Hooks                          | Use Case                            |
| ----------- | ------------------------------- | ------------------------------ | ----------------------------------- |
| **Reload**  | None                            | `pre_reload` / `post_reload`   | Config changes, manual intervention |
| **Restart** | Applied if prior restarts exist | `pre_restart` / `post_restart` | Crash recovery, automated restarts  |

Use reload when you deliberately want to cycle a process (e.g., after updating a config file) and don't want it to accumulate backoff delays.

### Usage

```bash
# Reload a single process
runix reload api-server

# Reload is not available for "all" — target a specific process
```

### How It Works

```
pre_reload hook (blocking)
       │
       v
  Stop process (SIGTERM, wait stop_timeout)
       │
       v
  Start process (fresh exec.Cmd)
       │
       v
post_reload hook (non-blocking)
```

Unlike restart, reload skips the exponential backoff delay between stop and start.

### Configuration

Configure reload behavior through hooks in `runix.yaml`:

```yaml
processes:
  - name: "api"
    entrypoint: "./cmd/api"
    hooks:
      pre_reload:
        command: "make generate"
        timeout: "30s"
      post_reload:
        command: "curl -sf http://localhost:8080/health || true"
        ignore_failure: true
```

See the [Hooks documentation](hooks.md) for full hook configuration details.

---

## Watch

Watch mode monitors file system changes and automatically restarts a process when files are modified. It is built on `fsnotify` with debouncing and ignore pattern support.

### Enabling Watch

**Via configuration:**

```yaml
processes:
  - name: "frontend"
    runtime: "bun"
    entrypoint: "dev"
    cwd: "./web"
    watch:
      enabled: true
      paths: ["./src", "./pages"]
      ignore: ["node_modules/", ".next/"]
```

**Via CLI flags:**

```bash
runix start app.py --watch
runix start app.py --watch --watch-ignore "dist/*" --watch-ignore "*.test.ts"
```

**Attach to a running process:**

```bash
runix watch api-server
runix watch api-server --paths ./src --paths ./internal --debounce 500ms
```

### How It Works

```
File system event (create, write, remove, rename)
       │
       v
  Event loop receives raw fsnotify events
       │
       v
  Filter against ignore patterns (glob matching)
       │
       v
  Coalesce loop: collect events within debounce window
       │
       v
  Deduplicate events
       │
       v
  Trigger supervisor.RestartProcess()
```

The two-stage event pipeline prevents rapid restarts when many files change simultaneously (e.g., `git checkout` or `npm install`).

### Watch Configuration

| Field     | Type        | Default       | Description                                |
| --------- | ----------- | ------------- | ------------------------------------------ |
| `enabled` | bool        | `false`       | Enable file watching                       |
| `paths`   | string list | `[cwd]`       | Directories to watch (watched recursively) |
| `ignore`  | string list | built-in list | Glob patterns to ignore                    |

### Default Ignore Patterns

These patterns are always ignored unless you explicitly override them:

```
.git
node_modules
__pycache__
*.pyc
.DS_Store
vendor
dist
build
bin
```

### CLI Flags for `runix watch`

| Flag         | Default           | Description                          |
| ------------ | ----------------- | ------------------------------------ |
| `--paths`    | process cwd       | Paths to watch (repeatable)          |
| `--ignore`   | built-in defaults | Glob patterns to ignore (repeatable) |
| `--debounce` | `100ms`           | Time window to coalesce events       |

### Best Practices

- Set `debounce` higher (200ms–500ms) for large codebases to avoid thrashing
- Ignore generated output directories (`dist/`, `build/`, `.next/`) to prevent feedback loops
- Ignore test files if you don't want test edits triggering restarts
- Use watch mode primarily in development; avoid it in production

---

## Cron

The cron system schedules recurring tasks and supports scheduled process restarts. It uses `robfig/cron` with seconds-level precision.

### Cron Jobs

Define scheduled tasks in `runix.yaml`:

```yaml
cron:
  - name: "cleanup"
    schedule: "0 0 2 * * *"
    command: "scripts/cleanup.sh"
    cwd: "."
    env:
      CLEANUP_MODE: "full"
    timeout: "5m"
    enabled: true

  - name: "report"
    schedule: "0 0 9 * * 1-5"
    command: "python reports/daily.py"
    cwd: "./reports"
    enabled: true
```

### Cron Job Fields

| Field      | Type     | Required | Default | Description                    |
| ---------- | -------- | -------- | ------- | ------------------------------ |
| `name`     | string   | Yes      |         | Unique job name                |
| `schedule` | string   | Yes      |         | Cron expression (5 or 6 field) |
| `command`  | string   | Yes      |         | Shell command to execute       |
| `runtime`  | string   | No       |         | Runtime to use for execution   |
| `cwd`      | string   | No       |         | Working directory              |
| `env`      | map      | No       | `{}`    | Environment variables          |
| `timeout`  | duration | No       |         | Maximum execution time         |
| `enabled`  | bool     | No       | `true`  | Whether the job is active      |

### Scheduled Process Restarts

Use `cron_restart` on a process to schedule automatic restarts:

```yaml
processes:
  - name: "api"
    entrypoint: "./cmd/api"
    cron_restart: "0 0 3 * * *" # Restart daily at 3 AM
```

### Cron Expression Format

Runix supports both standard 5-field and extended 6-field (with seconds) expressions:

```
# 6-field (with seconds)
┌──────── second (0-59)
│ ┌──────── minute (0-59)
│ │ ┌──────── hour (0-23)
│ │ │ ┌──────── day of month (1-31)
│ │ │ │ ┌──────── month (1-12)
│ │ │ │ │ ┌──────── day of week (0-6, Sun=0)
* * * * * *

# 5-field (standard, seconds default to 0)
┌──────── minute (0-59)
│ ┌──────── hour (0-23)
│ │ ┌──────── day of month (1-31)
│ │ │ ┌──────── month (1-12)
│ │ │ │ ┌──────── day of week (0-6, Sun=0)
* * * * *
```

If a 5-field expression is provided, `"0 "` is automatically prepended for the seconds field.

### Expression Examples

| Expression       | Meaning                          |
| ---------------- | -------------------------------- |
| `0 0 2 * * *`    | Every day at 2:00 AM             |
| `*/30 * * * * *` | Every 30 seconds                 |
| `0 0 */6 * * *`  | Every 6 hours                    |
| `0 30 9 * * 1-5` | Weekdays at 9:30 AM              |
| `0 0 0 1 * *`    | First of every month at midnight |
| `0 */15 * * * *` | Every 15 minutes                 |

### Managing Cron Jobs

```bash
# List all configured jobs
runix cron list

# Manually trigger a job
runix cron run cleanup

# Disable a job
runix cron stop cleanup

# Re-enable a job
runix cron start cleanup
```

### How Cron Jobs Execute

Cron jobs run via `sh -c <command>` in the specified working directory. They inherit the environment of the daemon process, with any configured `env` values overlaid.

If a timeout is configured and the job exceeds it, the job process is killed.

### Best Practices

- Use `timeout` on all cron jobs to prevent runaway processes
- Schedule resource-intensive jobs during off-peak hours
- Use `enabled: false` during development to disable jobs without removing them
- Give each job a descriptive name for easy identification in `cron list`
- Test jobs manually with `runix cron run <name>` before relying on the schedule
