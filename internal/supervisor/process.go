package supervisor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/runixio/runix/internal/healthcheck"
	"github.com/runixio/runix/internal/hooks"
	"github.com/runixio/runix/pkg/types"
)

// ManagedProcess wraps an OS process with lifecycle management, state tracking,
// restart accounting, and log capture.
type ManagedProcess struct {
	ID        string
	NumericID int
	Config    types.ProcessConfig
	PID       int
	ExitCode  int
	Restarts  int
	CreatedAt time.Time

	mu               sync.RWMutex
	state            atomic.Value // stores types.ProcessState
	ready            atomic.Bool
	cmd              *exec.Cmd
	cancel           context.CancelFunc
	startedAt        time.Time
	finishedAt       time.Time
	backoff          Backoff
	restartTimes     []time.Time
	stdoutWriter     io.Writer
	stderrWriter     io.Writer
	stdoutFile       *os.File // stdout log file, closed on shutdown
	stderrFile       *os.File // stderr log file, closed on shutdown
	logFile          *os.File // kept for backward compat, set to stdoutFile
	stopSignal       syscall.Signal
	stopTimeout      time.Duration
	autoRestart      bool
	maxRestarts      int
	restartWindow    time.Duration
	restartDelay     time.Duration
	minUptime        time.Duration
	maxMemoryRestart int64
	restartStreak    int
	hookExec         *hooks.Executor
	metricsCol       MetricsCollector // set by Supervisor to enrich Info()
	lastEvent        string
	lastReason       string

	// exited is closed once by handleExit to signal that the process has terminated.
	exited    chan struct{}
	closeOnce *sync.Once // guards close(p.exited) against double-close
}

// NewManagedProcess creates a new ManagedProcess from the given config.
func NewManagedProcess(cfg types.ProcessConfig) *ManagedProcess {
	p := &ManagedProcess{
		ID:        uuid.New().String(),
		Config:    cfg,
		CreatedAt: time.Now(),
		backoff:   NewBackoff(),
		exited:    make(chan struct{}),
		closeOnce: &sync.Once{},
		hookExec:  hooks.NewExecutor(),
	}
	p.state.Store(types.StateStopped)
	return p
}

// SetLogWriters sets the stdout and stderr writers for process output capture.
func (p *ManagedProcess) SetLogWriters(stdout, stderr io.Writer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stdoutWriter = stdout
	p.stderrWriter = stderr
}

// SetLogFiles stores both stdout and stderr log file handles.
func (p *ManagedProcess) SetLogFiles(stdout, stderr *os.File) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stdoutFile = stdout
	p.stderrFile = stderr
	p.logFile = stdout // backward compat
}

