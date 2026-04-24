package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// StatusBar renders a bottom status bar with process count and key hints.
type StatusBar struct {
	width        int
	processCount int
	running      int
	crashed      int
	stopped      int
	selected     string
}

// NewStatusBar creates a new status bar.
func NewStatusBar(width int) StatusBar {
	return StatusBar{width: width}
}

// Update updates the status bar data.
func (sb *StatusBar) Update(processCount, running, crashed int, selected string) {
	sb.processCount = processCount
	sb.running = running
	sb.crashed = crashed
	sb.stopped = processCount - running - crashed
	sb.selected = selected
}

// View renders the status bar.
func (sb *StatusBar) View() string {
	// Build left section with counts.
	var leftParts []string
	leftParts = append(leftParts, fmt.Sprintf("Total: %d", sb.processCount))

	runLabel := lipgloss.NewStyle().Foreground(colorSuccess).Bold(true).Render(fmt.Sprintf("● %d running", sb.running))
	leftParts = append(leftParts, runLabel)

	if sb.crashed > 0 {
		crashLabel := lipgloss.NewStyle().Foreground(colorError).Bold(true).Render(fmt.Sprintf("✖ %d crashed", sb.crashed))
		leftParts = append(leftParts, crashLabel)
	}

	if sb.stopped > 0 {
		stopLabel := lipgloss.NewStyle().Foreground(colorMuted).Render(fmt.Sprintf("○ %d stopped", sb.stopped))
		leftParts = append(leftParts, stopLabel)
	}

	if sb.selected != "" {
		selLabel := lipgloss.NewStyle().Foreground(colorHighlight).Render(fmt.Sprintf("▸ %s", sb.selected))
		leftParts = append(leftParts, selLabel)
	}

	left := " " + strings.Join(leftParts, "  ") + " "

	// Build right section with key hints.
	right := " q:quit  ↑↓:nav  l:logs  r:restart  s:stop  ?:help "

	// Calculate padding.
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	padding := sb.width - leftWidth - rightWidth
	if padding < 1 {
		padding = 1
	}

	leftRendered := StatusStyle.Render(left)
	rightRendered := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Background(colorSurface).
		Padding(0, 1).
		Render(right)

	return leftRendered + strings.Repeat(" ", padding) + rightRendered
}

// SetSize updates the status bar width.
func (sb *StatusBar) SetSize(width int) {
	sb.width = width
}
