# Developer Guide

How to contribute to Runix, understand the project structure, and extend the system.

## Project Structure

```
runix/
├── cmd/
│   └── runix/              # CLI commands (cobra)
│       ├── main.go         # Entry point
│       ├── root.go         # Root command, config init
│       ├── start.go        # Individual command files
│       ├── stop.go
│       ├── ...
│       └── version.go
├── configs/
│   └── runix.example.yaml  # Example configuration
├── internal/               # Internal packages (not importable)
│   ├── auth/               # Authentication (basic, token)
│   ├── cgroups/            # Linux cgroup resource limits
│   ├── config/             # Config loading and validation
│   ├── daemon/             # Daemon lifecycle, IPC server/client
│   ├── e2e/                # End-to-end test suite
│   ├── events/             # Event bus
│   ├── healthcheck/        # Health checkers (HTTP, TCP, command)
│   ├── hooks/              # Hook execution engine
│   ├── logrot/             # Log rotation
│   ├── mcp/                # MCP server, tools, resources
│   ├── metrics/            # Resource metrics collection
│   ├── migrate/            # Data migration
│   ├── output/             # Output formatting
│   ├── runtime/            # Runtime adapters (Go, Python, Node, Bun, Deno, Ruby, PHP)
│   ├── scheduler/          # Cron scheduler
│   ├── secrets/            # Secret resolvers (env, file, vault)
│   ├── supervisor/         # Core supervisor engine
│   ├── tui/                # Terminal UI (BubbleTea)
│   ├── updater/            # Self-update mechanism
│   ├── version/            # Version info (set via ldflags)
│   ├── watcher/            # File watching with debounce
│   └── web/                # Web UI + REST API + WebSocket
├── pkg/
│   └── types/              # Public type definitions
│       ├── config.go       # RunixConfig, DaemonConfig, etc.
│       ├── process.go      # ProcessConfig, ProcessInfo, ProcessState
│       ├── hooks.go        # HookConfig, ProcessHooks
│       ├── watch.go        # WatchConfig
│       └── cron.go         # CronJobConfig
├── scripts/
│   ├── install.sh          # One-line installer
│   └── build.sh            # Build script with version embedding
├── docs/                   # Documentation
├── go.mod
├── go.sum
├── Makefile
├── .goreleaser.yaml        # Release automation
└── README.md
```

## Getting Started

### Prerequisites

- Go 1.25.7 or later
- Make (optional, for build commands)

### Build

```bash
# Build with version info
make build

# Or build directly
./scripts/build.sh

# The binary is output to bin/runix
```

### Test

```bash
# Run all tests
make test

# Run a specific package's tests
go test ./internal/supervisor/...

# Run with verbose output
go test -v ./...

# Run e2e tests
go test ./internal/e2e/...
```

### Lint

```bash
# go vet
make vet

# golangci-lint (requires installation)
make lint
```

### Clean

```bash
make clean
```

## Adding a Runtime Adapter

Runtime adapters detect and start applications written in a specific language. Each adapter implements the `Runtime` interface from `internal/runtime/runtime.go`.

Detection order: Go → Python → Node → Bun → Deno → Ruby → PHP.

### The Runtime Interface

```go
type Runtime interface {
    Name() string
    Detect(dir string) bool
    StartCmd(opts StartOptions) (*exec.Cmd, error)
}
```

| Method           | Purpose                                                    |
| ---------------- | ---------------------------------------------------------- |
| `Name()`         | Returns the adapter name (e.g., `"go"`, `"python"`)        |
| `Detect(dir)`    | Returns `true` if the adapter should handle this directory |
| `StartCmd(opts)` | Builds the `exec.Cmd` to start the process                 |

### Steps to Add a New Adapter

1. **Create the adapter file** in `internal/runtime/` (e.g., `rust.go`)

2. **Implement the `Runtime` interface:**

```go
package runtime

import (
    "os/exec"
    "path/filepath"
)

type RustRuntime struct{}

func (r *RustRuntime) Name() string {
    return "rust"
}

func (r *RustRuntime) Detect(dir string) bool {
    // Check for Cargo.toml
    if _, err := filepath.Abs(filepath.Join(dir, "Cargo.toml")); err == nil {
        return true
    }
    return false
}

func (r *RustRuntime) StartCmd(opts StartOptions) (*exec.Cmd, error) {
    // Build the command
    cmd := exec.Command("cargo", "run", "--")
    cmd.Args = append(cmd.Args, opts.Args...)
    cmd.Dir = opts.Cwd
    cmd.Env = buildEnv(opts.Env)
    return cmd, nil
}
```

3. **Register the adapter** in `internal/runtime/detector.go`:

Add the new adapter to the `NewDetector` function or the detection order.

