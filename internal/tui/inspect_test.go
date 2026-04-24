package tui

import (
	"testing"
	"time"

	"github.com/runixio/runix/pkg/types"
)

func TestShouldReloadLogsOnStateChange(t *testing.T) {
	model := NewInspectModel(nil, "api")
	model.lastLogLoad = time.Now()

	prev := types.ProcessInfo{Name: "api", State: types.StateRunning, PID: 10}
	next := prev

	if model.shouldReloadLogs(prev, next) {
		t.Fatal("expected unchanged process info to skip log reload")
	}

	next.Restarts = 1
	if !model.shouldReloadLogs(prev, next) {
		t.Fatal("expected restart change to trigger log reload")
	}
}

func TestShouldPeriodicLogRefresh(t *testing.T) {
	model := NewInspectModel(nil, "api")
	model.info = types.ProcessInfo{Name: "api", State: types.StateRunning}

	if !model.shouldPeriodicLogRefresh() {
		t.Fatal("expected first log refresh to run")
	}

	model.lastLogLoad = time.Now()
	if model.shouldPeriodicLogRefresh() {
		t.Fatal("expected recent log refresh to be skipped")
	}

	model.lastLogLoad = time.Now().Add(-model.logRefreshEvery)
	if !model.shouldPeriodicLogRefresh() {
		t.Fatal("expected stale running log refresh to run")
	}

	model.info.State = types.StateStopped
	if model.shouldPeriodicLogRefresh() {
		t.Fatal("expected stopped process to skip periodic log refresh")
	}
}
