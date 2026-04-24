package cgroups

import (
	"strconv"
	"strings"
)

// Limits defines resource limits for a process.
type Limits struct {
	CPUQuota    string
	MemoryLimit string
}

// Manager manages cgroup-based resource limits.
type Manager interface {
	// Apply sets resource limits for a process.
	Apply(pid int, limits Limits) error
	// Remove cleans up the cgroup for a process.
	Remove(pid int) error
	// Stats returns current CPU and memory usage.
	Stats(pid int) (cpu float64, mem int64, err error)
}

// NewManager creates a new cgroup manager for the current platform.
func NewManager() Manager {
	return newManager()
}

func parseCPUQuota(s string) int64 {
	s = strings.TrimSuffix(s, "%")
	// Simple percentage parsing.
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	// Convert percentage to micro-seconds per 100ms period.
	return int64(val * 1000)
}

func parseMemoryLimit(s string) int64 {
	s = strings.ToUpper(s)
	mult := int64(1)
	switch {
	case strings.HasSuffix(s, "GB"):
		mult = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "GB")
	case strings.HasSuffix(s, "MB"):
		mult = 1024 * 1024
		s = strings.TrimSuffix(s, "MB")
	case strings.HasSuffix(s, "KB"):
		mult = 1024
		s = strings.TrimSuffix(s, "KB")
	}
	val, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return val * mult
}
