package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/runixio/runix/internal/tui/components"
	"github.com/runixio/runix/pkg/types"
)

// ViewMode represents the current TUI view.
type ViewMode int

const (
	ViewTable ViewMode = iota
	ViewLogs
	ViewHelp
)

// ProcessLister is the interface for fetching process info (supervisor or daemon client).
type ProcessLister interface {
	ListProcesses() ([]types.ProcessInfo, error)
	GetProcess(idOrName string) (types.ProcessInfo, error)
}

// App is the root Bubbletea model for the Runix TUI.
type App struct {
	mode     ViewMode
	width    int
	height   int
	ready    bool
	quitting bool

	procTable   components.ProcessTable
	logView     components.LogView
	statusBar   components.StatusBar
	helpView    components.HelpView
	logViewport viewport.Model

	// Data.
	processes []types.ProcessInfo
	selected  int
	lister    ProcessLister

	// Last refresh.
	lastRefresh time.Time
}

// Key bindings.
var keys = struct {
	up      key.Binding
	down    key.Binding
	enter   key.Binding
	logs    key.Binding
	restart key.Binding
	stop    key.Binding
	delete  key.Binding
	help    key.Binding
	quit    key.Binding
	refresh key.Binding
	esc     key.Binding
}{
	up:      key.NewBinding(key.WithKeys("up", "k")),
	down:    key.NewBinding(key.WithKeys("down", "j")),
	enter:   key.NewBinding(key.WithKeys("enter")),
	logs:    key.NewBinding(key.WithKeys("l")),
	restart: key.NewBinding(key.WithKeys("r")),
	stop:    key.NewBinding(key.WithKeys("s")),
	delete:  key.NewBinding(key.WithKeys("d")),
	help:    key.NewBinding(key.WithKeys("?")),
	quit:    key.NewBinding(key.WithKeys("q", "ctrl+c")),
	refresh: key.NewBinding(key.WithKeys("ctrl+r")),
	esc:     key.NewBinding(key.WithKeys("esc")),
}

// NewApp creates a new TUI application.
func NewApp(lister ProcessLister) *App {
	return &App{
		lister: lister,
	}
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return refreshMsg(t)
	})
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.setLayout()
		if !a.ready {
			a.ready = true
		}
		return a, nil

	case tea.KeyMsg:
		return a.handleKey(msg)

	case refreshMsg:
		a.refreshProcesses()
		a.lastRefresh = time.Now()
		return a, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return refreshMsg(t)
		})

	case viewLogsMsg:
		a.mode = ViewLogs
		a.loadLogs(msg.id)
		return a, nil
	}

	// Forward to focused component.
	var cmd tea.Cmd
	switch a.mode {
	case ViewLogs:
		a.logViewport, cmd = a.logViewport.Update(msg)
	case ViewTable:
		if a.ready {
			tbl := a.procTable.Table()
			*tbl, cmd = tbl.Update(msg)
		}
	}
	return a, cmd
}

// View implements tea.Model.
func (a *App) View() string {
	if !a.ready {
		return lipgloss.NewStyle().
			Foreground(components.DimStyle.GetForeground()).
			Render("  Initializing Runix...")
	}
	if a.quitting {
		return "\n  Goodbye!\n\n"
	}

	// Title bar with version-like subtitle.
	title := components.TitleStyle.Render(" Runix ")

	// Main content area.
	var content string
	switch a.mode {
	case ViewLogs:
		content = a.renderLogView()
	case ViewHelp:
		content = a.helpView.View()
	default:
		content = a.renderTableView()
	}

	// Layout: title + content + status bar.
	return title + "\n" + content + "\n" + a.statusBar.View()
}

func (a *App) renderTableView() string {
	if len(a.processes) == 0 {
		return components.EmptyStyle.Render("  No processes configured. Add processes to your runix.yaml to get started.")
	}
	return a.procTable.View()
}

func (a *App) renderLogView() string {
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Background(lipgloss.Color("236")).
		Padding(0, 1).
		Render(" Logs ")
	escHint := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render(" (Esc to go back)")

	headerWidth := lipgloss.Width(header) + lipgloss.Width(escHint)
	padding := ""
	if a.width > headerWidth {
		padding = strings.Repeat(" ", a.width-headerWidth)
	}

	headerLine := header + padding + escHint
	content := a.logViewport.View()
	return headerLine + "\n" + components.LogStyle.Render(content)
}

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys.
	switch {
	case key.Matches(msg, keys.quit):
		if a.mode == ViewLogs || a.mode == ViewHelp {
			a.mode = ViewTable
			return a, nil
		}
		a.quitting = true
		return a, tea.Quit

	case key.Matches(msg, keys.esc):
		a.mode = ViewTable
		return a, nil

	case key.Matches(msg, keys.help):
		a.mode = ViewHelp
		a.helpView.Toggle()
		if !a.helpView.Visible() {
			a.mode = ViewTable
		}
		return a, nil

	case key.Matches(msg, keys.refresh):
		a.refreshProcesses()
		return a, nil
	}

	// Mode-specific keys.
	if a.mode == ViewTable {
		return a.handleTableKey(msg)
	}
	return a, nil
}