// ApplyDefaults merges defaults config values into the process where not already set.
func (p *ManagedProcess) ApplyDefaults(defaults types.DefaultsConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Config.RestartPolicy == "" {
		p.Config.RestartPolicy = defaults.RestartPolicy
	}
	if p.Config.RestartPolicy == "" {
		p.Config.RestartPolicy = types.RestartAlways
	}
	p.autoRestart = true
	if defaults.AutoRestart != nil {
		p.autoRestart = *defaults.AutoRestart
	}
	if p.Config.AutoRestart != nil {
		p.autoRestart = *p.Config.AutoRestart
	}
	if p.maxRestarts == 0 && defaults.MaxRestarts > 0 {
		p.maxRestarts = defaults.MaxRestarts
	}
	if p.maxRestarts == 0 && p.Config.MaxRestarts > 0 {
		p.maxRestarts = p.Config.MaxRestarts
	}
	if p.restartWindow == 0 && defaults.RestartWindow > 0 {
		p.restartWindow = defaults.RestartWindow
	}
	if p.restartWindow == 0 && p.Config.RestartWindow > 0 {
		p.restartWindow = p.Config.RestartWindow
	}
	if p.Config.MaxMemoryRestart == "" && defaults.MaxMemoryRestart != "" {
		p.Config.MaxMemoryRestart = defaults.MaxMemoryRestart
	}
	if p.Config.MaxMemoryRestart != "" {
		if limit, err := types.ParseMemorySize(p.Config.MaxMemoryRestart); err == nil {
			p.maxMemoryRestart = limit
		}
	}
	if p.restartDelay == 0 && defaults.RestartDelay > 0 {
		p.restartDelay = defaults.RestartDelay
	}
	if p.restartDelay == 0 && p.Config.RestartDelay > 0 {
		p.restartDelay = p.Config.RestartDelay
	}
	if p.minUptime == 0 && defaults.MinUptime > 0 {
		p.minUptime = defaults.MinUptime
	}
	if p.minUptime == 0 && p.Config.MinUptime > 0 {
		p.minUptime = p.Config.MinUptime
	}
	if p.stopTimeout == 0 && p.Config.StopTimeout > 0 {
		p.stopTimeout = p.Config.StopTimeout
	}
	if p.stopTimeout == 0 {
		p.stopTimeout = 5 * time.Second
	}
	if p.Config.WaitReady && p.Config.ListenTimeout == 0 {
		p.Config.ListenTimeout = 3 * time.Second
	}
	if p.stopSignal == 0 {
		sig := p.Config.StopSignal
		if sig == "" {
			p.stopSignal = defaultStopSignal()
		} else {
			p.stopSignal = parseSignal(sig)
		}
	}
	if defaults.BackoffBase > 0 {
		p.backoff.Base = defaults.BackoffBase
	}
	if defaults.BackoffMax > 0 {
		p.backoff.Max = defaults.BackoffMax
	}
}

// GetState returns the current process state (lock-free via atomic).
func (p *ManagedProcess) GetState() types.ProcessState {
	v := p.state.Load()
	if v == nil {
		return types.StateStopped
	}
	return v.(types.ProcessState)
}

// SetState attempts to transition to the given state. Returns true if the
// transition was valid and applied.
func (p *ManagedProcess) SetState(state types.ProcessState) bool {
	if !IsValidState(state) {
		return false
	}

	for {
		old := p.state.Load()
		var oldState types.ProcessState
		if old != nil {
			oldState = old.(types.ProcessState)
		} else {
			oldState = types.StateStopped
		}
		if !CanTransition(oldState, state) {
			log.Warn().
				Str("process", p.Config.Name).
				Str("id", p.ID).
				Str("from", string(oldState)).
				Str("to", string(state)).
				Msg("invalid state transition rejected")
			return false
		}
		if p.state.CompareAndSwap(old, state) {
			log.Info().
				Str("process", p.Config.Name).
				Str("id", p.ID).
				Str("from", string(oldState)).
				Str("to", string(state)).
				Msg("state transitioned")
			return true
		}
	}
}

// SetStateDirect forces the state without transition validation. Used for
// initial setup and error recovery.
func (p *ManagedProcess) SetStateDirect(state types.ProcessState) {
	p.state.Store(state)
}

