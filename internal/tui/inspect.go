package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/runixio/runix/internal/daemon"
	"github.com/runixio/runix/internal/output"
	"github.com/runixio/runix/internal/tui/components"
	"github.com/runixio/runix/pkg/types"
)

// ProcessInspector extends ProcessLister with action capabilities for the inspect TUI.
type ProcessInspector interface {
	ProcessLister
	StopProcess(target string, force bool) error
	StartProcess(target string) error
	RestartProcess(target string) error
	ReloadProcess(target string) error
	RemoveProcess(target string) error
	FlushLogs(target string) error
	ReadLogs(target string, lines int) (string, error)
}

// --- DirectInspector ---

// DirectInspector implements ProcessInspector using a direct supervisor reference.
type DirectInspector struct {
	*DirectLister
	sendFunc func(action string, payload interface{}) (daemon.Response, error)
	dataDir  string
}

// NewDirectInspector creates an inspector backed by direct IPC calls.
func NewDirectInspector(dataDir string) *DirectInspector {
	return &DirectInspector{
		DirectLister: &DirectLister{},
		dataDir:      dataDir,
	}
}

// SetSendFunc sets the IPC send function for daemon communication.
func (d *DirectInspector) SetSendFunc(fn func(string, interface{}) (daemon.Response, error)) {
	d.sendFunc = fn
}

func (d *DirectInspector) sendIPC(action string, payload interface{}) (daemon.Response, error) {
	if d.sendFunc != nil {
		return d.sendFunc(action, payload)
	}
	return daemon.Response{}, fmt.Errorf("no IPC connection")
}

