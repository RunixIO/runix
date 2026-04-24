package scheduler

import "testing"

func TestBoundedTailBufferKeepsTail(t *testing.T) {
	buf := newBoundedTailBuffer(6)

	if _, err := buf.Write([]byte("1234")); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if _, err := buf.Write([]byte("56789")); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if got := buf.String(); got != "456789" {
		t.Fatalf("String() = %q, want %q", got, "456789")
	}
	if buf.total != 9 {
		t.Fatalf("total = %d, want %d", buf.total, 9)
	}
}
