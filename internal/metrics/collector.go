package metrics

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ProcessMetrics holds resource usage for a single process.
type ProcessMetrics struct {
	PID         int       `json:"pid"`
	CPUPercent  float64   `json:"cpu_percent"`
	MemBytes    int64     `json:"mem_bytes"`
	MemPercent  float64   `json:"mem_percent"`
	Threads     int       `json:"threads"`
	FDs         int       `json:"fds"`
	CollectedAt time.Time `json:"collected_at"`
}

// cpuSnapshot stores the previous CPU tick count and collection time for
// a single PID, enabling delta-based CPU% calculation between cycles.
type cpuSnapshot struct {
	totalTicks int64
	createdAt  time.Time
}

// Collector periodically collects per-process metrics.
type Collector struct {
	mu       sync.RWMutex
	metrics  map[int]ProcessMetrics // PID -> latest metrics
	stop     chan struct{}
	stopOnce sync.Once

	// CPU delta tracking: previous tick snapshot per PID.
	cpuMu   sync.Mutex
	cpuPrev map[int]cpuSnapshot

	// Cached system total memory in bytes (refreshed periodically).
	totalMem atomic.Int64
}

// clockTicksPerSecond returns the system clock ticks per second (Linux only).
// Falls back to 100 (typical Linux default) if sysconf is unavailable.
var clockTicksPerSecond = getClockTicks()

// NewCollector creates a new metrics collector.
func NewCollector() *Collector {
	c := &Collector{
		metrics: make(map[int]ProcessMetrics),
		stop:    make(chan struct{}),
		cpuPrev: make(map[int]cpuSnapshot),
	}
	// Initialize system total memory.
	c.refreshTotalMemory()
	return c
}

// Start begins periodic metric collection at the given interval.
func (c *Collector) Start(interval time.Duration) {
	// Refresh total memory before first collection.
	c.refreshTotalMemory()

	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-c.stop:
				ticker.Stop()
				return
			case <-ticker.C:
				c.CollectAll()
			}
		}
	}()
}

// Stop stops the collector.
func (c *Collector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stop)
	})
}

// Get returns the latest metrics for a PID.
func (c *Collector) Get(pid int) (ProcessMetrics, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m, ok := c.metrics[pid]
	return m, ok
}

// GetAll returns metrics for all tracked PIDs.
func (c *Collector) GetAll() map[int]ProcessMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[int]ProcessMetrics, len(c.metrics))
	for k, v := range c.metrics {
		result[k] = v
	}
	return result
}

// Track adds a PID to the metrics tracking set.
func (c *Collector) Track(pid int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.metrics[pid]; !ok {
		c.metrics[pid] = ProcessMetrics{PID: pid}
	}
}

// Untrack removes a PID from the metrics tracking set.
func (c *Collector) Untrack(pid int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.metrics, pid)

	// Clean up CPU delta tracking too.
	c.cpuMu.Lock()
	delete(c.cpuPrev, pid)
	c.cpuMu.Unlock()
}

// TotalMemory returns the cached system total memory in bytes.
func (c *Collector) TotalMemory() int64 {
	return c.totalMem.Load()
}

// CollectAll gathers metrics for all tracked PIDs.
func (c *Collector) CollectAll() {
	c.mu.RLock()
	if len(c.metrics) == 0 {
		c.mu.RUnlock()
		return
	}
	pids := make([]int, 0, len(c.metrics))
	for pid := range c.metrics {
		pids = append(pids, pid)
	}
	c.mu.RUnlock()

	// Refresh total memory periodically (cheap read).
	c.refreshTotalMemory()

	// Collect all metrics without the lock.
	now := time.Now()
	totalMem := c.totalMem.Load()
	numCPU := runtime.NumCPU()

	results := make([]ProcessMetrics, len(pids))
	rawTicks := make([]int64, len(pids))
	for i, pid := range pids {
		m, ticks := collectProcessMetrics(pid)
		results[i] = m
		rawTicks[i] = ticks
	}

	// Compute CPU% from delta and MemPercent.
	c.cpuMu.Lock()
	for i, m := range results {
		m.CPUPercent = c.computeCPUDelta(m.PID, rawTicks[i], now)
		if totalMem > 0 && m.MemBytes > 0 {
			m.MemPercent = float64(m.MemBytes) / float64(totalMem) * 100.0
		}
		results[i] = m
	}
	c.cpuMu.Unlock()

	// Single write-lock to update all metrics at once.
	c.mu.Lock()
	for _, m := range results {
		// Prune PIDs whose /proc dir no longer exists (process exited
		// without an explicit Untrack call).
		if m.MemBytes == 0 && m.Threads == 0 && m.FDs == 0 {
			procPath := filepath.Join("/proc", strconv.Itoa(m.PID))
			if _, err := os.Stat(procPath); err != nil {
				delete(c.metrics, m.PID)
				// Clean up CPU delta too.
				c.cpuMu.Lock()
				delete(c.cpuPrev, m.PID)
				c.cpuMu.Unlock()
				continue
			}
		}
		c.metrics[m.PID] = m
	}
	c.mu.Unlock()

	// Suppress unused warning.
	_ = numCPU
}

