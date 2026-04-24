package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectPHPComposer(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "composer.json"), []byte(`{"require":{}}`), 0o644)

	d := NewDetector()
	rt, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if rt.Name() != "php" {
		t.Errorf("Detected runtime = %q, want %q", rt.Name(), "php")
	}
}

func TestDetectPHPArtisan(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "artisan"), []byte("#!/usr/bin/env php\n"), 0o644)

	d := NewDetector()
	rt, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if rt.Name() != "php" {
		t.Errorf("Detected runtime = %q, want %q", rt.Name(), "php")
	}
}

func TestDetectPHPFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "app.php"), []byte("<?php echo 'hello';\n"), 0o644)

	d := NewDetector()
	rt, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if rt.Name() != "php" {
		t.Errorf("Detected runtime = %q, want %q", rt.Name(), "php")
	}
}

func TestPHPStartCmd(t *testing.T) {
	rt := &PHPRuntime{}
	cmd, err := rt.StartCmd(StartOptions{
		Entrypoint: "app.php",
		Cwd:        "/app",
	})
	if err != nil {
		t.Fatalf("StartCmd() error: %v", err)
	}
	// Command should be: <php> app.php
	if len(cmd.Args) != 2 {
		t.Fatalf("cmd.Args = %v, want 2 elements", cmd.Args)
	}
	if cmd.Args[1] != "app.php" {
		t.Errorf("cmd.Args[1] = %q, want %q", cmd.Args[1], "app.php")
	}
	if cmd.Dir != "/app" {
		t.Errorf("cmd.Dir = %q, want %q", cmd.Dir, "/app")
	}
}

func TestPHPStartCmdWithArgs(t *testing.T) {
	rt := &PHPRuntime{}
	cmd, err := rt.StartCmd(StartOptions{
		Entrypoint: "artisan",
		Args:       []string{"serve", "--host=0.0.0.0", "--port=8000"},
		Cwd:        "/app",
	})
	if err != nil {
		t.Fatalf("StartCmd() error: %v", err)
	}
	// <php> artisan serve --host=0.0.0.0 --port=8000
	if len(cmd.Args) != 5 {
		t.Fatalf("cmd.Args = %v, want 5 elements", cmd.Args)
	}
	if cmd.Args[1] != "artisan" {
		t.Errorf("cmd.Args[1] = %q, want %q", cmd.Args[1], "artisan")
	}
	if cmd.Args[2] != "serve" {
		t.Errorf("cmd.Args[2] = %q, want %q", cmd.Args[2], "serve")
	}
	if cmd.Args[3] != "--host=0.0.0.0" {
		t.Errorf("cmd.Args[3] = %q, want %q", cmd.Args[3], "--host=0.0.0.0")
	}
	if cmd.Args[4] != "--port=8000" {
		t.Errorf("cmd.Args[4] = %q, want %q", cmd.Args[4], "--port=8000")
	}
}

func TestPHPStartCmdInterpreter(t *testing.T) {
	rt := &PHPRuntime{}
	cmd, err := rt.StartCmd(StartOptions{
		Entrypoint:  "app.php",
		Interpreter: "/usr/bin/php8.2",
		Cwd:         "/app",
	})
	if err != nil {
		t.Fatalf("StartCmd() error: %v", err)
	}
	if cmd.Args[0] != "/usr/bin/php8.2" {
		t.Errorf("cmd.Args[0] = %q, want %q", cmd.Args[0], "/usr/bin/php8.2")
	}
	if cmd.Args[1] != "app.php" {
		t.Errorf("cmd.Args[1] = %q, want %q", cmd.Args[1], "app.php")
	}
}

func TestPHPStartCmdNoEntrypoint(t *testing.T) {
	rt := &PHPRuntime{}
	_, err := rt.StartCmd(StartOptions{
		Cwd: "/app",
	})
	if err == nil {
		t.Error("StartCmd() with empty entrypoint should return error")
	}
}

func TestPHPStartCmdWithEnv(t *testing.T) {
	rt := &PHPRuntime{}
	cmd, err := rt.StartCmd(StartOptions{
		Entrypoint: "app.php",
		Cwd:        "/app",
		Env:        map[string]string{"APP_ENV": "production"},
	})
	if err != nil {
		t.Fatalf("StartCmd() error: %v", err)
	}
	found := false
	for _, e := range cmd.Env {
		if e == "APP_ENV=production" {
			found = true
			break
		}
	}
	if !found {
		t.Error("cmd.Env should contain APP_ENV=production")
	}
}
