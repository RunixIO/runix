# cmd/runix/ — CLI Layer

28 cobra command files. Entry point: `main.go` → `root.go`.

## Registration Pattern

All commands registered in `root.go` via `root.AddCommand(newXxxCmd())`. Each command lives in its own file named after the command.

To add a new command:
1. Create `<command>.go` with `func new<Command>Cmd() *cobra.Command`
2. Add `root.AddCommand(new<Command>Cmd())` in `root.go` init block
3. Follow dual-mode: try `sendIPC()` first, fallback to `getSupervisor()`

## Dual-Mode Execution

Every command that interacts with processes follows this pattern:
```
ensureDaemon() → sendIPC(action, payload)    // daemon mode
                        ↓ failure
              getSupervisor() → direct ops     // in-process fallback
```

- `ensureDaemon()` (root.go): starts daemon if not running, polls `IsAlive()` up to 5s
- `sendIPC()` (root.go): marshals `daemon.Request`, POSTs to `http://unix/api/{action}`
- `getSupervisor()` (start.go): creates an in-process `supervisor.New()` with config

## Key Files

| File | Purpose |
|------|---------|
| `root.go` | Root command, viper init, persistent flags (`--config`, `--verbose`, `--data-dir`) |
| `start.go` | Most complex command: runtime detection, multi-instance (`--instances N`), env parsing |
| `logs.go` | 365 lines — tail/follow with polling, supports multiple targets |
| `doctor.go` | Diagnostic checks: binary, config, daemon, socket, data dir |
| `startup.go` | Platform-specific service install (systemd unit / launchd plist) |
| `helpers.go` | `displayState()` maps internal states to user-facing strings |

## Conventions

- Business logic belongs in `internal/`. CLI files are thin wrappers.
- `SilenceUsage: true` and `SilenceErrors: true` on root — errors printed explicitly.
- Zerolog logging initialized in `PersistentPreRunE`.
- `--namespace` and `--label` flags for process filtering where applicable.
