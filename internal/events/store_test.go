package events

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreCompactsLargeLog(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	if store == nil {
		t.Fatal("expected non-nil store")
	}

	store.mu.Lock()
	path := filepath.Join(dir, "events.log")
	oversized := strings.Repeat(`{"bad":true}`+"\n", int(maxEventLogBytes/13)+10)
	if err := os.WriteFile(path, []byte(oversized), 0o644); err != nil {
		store.mu.Unlock()
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := store.compactLocked(); err != nil {
		store.mu.Unlock()
		t.Fatalf("compactLocked() error: %v", err)
	}
	store.mu.Unlock()

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if fi.Size() > maxEventLogBytes {
		t.Fatalf("compacted log size = %d, want <= %d", fi.Size(), maxEventLogBytes)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if len(data) > 0 && data[0] == '\n' {
		t.Fatal("expected compacted log to skip partial first line")
	}
}

func TestStoreQueryAfterCompaction(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	for i := 0; i < 5; i++ {
		store.Append(newEvent(ProcessStarted, "p1", "web", map[string]interface{}{"i": i}))
	}

	results := store.Query(time.Time{})
	if len(results) == 0 {
		t.Fatal("expected events after append/query")
	}
}
