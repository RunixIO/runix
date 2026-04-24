package components

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	"github.com/runixio/runix/internal/output"
	"github.com/runixio/runix/pkg/types"
)

// ProcessTable wraps a bubbles table for displaying process info.
type ProcessTable struct {
	table table.Model
}

// NewProcessTable creates a process table with standard columns.
func NewProcessTable(width int, height int) ProcessTable {
	columns := []table.Column{
		{Title: "ID", Width: 10},
		{Title: "Name", Width: 20},
		{Title: "Runtime", Width: 9},
		{Title: "State", Width: 11},
		{Title: "Ready", Width: 7},
		{Title: "PID", Width: 7},
		{Title: "CPU%", Width: 8},
		{Title: "Mem", Width: 9},
		{Title: "Rstrt", Width: 6},
		{Title: "Uptime", Width: 10},
	}

	adjustColumnWidths(columns, width)

	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(maxInt(height, 3)),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		Bold(true).
		Foreground(colorPrimary)
	s.Selected = s.Selected.
		Foreground(SelectedStyle.GetForeground()).
		Background(SelectedStyle.GetBackground())
	t.SetStyles(s)

	return ProcessTable{table: t}
}

// stateSymbol returns a visual indicator for a process state.
func stateSymbol(state string) string {
	switch state {
	case "running":
		return "●"
	case "stopped":
		return "○"
	case "crashed":
		return "✖"
	case "starting":
		return "◌"
	case "stopping":
		return "◌"
	case "waiting":
		return "◌"
	case "errored":
		return "✖"
	default:
		return "·"
	}
}

// UpdateProcesses refreshes the table rows from process info.
func (pt *ProcessTable) UpdateProcesses(procs []types.ProcessInfo) {
	rows := make([]table.Row, 0, len(procs))
	for _, p := range procs {
		id := p.ID
		if len(id) > 8 {
			id = id[:8]
		}
		mem := output.FormatBytes(p.MemBytes)
		uptime := p.UptimeString()

		// State with color and symbol.
		symbol := stateSymbol(string(p.State))
		stateStr := BadgeStyle(string(p.State)).Render(symbol + " " + string(p.State))

		rows = append(rows, table.Row{
			id,
			p.Name,
			p.Runtime,
			stateStr,
			output.EnabledSprint(p.Ready),
			fmt.Sprintf("%d", p.PID),
			fmt.Sprintf("%.1f", p.CPUPercent),
			mem,
			fmt.Sprintf("%d", p.Restarts),
			uptime,
		})
	}
	pt.table.SetRows(rows)
}

// Table returns the underlying bubbles table model.
func (pt *ProcessTable) Table() *table.Model {
	return &pt.table
}

// SelectedRow returns the currently selected row data.
func (pt *ProcessTable) SelectedRow() table.Row {
	return pt.table.SelectedRow()
}

// View renders the table.
func (pt *ProcessTable) View() string {
	return pt.table.View()
}

// SetSize updates the table dimensions.
func (pt *ProcessTable) SetSize(width, height int) {
	pt.table.SetWidth(width)
	pt.table.SetHeight(maxInt(height, 3))
	adjustColumnWidths(pt.table.Columns(), width)
}

func adjustColumnWidths(cols []table.Column, totalWidth int) {
	total := 0
	for _, c := range cols {
		total += c.Width
	}
	if total == 0 || totalWidth <= 0 {
		return
	}
	remaining := totalWidth - 4
	scale := float64(remaining) / float64(total)
	for i := range cols {
		cols[i].Width = maxInt(int(float64(cols[i].Width)*scale), 4)
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
