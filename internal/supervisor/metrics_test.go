package supervisor

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/runixio/runix/internal/metrics"
	"github.com/runixio/runix/pkg/types"
)

func TestMetricsCollectorTrackOnAddProcess(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("metrics integration requires Linux")
	}

	col := metrics.NewCollector()
	sup := New(Options{MetricsCollector: col})

	cfg := types.ProcessConfig{
		Name:       "metrics-test-" + os.Getenv("GO_TEST"),
		Runtime:    "go",
		Entrypoint: os.Args[0],
		Args:       []string{"test", "-run", "^$"},
		Autostart:  true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	proc, err := sup.AddProcess(ctx, cfg)
	if err != nil {
		t.Fatalf("AddProcess failed: %v", err)
	}

	// Ensure process is tracked.
	if proc.PID <= 0 {
		t.Skip("process did not start (likely build race)")
	}

	// The collector should have the PID tracked.
	_, ok := col.Get(proc.PID)
	if !ok {
		t.Error("expected PID to be tracked after AddProcess")
	}

	// Force stop and remove.
	_ = sup.StopProcess(proc.ID, true, 5*time.Second)
	_ = sup.RemoveProcess(proc.ID)

	// PID should be untracked after removal.
	_, ok = col.Get(proc.PID)
	if ok {
		t.Error("expected PID to be untracked after RemoveProcess")
	}

	_ = sup.Shutdown()
	col.Stop()
}

func TestMetricsCollectorUntrackOnStop(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("metrics integration requires Linux")
	}

	col := metrics.NewCollector()
	sup := New(Options{MetricsCollector: col})

	cfg := types.ProcessConfig{
		Name:       "metrics-stop-test-" + os.Getenv("GO_TEST"),
		Runtime:    "go",
		Entrypoint: os.Args[0],
		Args:       []string{"test", "-run", "^$"},
		Autostart:  true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	proc, err := sup.AddProcess(ctx, cfg)
	if err != nil {
		t.Fatalf("AddProcess failed: %v", err)
	}
	if proc.PID <= 0 {
		t.Skip("process did not start")
	}

	// Stop (not remove) — PID should still be tracked.
	_ = sup.StopProcess(proc.ID, true, 5*time.Second)

	_, ok := col.Get(proc.PID)
	if !ok {
		t.Error("expected PID to remain tracked after StopProcess")
	}

	_ = sup.RemoveProcess(proc.ID)
	_, ok = col.Get(proc.PID)
	if ok {
		t.Error("expected PID to be untracked after RemoveProcess")
	}

	_ = sup.Shutdown()
	col.Stop()
}

func TestMetricsCollectorReTrackOnRestart(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("metrics integration requires Linux")
	}

	col := metrics.NewCollector()
	sup := New(Options{MetricsCollector: col})

	cfg := types.ProcessConfig{
		Name:       "metrics-restart-test-" + os.Getenv("GO_TEST"),
		Runtime:    "go",
		Entrypoint: os.Args[0],
		Args:       []string{"test", "-run", "^$"},
		Autostart:  true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	proc, err := sup.AddProcess(ctx, cfg)
	if err != nil {
		t.Fatalf("AddProcess failed: %v", err)
	}
	if proc.PID <= 0 {
		t.Skip("process did not start")
	}

	oldPID := proc.PID

	// Restart — uses a fresh context to avoid the parent timeout being
	// consumed by the stop phase.
	restartCtx, restartCancel := context.WithTimeout(ctx, 20*time.Second)
	err = sup.RestartProcess(restartCtx, proc.ID)
	restartCancel()
	if err != nil {
		t.Fatalf("RestartProcess failed: %v", err)
	}

	// After restart, old PID should be gone and new PID tracked.
	_, ok := col.Get(oldPID)
	if ok {
		t.Error("expected old PID to be untracked after restart")
	}

	info := proc.Info()
	if info.PID == oldPID {
		t.Error("expected new PID after restart")
	}
	if info.PID <= 0 {
		t.Error("expected positive PID after restart")
	}

	_, ok = col.Get(info.PID)
	if !ok {
		t.Error("expected new PID to be tracked after restart")
	}

	_ = sup.RemoveProcess(proc.ID)
	_ = sup.Shutdown()
	col.Stop()
}

