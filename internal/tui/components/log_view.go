package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

// LogView wraps a viewport for displaying scrollable log output.
type LogView struct {
	viewport viewport.Model
	ready    bool
	style    lipgloss.Style
	content  string // full log content (not just visible portion)
}

// NewLogView creates a log viewer with the given dimensions.
func NewLogView(width, height int) LogView {
	vp := viewport.New(width, maxInt(height, 3))
	vp.SetContent("(no process selected)")

	style := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1)

	return LogView{
		viewport: vp,
		ready:    true,
		style:    style,
	}
}

// SetContent updates the log content.
func (lv *LogView) SetContent(content string) {
	lv.content = content
	lv.viewport.SetContent(content)
	lv.viewport.GotoBottom()
}

// AppendContent adds content to the existing log.
func (lv *LogView) AppendContent(content string) {
	lv.content += content
	lv.viewport.SetContent(lv.content)
	lv.viewport.GotoBottom()
}

// SetLogLines sets the log content from a slice of lines.
func (lv *LogView) SetLogLines(lines []string) {
	content := strings.Join(lines, "\n")
	lv.content = content
	lv.viewport.SetContent(content)
	lv.viewport.GotoBottom()
}

// View returns the rendered log view.
func (lv *LogView) View() string {
	return lv.style.Render(lv.viewport.View())
}

// SetSize updates the log view dimensions.
func (lv *LogView) SetSize(width, height int) {
	lv.viewport.Width = maxInt(width-4, 10) // account for border/padding
	lv.viewport.Height = maxInt(height-2, 3)
}

// Viewport returns the underlying viewport model.
func (lv *LogView) Viewport() *viewport.Model {
	return &lv.viewport
}

// ScrollPercent returns the current scroll position as a fraction.
func (lv *LogView) ScrollPercent() float64 {
	return lv.viewport.ScrollPercent()
}