4. **Update the CLI** to accept the new runtime name in the `--runtime` flag validation (in `cmd/runix/start.go`).

5. **Update the config validation** in `internal/config/config.go` if runtime name validation exists.

6. **Write tests** in `internal/runtime/detector_test.go` for detection and command building.

7. **Update documentation** — CLI reference, configuration, and the example config.

## Adding a New CLI Command

Commands are implemented as cobra command files in `cmd/runix/`.

### Template

```go
package runix

import (
    "fmt"
    "github.com/spf13/cobra"
)

func newMyCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "mycommand <target>",
        Short: "Short description",
        Long:  "Longer description of what the command does.",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            target := args[0]

            // Try daemon IPC first
            client, err := ensureDaemon()
            if err == nil {
                // Send request to daemon
                resp, err := client.Do("my_action", payload)
                // handle response
            }

            // Fallback to direct mode
            sup := getSupervisor()
            // execute against supervisor directly

            fmt.Println("Done")
            return nil
        },
    }

    // Add flags
    cmd.Flags().StringP("format", "f", "text", "Output format")
    return cmd
}
```

### Register the Command

In `cmd/runix/root.go`, add the command to the root:

```go
rootCmd.AddCommand(newMyCommand())
```

### Add Daemon Support

If the command should work over IPC:

1. Add the action constant in `internal/daemon/protocol.go`
2. Add a payload type if needed
3. Add a handler in `internal/daemon/server.go`
4. Add a client method in `internal/daemon/client.go`

## Adding MCP Tools

MCP tools are defined in `internal/mcp/tools.go` and implemented in `internal/mcp/handlers.go`.

### Steps

1. **Define the tool** in `tools.go` using the `mcp-go` library:

```go
{
    Name:        "my_tool",
    Description: "What this tool does",
    InputSchema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "target": map[string]interface{}{
                "type":        "string",
                "description": "Process ID or name",
            },
        },
        "required": []string{"target"},
    },
}
```

2. **Implement the handler** in `handlers.go`:

```go
func (s *MCPServer) handleMyTool(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
    target := args["target"].(string)
    // implement logic
    return &mcp.CallToolResult{
        Content: []interface{}{
            mcp.TextContent{Type: "text", Text: resultText},
        },
    }, nil
}
```

3. **Register the handler** in the tool definition map in `server.go`.

4. **Update the MCP documentation.**

## Coding Conventions

- Follow standard Go conventions and `gofmt` formatting
- Use `zerolog` for structured logging
- Return errors rather than logging and returning nil
- Write table-driven tests where appropriate
- Keep commands thin — business logic belongs in `internal/` packages
- Use the existing patterns for IPC: daemon client first, direct supervisor fallback

## Key Dependencies

| Dependency                | Purpose                          |
| ------------------------- | -------------------------------- |
| `spf13/cobra`             | CLI framework                    |
| `spf13/viper`             | Configuration loading            |
| `charmbracelet/bubbletea` | Terminal UI framework            |
| `charmbracelet/bubbles`   | TUI components (table, viewport) |
| `charmbracelet/lipgloss`  | TUI styling                      |
| `go-chi/chi`              | HTTP router (Web UI, API)        |
| `gorilla/websocket`       | WebSocket support                |
| `mark3labs/mcp-go`        | MCP server implementation        |
| `robfig/cron`             | Cron scheduling                  |
| `fsnotify/fsnotify`       | File system watching             |
| `rs/zerolog`              | Structured logging               |

## Release Process

Releases are automated via GitHub Actions and GoReleaser:

1. Tag a release: `git tag v0.x.0 && git push --tags`
2. The release workflow builds for all platforms (Linux, macOS, Windows; amd64, arm64)
3. Artifacts include: binaries, DEB/RPM/APK packages, Homebrew formula, checksums
4. Changelog is generated from conventional commits

### Commit Convention

```
feat: add new feature
fix: resolve bug
docs: update documentation
refactor: restructure code
test: add tests
chore: maintenance tasks
perf: performance improvement
```

## Testing

### Unit Tests

```bash
# All tests
go test ./...

# Specific package
go test ./internal/supervisor/...

# With coverage
go test -cover ./...

# Verbose
go test -v ./internal/runtime/...
```

### E2E Tests

End-to-end tests in `internal/e2e/` cover:

- Supervisor lifecycle (start, stop, list)
- Restart policy with backoff exhaustion
- Save and resurrect persistence
- Daemon IPC round-trip
- Process log capture
- Watch mode restart
- Runtime detection
- Web API integration

```bash
go test ./internal/e2e/... -v -timeout 120s
```

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/my-feature`
3. Make changes with tests
4. Run the full test suite: `make test`
5. Run linting: `make vet && make lint`
6. Commit with conventional commit messages
7. Open a pull request against `main`