func (a *App) handleTableKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	tbl := a.procTable.Table()
	*tbl, cmd = tbl.Update(msg)

	switch {
	case key.Matches(msg, keys.enter):
		// Enter could trigger inspect view in future.

	case key.Matches(msg, keys.logs):
		row := a.procTable.SelectedRow()
		if len(row) > 0 {
			return a, func() tea.Msg {
				return viewLogsMsg{id: row[0]}
			}
		}

	case key.Matches(msg, keys.up):
		if a.selected > 0 {
			a.selected--
		}

	case key.Matches(msg, keys.down):
		if a.selected < len(a.processes)-1 {
			a.selected++
		}
	}

	return a, cmd
}

func (a *App) setLayout() {
	headerHeight := 1
	footerHeight := 1
	contentHeight := a.height - headerHeight - footerHeight

	if contentHeight < 5 {
		contentHeight = 5
	}

	a.procTable = components.NewProcessTable(a.width, contentHeight)
	a.logViewport = viewport.New(a.width-4, contentHeight-3)
	a.statusBar = components.NewStatusBar(a.width)
	a.helpView = components.NewHelpView(a.width)
}

func (a *App) refreshProcesses() {
	if a.lister == nil {
		return
	}

	procs, err := a.lister.ListProcesses()
	if err != nil {
		return
	}

	if !sameProcessList(a.processes, procs) {
		a.processes = procs
		a.procTable.UpdateProcesses(procs)
	} else {
		a.processes = procs
	}

	// Clamp selected index to valid range after refresh.
	if len(procs) > 0 && a.selected >= len(procs) {
		a.selected = len(procs) - 1
	}

	// Update status bar counts.
	running, crashed := 0, 0
	for _, p := range procs {
		switch p.State {
		case types.StateRunning:
			running++
		case types.StateCrashed:
			crashed++
		}
	}

	selected := ""
	if a.selected < len(procs) {
		selected = procs[a.selected].Name
	}
	a.statusBar.Update(len(procs), running, crashed, selected)
}

func sameProcessList(a, b []types.ProcessInfo) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if !sameProcessSummary(a[i], b[i]) {
			return false
		}
	}

	return true
}

func sameProcessSummary(a, b types.ProcessInfo) bool {
	return a.ID == b.ID &&
		a.NumericID == b.NumericID &&
		a.Name == b.Name &&
		a.Runtime == b.Runtime &&
		a.State == b.State &&
		a.Ready == b.Ready &&
		a.PID == b.PID &&
		a.ExitCode == b.ExitCode &&
		a.Restarts == b.Restarts &&
		a.CPUPercent == b.CPUPercent &&
		a.MemBytes == b.MemBytes &&
		a.MemPercent == b.MemPercent &&
		a.Threads == b.Threads &&
		a.FDs == b.FDs &&
		a.Uptime == b.Uptime &&
		timePointersEqual(a.StartedAt, b.StartedAt) &&
		timePointersEqual(a.FinishedAt, b.FinishedAt)
}

func timePointersEqual(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

func (a *App) loadLogs(id string) {
	if a.lister == nil {
		a.logViewport.SetContent("(no process lister available)")
		return
	}

	// Find the process name for display.
	for _, p := range a.processes {
		if strings.HasPrefix(p.ID, id) {
			a.logViewport.SetContent(fmt.Sprintf("Logs for %s (id: %s)\n\n(log streaming not available in direct mode)", p.Name, p.ID[:8]))
			return
		}
	}
	a.logViewport.SetContent(fmt.Sprintf("Loading logs for process %s...", id))
}

// Messages.

type refreshMsg time.Time

type viewLogsMsg struct {
	id string
}

// DirectLister is a ProcessLister backed by a process list.
type DirectLister struct {
	Procs []types.ProcessInfo
}

// ListProcesses returns the stored process list.
func (d *DirectLister) ListProcesses() ([]types.ProcessInfo, error) {
	return d.Procs, nil
}

// GetProcess finds a process by ID or name prefix.
func (d *DirectLister) GetProcess(idOrName string) (types.ProcessInfo, error) {
	for _, p := range d.Procs {
		if strings.HasPrefix(p.ID, idOrName) || p.Name == idOrName {
			return p, nil
		}
	}
	return types.ProcessInfo{}, fmt.Errorf("process %q not found", idOrName)
}

// Ensure context is available.
var _ = context.Background
