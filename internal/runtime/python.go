package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

// PythonRuntime handles Python project detection and process launching.
type PythonRuntime struct{}

func (p *PythonRuntime) Name() string { return "python" }

func (p *PythonRuntime) Detect(dir string) bool {
	// Check for standard Python project files.
	markers := []string{
		"requirements.txt",
		"pyproject.toml",
		"setup.py",
		"Pipfile",
	}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
			log.Debug().Str("file", m).Msg("python runtime detected via marker file")
			return true
		}
	}

	// Check for any .py files in the directory.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".py") {
			log.Debug().Str("file", e.Name()).Msg("python runtime detected via .py file")
			return true
		}
	}

	return false
}

// resolvePythonInterpreter determines the Python binary to use.
// It prefers a virtual environment if one exists, then falls back to the
// interpreter override, then tries python3 and python.
func resolvePythonInterpreter(dir string, interpreter string) string {
	if interpreter != "" {
		return interpreter
	}

	// Prefer venv.
	for _, venv := range []string{"venv/bin/python", ".venv/bin/python"} {
		p := filepath.Join(dir, venv)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			log.Debug().Str("path", p).Msg("using venv python")
			return p
		}
	}

	// Try python3, then python.
	for _, candidate := range []string{"python3", "python"} {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate
		}
	}

	return "python3"
}

func (p *PythonRuntime) StartCmd(opts StartOptions) (*exec.Cmd, error) {
	if opts.Entrypoint == "" {
		return nil, fmt.Errorf("python: entrypoint is required")
	}

	interpreter := resolvePythonInterpreter(opts.Cwd, opts.Interpreter)

	args := append([]string{opts.Entrypoint}, opts.Args...)
	cmd := exec.Command(interpreter, args...)
	cmd.Dir = opts.Cwd
	cmd.Env = buildEnv(opts.Env)

	log.Debug().
		Str("runtime", "python").
		Str("interpreter", interpreter).
		Str("entrypoint", opts.Entrypoint).
		Strs("args", opts.Args).
		Str("cwd", opts.Cwd).
		Msg("built start command")

	return cmd, nil
}
