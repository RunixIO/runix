package metrics

import (
	"os"
	"runtime"
	"testing"
	"time"
)

func TestCollectorTrackUntrack(t *testing.T) {
	c := NewCollector()

	pid := os.Getpid()
	c.Track(pid)

	m, ok := c.Get(pid)
	if !ok {
		t.Fatal("expected to find tracked PID")
	}
	if m.PID != pid {
		t.Errorf("expected PID %d, got %d", pid, m.PID)
	}

	c.Untrack(pid)
	_, ok = c.Get(pid)
	if ok {
		t.Error("expected PID to be untracked")
	}
}

func TestCollectorGetAll(t *testing.T) {
	c := NewCollector()

	c.Track(1)
	c.Track(2)

	all := c.GetAll()
	if len(all) != 2 {
		t.Errorf("expected 2 entries, got %d", len(all))
	}
}

func TestCollectorStop(t *testing.T) {
	c := NewCollector()
	c.Start(1 * time.Second)
	c.Stop()
	// Should not panic.
}

func TestCollectProcessMetrics(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("skipping on non-Linux")
	}

	pid := os.Getpid()

	// Spin to accumulate some CPU ticks before reading.
	busyLoop(100 * time.Millisecond)

	m, ticks := collectProcessMetrics(pid)

	if m.PID != pid {
		t.Errorf("expected PID %d, got %d", pid, m.PID)
	}
	if m.CollectedAt.IsZero() {
		t.Error("expected CollectedAt to be set")
	}
	// Our own process should have some memory.
	if m.MemBytes <= 0 {
		t.Error("expected positive MemBytes for current process")
	}
	if m.Threads <= 0 {
		t.Error("expected positive thread count")
	}
	// CPU ticks should be non-zero for a running process.
	if ticks <= 0 {
		t.Error("expected positive totalTicks for current process")
	}
}

func TestCPUDeltaComputation(t *testing.T) {
	c := NewCollector()
	defer c.Stop()

	pid := os.Getpid()
	c.Track(pid)

	// First collection should yield 0% (no previous snapshot).
	c.CollectAll()
	m, ok := c.Get(pid)
	if !ok {
		t.Fatal("expected PID to be tracked")
	}
	// First cycle: no delta yet, CPU% should be 0.
	if m.CPUPercent != 0 {
		t.Errorf("first collection CPU%% should be 0, got %f", m.CPUPercent)
	}

	// Do some CPU work to ensure ticks increase.
	busyLoop(50 * time.Millisecond)

	// Second collection should yield non-zero CPU%.
	c.CollectAll()
	m, ok = c.Get(pid)
	if !ok {
		t.Fatal("expected PID to still be tracked")
	}
	// After doing work, CPU% should be > 0.
	if m.CPUPercent <= 0 {
		t.Errorf("second collection CPU%% should be > 0, got %f", m.CPUPercent)
	}
}

func TestMemPercent(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("skipping on non-Linux")
	}

	c := NewCollector()
	defer c.Stop()

	pid := os.Getpid()
	c.Track(pid)

	c.CollectAll()

	m, ok := c.Get(pid)
	if !ok {
		t.Fatal("expected PID to be tracked")
	}
	// Our process should use some memory and MemPercent should be > 0.
	if m.MemBytes <= 0 {
		t.Skip("skipping: MemBytes is 0")
	}
	if m.MemPercent <= 0 {
		t.Errorf("expected MemPercent > 0 for process using %d bytes, got %f", m.MemBytes, m.MemPercent)
	}
	if m.MemPercent > 100 {
		t.Errorf("MemPercent should not exceed 100, got %f", m.MemPercent)
	}
}

func TestTotalMemory(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("skipping on non-Linux")
	}

	c := NewCollector()
	total := c.TotalMemory()
	if total <= 0 {
		t.Errorf("expected positive total memory, got %d", total)
	}
	// Should be at least 1 GB on any reasonable system.
	if total < 1<<30 {
		t.Errorf("total memory seems too low: %d bytes", total)
	}
}

func TestAutoPruneDeadPID(t *testing.T) {
	c := NewCollector()

	// Track a PID that definitely doesn't exist.
	deadPID := 9999999
	c.Track(deadPID)

	c.CollectAll()

	_, ok := c.Get(deadPID)
	if ok {
		t.Error("dead PID should have been pruned")
	}
}

func TestUntrackCleansUpCPUTransition(t *testing.T) {
	c := NewCollector()

	c.Track(12345)
	// Simulate a previous CPU snapshot.
	c.cpuMu.Lock()
	c.cpuPrev[12345] = cpuSnapshot{totalTicks: 1000, createdAt: time.Now()}
	c.cpuMu.Unlock()

	c.Untrack(12345)

	// CPU delta state should be cleaned up.
	c.cpuMu.Lock()
	_, exists := c.cpuPrev[12345]
	c.cpuMu.Unlock()
	if exists {
		t.Error("expected CPU delta to be cleaned up after Untrack")
	}
}

func TestComputeCPUDeltaNoPrevious(t *testing.T) {
	c := NewCollector()
	now := time.Now()

	// No previous snapshot → should return 0.
	pct := c.computeCPUDelta(99999, 500, now)
	if pct != 0 {
		t.Errorf("expected 0 for first snapshot, got %f", pct)
	}
}

func TestComputeCPUDeltaNegativeTickDelta(t *testing.T) {
	c := NewCollector()
	now := time.Now()

	// Set up previous with higher ticks.
	c.cpuMu.Lock()
	c.cpuPrev[1] = cpuSnapshot{totalTicks: 2000, createdAt: now.Add(-time.Second)}
	c.cpuMu.Unlock()

	// Lower ticks now → should return 0.
	pct := c.computeCPUDelta(1, 1000, now)
	if pct != 0 {
		t.Errorf("expected 0 for negative tick delta, got %f", pct)
	}
}

func TestCollectProcessMetricsNonExistentPID(t *testing.T) {
	m, ticks := collectProcessMetrics(9999999)
	if m.PID != 9999999 {
		t.Errorf("expected PID to be set even for non-existent process")
	}
	if ticks != 0 {
		t.Errorf("expected 0 ticks for non-existent process, got %d", ticks)
	}
	if m.MemBytes != 0 {
		t.Errorf("expected 0 MemBytes for non-existent process")
	}
}

// busyLoop consumes CPU for the given duration to ensure tick counters advance.
func busyLoop(d time.Duration) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		// Spin.
	}
}