// computeCPUDelta calculates CPU% from the tick delta since the last collection.
// Must be called while holding c.cpuMu.
func (c *Collector) computeCPUDelta(pid int, totalTicks int64, now time.Time) float64 {
	if totalTicks == 0 {
		return 0
	}

	ticksPerSec := int64(clockTicksPerSecond)
	if ticksPerSec <= 0 {
		ticksPerSec = 100
	}
	numCPU := runtime.NumCPU()
	if numCPU <= 0 {
		numCPU = 1
	}

	prev, hasPrev := c.cpuPrev[pid]
	c.cpuPrev[pid] = cpuSnapshot{totalTicks: totalTicks, createdAt: now}

	if !hasPrev {
		return 0
	}

	tickDelta := totalTicks - prev.totalTicks
	if tickDelta <= 0 {
		return 0
	}

	timeDelta := now.Sub(prev.createdAt)
	if timeDelta <= 0 {
		return 0
	}

	// CPU% = (ticks_delta / ticks_per_second) / time_delta_seconds * 100 * numCPU
	// This gives the percentage across all CPUs (can exceed 100% for multi-threaded).
	cpuPercent := (float64(tickDelta) / float64(ticksPerSec)) / timeDelta.Seconds() * 100.0

	// Cap at reasonable maximum.
	if cpuPercent > float64(numCPU)*100.0 {
		cpuPercent = float64(numCPU) * 100.0
	}

	return cpuPercent
}

// collectProcessMetrics reads /proc/[pid]/stat for Linux process metrics.
// Returns the metrics struct and the raw total CPU ticks for delta computation.
func collectProcessMetrics(pid int) (ProcessMetrics, int64) {
	m := ProcessMetrics{
		PID:         pid,
		CollectedAt: time.Now(),
	}

	if runtime.GOOS != "linux" {
		return m, 0
	}

	// Read /proc/[pid]/stat for CPU and memory info.
	statPath := filepath.Join("/proc", strconv.Itoa(pid), "stat")
	data, err := os.ReadFile(statPath)
	if err != nil {
		return m, 0
	}

	// The comm field (field 2) is enclosed in parentheses and may contain
	// spaces (e.g., "5762 (go test -v -race) S ..."). Find the last ')'
	// and split from there to get correct field indices.
	line := string(data)
	closeParen := strings.LastIndex(line, ")")
	if closeParen < 0 {
		return m, 0
	}

	fields := strings.Fields(line[closeParen+1:])
	// fields[0] = state, fields[1] = ppid, ... fields[11] = utime, fields[12] = stime, ...
	// Original fields 3-24 map to fields[0]-[21].
	if len(fields) < 22 {
		return m, 0
	}

	// utime (field 14, index 11 after comm) and stime (field 15, index 12) in clock ticks.
	utime, _ := strconv.ParseInt(fields[11], 10, 64)
	stime, _ := strconv.ParseInt(fields[12], 10, 64)
	totalTicks := utime + stime

	// RSS pages (field 24, index 21 after comm).
	rssPages, _ := strconv.ParseInt(fields[21], 10, 64)
	pageSize := int64(os.Getpagesize())
	m.MemBytes = rssPages * pageSize

	// Number of threads (field 20, index 17 after comm).
	m.Threads, _ = strconv.Atoi(fields[17])

	// Count file descriptors via /proc/[pid]/fd.
	fdPath := filepath.Join("/proc", strconv.Itoa(pid), "fd")
	if entries, err := os.ReadDir(fdPath); err == nil {
		m.FDs = len(entries)
	}

	return m, totalTicks
}

// refreshTotalMemory reads system total memory from /proc/meminfo (Linux)
// or returns 0 on other platforms.
func (c *Collector) refreshTotalMemory() {
	if runtime.GOOS != "linux" {
		return
	}

	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return
	}

	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSuffix(parts[0], ":")
		if key == "MemTotal" {
			val, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				return
			}
			// Convert from kB to bytes.
			c.totalMem.Store(val * 1024)
			return
		}
	}
}

// getClockTicks returns the system clock ticks per second.
// The value is 100 on virtually all Linux systems (USER_HZ).
// CGO is prohibited (CGO_ENABLED=0), so we use the standard value.
func getClockTicks() int64 {
	return 100
}

// SystemMetrics holds system-wide resource metrics.
type SystemMetrics struct {
	TotalMemory  int64   `json:"total_memory"`
	UsedMemory   int64   `json:"used_memory"`
	FreeMemory   int64   `json:"free_memory"`
	CPULoad1     float64 `json:"cpu_load_1"`
	CPULoad5     float64 `json:"cpu_load_5"`
	CPULoad15    float64 `json:"cpu_load_15"`
	Uptime       int64   `json:"uptime_seconds"`
	ProcessCount int     `json:"process_count"`
}