func (d *DirectInspector) StopProcess(target string, force bool) error {
	resp, err := d.sendIPC(daemon.ActionStop, daemon.StopPayload{Target: target, Force: force})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (d *DirectInspector) StartProcess(target string) error {
	resp, err := d.sendIPC(daemon.ActionStart, daemon.StartPayload{Name: target})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (d *DirectInspector) RestartProcess(target string) error {
	resp, err := d.sendIPC(daemon.ActionRestart, daemon.StopPayload{Target: target})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (d *DirectInspector) ReloadProcess(target string) error {
	resp, err := d.sendIPC(daemon.ActionReload, daemon.StopPayload{Target: target})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (d *DirectInspector) RemoveProcess(target string) error {
	resp, err := d.sendIPC(daemon.ActionDelete, daemon.StopPayload{Target: target})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (d *DirectInspector) FlushLogs(target string) error {
	appDir := filepath.Join(d.dataDir, "apps", target)
	return flushAppLogs(appDir)
}

func (d *DirectInspector) ReadLogs(target string, lines int) (string, error) {
	resp, err := d.sendIPC(daemon.ActionLogs, daemon.LogsPayload{Target: target, Lines: lines})
	if err != nil {
		return "", err
	}
	if !resp.Success {
		return "", fmt.Errorf("%s", resp.Error)
	}
	var result struct {
		Logs string `json:"logs"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return "", err
	}
	return result.Logs, nil
}

// --- DaemonInspector ---

// DaemonInspector implements ProcessInspector using daemon IPC exclusively.
type DaemonInspector struct {
	client *daemon.Client
}

// NewDaemonInspector creates an inspector backed by a daemon client.
func NewDaemonInspector(client *daemon.Client) *DaemonInspector {
	return &DaemonInspector{client: client}
}

func (d *DaemonInspector) send(action string, payload interface{}) (daemon.Response, error) {
	var rawPayload json.RawMessage
	if payload != nil {
		var err error
		rawPayload, err = json.Marshal(payload)
		if err != nil {
			return daemon.Response{}, err
		}
	}
	return d.client.Send(context.Background(), daemon.Request{Action: action, Payload: rawPayload})
}

func (d *DaemonInspector) ListProcesses() ([]types.ProcessInfo, error) {
	resp, err := d.send(daemon.ActionList, nil)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	var procs []types.ProcessInfo
	if err := json.Unmarshal(resp.Data, &procs); err != nil {
		return nil, err
	}
	return procs, nil
}

func (d *DaemonInspector) GetProcess(idOrName string) (types.ProcessInfo, error) {
	resp, err := d.send(daemon.ActionStatus, daemon.StopPayload{Target: idOrName})
	if err != nil {
		return types.ProcessInfo{}, err
	}
	if !resp.Success {
		return types.ProcessInfo{}, fmt.Errorf("%s", resp.Error)
	}
	var info types.ProcessInfo
	if err := json.Unmarshal(resp.Data, &info); err != nil {
		return types.ProcessInfo{}, err
	}
	return info, nil
}

func (d *DaemonInspector) StopProcess(target string, force bool) error {
	resp, err := d.send(daemon.ActionStop, daemon.StopPayload{Target: target, Force: force})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (d *DaemonInspector) StartProcess(target string) error {
	resp, err := d.send(daemon.ActionStart, daemon.StartPayload{Name: target})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (d *DaemonInspector) RestartProcess(target string) error {
	resp, err := d.send(daemon.ActionRestart, daemon.StopPayload{Target: target})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (d *DaemonInspector) ReloadProcess(target string) error {
	resp, err := d.send(daemon.ActionReload, daemon.StopPayload{Target: target})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (d *DaemonInspector) RemoveProcess(target string) error {
	resp, err := d.send(daemon.ActionDelete, daemon.StopPayload{Target: target})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (d *DaemonInspector) FlushLogs(target string) error {
	// The daemon does not have a dedicated flush action.
	// Flush is a local file operation — truncate the log files directly.
	// Since we may not know the data dir from the daemon client, we try IPC status
	// to get the process name, then use a conventional path.
	// As a fallback, we simply return nil (logs are not critical to flush).
	return fmt.Errorf("flush not available in daemon-only mode")
}

func (d *DaemonInspector) ReadLogs(target string, lines int) (string, error) {
	resp, err := d.send(daemon.ActionLogs, daemon.LogsPayload{Target: target, Lines: lines})
	if err != nil {
		return "", err
	}
	if !resp.Success {
		return "", fmt.Errorf("%s", resp.Error)
	}
	var result struct {
		Logs string `json:"logs"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return "", err
	}
	return result.Logs, nil
}

// --- Inspect TUI Model ---

// Messages for the inspect TUI.
type (
	inspectProcessMsg types.ProcessInfo
	inspectActionMsg  struct {
		action string
		err    error
	}
	inspectLogsMsg string
	inspectTickMsg time.Time
)

// InspectModel is a BubbleTea model for an interactive process inspector.
type InspectModel struct {
	inspector ProcessInspector
	target    string
	width     int
	height    int
	ready     bool

	info   types.ProcessInfo
	logs   string
	status string
	err    error

	logViewport viewport.Model
	confirm     components.ConfirmDialog
	confirmMode string // which action is pending confirmation

	refreshInterval time.Duration
	logRefreshEvery time.Duration
	lastLogLoad     time.Time
	lastLogState    inspectLogState
}

// NewInspectModel creates a new interactive process inspector TUI.
func NewInspectModel(inspector ProcessInspector, target string) *InspectModel {
	return &InspectModel{
		inspector:       inspector,
		target:          target,
		logViewport:     viewport.New(80, 10),
		refreshInterval: 2 * time.Second,
		logRefreshEvery: 5 * time.Second,
	}
}

// Init implements tea.Model.
func (m *InspectModel) Init() tea.Cmd {
	return tea.Batch(
		m.loadProcessInfo,
		m.tickRefresh(),
	)
}

// Update implements tea.Model.
func (m *InspectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()
		if !m.ready {
			m.ready = true
		}
		return m, nil

	case tea.KeyMsg:
		// If confirm dialog is visible, route keys there.
		if m.confirm.Visible() {
			return m.handleConfirmKey(msg)
		}
		return m.handleKey(msg)

	case inspectProcessMsg:
		prevInfo := m.info
		m.info = types.ProcessInfo(msg)
		m.err = nil
		if m.status != "" && !strings.Contains(m.status, "error") {
			m.status = ""
		}
		if m.shouldReloadLogs(prevInfo, m.info) {
			return m, m.loadLogs
		}
		return m, nil

	case inspectActionMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("error: %v", msg.err)
		} else {
			m.status = fmt.Sprintf("%s: done", msg.action)
		}
		if msg.action == "delete" && msg.err == nil {
			return m, tea.Quit
		}
		return m, m.loadProcessInfo

	case inspectLogsMsg:
		m.logs = string(msg)
		if m.ready {
			m.logViewport.SetContent(m.logs)
			m.logViewport.GotoBottom()
		}
		return m, nil

	case inspectTickMsg:
		cmds := []tea.Cmd{m.loadProcessInfo, m.tickRefresh()}
		if m.shouldPeriodicLogRefresh() {
			cmds = append(cmds, m.loadLogs)
		}
		return m, tea.Batch(cmds...)
	}

	return m, nil
}

// View implements tea.Model.
func (m *InspectModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	title := components.TitleStyle.Render(fmt.Sprintf(" Runix Inspect: %s ", m.info.Name))

	infoSection := m.renderInfoSection()
	logsSection := m.renderLogsSection()
	actionsSection := m.renderActionsBar()

	// Status/error bar.
	statusBar := ""
	if m.status != "" {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Padding(0, 1)
		if strings.Contains(m.status, "error") {
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Padding(0, 1)
		}
		statusBar = style.Render(">> " + m.status)
	}

	parts := []string{title, infoSection, logsSection, actionsSection}
	if statusBar != "" {
		parts = append(parts, statusBar)
	}

	content := strings.Join(parts, "\n")

	// Confirmation dialog overlay.
	if m.confirm.Visible() {
		overlay := m.confirm.View(m.width)
		return content + "\n" + overlay
	}

	return content
}

func (m *InspectModel) renderInfoSection() string {
	// Left column: process details.
	leftKV := output.NewKeyValue()
	leftKV.Add("Name", m.info.Name)
	if m.info.Namespace != "" {
		leftKV.Add("Namespace", m.info.Namespace)
	}
	leftKV.Add("ID", fmt.Sprintf("%d", m.info.NumericID))

	pid := "-"
	if m.info.State == types.StateRunning || m.info.State == types.StateStarting {
		pid = fmt.Sprintf("%d", m.info.PID)
	}
	leftKV.Add("PID", pid)

	leftKV.Add("Status", m.renderStateLabel(m.info.State))
	leftKV.Add("Ready", output.EnabledSprint(m.info.Ready))
	leftKV.Add("Uptime", m.info.UptimeString())
	leftKV.Add("Restarts", fmt.Sprintf("%d", m.info.Restarts))
	leftKV.Add("Runtime", m.info.Runtime)
	leftKV.Add("Restart", string(m.info.Config.RestartPolicy))
	leftKV.Add("Auto Restart", output.EnabledSprint(m.info.Config.AutoRestart == nil || *m.info.Config.AutoRestart))
	if m.info.Config.RestartDelay > 0 {
		leftKV.Add("Restart Delay", m.info.Config.RestartDelay.String())
	}
	if m.info.Config.MinUptime > 0 {
		leftKV.Add("Min Uptime", m.info.Config.MinUptime.String())
	}
	if m.info.Config.MaxMemoryRestart != "" {
		leftKV.Add("Max Mem Restart", m.info.Config.MaxMemoryRestart)
	}
	if m.info.LastEvent != "" {
		leftKV.Add("Last Event", m.info.LastEvent)
	}
	if m.info.LastReason != "" {
		leftKV.Add("Last Reason", m.info.LastReason)
	}

	leftContent := components.InspectHeaderStyle.Render(" Process Details ") + "\n" +
		lipgloss.NewStyle().Padding(0, 1).Render(leftKV.Render())

	// Right column: metrics and command.
	rightKV := output.NewKeyValue()
	rightKV.Add("CPU", output.FormatCPU(m.info.CPUPercent))
	rightKV.Add("Memory", output.FormatBytes(m.info.MemBytes))
	if m.info.MemPercent > 0 {
		rightKV.Add("Mem %", fmt.Sprintf("%.1f%%", m.info.MemPercent))
	}
	rightKV.Add("Threads", fmt.Sprintf("%d", m.info.Threads))
	rightKV.Add("FDs", fmt.Sprintf("%d", m.info.FDs))

	cmd := m.info.Config.Entrypoint
	if len(m.info.Config.Args) > 0 {
		cmd += " " + strings.Join(m.info.Config.Args, " ")
	}
	rightKV.Add("Command", cmd)
	if m.info.Config.Cwd != "" {
		rightKV.Add("Work Dir", m.info.Config.Cwd)
	}

	rightContent := components.InspectHeaderStyle.Render(" Metrics ") + "\n" +
		lipgloss.NewStyle().Padding(0, 1).Render(rightKV.Render())

	// Side by side layout.
	var leftBox, rightBox string
	halfWidth := m.width / 2
	leftBox = lipgloss.NewStyle().
		Width(halfWidth).
		MaxWidth(halfWidth).
		Render(leftContent)
	rightBox = lipgloss.NewStyle().
		Width(m.width - halfWidth).
		MaxWidth(m.width - halfWidth).
		Render(rightContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)
}

func (m *InspectModel) renderStateLabel(state types.ProcessState) string {
	style := components.StateStyle(string(state))
	return style.Render(string(state))
}

func (m *InspectModel) renderLogsSection() string {
	header := components.InspectHeaderStyle.Render(" Live Logs ")
	scrollInfo := ""
	if m.ready {
		scrollInfo = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render(fmt.Sprintf(" %.0f%%", m.logViewport.ScrollPercent()*100))
	}

	// Pad header to fill width.
	headerLine := header
	if m.width > 0 {
		renderedWidth := lipgloss.Width(headerLine)
		scrollWidth := lipgloss.Width(scrollInfo)
		padding := m.width - renderedWidth - scrollWidth
		if padding > 0 {
			headerLine += strings.Repeat(" ", padding)
		}
	}
	headerLine += scrollInfo

	return headerLine + "\n" + m.logViewport.View()
}

func (m *InspectModel) renderActionsBar() string {
	isRunning := m.info.State == types.StateRunning
	isStopped := m.info.State == types.StateStopped
	isCrashed := m.info.State == types.StateCrashed

	actions := []struct {
		key   string
		label string
		ok    bool
	}{
		{"s", "stop", isRunning},
		{"S", "force stop", isRunning},
		{"t", "start", isStopped},
		{"r", "restart", isRunning || isCrashed},
		{"l", "reload", isRunning},
		{"d", "delete", true},
		{"f", "flush", true},
		{"q", "quit", true},
	}

	var parts []string
	for _, a := range actions {
		keyStyle := lipgloss.NewStyle().Bold(true)
		if a.ok {
			keyStyle = keyStyle.Foreground(lipgloss.Color("12"))
		} else {
			keyStyle = keyStyle.Foreground(lipgloss.Color("241"))
		}
		labelStyle := lipgloss.NewStyle()
		if a.ok {
			labelStyle = labelStyle.Foreground(lipgloss.Color("15"))
		} else {
			labelStyle = labelStyle.Foreground(lipgloss.Color("241"))
		}
		parts = append(parts, keyStyle.Render(a.key)+":"+labelStyle.Render(a.label))
	}

	bar := " " + strings.Join(parts, " │ ") + " "

	return lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("15")).
		Width(m.width).
		Render(bar)
}

// --- Key handling ---

func (m *InspectModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	isRunning := m.info.State == types.StateRunning
	isStopped := m.info.State == types.StateStopped
	isCrashed := m.info.State == types.StateCrashed

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "s":
		if isRunning {
			m.confirm = components.NewConfirmDialog(
				fmt.Sprintf("Stop process %q?", m.info.Name), false)
			m.confirmMode = "stop"
			return m, nil
		}

	case "S":
		if isRunning {
			m.confirm = components.NewConfirmDialog(
				fmt.Sprintf("Force stop process %q? (SIGKILL)", m.info.Name), true)
			m.confirmMode = "force_stop"
			return m, nil
		}

	case "t":
		if isStopped {
			m.status = "starting..."
			return m, m.doAction("start", m.info.Name)
		}

	case "r":
		if isRunning || isStopped || isCrashed {
			m.confirm = components.NewConfirmDialog(
				fmt.Sprintf("Restart process %q?", m.info.Name), false)
			m.confirmMode = "restart"
			return m, nil
		}

	case "l":
		if isRunning {
			m.status = "reloading..."
			return m, m.doAction("reload", m.info.Name)
		}

	case "d":
		m.confirm = components.NewConfirmDialog(
			fmt.Sprintf("Delete process %q? This cannot be undone.", m.info.Name), true)
		m.confirmMode = "delete"
		return m, nil

	case "f":
		m.status = "flushing logs..."
		target := m.info.Name
		return m, func() tea.Msg {
			err := m.inspector.FlushLogs(target)
			return inspectActionMsg{action: "flush", err: err}
		}
	}

	// Forward to log viewport for scrolling.
	var cmd tea.Cmd
	m.logViewport, cmd = m.logViewport.Update(msg)
	return m, cmd
}

func (m *InspectModel) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	resolved, confirmed := m.confirm.HandleKey(msg.String())
	if !resolved {
		return m, nil
	}
	if !confirmed {
		m.confirmMode = ""
		m.status = ""
		return m, nil
	}

	action := m.confirmMode
	m.confirmMode = ""

	switch action {
	case "stop":
		m.status = "stopping..."
		return m, m.doAction("stop", m.info.Name)
	case "force_stop":
		m.status = "force stopping..."
		target := m.info.Name
		return m, func() tea.Msg {
			err := m.inspector.StopProcess(target, true)
			return inspectActionMsg{action: "force stop", err: err}
		}
	case "restart":
		m.status = "restarting..."
		return m, m.doAction("restart", m.info.Name)
	case "delete":
		m.status = "deleting..."
		return m, m.doAction("delete", m.info.Name)
	}

	return m, nil
}

