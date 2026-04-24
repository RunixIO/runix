package hooks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/runixio/runix/pkg/types"
)

const (
	defaultTimeout        = 30 * time.Second
	maxCapturedHookOutput = 64 * 1024 // 64 KiB tail per stream
)

// Executor runs lifecycle hook commands safely.
type Executor struct{}

// NewExecutor creates a new hook executor.
func NewExecutor() *Executor {
	return &Executor{}
}

// Run executes a lifecycle hook command.
//
// The command is run via `sh -c <command>` in the process's working directory
// with the process's environment. stdout and stderr are captured and logged.
//
// If the hook's IgnoreFailure is true, errors are logged but not returned.
// The context is used for cancellation and augmented with the hook's timeout.
func (e *Executor) Run(ctx context.Context, hook *types.HookConfig, event string, cfg types.ProcessConfig) error {
	if hook == nil || hook.Command == "" {
		return nil
	}

	timeout := hook.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	logger := log.With().
		Str("hook", event).
		Str("process", cfg.Name).
		Str("command", hook.Command).
		Dur("timeout", timeout).
		Logger()

	logger.Info().Msg("executing lifecycle hook")
	start := time.Now()

	// Intentional: hooks are user-defined shell commands from runix.yaml.
	// sanitizeCommand strips null bytes; shell features (pipes, &&, etc.) are expected.
	cmd := exec.CommandContext(ctx, "sh", "-c", sanitizeCommand(hook.Command)) //codeql[go/command-injection]
	setProcessGroup(cmd)
	cmd.Cancel = func() error {
		return killProcessGroup(cmd.Process)
	}
	cmd.WaitDelay = time.Second
	cmd.Dir = cfg.Cwd
	if cmd.Dir == "" {
		cmd.Dir, _ = os.Getwd()
	}
	cmd.Env = buildHookEnv(cfg.Env)

	stdout := newBoundedTailBuffer(maxCapturedHookOutput)
	stderr := newBoundedTailBuffer(maxCapturedHookOutput)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start)

	outStr := strings.TrimSpace(stdout.String())
	errStr := strings.TrimSpace(stderr.String())

	hookLog := logger.With().
		Dur("duration", duration).
		Bool("success", err == nil).
		Logger()

	if outStr != "" {
		hookLog = hookLog.With().Str("stdout", outStr).Logger()
	}
	if errStr != "" {
		hookLog = hookLog.With().Str("stderr", errStr).Logger()
	}

	if err != nil {
		hookLog.Error().Err(err).Msg("hook failed")
		fmt.Fprintf(os.Stderr, "[Runix] hook %s failed for %q: %v\n", event, cfg.Name, err)
		if outStr != "" {
			fmt.Fprintf(os.Stderr, "  stdout: %s\n", outStr)
		}
		if errStr != "" {
			fmt.Fprintf(os.Stderr, "  stderr: %s\n", errStr)
		}

		if hook.IgnoreFailure {
			hookLog.Warn().Msg("hook failure ignored")
			return nil
		}
		return fmt.Errorf("hook %s failed for process %q: %w", event, cfg.Name, err)
	}

	hookLog.Info().Msg("hook completed")
	return nil
}

type boundedTailBuffer struct {
	max   int
	buf   []byte
	total int
}

func newBoundedTailBuffer(max int) boundedTailBuffer {
	return boundedTailBuffer{max: max}
}

func (b *boundedTailBuffer) Write(p []byte) (int, error) {
	b.total += len(p)
	if b.max <= 0 {
		return len(p), nil
	}
	if len(p) >= b.max {
		b.buf = append(b.buf[:0], p[len(p)-b.max:]...)
		return len(p), nil
	}

	needed := len(b.buf) + len(p) - b.max
	if needed > 0 {
		copy(b.buf, b.buf[needed:])
		b.buf = b.buf[:len(b.buf)-needed]
	}
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *boundedTailBuffer) String() string {
	return string(b.buf)
}

// RunPre is a convenience for running pre-* hooks. These block the lifecycle
// action on failure (unless IgnoreFailure is set).
func (e *Executor) RunPre(ctx context.Context, hook *types.HookConfig, event string, cfg types.ProcessConfig) error {
	return e.Run(ctx, hook, event, cfg)
}

// RunPost is a convenience for running post-* hooks. These never block —
// errors are always logged but suppressed.
func (e *Executor) RunPost(ctx context.Context, hook *types.HookConfig, event string, cfg types.ProcessConfig) {
	if hook == nil || hook.Command == "" {
		return
	}
	if err := e.Run(ctx, hook, event, cfg); err != nil {
		log.Warn().
			Err(err).
			Str("hook", event).
			Str("process", cfg.Name).
			Msg("post-hook failed (non-blocking)")
	}
}

// buildHookEnv constructs the environment for a hook process.
func buildHookEnv(overlay map[string]string) []string {
	env := os.Environ()

	// Add runix-specific env vars.
	env = append(env,
		"RUNIX=true",
		"RUNIX_HOOK=true",
	)

	if len(overlay) == 0 {
		return env
	}

	prefixes := make(map[string]string, len(overlay))
	for k, v := range overlay {
		prefixes[k+"="] = k + "=" + v
	}

	result := make([]string, 0, len(env)+len(overlay))
	for _, e := range env {
		replaced := false
		for prefix := range prefixes {
			if strings.HasPrefix(e, prefix) {
				result = append(result, prefixes[prefix])
				delete(prefixes, prefix)
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, e)
		}
	}
	for _, v := range prefixes {
		result = append(result, v)
	}
	return result
}

// HookEnabled checks if a specific hook is configured and has a non-empty command.
func HookEnabled(hooks *types.ProcessHooks, event string) bool {
	if hooks == nil {
		return false
	}
	switch event {
	case "pre_start":
		return hooks.PreStart != nil && hooks.PreStart.Command != ""
	case "post_start":
		return hooks.PostStart != nil && hooks.PostStart.Command != ""
	case "pre_stop":
		return hooks.PreStop != nil && hooks.PreStop.Command != ""
	case "post_stop":
		return hooks.PostStop != nil && hooks.PostStop.Command != ""
	case "pre_restart":
		return hooks.PreRestart != nil && hooks.PreRestart.Command != ""
	case "post_restart":
		return hooks.PostRestart != nil && hooks.PostRestart.Command != ""
	case "pre_reload":
		return hooks.PreReload != nil && hooks.PreReload.Command != ""
	case "post_reload":
		return hooks.PostReload != nil && hooks.PostReload.Command != ""
	case "pre_healthcheck":
		return hooks.PreHealthCheck != nil && hooks.PreHealthCheck.Command != ""
	case "post_healthcheck":
		return hooks.PostHealthCheck != nil && hooks.PostHealthCheck.Command != ""
	}
	return false
}

// Silence unused import warning.
var _ = zerolog.DebugLevel

// sanitizeCommand strips null bytes from a hook command sourced from config.
func sanitizeCommand(cmd string) string {
	for i := range len(cmd) {
		if cmd[i] == 0 {
			return cmd[:i]
		}
	}
	return cmd
}
