package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

// PHPRuntime handles PHP project detection and process launching.
type PHPRuntime struct{}

func (p *PHPRuntime) Name() string { return "php" }

func (p *PHPRuntime) Detect(dir string) bool {
	// Check for standard PHP project markers.
	markers := []string{"composer.json", "artisan"}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
			log.Debug().Str("file", m).Msg("php runtime detected via marker file")
			return true
		}
	}

	// Check for any .php files in the directory.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".php") {
			log.Debug().Str("file", e.Name()).Msg("php runtime detected via .php file")
			return true
		}
	}

	return false
}

// resolvePHPInterpreter determines the PHP binary to use.
// It prefers the explicit interpreter override, then resolves via PATH,
// falling back to "php".
func resolvePHPInterpreter(interpreter string) string {
	if interpreter != "" {
		return interpreter
	}
	if p, err := exec.LookPath("php"); err == nil {
		return p
	}
	return "php"
}

func (p *PHPRuntime) StartCmd(opts StartOptions) (*exec.Cmd, error) {
	if opts.Entrypoint == "" {
		return nil, fmt.Errorf("php: entrypoint is required")
	}

	interpreter := resolvePHPInterpreter(opts.Interpreter)

	args := append([]string{opts.Entrypoint}, opts.Args...)
	cmd := exec.Command(interpreter, args...)
	cmd.Dir = opts.Cwd
	cmd.Env = buildEnv(opts.Env)

	log.Debug().
		Str("runtime", "php").
		Str("cmd", interpreter).
		Strs("args", args).
		Str("cwd", opts.Cwd).
		Msg("built start command")

	return cmd, nil
}
