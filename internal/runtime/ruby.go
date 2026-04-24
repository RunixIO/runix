package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

// RubyRuntime handles Ruby project detection and process launching.
type RubyRuntime struct{}

func (r *RubyRuntime) Name() string { return "ruby" }

func (r *RubyRuntime) Detect(dir string) bool {
	markers := []string{"Gemfile", "Gemfile.lock"}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
			log.Debug().Str("file", m).Msg("ruby runtime detected via marker file")
			return true
		}
	}
	return false
}

// resolveRubyInterpreter determines the Ruby binary to use.
// It prefers the explicit interpreter override, then resolves via PATH,
// falling back to "ruby".
func resolveRubyInterpreter(interpreter string) string {
	if interpreter != "" {
		return interpreter
	}
	if p, err := exec.LookPath("ruby"); err == nil {
		return p
	}
	return "ruby"
}

func (r *RubyRuntime) StartCmd(opts StartOptions) (*exec.Cmd, error) {
	if opts.Entrypoint == "" {
		return nil, fmt.Errorf("ruby: entrypoint is required")
	}

	interpreter := resolveRubyInterpreter(opts.Interpreter)

	var args []string
	if opts.UseBundle {
		args = append([]string{"exec", interpreter, opts.Entrypoint}, opts.Args...)
	} else {
		args = append([]string{opts.Entrypoint}, opts.Args...)
	}

	bin := interpreter
	if opts.UseBundle {
		bin = "bundle"
	}

	cmd := exec.Command(bin, args...)
	cmd.Dir = opts.Cwd
	cmd.Env = buildEnv(opts.Env)

	log.Debug().
		Str("runtime", "ruby").
		Str("cmd", bin).
		Strs("args", args).
		Str("cwd", opts.Cwd).
		Bool("use_bundle", opts.UseBundle).
		Msg("built start command")

	return cmd, nil
}
