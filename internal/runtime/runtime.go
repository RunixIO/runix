package runtime

import (
	"os"
	"os/exec"
	"strings"
)

// StartOptions holds options for creating a start command.
type StartOptions struct {
	Entrypoint  string
	Args        []string
	Cwd         string
	Env         map[string]string
	Interpreter string // optional override
	UseBundle   bool   // wrap with bundle exec
}

// Runtime abstracts language-specific process launching.
type Runtime interface {
	// Name returns the runtime identifier (e.g. "go", "python", "node", "bun").
	Name() string

	// Detect returns true if the project at dir uses this runtime.
	Detect(dir string) bool

	// StartCmd builds an *exec.Cmd ready to start the process described by opts.
	StartCmd(opts StartOptions) (*exec.Cmd, error)
}

// buildEnv merges the current process environment with the provided overlay.
// Keys in overlay replace existing values; new keys are appended.
func buildEnv(overlay map[string]string) []string {
	if len(overlay) == 0 {
		return os.Environ()
	}

	base := os.Environ()
	seen := make(map[string]bool, len(overlay))

	out := make([]string, 0, len(base)+len(overlay))
	for _, e := range base {
		k, _, ok := strings.Cut(e, "=")
		if ok && overlay[k] != "" {
			out = append(out, k+"="+overlay[k])
			seen[k] = true
			continue
		}
		out = append(out, e)
	}

	for k, v := range overlay {
		if !seen[k] {
			out = append(out, k+"="+v)
		}
	}

	return out
}