// Start launches the process. It creates the exec.Cmd, configures the
// environment, wires up stdout/stderr, and transitions to StateRunning
// immediately unless readiness gating is enabled.
func (p *ManagedProcess) Start(ctx context.Context) error {
	if !p.SetState(types.StateStarting) {
		current := p.GetState()
		return fmt.Errorf("cannot start process %q: invalid transition from %s", p.Config.Name, current)
	}

	// Pre-start hook: can block the start.
	if p.Config.Hooks != nil && p.Config.Hooks.PreStart != nil {
		if err := p.hookExec.RunPre(ctx, p.Config.Hooks.PreStart, "pre_start", p.Config); err != nil {
			p.SetStateDirect(types.StateStopped)
			return fmt.Errorf("pre_start hook blocked start of %q: %w", p.Config.Name, err)
		}
	}

	ctx, cancel := context.WithCancel(ctx)

	// Reset the exited channel for a fresh lifecycle.
	p.mu.Lock()
	p.exited = make(chan struct{})
	p.closeOnce = &sync.Once{}
	p.cancel = cancel
	p.ExitCode = 0
	p.PID = 0
	p.ready.Store(false)

	args := buildArgs(p.Config)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	// Build environment: inherit current env, then overlay config env.
	cmd.Env = buildEnv(p.Config.Env)
	if p.Config.Cwd != "" {
		cmd.Dir = p.Config.Cwd
	}

	// Process group: create new group so we can signal the whole tree.
	setProcessGroup(cmd)

	// Wire stdout/stderr.
	if p.stdoutWriter != nil {
		cmd.Stdout = p.stdoutWriter
	} else {
		cmd.Stdout = io.Discard
	}
	if p.stderrWriter != nil {
		cmd.Stderr = p.stderrWriter
	} else {
		cmd.Stderr = io.Discard
	}

	p.cmd = cmd
	p.mu.Unlock()

	log.Info().
		Str("process", p.Config.Name).
		Str("id", p.ID).
		Str("command", strings.Join(args, " ")).
		Msg("starting process")

	if err := cmd.Start(); err != nil {
		cancel()
		p.SetState(types.StateErrored)
		log.Error().Err(err).Str("process", p.Config.Name).Str("command", strings.Join(args, " ")).
			Msg("failed to start process")
		return fmt.Errorf("failed to start process %q: %w", p.Config.Name, err)
	}

	p.mu.Lock()
	p.PID = cmd.Process.Pid
	p.startedAt = time.Now()
	p.mu.Unlock()

	if !p.Config.WaitReady {
		p.ready.Store(true)
		p.SetState(types.StateRunning)
	}

	log.Info().
		Str("process", p.Config.Name).
		Str("id", p.ID).
		Int("pid", p.PID).
		Msg("process started")

	// Post-start hook: non-blocking.
	if p.Config.Hooks != nil && p.Config.Hooks.PostStart != nil {
		p.hookExec.RunPost(ctx, p.Config.Hooks.PostStart, "post_start", p.Config)
	}

	return nil
}

// WaitForReady blocks until startup readiness succeeds when wait_ready is enabled.
func (p *ManagedProcess) WaitForReady(ctx context.Context) error {
	if !p.Config.WaitReady {
		return nil
	}
	return p.waitUntilReady(ctx, p.Config.ListenTimeout, true)
}

// WaitUntilReady blocks until the process is considered ready, using health
// checks when configured or the running state otherwise.
func (p *ManagedProcess) WaitUntilReady(ctx context.Context, timeout time.Duration) error {
	return p.waitUntilReady(ctx, timeout, false)
}

// Stop sends the configured stop signal and waits up to timeout for the
// process to exit. If it hasn't exited after the timeout, SIGKILL is sent.
// The actual exit handling is done by the monitor goroutine via handleExit.
func (p *ManagedProcess) Stop(timeout time.Duration) error {
	p.mu.RLock()
	cmd := p.cmd
	pid := p.PID
	exited := p.exited
	p.mu.RUnlock()

	if cmd == nil || cmd.Process == nil {
		return fmt.Errorf("process %q is not running", p.Config.Name)
	}

	if !p.SetState(types.StateStopping) {
		return fmt.Errorf("cannot stop process %q from state %s", p.Config.Name, p.GetState())
	}

	// Pre-stop hook: non-blocking (process is already being stopped).
	ctx := context.Background()
	if p.Config.Hooks != nil && p.Config.Hooks.PreStop != nil {
		p.hookExec.RunPost(ctx, p.Config.Hooks.PreStop, "pre_stop", p.Config)
	}

	log.Info().
		Str("process", p.Config.Name).
		Str("id", p.ID).
		Int("pid", pid).
		Str("signal", p.stopSignal.String()).
		Msg("sending stop signal to process group")

	// Signal the entire process group.
	signalProcessGroup(pid, p.stopSignal)

	// Wait for the monitor goroutine to call handleExit and close exited.
	select {
	case <-exited:
		// Post-stop hook: non-blocking.
		if p.Config.Hooks != nil && p.Config.Hooks.PostStop != nil {
			p.hookExec.RunPost(ctx, p.Config.Hooks.PostStop, "post_stop", p.Config)
		}
		return nil
	case <-time.After(timeout):
		log.Warn().
			Str("process", p.Config.Name).
			Str("id", p.ID).
			Int("pid", pid).
			Msg("process did not exit in time, sending SIGKILL")
		return p.ForceStop()
	}
}

