package supervisor

import (
	"github.com/runixio/runix/pkg/types"
)

// CanTransition checks whether a transition from one state to another is allowed.
func CanTransition(from, to types.ProcessState) bool {
	allowed, ok := types.ValidTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// IsValidState reports whether s is a recognized ProcessState value.
func IsValidState(s types.ProcessState) bool {
	switch s {
	case types.StateStarting, types.StateRunning, types.StateStopping,
		types.StateStopped, types.StateCrashed, types.StateErrored,
		types.StateWaiting:
		return true
	}
	return false
}
