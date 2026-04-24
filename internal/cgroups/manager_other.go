//go:build !linux

package cgroups

import "fmt"

type noopManager struct{}

func newManager() Manager {
	return &noopManager{}
}

func (m *noopManager) Apply(pid int, limits Limits) error {
	if limits.CPUQuota != "" || limits.MemoryLimit != "" {
		return fmt.Errorf("resource limits require cgroups v2 (Linux only)")
	}
	return nil
}

func (m *noopManager) Remove(pid int) error {
	return nil
}

func (m *noopManager) Stats(pid int) (float64, int64, error) {
	return 0, 0, fmt.Errorf("resource stats require cgroups v2 (Linux only)")
}
