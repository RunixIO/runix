package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectRubyGemfile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Gemfile"), []byte("source 'https://rubygems.org'\n"), 0o644)

	d := NewDetector()
	rt, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if rt.Name() != "ruby" {
		t.Errorf("Detected runtime = %q, want %q", rt.Name(), "ruby")
	}
}

func TestDetectRubyGemfileLock(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Gemfile.lock"), []byte("GEM\n  remote: https://rubygems.org/\n"), 0o644)

	d := NewDetector()
	rt, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if rt.Name() != "ruby" {
		t.Errorf("Detected runtime = %q, want %q", rt.Name(), "ruby")
	}
}

func TestRubyStartCmd(t *testing.T) {
	rt := &RubyRuntime{}
	cmd, err := rt.StartCmd(StartOptions{
		Entrypoint: "app.rb",
		Cwd:        "/app",
	})
	if err != nil {
		t.Fatalf("StartCmd() error: %v", err)
	}
	// Command should be: <ruby> app.rb
	if len(cmd.Args) != 2 {
		t.Fatalf("cmd.Args = %v, want 2 elements", cmd.Args)
	}
	if cmd.Args[1] != "app.rb" {
		t.Errorf("cmd.Args[1] = %q, want %q", cmd.Args[1], "app.rb")
	}
	if cmd.Dir != "/app" {
		t.Errorf("cmd.Dir = %q, want %q", cmd.Dir, "/app")
	}
}

func TestRubyStartCmdWithBundle(t *testing.T) {
	rt := &RubyRuntime{}
	cmd, err := rt.StartCmd(StartOptions{
		Entrypoint: "app.rb",
		UseBundle:  true,
		Cwd:        "/app",
	})
	if err != nil {
		t.Fatalf("StartCmd() error: %v", err)
	}
	// Command should be: bundle exec <ruby> app.rb
	if cmd.Args[0] != "bundle" {
		t.Errorf("cmd.Args[0] = %q, want %q", cmd.Args[0], "bundle")
	}
	if cmd.Args[1] != "exec" {
		t.Errorf("cmd.Args[1] = %q, want %q", cmd.Args[1], "exec")
	}
	// Args[2] is the resolved ruby interpreter path
	if cmd.Args[3] != "app.rb" {
		t.Errorf("cmd.Args[3] = %q, want %q", cmd.Args[3], "app.rb")
	}
}

func TestRubyStartCmdWithBundleCustomInterpreter(t *testing.T) {
	rt := &RubyRuntime{}
	cmd, err := rt.StartCmd(StartOptions{
		Entrypoint:  "app.rb",
		Interpreter: "/opt/ruby/bin/ruby",
		UseBundle:   true,
		Cwd:         "/app",
	})
	if err != nil {
		t.Fatalf("StartCmd() error: %v", err)
	}
	want := []string{"bundle", "exec", "/opt/ruby/bin/ruby", "app.rb"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("cmd.Args = %v, want %v", cmd.Args, want)
	}
	for i, v := range want {
		if cmd.Args[i] != v {
			t.Errorf("cmd.Args[%d] = %q, want %q", i, cmd.Args[i], v)
		}
	}
}

func TestRubyStartCmdInterpreter(t *testing.T) {
	rt := &RubyRuntime{}
	cmd, err := rt.StartCmd(StartOptions{
		Entrypoint:  "app.rb",
		Interpreter: "/custom/ruby",
		Cwd:         "/app",
	})
	if err != nil {
		t.Fatalf("StartCmd() error: %v", err)
	}
	if cmd.Args[0] != "/custom/ruby" {
		t.Errorf("cmd.Args[0] = %q, want %q", cmd.Args[0], "/custom/ruby")
	}
	if cmd.Args[1] != "app.rb" {
		t.Errorf("cmd.Args[1] = %q, want %q", cmd.Args[1], "app.rb")
	}
}

func TestRubyStartCmdNoEntrypoint(t *testing.T) {
	rt := &RubyRuntime{}
	_, err := rt.StartCmd(StartOptions{
		Cwd: "/app",
	})
	if err == nil {
		t.Error("StartCmd() with empty entrypoint should return error")
	}
}

func TestRubyStartCmdWithArgs(t *testing.T) {
	rt := &RubyRuntime{}
	cmd, err := rt.StartCmd(StartOptions{
		Entrypoint: "app.rb",
		Args:       []string{"--verbose", "--port", "3000"},
		Cwd:        "/app",
	})
	if err != nil {
		t.Fatalf("StartCmd() error: %v", err)
	}
	// <ruby> app.rb --verbose --port 3000
	if len(cmd.Args) != 5 {
		t.Fatalf("cmd.Args = %v, want 5 elements", cmd.Args)
	}
	if cmd.Args[1] != "app.rb" {
		t.Errorf("cmd.Args[1] = %q, want %q", cmd.Args[1], "app.rb")
	}
	if cmd.Args[2] != "--verbose" {
		t.Errorf("cmd.Args[2] = %q, want %q", cmd.Args[2], "--verbose")
	}
	if cmd.Args[3] != "--port" {
		t.Errorf("cmd.Args[3] = %q, want %q", cmd.Args[3], "--port")
	}
	if cmd.Args[4] != "3000" {
		t.Errorf("cmd.Args[4] = %q, want %q", cmd.Args[4], "3000")
	}
}

func TestRubyStartCmdWithEnv(t *testing.T) {
	rt := &RubyRuntime{}
	cmd, err := rt.StartCmd(StartOptions{
		Entrypoint: "app.rb",
		Cwd:        "/app",
		Env:        map[string]string{"RAILS_ENV": "test"},
	})
	if err != nil {
		t.Fatalf("StartCmd() error: %v", err)
	}
	found := false
	for _, e := range cmd.Env {
		if e == "RAILS_ENV=test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("cmd.Env should contain RAILS_ENV=test")
	}
}
