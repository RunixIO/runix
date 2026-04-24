package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectGo(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	d := NewDetector()
	rt, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if rt.Name() != "go" {
		t.Errorf("Detected runtime = %q, want %q", rt.Name(), "go")
	}
}

func TestDetectPython(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	d := NewDetector()
	rt, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if rt.Name() != "python" {
		t.Errorf("Detected runtime = %q, want %q", rt.Name(), "python")
	}
}

func TestDetectNode(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	d := NewDetector()
	rt, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if rt.Name() != "node" {
		t.Errorf("Detected runtime = %q, want %q", rt.Name(), "node")
	}
}

func TestDetectBun(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bun.lockb"), []byte("bun"), 0o644); err != nil {
		t.Fatal(err)
	}

	d := NewDetector()
	rt, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if rt.Name() != "bun" {
		t.Errorf("Detected runtime = %q, want %q", rt.Name(), "bun")
	}
}

func TestDetectEmpty(t *testing.T) {
	dir := t.TempDir()

	d := NewDetector()
	_, err := d.Detect(dir)
	if err == nil {
		t.Error("Detect() on empty dir should return error")
	}
}

func TestDetectRuby(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Gemfile"), []byte("source 'https://rubygems.org'\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	d := NewDetector()
	rt, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if rt.Name() != "ruby" {
		t.Errorf("Detected runtime = %q, want %q", rt.Name(), "ruby")
	}
}

func TestDetectPHP(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "composer.json"), []byte(`{"require":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	d := NewDetector()
	rt, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if rt.Name() != "php" {
		t.Errorf("Detected runtime = %q, want %q", rt.Name(), "php")
	}
}

func TestGetByName(t *testing.T) {
	d := NewDetector()

	rt, err := d.Get("go")
	if err != nil || rt.Name() != "go" {
		t.Errorf("Get(go) = %v, %v", rt, err)
	}

	rt, err = d.Get("python")
	if err != nil || rt.Name() != "python" {
		t.Errorf("Get(python) = %v, %v", rt, err)
	}

	rt, err = d.Get("deno")
	if err != nil || rt.Name() != "deno" {
		t.Errorf("Get(deno) = %v, %v", rt, err)
	}

	rt, err = d.Get("ruby")
	if err != nil || rt.Name() != "ruby" {
		t.Errorf("Get(ruby) = %v, %v", rt, err)
	}

	rt, err = d.Get("php")
	if err != nil || rt.Name() != "php" {
		t.Errorf("Get(php) = %v, %v", rt, err)
	}

	_, err = d.Get("nonexistent")
	if err == nil {
		t.Error("Get(nonexistent) should return error")
	}
}
