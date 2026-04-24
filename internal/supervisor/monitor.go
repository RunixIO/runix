package supervisor

import (
	"context"
	"os/exec"

	"github.com/rs/zerolog/log"
)

// Monitor watches a ManagedProcess and invokes a callback when the process
// exits. The onExit callback is where the Supervisor decides whether to
// restart.
type Monitor struct{}

// NewMonitor creates a new Monitor.
func NewMonitor() *Monitor {
	return &Monitor{}
}

// Run blocks until the managed process exits or the context is cancelled.
// When the process exits, it calls handleExit on the process to update state,
// then invokes onExit with the exit code for the supervisor to act on.
//
// Run should be started in a goroutine immediately after the process is
// started. The Stop/ForceStop methods send signals; this goroutine handles
// waiting for the actual exit and state transitions.
func (m *Monitor) Run(ctx context.Context, proc *ManagedProcess, onExit func(*ManagedProcess, int)) {
	// Capture exit signaling before waiting on the process. The drain
	// goroutine (ctx.Done path) uses this captured guard so that a
	// concurrent Start() resetting the struct fields won't cause it to
	// close the wrong channel.
	g := proc.currentGuard()

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- proc.Wait()
	}()

	select {
	case err := <-waitCh:
		exitCode := extractExitCode(err)
		proc.handleExitWith(err, g)
		log.Info().
			Str("process", proc.Config.Name).
			Str("id", proc.ID).
			Int("exit_code", exitCode).
			Msg("monitor detected process exit")
		onExit(proc, exitCode)
	case <-ctx.Done():
		// Context was cancelled (restart/reload/removal). Keep draining Wait in
		// the background so Stop/ForceStop still observe p.exited without forcing
		// a timeout path just because the monitor was canceled.
		go func() {
			err := <-waitCh
			proc.handleExitWith(err, g)
			log.Debug().
				Str("process", proc.Config.Name).
				Str("id", proc.ID).
				Int("exit_code", extractExitCode(err)).
				Msg("monitor drained process exit after cancellation")
		}()
		log.Debug().
			Str("process", proc.Config.Name).
			Str("id", proc.ID).
			Msg("monitor cancelled by context")
	}
}

// extractExitCode returns the exit code from a cmd.Wait error, or 0 if nil.
func extractExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}
