package hooks

import (
	"strings"
	"testing"
)

func TestBoundedTailBufferKeepsTail(t *testing.T) {
	buf := newBoundedTailBuffer(8)

	if _, err := buf.Write([]byte("hello")); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if _, err := buf.Write([]byte("-world")); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if got := buf.String(); got != "lo-world" {
		t.Fatalf("String() = %q, want %q", got, "lo-world")
	}
	if buf.total != len("hello-world") {
		t.Fatalf("total = %d, want %d", buf.total, len("hello-world"))
	}
}

func TestBoundedTailBufferHandlesLargeChunk(t *testing.T) {
	buf := newBoundedTailBuffer(4)

	if _, err := buf.Write([]byte("abcdefgh")); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if got := buf.String(); got != "efgh" {
		t.Fatalf("String() = %q, want %q", got, "efgh")
	}
	if !strings.HasSuffix("abcdefgh", buf.String()) {
		t.Fatalf("expected retained buffer to be the tail")
	}
}
