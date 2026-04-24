package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

// GoRuntime handles Go project detection and process launching.
type GoRuntime struct{}

func (g *GoRuntime) Name() string { return "go" }

func (g *GoRuntime) Detect(dir string) bool {
	mod := filepath.Join(dir, "go.mod")
	if _, err := os.Stat(mod); err == nil {
		log.Debug().Str("file", mod).Msg("go runtime detected via go.mod")
		return true
	}
	return false
}

func (g *GoRuntime) StartCmd(opts StartOptions) (*exec.Cmd, error) {
	goBin := "go"
	if opts.Interpreter != "" {
		goBin = opts.Interpreter
	}

	entrypoint := opts.Entrypoint
	if entrypoint == "" {
		return nil, fmt.Errorf("go: entrypoint is required")
	}

	var args []string
	var cmdPath string

	switch {
	case entrypoint == ".":
		args = []string{"run", "."}
		cmdPath = goBin
	case strings.HasSuffix(entrypoint, ".go"):
		args = []string{"run", entrypoint}
		cmdPath = goBin
	default:
		// Treat entrypoint as a binary path.
		cmdPath = entrypoint
	}

	args = append(args, opts.Args...)

	cmd := exec.Command(cmdPath, args...)
	cmd.Dir = opts.Cwd
	cmd.Env = buildEnv(opts.Env)

	log.Debug().
		Str("runtime", "go").
		Str("cmd", cmdPath).
		Strs("args", args).
		Str("cwd", opts.Cwd).
		Msg("built start command")

	return cmd, nil
}