// ForceStop sends SIGKILL to the process group and waits for the exit to be
// processed. If the monitor is already cancelled, it handles the exit directly.
func (p *ManagedProcess) ForceStop() error {
	p.mu.RLock()
	cmd := p.cmd
	pid := p.PID
	exited := p.exited
	p.mu.RUnlock()

	if cmd == nil || cmd.Process == nil {
		return fmt.Errorf("process %q is not running", p.Config.Name)
	}

	log.Warn().
		Str("process", p.Config.Name).
		Str("id", p.ID).
		Int("pid", pid).
		Msg("force-killing process group")

	signalProcessGroup(pid, killSignal())

	// Wait for exit, with a shorter timeout since SIGKILL is immediate.
	select {
	case <-exited:
		return nil
	case <-time.After(5 * time.Second):
		// Monitor may have been cancelled. Handle exit ourselves.
		p.handleExit(fmt.Errorf("killed"))
		return nil
	}
}

// Wait blocks until the process exits and returns the error from cmd.Wait.
func (p *ManagedProcess) Wait() error {
	p.mu.RLock()
	cmd := p.cmd
	p.mu.RUnlock()
	if cmd == nil {
		return fmt.Errorf("process %q has no command", p.Config.Name)
	}
	return cmd.Wait()
}

// exitGuard bundles the exit signaling primitives for a single process lifecycle.
// Each call to Start() creates a fresh guard. The monitor captures the guard at
// the start of Run so that drain goroutines always close the correct channel,
// even if a concurrent Start() resets the struct fields for a new lifecycle.
type exitGuard struct {
	exited    chan struct{}
	closeOnce *sync.Once
}

// currentGuard returns the current lifecycle's exit signaling primitives.
func (p *ManagedProcess) currentGuard() exitGuard {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return exitGuard{p.exited, p.closeOnce}
}

// handleExit processes the result of a process exit using the provided guard.
func (p *ManagedProcess) handleExitWith(waitErr error, g exitGuard) {
	exitCode := 0
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	p.mu.Lock()
	p.ExitCode = exitCode
	p.finishedAt = time.Now()
	p.mu.Unlock()

	// Transition based on current state.
	current := p.GetState()
	p.ready.Store(false)
	switch current {
	case types.StateStopping:
		p.SetStateDirect(types.StateStopped)
		p.RecordObservation("stopped", fmt.Sprintf("stopped with exit code %d", exitCode))
		log.Info().
			Str("process", p.Config.Name).
			Str("id", p.ID).
			Int("exit_code", exitCode).
			Msg("process stopped")
	default:
		if exitCode == 0 {
			p.SetStateDirect(types.StateStopped)
		} else {
			p.SetStateDirect(types.StateCrashed)
		}
		p.RecordObservation("exited", fmt.Sprintf("process exited with code %d", exitCode))
		log.Info().
			Str("process", p.Config.Name).
			Str("id", p.ID).
			Int("exit_code", exitCode).
			Msg("process exited")
	}

	g.closeOnce.Do(func() { close(g.exited) })
}

// handleExit processes a process exit using the current guard. Used by
// ForceStop when it handles exit directly instead of via the monitor.
func (p *ManagedProcess) handleExit(waitErr error) {
	p.handleExitWith(waitErr, p.currentGuard())
}

