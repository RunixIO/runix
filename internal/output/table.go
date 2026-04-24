package output

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Alignment controls how a column's values are aligned.
type Alignment int

const (
	AlignLeft Alignment = iota
	AlignRight
)

// Table renders data rows as a Unicode box-drawing table.
type Table struct {
	headers    []string
	rows       [][]string
	alignments []Alignment
}

// NewTable creates a table with the given column headers.
func NewTable(headers ...string) *Table {
	return &Table{
		headers:    headers,
		alignments: make([]Alignment, len(headers)),
	}
}

// SetAlign sets the alignment for the column at zero-based index i.
func (t *Table) SetAlign(i int, a Alignment) {
	if i >= 0 && i < len(t.alignments) {
		t.alignments[i] = a
	}
}

// AddRow appends a row of values. Values beyond the header count are ignored.
func (t *Table) AddRow(vals ...string) {
	row := make([]string, len(t.headers))
	for i := 0; i < len(t.headers) && i < len(vals); i++ {
		row[i] = vals[i]
	}
	t.rows = append(t.rows, row)
}

// Render returns the full table as a string with box-drawing borders.
// Header cells are styled with HeaderSprint when color is enabled.
func (t *Table) Render() string {
	if len(t.headers) == 0 {
		return ""
	}

	widths := t.computeWidths()
	var b strings.Builder

	// Top border.
	t.writeBorder(&b, "┌", "┬", "┐", "─", widths)

	// Header row.
	t.writeRow(&b, t.headers, widths, true)

	// Separator.
	t.writeBorder(&b, "├", "┼", "┤", "─", widths)

	// Data rows.
	for _, row := range t.rows {
		t.writeRow(&b, row, widths, false)
	}

	// Bottom border.
	t.writeBorder(&b, "└", "┴", "┘", "─", widths)

	return b.String()
}

// computeWidths returns the maximum visual width for each column.
func (t *Table) computeWidths() []int {
	widths := make([]int, len(t.headers))
	for i, h := range t.headers {
		widths[i] = visualWidth(h)
	}
	for _, row := range t.rows {
		for i, val := range row {
			if i < len(widths) {
				w := visualWidth(val)
				if w > widths[i] {
					widths[i] = w
				}
			}
		}
	}
	return widths
}

// writeBorder writes a horizontal border line.
func (t *Table) writeBorder(b *strings.Builder, left, mid, right, fill string, widths []int) {
	b.WriteString(left)
	for i, w := range widths {
		if i > 0 {
			b.WriteString(mid)
		}
		b.WriteString(strings.Repeat(fill, w+2)) // +2 for inner padding
	}
	b.WriteString(right)
	b.WriteByte('\n')
}

// writeRow writes a data or header row.
func (t *Table) writeRow(b *strings.Builder, vals []string, widths []int, isHeader bool) {
	b.WriteString("│")
	for i, w := range widths {
		val := ""
		if i < len(vals) {
			val = vals[i]
		}

		// Strip ANSI codes for width calculation.
		display := val
		if isHeader {
			display = HeaderSprint(val)
		}

		pad := w - visualWidth(val)
		align := AlignLeft
		if i < len(t.alignments) {
			align = t.alignments[i]
		}

		b.WriteByte(' ')
		if align == AlignRight {
			b.WriteString(strings.Repeat(" ", pad))
			b.WriteString(display)
		} else {
			b.WriteString(display)
			b.WriteString(strings.Repeat(" ", pad))
		}
		b.WriteString(" │")
	}
	b.WriteByte('\n')
}

// visualWidth returns the visible width of s, stripping ANSI escape sequences.
func visualWidth(s string) int {
	w := 0
	i := 0
	for i < len(s) {
		if s[i] == '\033' {
			// Skip ANSI escape sequence: \033[ ... m
			i++
			if i < len(s) && s[i] == '[' {
				i++
				for i < len(s) && s[i] != 'm' {
					i++
				}
				if i < len(s) {
					i++ // skip 'm'
				}
			}
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		w++
		i += size
	}
	return w
}

// String implements fmt.Stringer.
func (t *Table) String() string {
	return t.Render()
}

// Sprintf is a convenience for formatting a value into a string.
// This avoids cmd files needing to import fmt just for Sprintf.
func Sprintf(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}
