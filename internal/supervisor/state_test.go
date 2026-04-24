package supervisor

import (
	"testing"

	"github.com/runixio/runix/pkg/types"
)

func TestCanTransition(t *testing.T) {
	tests := []struct {
		from     types.ProcessState
		to       types.ProcessState
		expected bool
	}{
		{types.StateStopped, types.StateStarting, true},
		{types.StateStarting, types.StateRunning, true},
		{types.StateStarting, types.StateStopping, true},
		{types.StateRunning, types.StateStopping, true},
		{types.StateStopping, types.StateStopped, true},
		{types.StateRunning, types.StateCrashed, true},
		{types.StateCrashed, types.StateWaiting, true},
		{types.StateWaiting, types.StateStarting, true},
		{types.StateStopped, types.StateRunning, false},
		{types.StateRunning, types.StateStarting, false},
		{types.StateStopped, types.StateStopped, false},
	}

	for _, tt := range tests {
		got := CanTransition(tt.from, tt.to)
		if got != tt.expected {
			t.Errorf("CanTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.expected)
		}
	}
}

func TestIsValidState(t *testing.T) {
	valid := []types.ProcessState{
		types.StateStarting, types.StateRunning, types.StateStopping,
		types.StateStopped, types.StateCrashed, types.StateErrored, types.StateWaiting,
	}
	for _, s := range valid {
		if !IsValidState(s) {
			t.Errorf("IsValidState(%s) = false, want true", s)
		}
	}
	if IsValidState("invalid") {
		t.Error("IsValidState(invalid) = true, want false")
	}
}
