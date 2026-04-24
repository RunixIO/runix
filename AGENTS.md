# Runix — Project Knowledge Base

**Module:** `github.com/runixio/runix` | **Go:** 1.25.7+ | **LOC:** ~13k across 90+ `.go` files

## Overview

Process manager and application supervisor. Manages long-running processes with lifecycle control, restart policies, file watching, cron scheduling, and real-time observability via TUI, web dashboard, and MCP.

## Structure

```
runix/
├── cmd/runix/           CLI — 26 cobra command files, registered in root.go
├── internal/
│   ├── supervisor/      Core engine — process lifecycle, state machine, restart/backoff
│   ├── runtime/         Adapters (Go, Python, Node, Bun, Deno, Ruby, PHP) — implement Runtime interface
│   ├── daemon/          HTTP-over-Unix-socket IPC (server, client, protocol)
│   ├── mcp/             MCP server for AI agents (stdio + HTTP transports)
│   ├── web/             Chi HTTP + WebSocket dashboard (embedded frontend)
│   ├── tui/             BubbleTea terminal UI + components
│   ├── config/          Config loading (viper, YAML/JSON/TOML)
│   ├── hooks/           Lifecycle hook execution (sh -c)
│   ├── metrics/         /proc-based metrics collector (Linux-only)
│   ├── scheduler/       Cron jobs (robfig/cron)
│   ├── watcher/         fsnotify file watcher with debounce
│   ├── updater/         Self-update via GitHub Releases API
│   ├── version/         Version/BuildTime injected via ldflags
│   └── e2e/             End-to-end test suite
├── pkg/types/           Shared types — no internal imports (dependency root)
├── sdk/                 Embeddable Go SDK — Manager wraps Supervisor directly
├── configs/             Example runix.yaml
├── scripts/             build.sh, install.sh
└── docs/                Architecture, CLI reference, configuration guides
```

## Dependency Graph

```
pkg/types ← internal/config, internal/hooks, internal/supervisor, ...
internal/supervisor ← internal/hooks, pkg/types
internal/daemon ← internal/supervisor, internal/mcp, internal/metrics, pkg/types
cmd/runix ← internal/daemon, internal/supervisor, internal/runtime, internal/config, ...
sdk ← internal/supervisor, internal/runtime, pkg/types
```

`pkg/types` is the shared kernel — imported by everything, imports nothing from `internal/`.

## Commands

```bash
make build          # → bin/runix  (version + build-time via ldflags)
make test           # go test -race -count=1 ./...
make vet            # go vet ./...
make lint           # golangci-lint run ./...  (silently skipped if not installed)
make fmt            # gofmt -w . ./cmd ./internal ./pkg
make clean          # rm -rf bin/
```

### Targeted test runs

```bash
go test ./internal/supervisor/...           # single package
go test -run TestName ./internal/config/... # single test
go test ./internal/e2e/... -timeout 120s    # E2E suite (no -race flag in CI)
```

CI runs `go test -v -count=1 -race -timeout 120s ./...` then the e2e suite separately without `-race`. Replicate locally with:

```bash
make test && go test -v -count=1 -timeout 120s ./internal/e2e/...
```

## Where to Look

| Task                         | Location                                                                                          |
| ---------------------------- | ------------------------------------------------------------------------------------------------- |
| Add a CLI command            | `cmd/runix/<command>.go` — follow `root.go` registration pattern                                  |
| Add a daemon IPC action      | `internal/daemon/protocol.go` (constant + payload) → `server.go` (handler) → `client.go` (method) |
| Add a runtime adapter        | `internal/runtime/<name>.go` — implement `Runtime` interface                                      |
| Add a shared type            | `pkg/types/<name>.go`                                                                             |
| Add a lifecycle hook         | `internal/hooks/executor.go` — `Executor.RunPre/RunPost`                                          |
| Add a test helper            | Co-locate in `_test.go` file or `internal/e2e/` for integration                                   |
| Modify process state machine | `pkg/types/process.go` (transitions) + `internal/supervisor/process.go` (atomic CAS)              |
| Config defaults              | `internal/config/defaults.go`                                                                     |
| Embed Runix in Go app        | `sdk/` — `Manager` wraps `supervisor.Supervisor` directly                                         |

## Conventions

- **Logging:** `zerolog` structured logging. Return errors; never log-and-return-nil.
- **CLI commands:** Thin cobra files in `cmd/runix/`. All logic in `internal/`.
- **Dual-mode execution:** Every CLI command tries daemon IPC first (`ensureDaemon` → `sendIPC`), falls back to in-process `getSupervisor()`.
- **Error wrapping:** `fmt.Errorf("context: %w", err)` everywhere. Caller decides to log.
- **State machine:** Lock-free via `atomic.Value` + `CompareAndSwap` loop. See `pkg/types/process.go` for `ValidTransitions`.
- **Process lookup:** Exact ID → exact name → unique ID prefix (ambiguous prefix rejected).
- **Runtime detection:** Go → Python → Node.js → Bun → Deno → Ruby → PHP. `--runtime` flag skips detection.
- **Tests:** Table-driven where applicable. Standard library `testing` only (no testify). E2E tests use real supervisor with `sleep` commands.
- **Commits:** Conventional (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`, `perf:`).
- **Build:** `CGO_ENABLED=0`, `-trimpath`, version/build-time via `-ldflags -X`.
- **No `init()` functions** — all initialization is explicit via constructor functions.

## Anti-Patterns (This Project)

- Never use `init()` functions. All setup is explicit via `New*()` constructors.
- Never log and return nil. Return the error; let the caller log.
- Never use CGO. `CGO_ENABLED=0` everywhere (build, CI, release).
- No `context.TODO()` — use `context.Background()` or pass context from caller.
- Child AGENTS.md files must not repeat content from parent.

## Gotchas

- No golangci-lint config file — runs with defaults.
- Metrics (`internal/metrics/`) reads `/proc` and only works on Linux. Other OSes return empty.
- Config file: `runix.yaml` in project root. Example at `configs/runix.example.yaml`.
- Data dir: `~/.runix/` (`apps/`, `state/`, `tmp/`, `dump.json`). Legacy: `~/.local/share/runix/`.
- Save/resurrect reads `dump.json`, falls back to `state.json` for backward compat.
- Release is tag-triggered: `git tag v0.x.0 && git push --tags`. GoReleaser handles the rest.
- Process group signaling: `Setpgid: true` + `Kill(-pid, signal)` signals entire group.
- `exited` channel is `make()`-d fresh on each `Start()`, `close()`-d exactly once by `handleExit()`.
- Web frontend is `//go:embed`-ded into the binary at `internal/web/server.go`.
- Socket permissions: `0o660` (owner+group rw). PID file: `0o644`.
- Some test files use legacy octal `0644`/`0755` instead of Go 1.13+ `0o644`/`0o755`.
