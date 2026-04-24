# Lifecycle Hooks

Hooks let you run shell commands at specific points in a process's lifecycle. They are useful for setup, teardown, health checks, notifications, and deployment tasks.

## Configuration

Hooks are configured per-process under the `hooks` key:

```yaml
processes:
  - name: "api"
    entrypoint: "./cmd/api"
    hooks:
      pre_start:
        command: "echo 'Starting API'"
        timeout: "5s"
      post_start:
        command: "curl -sf http://localhost:8080/health || true"
        timeout: "5s"
        ignore_failure: true
      pre_stop:
        command: "echo 'Draining connections'"
      post_stop:
        command: "echo 'Stopped'"
      pre_restart:
        command: "echo 'Restarting'"
      post_restart:
        command: "curl -sf http://localhost:8080/health"
      pre_reload:
        command: "echo 'Reloading'"
      post_reload:
        command: "echo 'Reloaded'"
```

### Hook Fields

| Field            | Type     | Required | Default | Description                                           |
| ---------------- | -------- | -------- | ------- | ----------------------------------------------------- |
| `command`        | string   | Yes      |         | Shell command to execute (run via `sh -c`)            |
| `timeout`        | duration | No       | `"30s"` | Maximum execution time                                |
| `ignore_failure` | bool     | No       | `false` | If true, errors are logged but don't block the action |

## Available Hooks

| Hook               | Fires When                   | Blocking | Description                                       |
| ------------------ | ---------------------------- | -------- | ------------------------------------------------- |
| `pre_start`        | Before process starts        | Yes      | Preparation tasks (migrations, config generation) |
| `post_start`       | After process starts         | No       | Health checks, notifications                      |
| `pre_stop`         | Before process stops         | No       | Drain connections, save state                     |
| `post_stop`        | After process stops          | No       | Cleanup, notifications                            |
| `pre_restart`      | Before restart cycle         | Yes      | Pre-restart preparation                           |
| `post_restart`     | After restart completes      | No       | Post-restart validation                           |
| `pre_reload`       | Before reload cycle          | Yes      | Pre-reload preparation                            |
| `post_reload`      | After reload completes       | No       | Post-reload validation                            |
| `pre_healthcheck`  | Before health check runs     | No       | Log diagnostic info                               |
| `post_healthcheck` | After health check completes | No       | Log results                                       |

## Execution Behavior

### How Hooks Run

- All commands execute via `sh -c <command>` in the process's working directory
- The environment includes the current process environment plus:
  - `RUNIX=true` — indicates Runix is running the hook
  - `RUNIX_HOOK=true` — indicates this is a hook execution
- Both stdout and stderr are captured and written to structured logs
- If a timeout is set and exceeded, the hook process is killed

### Blocking vs Non-Blocking

**Pre-hooks** (`pre_start`, `pre_restart`, `pre_reload`) are **blocking**:

- If the hook exits with a non-zero code, the lifecycle action is aborted
- The process remains in its previous state
- Unless `ignore_failure: true` is set, in which case the error is logged and the action proceeds

**Post-hooks** (`post_start`, `post_stop`, `post_restart`, `post_reload`) and `pre_stop`, `pre_healthcheck`, `post_healthcheck` are **non-blocking**:

- Errors are always logged but never block or undo the action
- The process has already transitioned to its new state

### Hook Placement in Lifecycle

**Start:**

```
pre_start (blocking) → start process → post_start (non-blocking)
```

**Stop:**

```
pre_stop (non-blocking) → send signal → wait for exit → post_stop (non-blocking)
```

**Restart:**

```
pre_restart (blocking) → stop → backoff delay → start → post_restart (non-blocking)
```

**Reload:**

```
pre_reload (blocking) → stop → start → post_reload (non-blocking)
```

## Examples

### Database Migration Before Start

```yaml
processes:
  - name: "api"
    runtime: "go"
    entrypoint: "./cmd/api"
    hooks:
      pre_start:
        command: "goose -dir migrations postgres $DATABASE_URL up"
        timeout: "60s"
```

### Dependency Installation

```yaml
processes:
  - name: "frontend"
    runtime: "bun"
    entrypoint: "dev"
    cwd: "./web"
    hooks:
      pre_start:
        command: "bun install"
        timeout: "120s"
        ignore_failure: true
```

### Health Check After Start

```yaml
processes:
  - name: "api"
    runtime: "go"
    entrypoint: "./cmd/api"
    hooks:
      post_start:
        command: "curl -sf http://localhost:8080/health || exit 1"
        timeout: "10s"
        ignore_failure: true
```

### Graceful Drain Before Stop

```yaml
processes:
  - name: "worker"
    runtime: "python"
    entrypoint: "worker.py"
    hooks:
      pre_stop:
        command: "curl -X POST http://localhost:8080/drain"
        timeout: "30s"
      post_stop:
        command: "echo 'Worker stopped at $(date)' >> /var/log/worker-events.log"
```

### Notification on Restart

```yaml
processes:
  - name: "api"
    runtime: "go"
    entrypoint: "./cmd/api"
    hooks:
      post_restart:
        command: 'curl -sf -X POST https://hooks.slack.com/services/xxx -d ''{"text":"API restarted"}'' || true'
        ignore_failure: true
```

### Multi-Step Reload

```yaml
processes:
  - name: "api"
    runtime: "go"
    entrypoint: "./cmd/api"
    hooks:
      pre_reload:
        command: "make generate && make build"
        timeout: "120s"
      post_reload:
        command: "curl -sf http://localhost:8080/health"
        timeout: "5s"
        ignore_failure: true
```

## Failure Handling

| Scenario                                   | Behavior                                           |
| ------------------------------------------ | -------------------------------------------------- |
| Pre-hook fails (non-zero exit)             | Action is blocked, process stays in previous state |
| Pre-hook fails with `ignore_failure: true` | Error is logged, action proceeds                   |
| Pre-hook times out                         | Hook process is killed, treated as failure         |
| Post-hook fails                            | Error is logged, action is not affected            |
| Post-hook times out                        | Hook process is killed, error is logged            |

## Best Practices

- Keep pre-start hooks fast and idempotent — they block process startup
- Use `ignore_failure: true` for non-critical hooks like health checks that might race with startup
- Set explicit timeouts for hooks that make network calls
- Use post-start hooks for validation, not for actions that the process depends on
- Prefer reload hooks over restart hooks when you want a clean stop/start without backoff
