# internal/supervisor/ — Core Engine

Process lifecycle management, state machine, restart/backoff logic.

## Key Types

| Type | File | Purpose |
|------|------|---------|
| `Supervisor` | `supervisor.go` | Registry of `ManagedProcess` instances, CRUD + monitoring |
| `ManagedProcess` | `process.go` | Single process: start/stop/force-stop, state tracking, log capture |
| `Backoff` | `backoff.go` | Exponential backoff: `min(base * 2^attempt + rand(0,jitter), max)` |
| `monitor` | `monitor.go` | Goroutine per process: waits for exit, triggers restart decision |

## Concurrency Model

- `Supervisor.mu` (`sync.RWMutex`): protects `procs`, `byName`, `byNumeric`, `monitors`, `nextID`
- `ManagedProcess.mu` (`sync.RWMutex`): protects `cmd`, `PID`, writers, files
- `ManagedProcess.state` (`atomic.Value`): lock-free state via CAS loop in `SetState()`
- `exited` channel: `make()`-d fresh per `Start()`, `close()`-d once by `handleExit()`

## State Machine

Transitions defined in `pkg/types/process.go:ValidTransitions`. Enforced via CAS:
```
stopped → starting → running → stopping → stopped
running → crashed → waiting → starting (restart loop)
starting → errored → stopped (exec failure)
```

`SetState()` retries CAS in a loop if concurrent state changes occur.

## Process Lifecycle

1. `AddProcess()`: validate → `NewManagedProcess()` → register in maps → `Start()` → `startMonitor()`
2. `Start()`: run pre-hooks → `exec.Cmd.Start()` → state `starting` → `running`
3. `Stop()`: state `stopping` → send signal → wait `exited` or timeout → `ForceStop()`
4. `ForceStop()`: SIGKILL → wait 5s → cleanup
5. Monitor goroutine: on exit → `handleExit()` → `onExit` callback → restart decision

## Process Group Signaling

`cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` — signals sent via `syscall.Kill(-pid, signal)` to the entire process group.

## Save/Resurrect

- `Dump()`: writes all process configs to `~/.runix/dump.json`
- `Resurrect()`: reads `dump.json`, falls back to `state.json` for legacy compat
- Both use `0o644` file permissions

## Benchmarks

`bench_test.go` contains benchmarks for: `BuildEnv`, `List`, `Get`, `GetByName`, `ProcessInfo`, `BuildArgs`, `StateTransition`, `ShouldRestart`.
