package watcher

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatcherFileChange(t *testing.T) {
	dir := t.TempDir()
	triggered := make(chan []string, 1)

	w, err := New([]string{dir}, []string{".git"}, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	err = w.Start(func(paths []string) {
		select {
		case triggered <- paths:
		default:
		}
	})
	if err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}
	defer w.Stop()

	// Give the watcher time to initialize.
	time.Sleep(100 * time.Millisecond)

	// Write a file.
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Wait for the debounce handler to fire.
	select {
	case paths := <-triggered:
		if len(paths) == 0 {
			t.Error("expected at least one changed path")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for file change event")
	}
}

func TestWatcherIgnore(t *testing.T) {
	dir := t.TempDir()

	w, err := New([]string{dir}, []string{".git", "*.log"}, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	// Test that .git and .log files are ignored.
	if !w.isIgnored(filepath.Join(dir, ".git", "config")) {
		t.Error("expected .git path to be ignored")
	}
	if !w.isIgnored(filepath.Join(dir, "app.log")) {
		t.Error("expected *.log file to be ignored")
	}
	if w.isIgnored(filepath.Join(dir, "main.go")) {
		t.Error("expected main.go to NOT be ignored")
	}
}

func TestDebouncer(t *testing.T) {
	var count atomic.Int32
	results := make(chan []string, 1)

	d := NewDebouncer(50*time.Millisecond, func(events []string) {
		count.Add(1)
		select {
		case results <- events:
		default:
		}
	})

	// Add events rapidly.
	d.Add("file1.go")
	d.Add("file2.go")
	d.Add("file3.go")

	// Wait for the debounce window.
	select {
	case events := <-results:
		if len(events) != 3 {
			t.Errorf("expected 3 events, got %d", len(events))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for debounce handler")
	}

	if count.Load() != 1 {
		t.Errorf("expected handler called once, got %d", count.Load())
	}
}

func TestDebouncerStop(t *testing.T) {
	d := NewDebouncer(100*time.Millisecond, func(events []string) {})
	d.Add("file.go")
	d.Stop()
	// Should not panic.
}

func TestWatcherStop(t *testing.T) {
	dir := t.TempDir()
	w, err := New([]string{dir}, nil, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	if err := w.Start(func(paths []string) {}); err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}

	// Stop should not panic.
	w.Stop()

	// Double stop should be safe.
	w.Stop()
}
