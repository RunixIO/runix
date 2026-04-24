package output

import "testing"

func TestColorDisabled(t *testing.T) {
	SetColorEnabled(false)

	if ColorEnabled() {
		t.Fatal("expected color disabled")
	}
	if got := Sprint(Green, "hello"); got != "hello" {
		t.Errorf("Sprint with color off = %q, want %q", got, "hello")
	}
	if got := BoldSprint("x"); got != "x" {
		t.Errorf("BoldSprint with color off = %q, want %q", got, "x")
	}
	if got := HeaderSprint("H"); got != "H" {
		t.Errorf("HeaderSprint with color off = %q, want %q", got, "H")
	}
}

func TestColorEnabled(t *testing.T) {
	SetColorEnabled(true)

	if !ColorEnabled() {
		t.Fatal("expected color enabled")
	}
	if got := Sprint(Green, "hi"); got != Green+"hi"+Reset {
		t.Errorf("Sprint with color on = %q", got)
	}
	if got := BoldSprint("x"); got != Bold+"x"+Reset {
		t.Errorf("BoldSprint = %q", got)
	}
	if got := HeaderSprint("H"); got != Bold+Cyan+"H"+Reset {
		t.Errorf("HeaderSprint = %q", got)
	}
}

func TestStatusSprint(t *testing.T) {
	SetColorEnabled(true)
	tests := []struct {
		state string
		want  string
	}{
		{"online", Green + "online" + Reset},
		{"launching", Green + "launching" + Reset},
		{"errored", Red + "errored" + Reset},
		{"stopped", Dim + "stopped" + Reset},
		{"stopping", Yellow + "stopping" + Reset},
		{"waiting", Yellow + "waiting" + Reset},
		{"unknown", "unknown"},
	}
	for _, tt := range tests {
		got := StatusSprint(tt.state)
		if got != tt.want {
			t.Errorf("StatusSprint(%q) = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestStatusSprintNoColor(t *testing.T) {
	SetColorEnabled(false)
	if got := StatusSprint("online"); got != "online" {
		t.Errorf("StatusSprint no color = %q, want %q", got, "online")
	}
}

func TestEnabledSprint(t *testing.T) {
	SetColorEnabled(true)
	if got := EnabledSprint(true); got != Green+"yes"+Reset {
		t.Errorf("EnabledSprint(true) = %q", got)
	}
	if got := EnabledSprint(false); got != Red+"no"+Reset {
		t.Errorf("EnabledSprint(false) = %q", got)
	}

	SetColorEnabled(false)
	if got := EnabledSprint(true); got != "yes" {
		t.Errorf("EnabledSprint(true) no color = %q", got)
	}
	if got := EnabledSprint(false); got != "no" {
		t.Errorf("EnabledSprint(false) no color = %q", got)
	}
}

func TestInitColor(t *testing.T) {
	tests := []struct {
		name       string
		isTerminal bool
		noColor    bool
		noColorEnv bool
		want       bool
	}{
		{"terminal, no flags", true, false, false, true},
		{"not terminal", false, false, false, false},
		{"no-color flag", true, true, false, false},
		{"NO_COLOR env", true, false, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.noColorEnv {
				t.Setenv("NO_COLOR", "1")
			}
			InitColor(tt.isTerminal, tt.noColor)
			if ColorEnabled() != tt.want {
				t.Errorf("InitColor(%v,%v) enabled=%v, want %v", tt.isTerminal, tt.noColor, ColorEnabled(), tt.want)
			}
		})
	}
}
