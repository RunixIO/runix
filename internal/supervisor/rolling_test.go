package supervisor

import (
	"context"
	"strings"
	"testing"

	"github.com/runixio/runix/pkg/types"
)

func TestRollingReloadWaitReadyRequiresHealthCheck(t *testing.T) {
	dir := t.TempDir()
	sup := New(Options{
		LogDir: dir,
		Defaults: types.DefaultsConfig{
			RestartPolicy: types.RestartNever,
		},
	})
	defer func() { _ = sup.Shutdown() }()

	proc, err := sup.AddProcess(context.Background(), types.ProcessConfig{
		Name:          "no-healthcheck",
		Entrypoint:    "sleep",
		Args:          []string{"30"},
		Runtime:       "unknown",
		RestartPolicy: types.RestartNever,
	})
	if err != nil {
		t.Fatalf("AddProcess failed: %v", err)
	}

	oldPID := proc.PID
	err = sup.RollingReload(context.Background(), []string{proc.ID}, RollingReloadOptions{
		BatchSize: 1,
		WaitReady: true,
	})
	if err == nil {
		t.Fatal("expected rolling reload to fail without readiness check")
	}
	if !strings.Contains(err.Error(), "no readiness check configured") {
		t.Fatalf("expected readiness check error, got %v", err)
	}
	if proc.Info().PID != oldPID {
		t.Fatalf("expected PID to remain unchanged on validation failure, old=%d new=%d", oldPID, proc.Info().PID)
	}
}

func TestRollingReloadWaitReadyRejectsFullBatch(t *testing.T) {
	dir := t.TempDir()
	sup := New(Options{
		LogDir: dir,
		Defaults: types.DefaultsConfig{
			RestartPolicy: types.RestartNever,
		},
	})
	defer func() { _ = sup.Shutdown() }()

	for _, name := range []string{"api-1", "api-2"} {
		_, err := sup.AddProcess(context.Background(), types.ProcessConfig{
			Name:          name,
			Entrypoint:    "sleep",
			Args:          []string{"30"},
			Runtime:       "unknown",
			RestartPolicy: types.RestartNever,
			HealthCheck: &types.HealthCheckConfig{
				Type:    types.HealthCheckCommand,
				Command: "true",
				Timeout: "100ms",
			},
		})
		if err != nil {
			t.Fatalf("AddProcess(%q) failed: %v", name, err)
		}
	}

	err := sup.RollingReload(context.Background(), []string{"api-1", "api-2"}, RollingReloadOptions{
		BatchSize: 2,
		WaitReady: true,
	})
	if err == nil {
		t.Fatal("expected rolling reload to reject full-batch zero-downtime reload")
	}
	if !strings.Contains(err.Error(), "batch_size smaller than total processes") {
		t.Fatalf("expected batch size validation error, got %v", err)
	}
}
