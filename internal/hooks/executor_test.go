package hooks

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/runixio/runix/pkg/types"
)

func TestRunHookSuccess(t *testing.T) {
	exec := NewExecutor()
	cfg := types.ProcessConfig{
		Name: "test",
		Cwd:  t.TempDir(),
	}
	hook := &types.HookConfig{
		Command: "echo hello",
	}

	err := exec.Run(context.Background(), hook, "pre_start", cfg)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}

func TestRunHookFailure(t *testing.T) {
	exec := NewExecutor()
	cfg := types.ProcessConfig{
		Name: "test",
		Cwd:  t.TempDir(),
	}
	hook := &types.HookConfig{
		Command: "exit 1",
	}

	err := exec.Run(context.Background(), hook, "pre_start", cfg)
	if err == nil {
		t.Fatal("Run() should return error on hook failure")
	}
}

func TestRunHookIgnoreFailure(t *testing.T) {
	exec := NewExecutor()
	cfg := types.ProcessConfig{
		Name: "test",
		Cwd:  t.TempDir(),
	}
	hook := &types.HookConfig{
		Command:       "exit 1",
		IgnoreFailure: true,
	}

	err := exec.Run(context.Background(), hook, "pre_start", cfg)
	if err != nil {
		t.Fatalf("Run() with IgnoreFailure should return nil, got: %v", err)
	}
}

func TestRunHookTimeout(t *testing.T) {
	exec := NewExecutor()
	cfg := types.ProcessConfig{
		Name: "test",
		Cwd:  t.TempDir(),
	}
	hook := &types.HookConfig{
		Command: "sleep 10",
		Timeout: 100 * time.Millisecond,
	}

	start := time.Now()
	err := exec.Run(context.Background(), hook, "pre_start", cfg)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Run() should timeout")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Timeout took too long: %v", elapsed)
	}
}

func TestRunHookNil(t *testing.T) {
	exec := NewExecutor()
	cfg := types.ProcessConfig{Name: "test"}

	err := exec.Run(context.Background(), nil, "pre_start", cfg)
	if err != nil {
		t.Fatalf("Run() with nil hook should return nil, got: %v", err)
	}
}

func TestRunHookEmptyCommand(t *testing.T) {
	exec := NewExecutor()
	cfg := types.ProcessConfig{Name: "test"}
	hook := &types.HookConfig{Command: ""}

	err := exec.Run(context.Background(), hook, "pre_start", cfg)
	if err != nil {
		t.Fatalf("Run() with empty command should return nil, got: %v", err)
	}
}

func TestRunHookWithEnv(t *testing.T) {
	dir := t.TempDir()
	outFile := dir + "/env.txt"

	exec := NewExecutor()
	cfg := types.ProcessConfig{
		Name: "test",
		Cwd:  dir,
		Env:  map[string]string{"MY_VAR": "hello_from_runix"},
	}
	hook := &types.HookConfig{
		Command: "echo $MY_VAR > " + outFile,
	}

	err := exec.Run(context.Background(), hook, "pre_start", cfg)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	got := string(data)
	if got != "hello_from_runix\n" {
		t.Errorf("env not passed correctly: got %q", got)
	}
}

func TestRunPre(t *testing.T) {
	exec := NewExecutor()
	cfg := types.ProcessConfig{Name: "test", Cwd: t.TempDir()}
	hook := &types.HookConfig{Command: "exit 1"}

	err := exec.RunPre(context.Background(), hook, "pre_start", cfg)
	if err == nil {
		t.Fatal("RunPre should return error on failure")
	}
}

func TestRunPost(t *testing.T) {
	exec := NewExecutor()
	cfg := types.ProcessConfig{Name: "test", Cwd: t.TempDir()}
	hook := &types.HookConfig{Command: "exit 1"}

	// RunPost should not panic or return — it suppresses errors.
	exec.RunPost(context.Background(), hook, "post_start", cfg)
}

func TestHookEnabled(t *testing.T) {
	hooks := &types.ProcessHooks{
		PreStart:  &types.HookConfig{Command: "echo hi"},
		PostStart: nil,
	}

	if !HookEnabled(hooks, "pre_start") {
		t.Error("pre_start should be enabled")
	}
	if HookEnabled(hooks, "post_start") {
		t.Error("post_start should not be enabled (nil)")
	}
	if HookEnabled(hooks, "pre_stop") {
		t.Error("pre_stop should not be enabled (missing)")
	}
	if HookEnabled(nil, "pre_start") {
		t.Error("nil hooks should report disabled")
	}
	if HookEnabled(&types.ProcessHooks{}, "pre_start") {
		t.Error("empty hooks should report disabled")
	}
}
