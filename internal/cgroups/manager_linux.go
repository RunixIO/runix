//go:build linux

package cgroups

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const cgroupBase = "/sys/fs/cgroup"

type linuxManager struct{}

func newManager() Manager {
	return &linuxManager{}
}

func (m *linuxManager) Apply(pid int, limits Limits) error {
	if limits.CPUQuota == "" && limits.MemoryLimit == "" {
		return nil
	}

	groupPath := filepath.Join(cgroupBase, "runix", strconv.Itoa(pid))
	if err := os.MkdirAll(groupPath, 0o755); err != nil {
		return fmt.Errorf("failed to create cgroup %s: %w", groupPath, err)
	}

	// Add process to cgroup.
	if err := os.WriteFile(filepath.Join(groupPath, "cgroup.procs"), []byte(strconv.Itoa(pid)), 0o644); err != nil {
		return fmt.Errorf("failed to add pid %d to cgroup: %w", pid, err)
	}

	// Set CPU quota.
	if limits.CPUQuota != "" {
		quota := parseCPUQuota(limits.CPUQuota)
		// cpu.max format: "quota period" e.g. "50000 100000" for 50%.
		if quota > 0 {
			if err := os.WriteFile(filepath.Join(groupPath, "cpu.max"), []byte(fmt.Sprintf("%d 100000", quota)), 0o644); err != nil {
				return fmt.Errorf("failed to set cpu quota: %w", err)
			}
		}
	}

	// Set memory limit.
	if limits.MemoryLimit != "" {
		memBytes := parseMemoryLimit(limits.MemoryLimit)
		if memBytes > 0 {
			if err := os.WriteFile(filepath.Join(groupPath, "memory.max"), []byte(strconv.FormatInt(memBytes, 10)), 0o644); err != nil {
				return fmt.Errorf("failed to set memory limit: %w", err)
			}
		}
	}

	return nil
}

func (m *linuxManager) Remove(pid int) error {
	groupPath := filepath.Join(cgroupBase, "runix", strconv.Itoa(pid))
	return os.RemoveAll(groupPath)
}

func (m *linuxManager) Stats(pid int) (float64, int64, error) {
	groupPath := filepath.Join(cgroupBase, "runix", strconv.Itoa(pid))

	// Read memory current.
	memData, err := os.ReadFile(filepath.Join(groupPath, "memory.current"))
	if err != nil {
		return 0, 0, err
	}
	memBytes, _ := strconv.ParseInt(strings.TrimSpace(string(memData)), 10, 64)

	// CPU usage is more complex; return 0 for now.
	return 0, memBytes, nil
}
