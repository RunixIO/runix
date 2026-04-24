# Runix Memory Audit Report

**Date:** 2026-04-14  
**Scope:** Full codebase — supervisor, daemon, web, metrics, scheduler, watcher, hooks, MCP, TUI, SDK, runtime adapters, CLI  
**Severity Scale:** P0 (data loss / crash / zombie processes) → P1 (leak under load) → P2 (accumulates over time) → P3 (minor)

---

## A. Memory Audit Summary

### Overall Status: **Needs targeted fixes before production hardening**

The core supervisor lifecycle (start → stop → remove) is solid. The state machine, atomic CAS transitions, and basic process tracking are correct. However, there are systemic resource management gaps in the daemon/web layer, the restart/reload path has a critical monitor-cancellation bug, and several modules leak goroutines on shutdown.

### Key Strengths
- Core process lifecycle (start/stop/remove) is clean — files closed, maps cleaned, monitors cancelled
- State machine uses atomic CAS — no lock-induced deadlocks
- Runtime adapters are pure functions — no state, no leaks
- Config loading is stateless — viper instance per call, GC'd normally
- SDK type conversions are allocation-light

### Key Risks
- **P0: RestartProcess/ReloadProcess kill their own monitor** — restarted processes have no exit watcher
- **P0: Untracked AfterFunc timers** can start zombie processes after RemoveProcess/Shutdown
- **P0: SDK Logs() goroutine leaks** if caller abandons the reader
- **P1: Unbounded os.ReadFile** on log files can OOM the daemon
- **P1: 4 goroutine sources** never stopped on daemon shutdown (metrics, WS hub, MCP, scheduler jobs)

---

## B. File / Module Review

### `internal/supervisor/supervisor.go` — Core Engine

| Concern | Severity | Status |
|---------|----------|--------|
| RestartProcess/ReloadProcess `defer cancelMonitor` kills the NEW monitor | P0 | Bug |
| handleExit AfterFunc timer not tracked — races with RemoveProcess | P0 | Bug |
| AddProcess failure path missing `delete(s.byNumeric, proc.NumericID)` | P1 | Bug |
| prefixWriter.buf grows without limit on long lines (no newline) | P1 | Unbounded buffer |
| Resurrect not idempotent — wasted I/O, potential duplicate numeric IDs | P2 | Design |
| RemoveProcess doesn't flush prefixWriter before closing log files | P2 | Data loss |
| startMonitor ignores its ctx parameter | P3 | Dead code |

### `internal/supervisor/process.go` — Process Lifecycle

| Concern | Severity | Status |
|---------|----------|--------|
| exited channel not closed when cmd.Start() fails | P3 | Minor leak |
| restartTimes slice grows unbounded when restartWindow=0 | P1 | Unbounded growth |
| ForceStop handleExit fallback can double-close exited channel | P1 | Race/panic |

### `internal/supervisor/monitor.go` — Exit Watcher

Clean. `Monitor.Run` properly calls `handleExit` callback and exits on context cancellation. Stateless struct is minor GC pressure.

### `internal/daemon/daemon.go` — Daemon Lifecycle

| Concern | Severity | Status |
|---------|----------|--------|
| Metrics collector goroutine never stopped | P1 | Goroutine leak |
| MCP HTTP server goroutine uses context.Background() | P1 | Goroutine leak |
| StartDaemon leaks os.Process handle | P2 | FD leak |
| StartDaemon temp Client Transport never closed | P2 | Goroutine leak |

### `internal/daemon/server.go` — HTTP Handlers

| Concern | Severity | Status |
|---------|----------|--------|
| os.ReadFile loads entire log file into memory | P1 | OOM risk |
| No http.MaxBytesReader on any endpoint | P1 | OOM risk |
| signal.Notify never cleaned up | P3 | Minor |

### `internal/daemon/client.go` — IPC Client

| Concern | Severity | Status |
|---------|----------|--------|
| No Close() method — Transport goroutines leak | P2 | Goroutine leak |

