# Runix Go SDK

Embeddable Go SDK for the Runix process manager. Use it to embed a full process supervisor into your Go application — no CLI binary or daemon required.

## Install

```bash
go get github.com/runixio/runix/sdk
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/runixio/runix/sdk"
)

func main() {
    // Create a manager with a log directory.
    mgr, err := sdk.New(sdk.Config{
        LogDir: "/tmp/myapp-runix",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer mgr.Close()

    ctx := context.Background()

    // Start a Python script.
    id, err := mgr.AddProcess(ctx, sdk.ProcessConfig{
        Name:    "api",
        Script:  "main.py",
        Runtime: "python",
        Env:     map[string]string{"PORT": "8080"},
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Started: %s\n", id)

    // Inspect it.
    info, _ := mgr.Inspect(id)
    fmt.Printf("PID: %d, State: %s\n", info.PID, info.State)

    // Stop it gracefully.
    mgr.Stop(id, 5*time.Second)
}
```

## API

### Manager

The `*sdk.Manager` type wraps the Runix supervisor. Create one with `sdk.New()`:

```go
mgr, err := sdk.New(sdk.Config{
    LogDir:   "/var/lib/myapp/runix",
    Defaults: sdk.DefaultsConfig{
        RestartPolicy: "on-failure",
        MaxRestarts:   5,
    },
})
```

| Method                 | Description                          |
| ---------------------- | ------------------------------------ |
| `New(cfg)`             | Create a new manager                 |
| `AddProcess(ctx, cfg)` | Start a process, returns its ID      |
| `Stop(id, timeout)`    | Graceful stop (SIGTERM → SIGKILL)    |
| `ForceStop(id)`        | Immediate kill (SIGKILL)             |
| `Restart(ctx, id)`     | Stop and restart                     |
| `Reload(ctx, id)`      | Graceful reload (fires reload hooks) |
| `Remove(id)`           | Stop and unregister                  |
| `List()`               | List all processes                   |
| `Inspect(id)`          | Get process details                  |
| `LogPath(id)`          | Get stdout log file path             |
| `LogPathStderr(id)`    | Get stderr log file path             |
| `Save()`               | Persist state to disk                |
| `Resurrect()`          | Restore saved processes              |
| `Close()`              | Stop all and cleanup                 |

### ProcessConfig

```go
sdk.ProcessConfig{
    Name:          "my-worker",       // required
    Script:        "worker.py",       // script path (interpreted)
    // -- or --
    Binary:        "./bin/worker",    // binary path (compiled)

    Runtime:       "python",          // "go", "python", "node", "bun", "deno", "ruby", "php", "auto"
    Interpreter:   "/usr/bin/python3.12",  // explicit interpreter (optional)
    UseBundle:     false,             // wrap with "bundle exec" (Ruby)
    Args:          []string{"--verbose"},
    Cwd:           "/app",
    Env:           map[string]string{"PORT": "3000"},

    RestartPolicy: "on-failure",      // "always", "on-failure", "never"
    MaxRestarts:   10,
    StopSignal:    "SIGTERM",
    StopTimeout:   10 * time.Second,

    Autostart:     true,
    Namespace:     "backend",
    Labels:        map[string]string{"team": "platform"},
    Tags:          []string{"critical"},
    Instances:     3,
    DependsOn:     []string{"database"},
    Priority:      10,

    Watch:         &sdk.WatchConfig{Enabled: true, Paths: []string{"./src"}},
    HealthCheck:   &sdk.HealthCheckConfig{Type: "http", URL: "http://localhost:3000/health"},
    Hooks:         &sdk.HooksConfig{PreStart: &sdk.HookConfig{Command: "echo starting"}},
}
```

### Script vs Binary

Both `Script` and `Binary` map to the same underlying entrypoint. Use whichever reads naturally for your use case:

- **`Script`** — for interpreted files: `main.py`, `server.js`, `app.rb`
- **`Binary`** — for compiled executables or system commands: `./bin/server`, `sleep`, `nginx`

### Runtime Resolution

When you set `Runtime`, the SDK resolves the correct interpreter and arguments automatically:

| Runtime  | Resolution                                                                    |
| -------- | ----------------------------------------------------------------------------- |
| `python` | Resolves venv, prefers `python3`                                              |
| `node`   | `node <script>`, auto-TypeScript via `npx tsx`                                |
| `bun`    | `bun run <script>`                                                            |
| `deno`   | `deno run <script>`                                                           |
| `ruby`   | `ruby <script>`, `bundle exec ruby <script>` with `UseBundle`                 |
| `php`    | `php <script>`                                                                |
| `go`     | `go run <script>` for `.go` files, direct binary otherwise                    |
| `auto`   | Auto-detect from project files (Go → Python → Node → Bun → Deno → Ruby → PHP) |

### Log Access

```go
// Get log file paths.
stdoutPath := mgr.LogPath(processID)
stderrPath := mgr.LogPathStderr(processID)

// Read logs directly from the file.
data, err := os.ReadFile(stdoutPath)
if err != nil {
    log.Fatal(err)
}
fmt.Println(string(data))

// Or tail the file for live output.
cmd := exec.Command("tail", "-f", stdoutPath)
cmd.Stdout = os.Stdout
cmd.Run()
```

### Save and Resurrect

Persist process state across restarts:

```go
// Save current state.
mgr.Save()
mgr.Close()

// Later — restore everything.
mgr2, _ := sdk.New(sdk.Config{LogDir: "/var/lib/myapp/runix"})
mgr2.Resurrect()

for _, p := range mgr2.List() {
    fmt.Printf("Restored: %s (PID %d)\n", p.Name, p.PID)
}
```

## Supported Runtimes

| Runtime | Detection Files                              | Command                                       |
| ------- | -------------------------------------------- | --------------------------------------------- |
| Go      | `go.mod`                                     | `go run .` or binary                          |
| Python  | `requirements.txt`, `pyproject.toml`, `*.py` | `python3 <script>`                            |
| Node.js | `package.json`                               | `node <script>` / `npx tsx <script>`          |
| Bun     | `bun.lockb`, `bunfig.toml`                   | `bun run <script>`                            |
| Deno    | `deno.json`, `deno.jsonc`                    | `deno run <script>`                           |
| Ruby    | `Gemfile`                                    | `ruby <script>` / `bundle exec ruby <script>` |
| PHP     | `composer.json`, `*.php`                     | `php <script>`                                |

## Architecture

The SDK wraps `internal/supervisor` directly — same code path as the CLI. No subprocess spawning, no daemon required.

```
Your Application
    └── sdk.Manager
          └── supervisor.Supervisor (in-process)
                ├── ManagedProcess (per process)
                ├── Monitor goroutine (exit watching)
                └── Backoff (restart timing)
```

## License

Same as Runix.
