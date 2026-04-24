package output

import (
	"os"
	"sync"
)

// ANSI SGR codes.
const (
	Reset  = "\033[0m"
	Bold   = "\033[1m"
	Dim    = "\033[2m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Cyan   = "\033[36m"
	White  = "\033[37m"
)

var (
	colorMu     sync.Mutex
	colorOn     bool
	colorInited bool
)

// SetColorEnabled enables or disables colored output. When disabled, Sprint
// and related functions return plain strings without ANSI codes.
func SetColorEnabled(enabled bool) {
	colorMu.Lock()
	defer colorMu.Unlock()
	colorOn = enabled
	colorInited = true
}

// ColorEnabled reports whether color output is active.
func ColorEnabled() bool {
	colorMu.Lock()
	defer colorMu.Unlock()
	if !colorInited {
		return false
	}
	return colorOn
}

// InitColor auto-detects color support from environment and the provided
// isTerminal flag. It respects the NO_COLOR environment variable and
// runix-specific --no-color flag.
func InitColor(isTerminal bool, noColorFlag bool) {
	enabled := true
	if noColorFlag {
		enabled = false
	} else if os.Getenv("NO_COLOR") != "" {
		enabled = false
	} else if !isTerminal {
		enabled = false
	}
	SetColorEnabled(enabled)
}

// Sprint wraps text in an ANSI color code. Returns plain text when colors are
// disabled.
func Sprint(code, text string) string {
	if !ColorEnabled() {
		return text
	}
	return code + text + Reset
}

// BoldSprint wraps text in bold.
func BoldSprint(text string) string {
	return Sprint(Bold, text)
}

// HeaderSprint wraps text in bold cyan for table headers.
func HeaderSprint(text string) string {
	if !ColorEnabled() {
		return text
	}
	return Bold + Cyan + text + Reset
}

// StatusSprint returns a colorized status string based on the display state.
func StatusSprint(state string) string {
	if !ColorEnabled() {
		return state
	}
	var code string
	switch state {
	case "online", "launching":
		code = Green
	case "errored":
		code = Red
	case "stopped":
		code = Dim
	case "stopping", "waiting":
		code = Yellow
	default:
		return state
	}
	return code + state + Reset
}

// EnabledSprint returns a colorized yes/no string.
func EnabledSprint(yes bool) string {
	if !ColorEnabled() {
		if yes {
			return "yes"
		}
		return "no"
	}
	if yes {
		return Green + "yes" + Reset
	}
	return Red + "no" + Reset
}
