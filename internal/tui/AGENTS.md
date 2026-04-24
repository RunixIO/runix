# internal/tui/ — Terminal UI

BubbleTea-based terminal interface with modular components.

## Files

| File | Purpose |
|------|---------|
| `app.go` | Main `tea.Model`: view modes, key handling, process list polling |
| `components/process_table.go` | Table rendering with process state, PID, uptime, restarts |
| `components/log_view.go` | Scrollable log viewer for selected process |
| `components/status_bar.go` | Bottom status bar with key hints |
| `components/help_view.go` | Full-screen help overlay |
| `components/styles.go` | Lipgloss style definitions |

## View Modes

```go
const (
    ViewTable  // default — process list
    ViewLogs   // log viewer for selected process
    ViewHelp   // help overlay
)
```

## Decoupling

Uses `ProcessLister` interface (not direct supervisor dependency):
```go
type ProcessLister interface {
    ListProcesses() ([]types.ProcessInfo, error)
    GetProcess(idOrName string) (types.ProcessInfo, error)
}
```

## Key Bindings

- `↑/↓` or `j/k`: navigate process list
- `Enter`: view logs for selected process
- `s`: start, `S`: stop, `r`: restart selected process
- `?`: help, `q`: quit (or back from logs/help)
