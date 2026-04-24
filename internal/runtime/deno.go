package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

// DenoRuntime handles Deno project detection and process launching.
type DenoRuntime struct{}

func (d *DenoRuntime) Name() string { return "deno" }

func (d *DenoRuntime) Detect(dir string) bool {
	markers := []string{"deno.json", "deno.jsonc"}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
			log.Debug().Str("file", m).Msg("deno runtime detected via marker file")
			return true
		}
	}
	return false
}

func (d *DenoRuntime) StartCmd(opts StartOptions) (*exec.Cmd, error) {
	if opts.Entrypoint == "" {
		return nil, fmt.Errorf("deno: entrypoint is required")
	}

	bin := "deno"
	if opts.Interpreter != "" {
		bin = opts.Interpreter
	}

	// Build: deno run [opts.Args — permissions, flags...] <entrypoint>
	args := append([]string{"run"}, opts.Args...)
	args = append(args, opts.Entrypoint)

	cmd := exec.Command(bin, args...)
	cmd.Dir = opts.Cwd
	cmd.Env = buildEnv(opts.Env)

	log.Debug().
		Str("runtime", "deno").
		Str("cmd", bin).
		Strs("args", args).
		Str("cwd", opts.Cwd).
		Msg("built start command")

	return cmd, nil
}