### `internal/web/server.go` + `websocket.go` — Web Dashboard

| Concern | Severity | Status |
|---------|----------|--------|
| WSHub goroutine never stopped | P1 | Goroutine leak |
| WS client readPump blocks forever on hub stop (unbuffered unregister) | P0 | Goroutine leak |
| No WebSocket connection limit | P1 | Resource exhaustion |
| readPump/writePump have no context cancellation | P2 | Slow shutdown |
| Double conn.Close() from both pump goroutines | P3 | Code smell |

### `internal/web/api.go` — REST API

| Concern | Severity | Status |
|---------|----------|--------|
| os.ReadFile loads entire log file; `lines` param computed but ignored | P1 | OOM + bug |
| No http.MaxBytesReader | P1 | OOM risk |

### `internal/metrics/collector.go` — Metrics

| Concern | Severity | Status |
|---------|----------|--------|
| Collector goroutine never stopped | P1 | Goroutine leak |
| Dead PIDs never pruned from map | P2 | Unbounded map growth |
| CPU% always 0 (unimplemented) | P3 | Functional bug |

### `internal/scheduler/scheduler.go` + `job.go` — Cron

| Concern | Severity | Status |
|---------|----------|--------|
| RunJob spawns fire-and-forget goroutine | P1 | Goroutine leak |
| context.Background() with no default timeout in runDirect | P1 | Goroutine leak |
| No process group management for cron commands | P2 | Orphan processes |
| Stop() does not clear jobs map | P3 | Stale references |

### `internal/watcher/watcher.go` — File Watcher

| Concern | Severity | Status |
|---------|----------|--------|
| Stop() panics on double-close of done channel | P1 | Crash bug |
| Restart after Stop creates goroutines that exit immediately | P2 | Correctness |
| events channel never closed | P3 | Minor |

### `internal/hooks/executor.go` — Lifecycle Hooks

| Concern | Severity | Status |
|---------|----------|--------|
| No Setpgid for hook commands — orphan grandchildren on timeout | P2 | Process leak |

### `internal/mcp/transport_stdio.go` — MCP Stdio

| Concern | Severity | Status |
|---------|----------|--------|
| Context parameter completely ignored | P1 | Goroutine leak |

### `internal/mcp/handlers.go` + `resources.go` — MCP Tools

| Concern | Severity | Status |
|---------|----------|--------|
| readLogLines / handleLogsResource read entire log file | P1 | OOM risk |

### `sdk/logs.go` — SDK Log Streaming

| Concern | Severity | Status |
|---------|----------|--------|
| io.Pipe goroutine blocks forever if reader abandoned | P0 | Goroutine + FD leak |
| logStreamReader (the fix) is dead code, never used | P1 | Dead code |
| Default scanner token size (64KB) fails on long lines | P3 | Silent truncation |

### `sdk/sdk.go` — SDK Manager

| Concern | Severity | Status |
|---------|----------|--------|
| Multi-instance AddProcess only cleans up first instance on failure | P1 | Leaked processes |

### `cmd/runix/logs.go` — CLI Logs

| Concern | Severity | Status |
|---------|----------|--------|
| streamLogs/streamAllLogs infinite loops with no cancellation | P2 | Unbounded blocking |
| printFilteredLogs loads all matching lines unbounded | P2 | OOM risk |
| showAllLogs loads all lines from all apps unbounded | P2 | OOM risk |

### `cmd/runix/start.go` — CLI Start

| Concern | Severity | Status |
|---------|----------|--------|
| getSupervisor() creates new supervisor per call, never shut down | P2 | Goroutine leak |

### `cmd/runix/root.go` — CLI Root

| Concern | Severity | Status |
|---------|----------|--------|
| initConfig creates full duplicate command tree | P3 | Wasteful alloc |
| sendIPC uses context.Background() with no timeout | P3 | Unbounded blocking |

