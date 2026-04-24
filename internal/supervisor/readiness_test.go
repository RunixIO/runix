package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/runixio/runix/pkg/types"
)

func TestAddProcessWaitReadySuccess(t *testing.T) {
	dir := t.TempDir()
	readyFile := filepath.Join(dir, "ready")

	sup := New(Options{
		LogDir: dir,
		Defaults: types.DefaultsConfig{
			RestartPolicy: types.RestartNever,
		},
	})
	defer func() { _ = sup.Shutdown() }()

	proc, err := sup.AddProcess(context.Background(), types.ProcessConfig{
		Name:          "ready-success",
		Entrypoint:    "sh",
		Args:          []string{"-c", "sleep 0.2; touch ready; sleep 30"},
		Cwd:           dir,
		Runtime:       "unknown",
		RestartPolicy: types.RestartNever,
		WaitReady:     true,
		ListenTimeout: 2 * time.Second,
		HealthCheck: &types.HealthCheckConfig{
			Type:    types.HealthCheckCommand,
			Command: "test -f " + readyFile,
			Timeout: "100ms",
		},
	})
	if err != nil {
		t.Fatalf("AddProcess returned error: %v", err)
	}

	if _, err := os.Stat(readyFile); err != nil {
		t.Fatalf("expected readiness file to exist: %v", err)
	}

	info := proc.Info()
	if info.State != types.StateRunning {
		t.Fatalf("expected running state after readiness, got %s", info.State)
	}
	if !info.Ready {
		t.Fatal("expected ready=true after readiness succeeds")
	}
}

func TestAddProcessWaitReadyTimeout(t *testing.T) {
	dir := t.TempDir()

	sup := New(Options{
		LogDir: dir,
		Defaults: types.DefaultsConfig{
			RestartPolicy: types.RestartNever,
		},
	})
	defer func() { _ = sup.Shutdown() }()

	_, err := sup.AddProcess(context.Background(), types.ProcessConfig{
		Name:          "ready-timeout",
		Entrypoint:    "sh",
		Args:          []string{"-c", "sleep 30"},
		Cwd:           dir,
		Runtime:       "unknown",
		RestartPolicy: types.RestartNever,
		WaitReady:     true,
		ListenTimeout: 300 * time.Millisecond,
		HealthCheck: &types.HealthCheckConfig{
			Type:    types.HealthCheckCommand,
			Command: "test -f never-created",
			Timeout: "100ms",
		},
	})
	if err == nil {
		t.Fatal("expected readiness timeout error, got nil")
	}

	if got := len(sup.List()); got != 0 {
		t.Fatalf("expected no registered processes after readiness failure, got %d", got)
	}
}

func TestAddProcessWithoutWaitReadyStartsReady(t *testing.T) {
	dir := t.TempDir()

	sup := New(Options{
		LogDir: dir,
		Defaults: types.DefaultsConfig{
			RestartPolicy: types.RestartNever,
		},
	})
	defer func() { _ = sup.Shutdown() }()

	proc, err := sup.AddProcess(context.Background(), types.ProcessConfig{
		Name:          "no-wait-ready",
		Entrypoint:    "sleep",
		Args:          []string{"30"},
		Cwd:           dir,
		Runtime:       "unknown",
		RestartPolicy: types.RestartNever,
	})
	if err != nil {
		t.Fatalf("AddProcess returned error: %v", err)
	}

	info := proc.Info()
	if info.State != types.StateRunning {
		t.Fatalf("expected running state, got %s", info.State)
	}
	if !info.Ready {
		t.Fatal("expected ready=true when wait_ready is disabled")
	}
}

func TestRestartProcessWaitReadySuccess(t *testing.T) {
	dir := t.TempDir()
	readyFile := filepath.Join(dir, "ready")

	sup := New(Options{
		LogDir: dir,
		Defaults: types.DefaultsConfig{
			RestartPolicy: types.RestartNever,
		},
	})
	defer func() { _ = sup.Shutdown() }()

	proc, err := sup.AddProcess(context.Background(), types.ProcessConfig{
		Name:          "restart-ready-success",
		Entrypoint:    "sh",
		Args:          []string{"-c", "touch ready; sleep 30"},
		Cwd:           dir,
		Runtime:       "unknown",
		RestartPolicy: types.RestartNever,
		WaitReady:     true,
		ListenTimeout: 2 * time.Second,
		HealthCheck: &types.HealthCheckConfig{
			Type:    types.HealthCheckCommand,
			Command: "test -f " + readyFile,
			Timeout: "100ms",
		},
	})
	if err != nil {
		t.Fatalf("AddProcess returned error: %v", err)
	}

	if err := os.Remove(readyFile); err != nil {
		t.Fatalf("failed to remove readiness file before restart: %v", err)
	}

	proc.Config.Args = []string{"-c", "sleep 0.2; touch ready; sleep 30"}

	if err := sup.RestartProcess(context.Background(), proc.ID); err != nil {
		t.Fatalf("RestartProcess returned error: %v", err)
	}

	info := proc.Info()
	if info.State != types.StateRunning {
		t.Fatalf("expected running state after restart readiness, got %s", info.State)
	}
	if !info.Ready {
		t.Fatal("expected ready=true after restart readiness succeeds")
	}
}

func TestReloadProcessWaitReadyTimeout(t *testing.T) {
	dir := t.TempDir()
	readyFile := filepath.Join(dir, "ready")

	sup := New(Options{
		LogDir: dir,
		Defaults: types.DefaultsConfig{
			RestartPolicy: types.RestartNever,
		},
	})
	defer func() { _ = sup.Shutdown() }()

	proc, err := sup.AddProcess(context.Background(), types.ProcessConfig{
		Name:          "reload-ready-timeout",
		Entrypoint:    "sh",
		Args:          []string{"-c", "touch ready; sleep 30"},
		Cwd:           dir,
		Runtime:       "unknown",
		RestartPolicy: types.RestartNever,
		WaitReady:     true,
		ListenTimeout: time.Second,
		HealthCheck: &types.HealthCheckConfig{
			Type:    types.HealthCheckCommand,
			Command: "test -f " + readyFile,
			Timeout: "100ms",
		},
	})
	if err != nil {
		t.Fatalf("AddProcess returned error: %v", err)
	}

	if err := os.Remove(readyFile); err != nil {
		t.Fatalf("failed to remove readiness file before reload: %v", err)
	}

	proc.Config.Args = []string{"-c", "sleep 30"}
	proc.Config.ListenTimeout = 300 * time.Millisecond

	err = sup.ReloadProcess(context.Background(), proc.ID)
	if err == nil {
		t.Fatal("expected reload readiness timeout error, got nil")
	}

	info := proc.Info()
	if info.Ready {
		t.Fatal("expected ready=false after reload readiness timeout")
	}
	if info.State != types.StateErrored {
		t.Fatalf("expected errored state after reload readiness timeout, got %s", info.State)
	}
}
