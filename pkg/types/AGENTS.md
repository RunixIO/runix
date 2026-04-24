# pkg/types/ — Shared Type Kernel

Shared types imported by all `internal/` packages. **Imports nothing from `internal/`.**

## Files

| File | Contents |
|------|----------|
| `process.go` | `ProcessState` (7 states), `ValidTransitions` map, `RestartPolicy`, `ProcessConfig`, `ProcessInfo` |
| `config.go` | `RunixConfig`, `DaemonConfig`, `DefaultsConfig`, `WebConfig`, `MCPConfig` |
| `hooks.go` | `HookConfig`, `ProcessHooks` (pre_start, post_start, pre_stop, post_stop) |
| `cron.go` | `CronJobConfig` with `Validate()` |
| `watch.go` | `WatchConfig` (paths, ignore_patterns, debounce) |

## State Machine

```go
ValidTransitions: {
    starting: [running, errored],
    running:  [stopping, crashed],
    stopping: [stopped],
    stopped:  [starting],
    crashed:  [waiting, stopped],
    waiting:  [starting, stopped],
    errored:  [stopped, starting],
}
```

## Key Invariants

- `ProcessConfig.Validate()`: name and entrypoint are required, restart_policy must be recognized
- `ProcessConfig.FullName()`: returns `namespace/name:index` (e.g. `backend/api:0`)
- `CronJobConfig.Validate()`: name, schedule, and command are required
- `RunixConfig` is the top-level config structure — mirrors `runix.yaml` format

## Test Coverage

`process_test.go`: table-driven tests for `ProcessConfig.Validate()` and `CronJobConfig.Validate()`.
