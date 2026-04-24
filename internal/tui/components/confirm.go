package components

import (
	"github.com/charmbracelet/lipgloss"
)

// ConfirmDialog is a modal dialog for confirming destructive actions.
type ConfirmDialog struct {
	message     string
	destructive bool
	visible     bool
	confirmed   bool
}

// NewConfirmDialog creates a new confirmation dialog.
func NewConfirmDialog(message string, destructive bool) ConfirmDialog {
	return ConfirmDialog{
		message:     message,
		destructive: destructive,
		visible:     true,
	}
}

// HandleKey processes a key press and returns whether the dialog was resolved.
// Returns (resolved bool, confirmed bool).
func (d *ConfirmDialog) HandleKey(key string) (bool, bool) {
	if !d.visible {
		return false, false
	}

	switch key {
	case "y", "Y":
		d.visible = false
		d.confirmed = true
		return true, true
	case "n", "N", "esc":
		d.visible = false
		d.confirmed = false
		return true, false
	default:
		return false, false
	}
}

// View renders the confirmation dialog centered on screen.
func (d ConfirmDialog) View(width int) string {
	if !d.visible {
		return ""
	}

	var borderColor lipgloss.Color
	var icon string
	var msgRendered string

	if d.destructive {
		borderColor = colorError
		icon = lipgloss.NewStyle().Foreground(colorError).Bold(true).Render("⚠")
		msgRendered = lipgloss.NewStyle().Bold(true).Foreground(colorError).Render(d.message)
	} else {
		borderColor = colorPrimary
		icon = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Render("?")
		msgRendered = lipgloss.NewStyle().Bold(true).Render(d.message)
	}

	// Action buttons styled as key hints.
	confirmStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		Background(colorSuccess).
		Padding(0, 1)
	cancelStyle := lipgloss.NewStyle().
		Foreground(colorMuted).
		Padding(0, 1)

	buttons := confirmStyle.Render("y") + " confirm  " +
		cancelStyle.Render("n") + " cancel"

	content := icon + "  " + msgRendered + "\n\n  " + buttons

	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 3).
		Render(content)

	return lipgloss.Place(width, 7, lipgloss.Center, lipgloss.Center, box)
}

// Visible returns whether the dialog is currently shown.
func (d ConfirmDialog) Visible() bool {
	return d.visible
}