### Clean Modules (No Issues)
- `pkg/types/` — Pure value types, no state, no goroutines
- `internal/runtime/` — Stateless adapters, pure functions
- `sdk/config.go`, `sdk/convert.go` — Type definitions and conversions only
- `internal/config/` — Stateless loading, no persistent state

---

## C. Risk List — All Detected Issues

### P0 — Crash / Data Loss / Zombie Processes

| # | Module | Issue | Impact |
|---|--------|-------|--------|
| 1 | supervisor | RestartProcess/ReloadProcess defer cancels the NEW monitor | Restarted process has no exit watcher; crashes go unnoticed |
| 2 | supervisor | AfterFunc timer untracked, races with RemoveProcess/Shutdown | Zombie process starts after removal or shutdown |
| 3 | web | WS client readPump blocks forever when hub stops (unbuffered unregister) | 2 goroutines + websocket conn leaked per client on shutdown |
| 4 | sdk | Logs() goroutine blocks forever on io.Pipe write if reader abandoned | Goroutine + file handle leaked per abandoned Logs() call |

### P1 — Leaks Under Load / OOM Risk

| # | Module | Issue | Impact |
|---|--------|-------|--------|
| 5 | supervisor | AddProcess failure missing byNumeric delete | Stale map entry; nil pointer panic on Get by numeric ID |
| 6 | supervisor | prefixWriter.buf unbounded on long lines | OOM if process writes large output without newlines |
| 7 | supervisor | restartTimes grows unbounded when restartWindow=0 | ~2MB/day per crash-looping process |
| 8 | process | ForceStop double-close of exited channel in race | Panic: close of closed channel |
| 9 | daemon | Metrics collector goroutine + ticker never stopped | Permanent goroutine leak in daemon |
| 10 | daemon | MCP HTTP server goroutine uses context.Background() | Goroutine + HTTP server leaked on shutdown |
| 11 | daemon | os.ReadFile on log files (handleLogs) | OOM on large log files |
| 12 | web | os.ReadFile on log files; lines param ignored | OOM on large log files + functional bug |
| 13 | web | No http.MaxBytesReader on any endpoint | OOM via large request body |
| 14 | web | No WebSocket connection limit | Resource exhaustion attack |
| 15 | web | WSHub goroutine never stopped | Permanent goroutine leak |
| 16 | metrics | Dead PIDs never pruned from map | Steady map growth with process churn |
| 17 | scheduler | RunJob fire-and-forget goroutine | Goroutine leak on hanging commands |
| 18 | scheduler | context.Background() + no default timeout in runDirect | Permanent goroutine leak |
| 19 | mcp | Stdio transport context ignored | Goroutine leak on shutdown |
| 20 | mcp | readLogLines reads entire log file into memory | OOM on large logs |
| 21 | sdk | logStreamReader dead code (fix for #4 never wired) | The intended fix exists but is unused |
| 22 | sdk | Multi-instance AddProcess partial cleanup | Leaked running processes on failure |
| 23 | daemon | Client has no Close() method | Transport goroutine leak per client |

### P2 — Accumulates Over Time

| # | Module | Issue | Impact |
|---|--------|-------|--------|
| 24 | supervisor | Resurrect not idempotent | Wasted I/O, duplicate numeric IDs |
| 25 | supervisor | RemoveProcess doesn't flush prefixWriter | Last partial log line lost |
| 26 | hooks | No Setpgid for hook commands | Orphan grandchild processes |
| 27 | scheduler | No process group for cron commands | Orphan processes accumulate |
| 28 | watcher | Stop() panics on double-close | Crash if Stop called twice |
| 29 | watcher | Restart after Stop: goroutines exit immediately | Watcher is single-use |
| 30 | web | readPump/writePump no context cancellation | Slow shutdown (~60s) |
| 31 | cli | streamLogs infinite loop with no cancellation | Ctrl+C only exit path |
| 32 | cli | printFilteredLogs unbounded allocation | OOM on large filtered logs |
| 33 | cli | showAllLogs unbounded allocation | OOM with many apps |
| 34 | cli | getSupervisor() never shut down | Goroutine leak per CLI command |
| 35 | cli | Watcher never stopped in watch command | Resources held until process exit |
| 36 | daemon | StartDaemon leaks os.Process handle | FD leak |

### P3 — Minor / Code Quality

| # | Module | Issue |
|---|--------|-------|
| 37 | supervisor | startMonitor ignores ctx parameter |
| 38 | process | exited channel not closed on Start failure (no waiter though) |
| 39 | daemon | signal.Notify never cleaned up |
| 40 | metrics | CPU% always 0 (unimplemented) |
| 41 | web | Double conn.Close() from readPump + writePump |
| 42 | scheduler | Stop() doesn't clear jobs map |
| 43 | hooks | Plugin registry grows without reset mechanism |
| 44 | tui | All components recreated on every resize |
| 45 | tui | AppendContent doubles content temporarily |
| 46 | cli | initConfig creates duplicate full command tree |
| 47 | cli | sendIPC uses context.Background(), no timeout |
| 48 | sdk | Empty line filter causes fewer-than-expected tail lines |
| 49 | sdk | Default scanner token size (64KB) fails on long lines |
| 50 | sdk | NewDetector() allocated twice per toTypesConfig call |
| 51 | watcher | events channel never closed |
| 52 | watcher | Debounce flush() can nil out new timer |
| 53 | metrics | Prometheus writer uses Fprintf per metric line |

---

## D. Recommendations — Concrete Fixes

### Fix #1: RestartProcess/ReloadProcess monitor cancellation (P0)

```go
// supervisor.go — RestartProcess
func (s *Supervisor) RestartProcess(ctx context.Context, id string) error {
    proc, err := s.Get(id)
    if err != nil { return err }
    // ... pre-restart hooks ...

    // Cancel old monitor BEFORE stop.
    s.cancelMonitor(proc.ID)

    // Stop the process.
    if err := proc.Stop(proc.stopTimeout); err != nil { ... }
    // ... backoff delay ...
    if err := proc.Start(ctx); err != nil { ... }

    // Start NEW monitor.
    s.startMonitor(ctx, proc)
    // ... post-restart hooks ...
    return nil  // NO defer
}
```

Apply same pattern to `ReloadProcess`.

### Fix #2: Track AfterFunc timers (P0)

```go
// Add to Supervisor struct:
restartTimers map[string]*time.Timer

// In handleExit:
timer := time.AfterFunc(delay, func() {
    s.mu.Lock()
    delete(s.restartTimers, proc.ID)
    s.mu.Unlock()
    // ... existing restart logic ...
})
s.mu.Lock()
s.restartTimers[proc.ID] = timer
s.mu.Unlock()

// In cancelMonitor:
if t, ok := s.restartTimers[id]; ok {
    t.Stop()
    delete(s.restartTimers, id)
}
```

### Fix #3: SDK Logs() — use the logStreamReader that already exists (P0)

```go
// sdk/logs.go — Replace return at end of Logs():
// Create a cancellable context.
logCtx, cancel := context.WithCancel(ctx)

// ... existing goroutine using logCtx instead of ctx ...

return &logStreamReader{
    ReadCloser: r,
    cancel:     cancel,
}, nil
```

This wires up the existing `logStreamReader` struct. When the caller calls `Close()`, it cancels the context, which unblocks the goroutine's context checks.

### Fix #4: Unbounded log reads — seek-based tail (P1)

```go
// Replace os.ReadFile with seek-based tail in daemon/server.go and web/api.go:
func tailFile(path string, maxLines int) (string, error) {
    f, err := os.Open(path)
    if err != nil { return "", err }
    defer f.Close()
    
    const chunkSize = 4096
    buf := make([]byte, chunkSize)
    var lines []string
    offset, _ := f.Seek(0, io.SeekEnd)
    
    for offset > 0 && len(lines) < maxLines {
        readSize := min(chunkSize, offset)
        offset -= readSize
        f.ReadAt(buf[:readSize], offset)
        // count newlines, collect lines
    }
    // ... reverse and join
}
```

### Fix #5: prefixWriter buffer cap (P1)

```go
const maxLineBuffer = 64 * 1024 // 64 KB

func (pw *prefixWriter) Write(p []byte) (int, error) {
    pw.mu.Lock()
    defer pw.mu.Unlock()
    pw.buf = append(pw.buf, p...)
    if len(pw.buf) > maxLineBuffer {
        // Force-flush the partial line.
        ts := time.Now().Format("2006-01-02 15:04:05")
        fmt.Fprintf(pw.writer, "%s %s %s\n", ts, pw.prefix, pw.buf)
        pw.buf = pw.buf[:0]
    }
    // ... existing newline scan ...
}
```

### Fix #6: ForceStop double-close (P1)

```go
// Add to ManagedProcess struct:
closeOnce sync.Once

// In handleExit:
p.closeOnce.Do(func() { close(p.exited) })

// Reset in Start():
p.closeOnce = sync.Once{}
p.exited = make(chan struct{})
```

### Fix #7: Daemon shutdown cleanup (P1)

```go
// daemon.go — Track all resources:
type daemonResources struct {
    collector *metrics.Collector
    mcpCancel context.CancelFunc
    hub       *web.WSHub
}

// In shutdown():
func shutdown() {
    res.collector.Stop()
    res.mcpCancel()
    res.hub.Stop()
}
```

### Fix #8: AddProcess byNumeric cleanup (P1)

```go
// supervisor.go — AddProcess failure path:
s.mu.Lock()
delete(s.procs, proc.ID)
delete(s.byName, cfg.Name)
delete(s.byNumeric, proc.NumericID) // ADD THIS LINE
s.mu.Unlock()
```

### Fix #9: restartTimes bounding (P1)

```go
// process.go — recordRestart():
const maxRestartHistory = 1000

func (p *ManagedProcess) recordRestart() {
    p.mu.Lock()
    defer p.mu.Unlock()
    p.Restarts++
    p.restartTimes = append(p.restartTimes, time.Now())
    
    if p.restartWindow > 0 {
        cutoff := time.Now().Add(-p.restartWindow)
        i := 0
        for i < len(p.restartTimes) && !p.restartTimes[i].After(cutoff) {
            i++
        }
        p.restartTimes = p.restartTimes[i:]
    } else if len(p.restartTimes) > maxRestartHistory {
        p.restartTimes = p.restartTimes[len(p.restartTimes)-maxRestartHistory:]
    }
}
```

### Fix #10: Client Close() method (P2)

```go
// daemon/client.go:
func (c *Client) Close() {
    if t, ok := c.httpClient.Transport.(*http.Transport); ok {
        t.CloseIdleConnections()
    }
}
```

### Fix #11: WebSocket hub shutdown (P1)

```go
// web/server.go — Start method:
defer s.hub.Stop()  // ADD THIS

// web/websocket.go — Make unregister buffered:
unregister: make(chan *wsClient, 16),  // was unbuffered
```

### Fix #12: Scheduler job tracking (P1)

```go
// scheduler.go:
type Scheduler struct {
    cron    *cron.Cron
    jobs    map[string]*Job
    wg      sync.WaitGroup  // ADD
}

func (s *Scheduler) RunJob(name string) error {
    // ...
    s.wg.Add(1)
    go func() {
        defer s.wg.Done()
        job.Run()
    }()
    return nil
}

func (s *Scheduler) Stop() {
    s.cron.Stop()
    s.wg.Wait()  // Wait for in-flight jobs
}
```

### Fix #13: MCP stdio context (P1)

```go
// transport_stdio.go:
func (s *MCPServer) serveStdio(ctx context.Context) error {
    done := make(chan error, 1)
    go func() {
        done <- mcpserver.ServeStdio(s.mcpServer)
    }()
    select {
    case err := <-done:
        return err
    case <-ctx.Done():
        // Close stdin to unblock the server.
        // This requires access to os.Stdin or a wrapper.
        return ctx.Err()
    }
}
```

---

## E. Recommended Tests

### 1. Restart cycle stability
```go
func TestRepeatedRestartCycle(t *testing.T) {
    mgr := newTestManager(t)
    ctx := context.Background()
    id, _ := mgr.AddProcess(ctx, sdk.ProcessConfig{
        Name: "cycle", Binary: "sleep", Args: []string{"1"},
        RestartPolicy: "never",
    })
    for i := 0; i < 100; i++ {
        if err := mgr.Restart(ctx, id); err != nil {
            t.Fatalf("restart %d: %v", i, err)
        }
    }
    // Verify no goroutine leak via runtime.NumGoroutine()
}
```

### 2. Crash-loop restart accumulation
```go
func TestCrashLoopRestartTracking(t *testing.T) {
    // Start a process that exits immediately with RestartPolicy: always
    // Verify restartTimes stays bounded
    // Verify memory doesn't grow over 100+ restart cycles
}
```

### 3. Log streaming abandonment
```go
func TestSDKLogsAbandonedReader(t *testing.T) {
    // Call Logs() with Follow=true
    // Read 1 line, then abandon the reader (don't close)
    // Cancel context
    // Verify goroutine count doesn't increase
}
```

### 4. Large log file handling
```go
func TestLargeLogFileDoesNotOOM(t *testing.T) {
    // Create a 100MB log file
    // Call handleLogs / handleGetLogs
    // Verify memory stays bounded (use runtime.MemStats)
}
```

### 5. WebSocket churn
```go
func TestWebSocketConnectionChurn(t *testing.T) {
    // Open 100 WS connections
    // Close all
    // Verify no goroutine leak
}
```

### 6. Save/resurrect loop
```go
func TestSaveResurrectLoop(t *testing.T) {
    for i := 0; i < 50; i++ {
        mgr, _ := sdk.New(sdk.Config{LogDir: dir})
        mgr.AddProcess(ctx, cfg)
        mgr.Save()
        mgr.Close()
        
        mgr2, _ := sdk.New(sdk.Config{LogDir: dir})
        mgr2.Resurrect()
        mgr2.Close()
    }
    // Verify dump.json doesn't grow unbounded
    // Verify goroutine count stable
}
```

### 7. Daemon lifecycle
```go
func TestDaemonStartStopCycle(t *testing.T) {
    for i := 0; i < 10; i++ {
        StartDaemon()
        // wait for alive
        sendIPC(ActionShutdown, nil)
        // wait for socket removal
    }
    // Verify no leaked goroutines, temp files, or sockets
}
```

---

## F. Limits and Safeguards — Recommended Bounds

| Resource | Current | Recommended Bound | Location |
|----------|---------|-------------------|----------|
| prefixWriter line buffer | Unlimited | 64 KB | supervisor.go |
| restartTimes per process | Unlimited (window=0) | 1000 entries | process.go |
| Log file read (daemon) | Unlimited | 10 MB max read | daemon/server.go |
| Log file read (web) | Unlimited | 10 MB max read | web/api.go |
| Log file read (MCP) | Unlimited | 10 MB max read | mcp/handlers.go |
| HTTP request body | Unlimited | 1 MB | all HTTP handlers |
| WebSocket connections | Unlimited | 100 per server | web/websocket.go |
| WebSocket send buffer | 64 messages | 64 messages (OK) | websocket.go |
| Metrics tracked PIDs | Unlimited (never pruned) | Auto-prune on /proc miss | metrics/collector.go |
| Scheduler job timeout | Unlimited (default) | 5 minutes default | scheduler/job.go |
| SDK Logs tail lines | Unlimited (Tail=0) | Cap at 10000 | sdk/logs.go |
| Scanner token size | 64 KB (Go default) | 1 MB | all log scanners |
