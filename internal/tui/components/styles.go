package components

import "github.com/charmbracelet/lipgloss"

// --- Color palette (Tokyo Night inspired) ---
var (
	colorPrimary   = lipgloss.Color("12")  // bright cyan
	colorSecondary = lipgloss.Color("8")   // gray
	colorSuccess   = lipgloss.Color("10")  // bright green
	colorWarning   = lipgloss.Color("11")  // bright yellow
	colorError     = lipgloss.Color("9")   // bright red
	colorHighlight = lipgloss.Color("13")  // bright magenta
	colorDim       = lipgloss.Color("241") // dark gray
	colorInfo      = lipgloss.Color("14")  // bright blue
	colorMuted     = lipgloss.Color("245") // medium gray
	colorSurface   = lipgloss.Color("236") // dark surface
)

// --- Title bar ---
var TitleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("15")).
	Background(colorPrimary).
	Padding(0, 2)

// --- Status bar ---
var StatusStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("15")).
	Background(colorSecondary).
	Padding(0, 1)

// --- Process state styles ---
var (
	StateRunning  = lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)
	StateStopped  = lipgloss.NewStyle().Foreground(colorError)
	StateCrashed  = lipgloss.NewStyle().Foreground(colorError).Bold(true)
	StateStarting = lipgloss.NewStyle().Foreground(colorInfo)
	StateStopping = lipgloss.NewStyle().Foreground(colorWarning)
	StateWaiting  = lipgloss.NewStyle().Foreground(colorWarning)
	StateErrored  = lipgloss.NewStyle().Foreground(colorError)
	StateDefault  = lipgloss.NewStyle().Foreground(colorSecondary)
)

// --- Text styles ---
var (
	HelpStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			Padding(0, 2)

	DimStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	MutedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)
)

// --- Table styles ---
var SelectedStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("57")).
	Foreground(lipgloss.Color("229"))

// --- Log area ---
var LogStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(colorSecondary).
	Padding(0, 1)

// --- Inspect TUI styles ---
var (
	InspectHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPrimary).
				BorderBottom(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(colorDim).
				MarginBottom(1)

	InspectLabelStyle = lipgloss.NewStyle().
				Foreground(colorDim).
				Width(14)

	InspectValueStyle = lipgloss.NewStyle().
				Bold(true)

	InspectAccentStyle = lipgloss.NewStyle().
				Foreground(colorHighlight)

	SectionBorderStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(colorDim).
				Padding(0, 1)

	ActionActiveStyle = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true)

	ActionDimStyle = lipgloss.NewStyle().
			Foreground(colorDim)
)

// --- Separator ---
var SeparatorStyle = lipgloss.NewStyle().
	Foreground(colorSurface)

// --- Empty state ---
var EmptyStyle = lipgloss.NewStyle().
	Foreground(colorMuted).
	Italic(true).
	Padding(1, 2)

// --- Badge (inline state label) ---
var (
	BadgeRunning = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true)

	BadgeStopped = lipgloss.NewStyle().
			Foreground(colorError)

	BadgeCrashed = lipgloss.NewStyle().
			Foreground(colorError).
			Bold(true)

	BadgeStarting = lipgloss.NewStyle().
			Foreground(colorInfo)

	BadgeStopping = lipgloss.NewStyle().
			Foreground(colorWarning)

	BadgeWaiting = lipgloss.NewStyle().
			Foreground(colorWarning)

	BadgeErrored = lipgloss.NewStyle().
			Foreground(colorError)
)

// StateStyle returns the lipgloss style for a process state string.
func StateStyle(state string) lipgloss.Style {
	switch state {
	case "running":
		return StateRunning
	case "stopped":
		return StateStopped
	case "crashed":
		return StateCrashed
	case "starting":
		return StateStarting
	case "stopping":
		return StateStopping
	case "waiting":
		return StateWaiting
	case "errored":
		return StateErrored
	default:
		return StateDefault
	}
}

// BadgeStyle returns a compact badge style for a process state.
func BadgeStyle(state string) lipgloss.Style {
	switch state {
	case "running":
		return BadgeRunning
	case "stopped":
		return BadgeStopped
	case "crashed":
		return BadgeCrashed
	case "starting":
		return BadgeStarting
	case "stopping":
		return BadgeStopping
	case "waiting":
		return BadgeWaiting
	case "errored":
		return BadgeErrored
	default:
		return StateDefault
	}
}
