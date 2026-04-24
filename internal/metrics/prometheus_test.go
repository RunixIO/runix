package metrics

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/runixio/runix/pkg/types"
)

func TestWritePrometheus(t *testing.T) {
	now := time.Now()
	procs := []types.ProcessInfo{
		{
			Name:      "api",
			Namespace: "backend",
			State:     types.StateRunning,
			PID:       1234,
			Restarts:  2,
			StartedAt: &now,
			MemBytes:  1024000,
		},
		{
			Name:     "worker",
			State:    types.StateStopped,
			Restarts: 0,
		},
	}

	var buf bytes.Buffer
	WritePrometheus(&buf, procs)

	output := buf.String()

	if !strings.Contains(output, `runix_process_count{state="running"} 1`) {
		t.Error("expected running state count")
	}
	if !strings.Contains(output, `runix_process_count{state="stopped"} 1`) {
		t.Error("expected stopped state count")
	}
	if !strings.Contains(output, `runix_process_restarts_total`) {
		t.Error("expected restarts metric")
	}
	if !strings.Contains(output, `runix_process_uptime_seconds`) {
		t.Error("expected uptime metric")
	}
	if !strings.Contains(output, `runix_process_memory_bytes`) {
		t.Error("expected memory metric")
	}
	if !strings.Contains(output, `runix_process_count_total 2`) {
		t.Error("expected total count")
	}
}

func TestWritePrometheusEmpty(t *testing.T) {
	var buf bytes.Buffer
	WritePrometheus(&buf, nil)

	output := buf.String()
	if !strings.Contains(output, `runix_process_count_total 0`) {
		t.Error("expected total count 0")
	}
}
