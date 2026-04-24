package supervisor

import (
	"testing"
	"time"

	"github.com/runixio/runix/pkg/types"
)

func TestShouldRestartHonorsAutoRestartFalse(t *testing.T) {
	autoRestart := false
	proc := NewManagedProcess(types.ProcessConfig{
		Name:          "app",
		Entrypoint:    "sleep",
		RestartPolicy: types.RestartAlways,
		AutoRestart:   &autoRestart,
	})
	proc.ApplyDefaults(types.DefaultsConfig{RestartPolicy: types.RestartAlways})

	if proc.ShouldRestart(1) {
		t.Fatal("expected autorestart=false to disable automatic restart")
	}
}

func TestResetRestartStreakAfterStableRun(t *testing.T) {
	proc := NewManagedProcess(types.ProcessConfig{
		Name:          "app",
		Entrypoint:    "sleep",
		RestartPolicy: types.RestartAlways,
		MinUptime:     2 * time.Second,
	})
	proc.ApplyDefaults(types.DefaultsConfig{
		RestartPolicy: types.RestartAlways,
		MaxRestarts:   2,
	})

	if attempt := proc.recordRestart(); attempt != 1 {
		t.Fatalf("expected first restart attempt to be 1, got %d", attempt)
	}
	if attempt := proc.recordRestart(); attempt != 2 {
		t.Fatalf("expected second restart attempt to be 2, got %d", attempt)
	}
	if proc.ShouldRestart(1) {
		t.Fatal("expected max_restarts to block restart before streak reset")
	}

	proc.mu.Lock()
	proc.startedAt = time.Now().Add(-3 * time.Second)
	proc.mu.Unlock()

	if proc.ExitedBeforeMinUptime() {
		t.Fatal("expected process run to be considered stable")
	}

	proc.ResetRestartStreak()

	if !proc.ShouldRestart(1) {
		t.Fatal("expected restart to be allowed after stable run resets streak")
	}
	if attempt := proc.recordRestart(); attempt != 1 {
		t.Fatalf("expected streak to restart at attempt 1, got %d", attempt)
	}
}

func TestRestartBackoffHonorsRestartDelayFloor(t *testing.T) {
	proc := NewManagedProcess(types.ProcessConfig{
		Name:          "app",
		Entrypoint:    "sleep",
		RestartPolicy: types.RestartAlways,
		RestartDelay:  5 * time.Second,
	})
	proc.ApplyDefaults(types.DefaultsConfig{
		RestartPolicy: types.RestartAlways,
		BackoffBase:   time.Second,
		BackoffMax:    10 * time.Second,
	})

	if got := proc.RestartBackoff(0); got != 5*time.Second {
		t.Fatalf("expected restart delay floor of 5s, got %v", got)
	}
}
