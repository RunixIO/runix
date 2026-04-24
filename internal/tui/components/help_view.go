package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// HelpView renders a help overlay with keybindings.
type HelpView struct {
	width   int
	visible bool
}

// NewHelpView creates a new help view.
func NewHelpView(width int) HelpView {
	return HelpView{width: width}
}

// Toggle toggles help visibility.
func (h *HelpView) Toggle() {
	h.visible = !h.visible
}

// Visible returns whether help is shown.
func (h *HelpView) Visible() bool {
	return h.visible
}

// View renders the help overlay.
func (h *HelpView) View() string {
	if !h.visible {
		return ""
	}

	sections := []struct {
		title string
		keys  []struct{ key, desc string }
	}{
		{
			title: "Navigation",
			keys: []struct{ key, desc string }{
				{"↑ / k", "Move cursor up"},
				{"↓ / j", "Move cursor down"},
				{"Enter", "View process details"},
			},
		},
		{
			title: "Process Actions",
			keys: []struct{ key, desc string }{
				{"l", "View process logs"},
				{"r", "Restart selected process"},
				{"s", "Stop selected process"},
				{"d", "Delete selected process"},
			},
		},
		{
			title: "General",
			keys: []struct{ key, desc string }{
				{"Ctrl+R", "Refresh process list"},
				{"?", "Toggle help overlay"},
				{"q / Ctrl+C", "Quit (or go back)"},
				{"Esc", "Go back to process list"},
			},
		},
	}

	var b strings.Builder
	b.WriteString(TitleStyle.Render(" Runix Keybindings "))
	b.WriteString("\n\n")

	maxKey := 0
	for _, sec := range sections {
		for _, kb := range sec.keys {
			if len(kb.key) > maxKey {
				maxKey = len(kb.key)
			}
		}
	}

	for i, sec := range sections {
		sectionTitle := lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Render(sec.title)
		b.WriteString("  " + sectionTitle + "\n")

		for _, kb := range sec.keys {
			keyStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Bold(true).
				Width(maxKey + 2)
			descStyle := lipgloss.NewStyle().
				Foreground(colorMuted)
			fmt.Fprintf(&b, "    %s %s\n", keyStyle.Render(kb.key), descStyle.Render(kb.desc))
		}

		if i < len(sections)-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(DimStyle.Render("  Press ? or Esc to close"))

	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(1, 2).
		Render(b.String())

	return lipgloss.NewStyle().
		Width(h.width).
		Render(box)
}

// SetSize updates the help view width.
func (h *HelpView) SetSize(width int) {
	h.width = width
}
