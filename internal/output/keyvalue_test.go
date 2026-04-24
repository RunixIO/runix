package output

import (
	"strings"
	"testing"
)

func TestKeyValueBasic(t *testing.T) {
	SetColorEnabled(false)
	defer SetColorEnabled(false)

	kv := NewKeyValue()
	kv.Add("Name", "web")
	kv.Add("PID", "48291")
	kv.Add("Status", "online")

	got := kv.Render()
	lines := strings.Split(strings.TrimSpace(got), "\n")

	// 5 lines: top border, 3 pairs, bottom border.
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d:\n%s", len(lines), got)
	}

	if !strings.HasPrefix(lines[0], "┌") {
		t.Errorf("top border wrong: %q", lines[0])
	}
	if !strings.HasPrefix(lines[4], "└") {
		t.Errorf("bottom border wrong: %q", lines[4])
	}
	if !strings.Contains(lines[1], "Name") || !strings.Contains(lines[1], "web") {
		t.Errorf("first row wrong: %q", lines[1])
	}
	if !strings.Contains(lines[3], "Status") || !strings.Contains(lines[3], "online") {
		t.Errorf("third row wrong: %q", lines[3])
	}
}

func TestKeyValueEmpty(t *testing.T) {
	SetColorEnabled(false)
	kv := NewKeyValue()
	if got := kv.Render(); got != "" {
		t.Errorf("empty KeyValue should render empty string, got %q", got)
	}
}

func TestKeyValueConsistentWidth(t *testing.T) {
	SetColorEnabled(false)
	defer SetColorEnabled(false)

	kv := NewKeyValue()
	kv.Add("Short", "val1")
	kv.Add("VeryLongKey", "val2")

	got := kv.Render()
	lines := strings.Split(strings.TrimSpace(got), "\n")

	// All lines should have the same visual width.
	width := visualWidth(lines[0])
	for i, line := range lines {
		w := visualWidth(line)
		if w != width {
			t.Errorf("line %d visual width %d != expected %d: %q", i, w, width, line)
		}
	}
}

func TestKeyValueWithANSI(t *testing.T) {
	SetColorEnabled(true)
	defer SetColorEnabled(false)

	kv := NewKeyValue()
	kv.Add("Status", StatusSprint("online"))

	got := kv.Render()
	lines := strings.Split(strings.TrimSpace(got), "\n")

	// All lines should have the same visual width (ANSI codes don't count).
	width := visualWidth(lines[0])
	for i, line := range lines {
		w := visualWidth(line)
		if w != width {
			t.Errorf("line %d visual width %d != expected %d: %q", i, w, width, line)
		}
	}
}