// --- Actions ---

func (m *InspectModel) doAction(action, target string) tea.Cmd {
	return func() tea.Msg {
		var err error
		switch action {
		case "stop":
			err = m.inspector.StopProcess(target, false)
		case "start":
			err = m.inspector.StartProcess(target)
		case "restart":
			err = m.inspector.RestartProcess(target)
		case "reload":
			err = m.inspector.ReloadProcess(target)
		case "delete":
			err = m.inspector.RemoveProcess(target)
		}
		return inspectActionMsg{action: action, err: err}
	}
}

// --- Data loading ---

func (m *InspectModel) loadProcessInfo() tea.Msg {
	info, err := m.inspector.GetProcess(m.target)
	if err != nil {
		return inspectActionMsg{action: "refresh", err: err}
	}
	return inspectProcessMsg(info)
}

func (m *InspectModel) loadLogs() tea.Msg {
	logs, err := m.inspector.ReadLogs(m.info.Name, 100)
	m.lastLogLoad = time.Now()
	m.lastLogState = inspectLogStateFromInfo(m.info)
	if err != nil {
		return inspectLogsMsg(fmt.Sprintf("(error reading logs: %v)", err))
	}
	if logs == "" {
		return inspectLogsMsg("(no logs available)")
	}
	return inspectLogsMsg(logs)
}

