package supervisor

import (
	"testing"

	"github.com/runixio/runix/pkg/types"
)

func TestInfoIncludesLastObservation(t *testing.T) {
	proc := NewManagedProcess(types.ProcessConfig{Name: "app", Entrypoint: "sleep"})
	proc.RecordObservation("restarted", "manual restart")

	info := proc.Info()
	if info.LastEvent != "restarted" {
		t.Fatalf("expected last event restarted, got %q", info.LastEvent)
	}
	if info.LastReason != "manual restart" {
		t.Fatalf("expected last reason manual restart, got %q", info.LastReason)
	}
}
