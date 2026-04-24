package supervisor

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/runixio/runix/internal/metrics"
	"github.com/runixio/runix/pkg/types"
)

type fakeMetricsCollector struct {
	metrics map[int]metrics.ProcessMetrics
}

func (f *fakeMetricsCollector) Track(pid int) {
	if f.metrics == nil {
		f.metrics = make(map[int]metrics.ProcessMetrics)
	}
	if _, ok := f.metrics[pid]; !ok {
		f.metrics[pid] = metrics.ProcessMetrics{PID: pid}
	}
}

func (f *fakeMetricsCollector) Untrack(pid int) {
	delete(f.metrics, pid)
}

func (f *fakeMetricsCollector) Get(pid int) (metrics.ProcessMetrics, bool) {
	m, ok := f.metrics[pid]
	return m, ok
}

func TestEnforceMemoryLimitsRestartsProcess(t *testing.T) {
	col := &fakeMetricsCollector{}
	sup := New(Options{MetricsCollector: col})
	defer func() { _ = sup.Shutdown() }()

	proc, err := sup.AddProcess(context.Background(), types.ProcessConfig{
		Name:             "mem-restart-test",
		Entrypoint:       "sleep",
		Args:             []string{"30"},
		Runtime:          "unknown",
		RestartPolicy:    types.RestartNever,
		MaxMemoryRestart: "1MB",
	})
	if err != nil {
		t.Fatalf("AddProcess failed: %v", err)
	}

	oldPID := proc.PID
	col.metrics[oldPID] = metrics.ProcessMetrics{
		PID:      oldPID,
		MemBytes: 2 * 1024 * 1024,
	}

	sup.enforceMemoryLimits()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if proc.Info().PID != oldPID && proc.Info().PID > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("expected process PID to change after memory-threshold restart, old=%d new=%d", oldPID, proc.Info().PID)
}

func TestEnforceMemoryLimitsHonorsAutoRestartFalse(t *testing.T) {
	autoRestart := false
	col := &fakeMetricsCollector{}
	sup := New(Options{MetricsCollector: col})
	defer func() { _ = sup.Shutdown() }()

	proc, err := sup.AddProcess(context.Background(), types.ProcessConfig{
		Name:             "mem-no-restart-test-" + os.Getenv("GO_TEST"),
		Entrypoint:       "sleep",
		Args:             []string{"30"},
		Runtime:          "unknown",
		RestartPolicy:    types.RestartAlways,
		AutoRestart:      &autoRestart,
		MaxMemoryRestart: "1MB",
	})
	if err != nil {
		t.Fatalf("AddProcess failed: %v", err)
	}

	oldPID := proc.PID
	col.metrics[oldPID] = metrics.ProcessMetrics{
		PID:      oldPID,
		MemBytes: 2 * 1024 * 1024,
	}

	sup.enforceMemoryLimits()
	time.Sleep(200 * time.Millisecond)

	info := proc.Info()
	if info.PID != oldPID {
		t.Fatalf("expected PID to remain unchanged with autorestart=false, old=%d new=%d", oldPID, info.PID)
	}
}