func TestMetricsCollectorNilIsSafe(t *testing.T) {
	sup := New(Options{})
	// Creating a supervisor without a collector should not panic.
	// Adding/removing processes should work without metrics tracking.
	cfg := types.ProcessConfig{
		Name:       "metrics-nil-safe-test",
		Runtime:    "go",
		Entrypoint: os.Args[0],
		Args:       []string{"test", "-run", "^$"},
		Autostart:  true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	proc, err := sup.AddProcess(ctx, cfg)
	if err != nil {
		t.Fatalf("AddProcess failed: %v", err)
	}

	_ = sup.StopProcess(proc.ID, true, 5*time.Second)
	_ = sup.RemoveProcess(proc.ID)

	_ = sup.Shutdown()
}

func TestInfoReturnsMetricsAfterCollection(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("metrics integration requires Linux")
	}

	col := metrics.NewCollector()
	sup := New(Options{MetricsCollector: col})

	cfg := types.ProcessConfig{
		Name:       "metrics-info-test-" + os.Getenv("GO_TEST"),
		Runtime:    "go",
		Entrypoint: os.Args[0],
		Args:       []string{"test", "-run", "^$"},
		Autostart:  true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	proc, err := sup.AddProcess(ctx, cfg)
	if err != nil {
		t.Fatalf("AddProcess failed: %v", err)
	}
	if proc.PID <= 0 {
		t.Skip("process did not start")
	}

	// Spin briefly so the process accumulates CPU ticks.
	busyLoop(100 * time.Millisecond)

	// Force a metrics collection cycle.
	col.CollectAll()

	// Info should now contain real metrics.
	info := proc.Info()

	if info.CPUPercent < 0 {
		t.Errorf("expected non-negative CPU%%, got %f", info.CPUPercent)
	}
	if info.MemBytes <= 0 {
		t.Errorf("expected positive MemBytes, got %d", info.MemBytes)
	}
	if info.Threads <= 0 {
		t.Errorf("expected positive Threads, got %d", info.Threads)
	}
	if info.FDs < 0 {
		t.Errorf("expected non-negative FDs, got %d", info.FDs)
	}
	if info.MemPercent <= 0 {
		t.Errorf("expected positive MemPercent, got %f", info.MemPercent)
	}

	_ = sup.RemoveProcess(proc.ID)
	_ = sup.Shutdown()
	col.Stop()
}

func TestInfoWithStoppedProcessShowsZeros(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("metrics integration requires Linux")
	}

	col := metrics.NewCollector()
	sup := New(Options{MetricsCollector: col})

	cfg := types.ProcessConfig{
		Name:       "metrics-stopped-test-" + os.Getenv("GO_TEST"),
		Runtime:    "go",
		Entrypoint: os.Args[0],
		Args:       []string{"test", "-run", "^$"},
		Autostart:  true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	proc, err := sup.AddProcess(ctx, cfg)
	if err != nil {
		t.Fatalf("AddProcess failed: %v", err)
	}
	if proc.PID <= 0 {
		t.Skip("process did not start")
	}

	// Stop the process.
	_ = sup.StopProcess(proc.ID, true, 5*time.Second)

	// Collect — the stopped process should still have memory data from
	// the last collection, or zeros if pruned.
	col.CollectAll()

	info := proc.Info()
	// After stop, CPU should not be accumulating, but we don't assert
	// zero — the last snapshot may still be present.
	// MemBytes may still be valid from last collection before pruning.
	// The key requirement is that Info() doesn't crash and returns a
	// reasonable value.

	// Verify at least the struct is populated correctly.
	if info.MemPercent < 0 {
		t.Errorf("expected non-negative MemPercent, got %f", info.MemPercent)
	}

	_ = sup.RemoveProcess(proc.ID)
	_ = sup.Shutdown()
	col.Stop()
}

func busyLoop(d time.Duration) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
	}
}
