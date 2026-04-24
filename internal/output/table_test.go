package output

import (
	"strings"
	"testing"
)

func TestTableBasic(t *testing.T) {
	tbl := NewTable("ID", "NAME", "STATUS")
	tbl.AddRow("1", "web", "online")
	tbl.AddRow("2", "api", "errored")

	got := tbl.Render()

	// Should have 5 lines: top border, header, separator, 2 data rows, bottom border.
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 6 {
		t.Fatalf("expected 6 lines, got %d:\n%s", len(lines), got)
	}

	// Check borders.
	if !strings.HasPrefix(lines[0], "┌") || !strings.HasSuffix(lines[0], "┐") {
		t.Errorf("top border wrong: %q", lines[0])
	}
	if !strings.HasPrefix(lines[3], "│") || !strings.HasSuffix(lines[3], "│") {
		t.Errorf("data row missing borders: %q", lines[3])
	}
	if !strings.HasPrefix(lines[5], "└") || !strings.HasSuffix(lines[5], "┘") {
		t.Errorf("bottom border wrong: %q", lines[5])
	}

	// Check header contains headers.
	if !strings.Contains(lines[1], "ID") || !strings.Contains(lines[1], "NAME") {
		t.Errorf("header row wrong: %q", lines[1])
	}

	// Check data rows contain values.
	if !strings.Contains(lines[3], "web") || !strings.Contains(lines[3], "online") {
		t.Errorf("first data row wrong: %q", lines[3])
	}
	if !strings.Contains(lines[4], "api") || !strings.Contains(lines[4], "errored") {
		t.Errorf("second data row wrong: %q", lines[4])
	}
}

func TestTableAlignment(t *testing.T) {
	SetColorEnabled(false)
	defer SetColorEnabled(false)

	tbl := NewTable("ID", "NAME")
	tbl.SetAlign(0, AlignRight)
	tbl.AddRow("1", "alpha")

	got := tbl.Render()
	lines := strings.Split(strings.TrimSpace(got), "\n")

	// Data row (line 3): right-aligned ID should have padding before the value.
	dataRow := lines[3]
	if !strings.Contains(dataRow, " 1 ") {
		t.Errorf("right-aligned row: %q", dataRow)
	}
}

func TestTableEmpty(t *testing.T) {
	tbl := NewTable()
	if got := tbl.Render(); got != "" {
		t.Errorf("empty table should render empty string, got %q", got)
	}
}

func TestTableRowMoreValsThanHeaders(t *testing.T) {
	SetColorEnabled(false)
	defer SetColorEnabled(false)

	tbl := NewTable("A", "B")
	tbl.AddRow("1", "2", "3") // extra value ignored

	got := tbl.Render()
	if strings.Contains(got, "3") {
		t.Errorf("extra value should be ignored:\n%s", got)
	}
}

func TestTableFewerValsThanHeaders(t *testing.T) {
	SetColorEnabled(false)
	defer SetColorEnabled(false)

	tbl := NewTable("A", "B", "C")
	tbl.AddRow("1") // missing B and C

	got := tbl.Render()
	lines := strings.Split(strings.TrimSpace(got), "\n")
	dataRow := lines[3]
	if !strings.Contains(dataRow, "1") {
		t.Errorf("row with missing vals: %q", dataRow)
	}
}

func TestVisualWidth(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"hello", 5},
		{"", 0},
		{"abc", 3},
		{"\033[32monline\033[0m", 6}, // ANSI-wrapped "online"
		{"\033[1m\033[36mID\033[0m", 2},
	}
	for _, tt := range tests {
		got := visualWidth(tt.input)
		if got != tt.want {
			t.Errorf("visualWidth(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestTableWithColor(t *testing.T) {
	SetColorEnabled(true)
	defer SetColorEnabled(false)

	tbl := NewTable("ID", "NAME")
	tbl.AddRow("1", "web")

	got := tbl.Render()
	// Header row should contain ANSI codes.
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if !strings.Contains(lines[1], "\033[") {
		t.Errorf("header should have ANSI codes when color enabled:\n%s", lines[1])
	}
	// Data row should not have ANSI (no status coloring applied by table itself).
	if strings.Contains(lines[3], "\033[") {
		t.Errorf("plain data row should not have ANSI codes:\n%s", lines[3])
	}
}

func TestTableConsistentWidth(t *testing.T) {
	SetColorEnabled(false)
	defer SetColorEnabled(false)

	tbl := NewTable("NAME", "STATUS")
	tbl.AddRow("short", "ok")
	tbl.AddRow("verylongname", "ok")

	got := tbl.Render()
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
