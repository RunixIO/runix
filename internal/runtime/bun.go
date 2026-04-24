package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

// BunRuntime handles Bun project detection and process launching.
type BunRuntime struct{}

func (b *BunRuntime) Name() string { return "bun" }

func (b *BunRuntime) Detect(dir string) bool {
	markers := []string{"bun.lockb", "bunfig.toml", "bun.lock"}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
			log.Debug().Str("file", m).Msg("bun runtime detected via marker file")
			return true
		}
	}
	return false
}

func (b *BunRuntime) StartCmd(opts StartOptions) (*exec.Cmd, error) {
	if opts.Entrypoint == "" {
		return nil, fmt.Errorf("bun: entrypoint is required")
	}

	bin := "bun"
	if opts.Interpreter != "" {
		bin = opts.Interpreter
	}

	args := append([]string{"run", opts.Entrypoint}, opts.Args...)
	cmd := exec.Command(bin, args...)
	cmd.Dir = opts.Cwd
	cmd.Env = buildEnv(opts.Env)

	log.Debug().
		Str("runtime", "bun").
		Str("cmd", bin).
		Strs("args", args).
		Str("cwd", opts.Cwd).
		Msg("built start command")

	return cmd, nil
}