// ShouldRestart determines if the process should be restarted based on the
// restart policy, exit code, and max restarts limit.
func (p *ManagedProcess) ShouldRestart(exitCode int) bool {
	if !p.autoRestart {
		return false
	}

	policy := p.Config.RestartPolicy

	switch policy {
	case types.RestartNever:
		return false
	case types.RestartOnFailure:
		if exitCode == 0 {
			return false
		}
	case types.RestartAlways:
		// always restart regardless of exit code
	}

	p.mu.RLock()
	max := p.maxRestarts
	window := p.restartWindow
	count := p.restartStreak
	p.mu.RUnlock()

	if max > 0 && count >= max {
		log.Warn().
			Str("process", p.Config.Name).
			Str("id", p.ID).
			Int("max_restarts", max).
			Int("restarts", count).
			Msg("max restarts reached, not restarting")
		return false
	}

	if window > 0 && max > 0 {
		now := time.Now()
		cutoff := now.Add(-window)
		withinWindow := 0
		p.mu.RLock()
		for _, t := range p.restartTimes {
			if t.After(cutoff) {
				withinWindow++
			}
		}
		p.mu.RUnlock()
		if withinWindow >= max {
			log.Warn().
				Str("process", p.Config.Name).
				Str("id", p.ID).
				Int("window_restarts", withinWindow).
				Dur("window", window).
				Msg("max restarts within window reached, not restarting")
			return false
		}
	}

	return true
}

// maxRestartHistory caps the restartTimes slice when no restart window is set.
const maxRestartHistory = 1000

// recordRestart increments the restart counter and records the timestamp,
// pruning entries older than the restart window.
func (p *ManagedProcess) recordRestart() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Restarts++
	p.restartStreak++
	p.restartTimes = append(p.restartTimes, time.Now())

	if p.restartWindow > 0 {
		cutoff := time.Now().Add(-p.restartWindow)
		i := 0
		for i < len(p.restartTimes) && !p.restartTimes[i].After(cutoff) {
			i++
		}
		p.restartTimes = p.restartTimes[i:]
	} else if len(p.restartTimes) > maxRestartHistory {
		p.restartTimes = p.restartTimes[len(p.restartTimes)-maxRestartHistory:]
	}

	return p.restartStreak
}

// ResetRestartStreak clears consecutive restart tracking after a stable run.
func (p *ManagedProcess) ResetRestartStreak() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.restartStreak = 0
	p.restartTimes = nil
}

// ExitedBeforeMinUptime reports whether the process exited before min_uptime.
func (p *ManagedProcess) ExitedBeforeMinUptime() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.minUptime <= 0 || p.startedAt.IsZero() {
		return false
	}
	return time.Since(p.startedAt) < p.minUptime
}

// RestartBackoff returns the effective restart delay for the given attempt.
func (p *ManagedProcess) RestartBackoff(attempt int) time.Duration {
	delay := p.backoff.Next(attempt)
	if p.restartDelay > delay {
		return p.restartDelay
	}
	return delay
}

// CloseLogFile closes the process log files if they were opened.
func (p *ManagedProcess) CloseLogFile() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stdoutFile != nil {
		_ = p.stdoutFile.Close()
		p.stdoutFile = nil
	}
	if p.stderrFile != nil {
		_ = p.stderrFile.Close()
		p.stderrFile = nil
	}
	p.logFile = nil
}

// CancelContext cancels the process context (used during shutdown).
func (p *ManagedProcess) CancelContext() {
	p.mu.RLock()
	cancel := p.cancel
	p.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
}

func (p *ManagedProcess) RecordObservation(event string, reason string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastEvent = event
	p.lastReason = reason
}

