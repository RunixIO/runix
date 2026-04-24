package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

// NodeRuntime handles Node.js project detection and process launching.
type NodeRuntime struct{}

func (n *NodeRuntime) Name() string { return "node" }

func (n *NodeRuntime) Detect(dir string) bool {
	pj := filepath.Join(dir, "package.json")
	if _, err := os.Stat(pj); err == nil {
		log.Debug().Str("file", pj).Msg("node runtime detected via package.json")
		return true
	}
	return false
}

func (n *NodeRuntime) StartCmd(opts StartOptions) (*exec.Cmd, error) {
	if opts.Entrypoint == "" {
		return nil, fmt.Errorf("node: entrypoint is required")
	}

	entrypoint := opts.Entrypoint
	isTS := strings.HasSuffix(entrypoint, ".ts") || strings.HasSuffix(entrypoint, ".tsx")

	var cmdPath string
	var args []string

	switch {
	case opts.Interpreter != "":
		// Explicit interpreter override.
		cmdPath = opts.Interpreter
		args = append([]string{entrypoint}, opts.Args...)

	case isTS:
		// TypeScript file without explicit interpreter: use npx tsx.
		cmdPath = "npx"
		args = append([]string{"tsx", entrypoint}, opts.Args...)

	default:
		// Standard JavaScript file or other.
		cmdPath = "node"
		args = append([]string{entrypoint}, opts.Args...)
	}

	cmd := exec.Command(cmdPath, args...)
	cmd.Dir = opts.Cwd
	cmd.Env = buildEnv(opts.Env)

	log.Debug().
		Str("runtime", "node").
		Str("cmd", cmdPath).
		Strs("args", args).
		Str("cwd", opts.Cwd).
		Bool("typescript", isTS).
		Msg("built start command")

	return cmd, nil
}
