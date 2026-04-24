package web

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/runixio/runix/pkg/types"
)

func TestProcessListMessageOmitsEmbeddedConfig(t *testing.T) {
	startedAt := time.Unix(1_700_000_000, 0).UTC()
	procs := []types.ProcessInfo{
		{
			ID:         "12345678-1234-1234-1234-123456789abc",
			NumericID:  7,
			Name:       "api",
			Runtime:    "go",
			State:      types.StateRunning,
			PID:        4242,
			Restarts:   3,
			StartedAt:  &startedAt,
			CPUPercent: 12.5,
			MemBytes:   64 * 1024 * 1024,
			Config: types.ProcessConfig{
				Name:       "api",
				Entrypoint: "./server",
				Env: map[string]string{
					"SECRET": "should-not-be-broadcast",
				},
			},
		},
	}

	msg := ProcessListMessage{
		Type:      "process_list",
		Processes: summarizeProcesses(procs),
		Timestamp: time.Now().Unix(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	got := string(data)
	if strings.Contains(got, "should-not-be-broadcast") {
		t.Fatalf("websocket payload leaked process config: %s", got)
	}
	if strings.Contains(got, "\"config\"") {
		t.Fatalf("websocket payload should not include embedded config: %s", got)
	}
	if !strings.Contains(got, "\"cpu_percent\":12.5") {
		t.Fatalf("websocket payload missing cpu metric: %s", got)
	}
	if !strings.Contains(got, "\"memory_bytes\":67108864") {
		t.Fatalf("websocket payload missing memory metric: %s", got)
	}
}