func (m *InspectModel) tickRefresh() tea.Cmd {
	return tea.Tick(m.refreshInterval, func(t time.Time) tea.Msg {
		return inspectTickMsg(t)
	})
}

type inspectLogState struct {
	state      types.ProcessState
	pid        int
	exitCode   int
	restarts   int
	startedAt  int64
	finishedAt int64
}

func inspectLogStateFromInfo(info types.ProcessInfo) inspectLogState {
	state := inspectLogState{
		state:    info.State,
		pid:      info.PID,
		exitCode: info.ExitCode,
		restarts: info.Restarts,
	}
	if info.StartedAt != nil {
		state.startedAt = info.StartedAt.UnixNano()
	}
	if info.FinishedAt != nil {
		state.finishedAt = info.FinishedAt.UnixNano()
	}
	return state
}

func (m *InspectModel) shouldReloadLogs(prev, next types.ProcessInfo) bool {
	if m.lastLogLoad.IsZero() {
		return true
	}
	return inspectLogStateFromInfo(prev) != inspectLogStateFromInfo(next)
}

func (m *InspectModel) shouldPeriodicLogRefresh() bool {
	if m.info.Name == "" || m.lastLogLoad.IsZero() {
		return true
	}
	if m.info.State != types.StateRunning && m.info.State != types.StateStarting && m.info.State != types.StateStopping {
		return false
	}
	return time.Since(m.lastLogLoad) >= m.logRefreshEvery
}

func (m *InspectModel) updateLayout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	// Reserve: 1 (title) + info section (~10) + 1 (log header) + 1 (actions) + 1 (status) = ~14 fixed
	logHeight := m.height - 14
	if logHeight < 3 {
		logHeight = 3
	}

	m.logViewport.Width = m.width - 2
	m.logViewport.Height = logHeight
}

// --- Helpers ---

// flushAppLogs truncates stdout.log and stderr.log in the given app directory.
func flushAppLogs(appDir string) error {
	for _, logFile := range []string{"stdout.log", "stderr.log"} {
		path := filepath.Join(appDir, logFile)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return fmt.Errorf("failed to truncate %s: %w", logFile, err)
		}
		_ = f.Close()
	}
	return nil
}