// Info returns a snapshot of the current process state.
func (p *ManagedProcess) Info() types.ProcessInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	info := types.ProcessInfo{
		ID:            p.ID,
		NumericID:     p.NumericID,
		Name:          p.Config.Name,
		Namespace:     p.Config.Namespace,
		InstanceIndex: p.Config.InstanceIndex,
		Runtime:       p.Config.Runtime,
		State:         p.GetState(),
		Ready:         p.ready.Load(),
		PID:           p.PID,
		ExitCode:      p.ExitCode,
		Restarts:      p.Restarts,
		CreatedAt:     p.CreatedAt,
		Config:        p.Config,
		LastEvent:     p.lastEvent,
		LastReason:    p.lastReason,
	}

	if !p.startedAt.IsZero() {
		sa := p.startedAt
		info.StartedAt = &sa
		info.Uptime = time.Since(p.startedAt)
	}
	if !p.finishedAt.IsZero() {
		fa := p.finishedAt
		info.FinishedAt = &fa
	}

	// Enrich with live metrics if a collector is wired up.
	if p.metricsCol != nil && p.PID > 0 {
		if m, ok := p.metricsCol.Get(p.PID); ok {
			info.CPUPercent = m.CPUPercent
			info.MemBytes = m.MemBytes
			info.MemPercent = m.MemPercent
			info.Threads = m.Threads
			info.FDs = m.FDs
		}
	}

	return info
}

func (p *ManagedProcess) readinessHealthCheck() (types.HealthCheckConfig, bool) {
	if p.Config.HealthCheck != nil {
		return *p.Config.HealthCheck, true
	}
	if p.Config.HealthCheckURL != "" {
		return types.HealthCheckConfig{Type: types.HealthCheckHTTP, URL: p.Config.HealthCheckURL}, true
	}
	return types.HealthCheckConfig{}, false
}

func (p *ManagedProcess) waitUntilReady(ctx context.Context, timeout time.Duration, promote bool) error {
	healthCfg, hasHealthCheck := p.readinessHealthCheck()
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	readyCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	var checker *healthcheck.Checker
	if hasHealthCheck {
		checker = healthcheck.NewChecker(healthCfg, nil)
	}

	for {
		state := p.GetState()
		switch state {
		case types.StateRunning:
			if !hasHealthCheck {
				return nil
			}
		case types.StateStarting:
			if !hasHealthCheck {
				if promote {
					if !p.SetState(types.StateRunning) {
						return fmt.Errorf("process %q could not transition to running from %s", p.Config.Name, p.GetState())
					}
					p.ready.Store(true)
					return nil
				}
			}
		case types.StateStopped, types.StateCrashed, types.StateErrored:
			return fmt.Errorf("process %q exited before becoming ready (state: %s)", p.Config.Name, state)
		}

		if checker != nil {
			if err := checker.Check(readyCtx); err == nil {
				if promote && state == types.StateStarting {
					if !p.SetState(types.StateRunning) {
						return fmt.Errorf("process %q could not transition to running from %s", p.Config.Name, p.GetState())
					}
				}
				p.ready.Store(true)
				return nil
			}
		}

		select {
		case <-readyCtx.Done():
			return fmt.Errorf("process %q did not become ready within %s", p.Config.Name, timeout)
		case <-ticker.C:
		}
	}
}

// buildArgs constructs the command-line arguments from the process config.
func buildArgs(cfg types.ProcessConfig) []string {
	args := buildArgsRaw(cfg)
	for i := range args {
		args[i] = sanitizeArg(args[i])
	}
	return args
}

func buildArgsRaw(cfg types.ProcessConfig) []string {
	if cfg.UseBundle {
		interpreter := "ruby"
		if cfg.Interpreter != "" {
			interpreter = cfg.Interpreter
		}
		args := []string{"bundle", "exec", interpreter, cfg.Entrypoint}
		args = append(args, cfg.Args...)
		return args
	}
	if cfg.Interpreter != "" {
		args := []string{cfg.Interpreter, cfg.Entrypoint}
		args = append(args, cfg.Args...)
		return args
	}
	if cfg.Entrypoint == "" {
		return cfg.Args
	}
	args := []string{cfg.Entrypoint}
	args = append(args, cfg.Args...)
	return args
}

// buildEnv returns a complete environment list with the config env overlaid
// on top of the current process environment.
func buildEnv(overlay map[string]string) []string {
	env := os.Environ()
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

// sanitizeArg strips null bytes from a command argument sourced from config.
func sanitizeArg(s string) string {
	for i := range len(s) {
		if s[i] == 0 {
			return s[:i]
		}
	}
	return s
}
