package output

import (
	"strings"
)

// KeyValue renders a bordered key-value property card.
type KeyValue struct {
	pairs    []kv
	keyWidth int
}

type kv struct {
	key   string
	value string
}

// NewKeyValue creates a new bordered key-value renderer.
func NewKeyValue() *KeyValue {
	return &KeyValue{}
}

// Add appends a key-value pair.
func (k *KeyValue) Add(key, value string) {
	k.pairs = append(k.pairs, kv{key: key, value: value})
	w := visualWidth(key)
	if w > k.keyWidth {
		k.keyWidth = w
	}
}

// Render returns the bordered key-value card as a string.
func (k *KeyValue) Render() string {
	if len(k.pairs) == 0 {
		return ""
	}

	// Compute value column width.
	valWidth := 0
	for _, p := range k.pairs {
		w := visualWidth(p.value)
		if w > valWidth {
			valWidth = w
		}
	}

	var b strings.Builder

	// Top border: ┌──────┬──────────┐
	k.writeBorder(&b, "┌", "┬", "┐", k.keyWidth, valWidth)

	// Key-value rows.
	for _, p := range k.pairs {
		keyPad := k.keyWidth - visualWidth(p.key)
		valPad := valWidth - visualWidth(p.value)
		b.WriteString("│ ")
		b.WriteString(p.key)
		b.WriteString(strings.Repeat(" ", keyPad))
		b.WriteString(" │ ")
		b.WriteString(p.value)
		b.WriteString(strings.Repeat(" ", valPad))
		b.WriteString(" │\n")
	}

	// Bottom border: └──────┴──────────┘
	k.writeBorder(&b, "└", "┴", "┘", k.keyWidth, valWidth)

	return b.String()
}

func (k *KeyValue) writeBorder(b *strings.Builder, left, mid, right string, keyW, valW int) {
	b.WriteString(left)
	b.WriteString(strings.Repeat("─", keyW+2)) // +2 for inner padding
	b.WriteString(mid)
	b.WriteString(strings.Repeat("─", valW+2))
	b.WriteString(right)
	b.WriteByte('\n')
}

// String implements fmt.Stringer.
func (k *KeyValue) String() string {
	return k.Render()
}
