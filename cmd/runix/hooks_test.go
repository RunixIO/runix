package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/runixio/runix/internal/hooks"
	"github.com/runixio/runix/internal/supervisor"
	"github.com/runixio/runix/pkg/types"
)

func TestHooksIntegration(t *testing.T) {
	dir := t.TempDir()
	logDir := dir + "/logs"
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}

	sup := supervisor.New(supervisor.Options{
		LogDir: logDir,
		Defaults: types.DefaultsConfig{
			RestartPolicy: types.RestartNever,
		},
	})

	// Use a hook that writes to a marker file so we can verify execution.
	markerFile := dir + "/hook_markers.txt"

	cfg := types.ProcessConfig{
		Name:       "hooked",
		Runtime:    "unknown",
		Entrypoint: "sleep",
		Args:       []string{"2"},
		Cwd:        dir,
		Hooks: &types.ProcessHooks{
			PreStart: &types.HookConfig{
				Command: "echo pre_start >> " + markerFile,
				Timeout: 5 * time.Second,
			},
			PostStart: &types.HookConfig{
				Command: "echo post_start >> " + markerFile,
				Timeout: 5 * time.Second,
			},
			PreStop: &types.HookConfig{
				Command: "echo pre_stop >> " + markerFile,
				Timeout: 5 * time.Second,
			},
			PostStop: &types.HookConfig{
				Command: "echo post_stop >> " + markerFile,
				Timeout: 5 * time.Second,
			},
		},
	}

	proc, err := sup.AddProcess(context.Background(), cfg)
	if err != nil {
		t.Fatalf("AddProcess error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	info := proc.Info()
	if info.State != types.StateRunning {
		t.Fatalf("process state = %s, want running", info.State)
	}

	// Verify pre_start and post_start hooks already ran.
	data, err := os.ReadFile(markerFile)
	if err != nil {
		t.Fatalf("failed to read marker file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "pre_start") {
		t.Error("pre_start hook did not run")
	}
	if !strings.Contains(content, "post_start") {
		t.Error("post_start hook did not run")
	}

	// Stop the process.
	if err := sup.StopProcess(proc.ID, false, 5*time.Second); err != nil {
		t.Fatalf("StopProcess error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Verify all hooks ran.
	data, err = os.ReadFile(markerFile)
	if err != nil {
		t.Fatalf("failed to read marker file after stop: %v", err)
	}
	content = string(data)
	for _, hook := range []string{"pre_start", "post_start", "pre_stop", "post_stop"} {
		if !strings.Contains(content, hook) {
			t.Errorf("hook %q did not run. Markers:\n%s", hook, content)
		}
	}
}

func TestPreStartHookBlocksStart(t *testing.T) {
	exec := hooks.NewExecutor()
	cfg := types.ProcessConfig{
		Name: "blocked",
		Cwd:  t.TempDir(),
	}
	hook := &types.HookConfig{
		Command: "exit 1",
		Timeout: 5 * time.Second,
	}

	err := exec.RunPre(context.Background(), hook, "pre_start", cfg)
	if err == nil {
		t.Fatal("pre_start hook with exit 1 should block")
	}
	if !strings.Contains(err.Error(), "pre_start") {
		t.Errorf("error should mention hook name: %v", err)
	}
}
