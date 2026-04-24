package logrot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewRotator(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	r, err := NewRotator(path, 100, 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()

	if _, err := r.Write([]byte("hello\n")); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello\n" {
		t.Fatalf("unexpected content: %q", string(data))
	}
}

func TestRotationTrigger(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// Max size of 10 bytes.
	r, err := NewRotator(path, 10, 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()

	// Write enough to trigger rotation.
	for i := 0; i < 20; i++ {
		if _, err := r.Write([]byte("hello world\n")); err != nil {
			t.Fatal(err)
		}
	}

	// Should have rotated files.
	entries, _ := os.ReadDir(dir)
	rotated := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "test.log.") {
			rotated++
		}
	}
	if rotated == 0 {
		t.Fatal("expected rotated files")
	}
}

func TestMaxFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// Max 2 rotated files, max size 5 bytes.
	r, err := NewRotator(path, 5, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()

	// Trigger many rotations.
	for i := 0; i < 50; i++ {
		if _, err := r.Write([]byte("data\n")); err != nil {
			t.Fatal(err)
		}
	}

	entries, _ := os.ReadDir(dir)
	rotated := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "test.log.") {
			rotated++
		}
	}
	if rotated > 2 {
		t.Fatalf("expected at most 2 rotated files, got %d", rotated)
	}
}

func TestMaxAge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// Max age 1 nanosecond (will expire immediately).
	r, err := NewRotator(path, 5, 10, time.Nanosecond)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()

	// Trigger rotation.
	for i := 0; i < 50; i++ {
		if _, err := r.Write([]byte("data\n")); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}

	// Old files should be cleaned up.
	entries, _ := os.ReadDir(dir)
	rotated := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "test.log.") {
			rotated++
		}
	}
	// With nanosecond max age, most rotated files should be cleaned.
	if rotated > 5 {
		t.Fatalf("expected most rotated files cleaned, got %d", rotated)
	}
}
