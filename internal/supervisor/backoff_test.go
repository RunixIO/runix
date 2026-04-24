package supervisor

import (
	"testing"
	"time"
)

func TestBackoffNext(t *testing.T) {
	b := NewBackoff()

	// Attempt 0: base * 2^0 = 1s + jitter
	d0 := b.Next(0)
	if d0 < time.Second || d0 > 2*time.Second {
		t.Errorf("Next(0) = %v, want ~1s", d0)
	}

	// Attempt 3: base * 2^3 = 8s + jitter
	d3 := b.Next(3)
	if d3 < 8*time.Second || d3 > 9*time.Second {
		t.Errorf("Next(3) = %v, want ~8s", d3)
	}

	// High attempt should be capped at max (60s).
	dHigh := b.Next(20)
	if dHigh > 60*time.Second {
		t.Errorf("Next(20) = %v, want <= 60s", dHigh)
	}
}

func TestBackoffDefaults(t *testing.T) {
	b := NewBackoff()
	if b.Base != time.Second {
		t.Errorf("Base = %v, want 1s", b.Base)
	}
	if b.Max != 60*time.Second {
		t.Errorf("Max = %v, want 60s", b.Max)
	}
}
