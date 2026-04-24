package tui

import (
	"testing"
	"time"

	"github.com/runixio/runix/pkg/types"
)

func TestSameProcessList(t *testing.T) {
	startedAt := time.Unix(100, 0)
	a := []types.ProcessInfo{
		{
			ID:         "abc",
			NumericID:  1,
			Name:       "api",
			Runtime:    "go",
			State:      types.StateRunning,
			PID:        42,
			CPUPercent: 1.5,
			MemBytes:   1024,
			StartedAt:  &startedAt,
		},
	}
	b := []types.ProcessInfo{
		{
			ID:         "abc",
			NumericID:  1,
			Name:       "api",
			Runtime:    "go",
			State:      types.StateRunning,
			PID:        42,
			CPUPercent: 1.5,
			MemBytes:   1024,
			StartedAt:  &startedAt,
		},
	}

	if !sameProcessList(a, b) {
		t.Fatal("expected process lists to match")
	}

	b[0].MemBytes = 2048
	if sameProcessList(a, b) {
		t.Fatal("expected process lists to differ")
	}
}
