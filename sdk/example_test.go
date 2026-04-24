package sdk_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/runixio/runix/sdk"
)

// tmpDir creates a temporary directory for test logs.
func tmpDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

// skipIfMissing skips the test if the given binary is not on PATH.
func skipIfMissing(t *testing.T, bin string) {
	t.Helper()
	if _, err := exec.LookPath(bin); err != nil {
		t.Skipf("%s not found on PATH", bin)
	}
}

// newTestManager creates a Manager with a temp log directory.
func newTestManager(t *testing.T) *sdk.Manager {
	t.Helper()
	mgr, err := sdk.New(sdk.Config{LogDir: tmpDir(t)})
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	t.Cleanup(func() { mgr.Close() })
	return mgr
}

func TestNewManager(t *testing.T) {
	dir := tmpDir(t)
	mgr, err := sdk.New(sdk.Config{LogDir: dir})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer mgr.Close()

	if mgr == nil {
		t.Fatal("New() returned nil manager")
	}
}

func TestNewManager_DefaultLogDir(t *testing.T) {
	mgr, err := sdk.New(sdk.Config{})
	if err != nil {
		t.Fatalf("New() with empty config error: %v", err)
	}
	defer mgr.Close()
}

func TestAddProcess_Sleep(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	id, err := mgr.AddProcess(ctx, sdk.ProcessConfig{
		Name:   "sleep-test",
		Binary: "sleep",
		Args:   []string{"60"},
	})
	if err != nil {
		t.Fatalf("AddProcess() error: %v", err)
	}
	if id == "" {
		t.Fatal("AddProcess() returned empty ID")
	}

	info, err := mgr.Inspect(id)
	if err != nil {
		t.Fatalf("Inspect() error: %v", err)
	}
	if info.State != "running" {
		t.Errorf("expected state running, got %q", info.State)
	}
	if info.PID <= 0 {
		t.Errorf("expected positive PID, got %d", info.PID)
	}

	if err := mgr.Stop(id, 5*time.Second); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
}

func TestList(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	for i := range 3 {
		_, err := mgr.AddProcess(ctx, sdk.ProcessConfig{
			Name:   fmt.Sprintf("list-test-%d", i),
			Binary: "sleep",
			Args:   []string{"60"},
		})
		if err != nil {
			t.Fatalf("AddProcess(%d) error: %v", i, err)
		}
	}

	list := mgr.List()
	if len(list) != 3 {
		t.Fatalf("List() returned %d processes, want 3", len(list))
	}

	for i, info := range list {
		expected := fmt.Sprintf("list-test-%d", i)
		if info.Name != expected {
			t.Errorf("List()[%d].Name = %q, want %q", i, info.Name, expected)
		}
		if info.NumericID != i {
			t.Errorf("List()[%d].NumericID = %d, want %d", i, info.NumericID, i)
		}
	}
}

func TestRemove(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	id, err := mgr.AddProcess(ctx, sdk.ProcessConfig{
		Name:   "remove-test",
		Binary: "sleep",
		Args:   []string{"60"},
	})
	if err != nil {
		t.Fatalf("AddProcess() error: %v", err)
	}

	if err := mgr.Remove(id); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	list := mgr.List()
	for _, info := range list {
		if info.ID == id {
			t.Fatal("process still present after Remove()")
		}
	}
}

func TestRestart(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	id, err := mgr.AddProcess(ctx, sdk.ProcessConfig{
		Name:          "restart-test",
		Binary:        "sleep",
		Args:          []string{"60"},
		RestartPolicy: "never",
	})
	if err != nil {
		t.Fatalf("AddProcess() error: %v", err)
	}

	info, _ := mgr.Inspect(id)
	firstPID := info.PID

	if err := mgr.Restart(ctx, id); err != nil {
		t.Fatalf("Restart() error: %v", err)
	}

	info, _ = mgr.Inspect(id)
	if info.PID == firstPID {
		t.Error("expected different PID after restart")
	}
	if info.State != "running" {
		t.Errorf("expected state running after restart, got %q", info.State)
	}
}

func TestInspect_NotFound(t *testing.T) {
	mgr := newTestManager(t)

	_, err := mgr.Inspect("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent process")
	}
}

func TestSaveResurrect(t *testing.T) {
	dir := tmpDir(t)

	// Create a manager, add a process, save.
	mgr1, err := sdk.New(sdk.Config{LogDir: dir})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx := context.Background()
	_, err = mgr1.AddProcess(ctx, sdk.ProcessConfig{
		Name:   "save-test",
		Binary: "sleep",
		Args:   []string{"60"},
	})
	if err != nil {
		t.Fatalf("AddProcess() error: %v", err)
	}

	if err := mgr1.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	mgr1.Close()

	// Create a new manager and resurrect.
	mgr2, err := sdk.New(sdk.Config{LogDir: dir})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer mgr2.Close()

	if err := mgr2.Resurrect(); err != nil {
		t.Fatalf("Resurrect() error: %v", err)
	}

	list := mgr2.List()
	if len(list) == 0 {
		t.Fatal("no processes after Resurrect()")
	}

	found := false
	for _, info := range list {
		if info.Name == "save-test" {
			found = true
			if info.State != "running" {
				t.Errorf("expected running after resurrect, got %q", info.State)
			}
			break
		}
	}
	if !found {
		t.Fatal("save-test not found after Resurrect()")
	}
}

func TestLogs(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	dir := tmpDir(t)
	script := filepath.Join(dir, "echo.sh")
	content := "#!/bin/sh\necho hello from sdk\nsleep 60\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	id, err := mgr.AddProcess(ctx, sdk.ProcessConfig{
		Name:   "log-test",
		Binary: script,
	})
	if err != nil {
		t.Fatalf("AddProcess() error: %v", err)
	}

	// Wait for output to be written.
	time.Sleep(500 * time.Millisecond)

	logCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	r, err := mgr.Logs(logCtx, id, sdk.LogOptions{Tail: 10})
	if err != nil {
		t.Fatalf("Logs() error: %v", err)
	}
	defer r.Close()

	data, _ := io.ReadAll(r)
	if len(data) == 0 {
		t.Error("expected log output, got empty")
	}
}

func TestForceStop(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	id, err := mgr.AddProcess(ctx, sdk.ProcessConfig{
		Name:   "force-stop-test",
		Binary: "sleep",
		Args:   []string{"60"},
	})
	if err != nil {
		t.Fatalf("AddProcess() error: %v", err)
	}

	if err := mgr.ForceStop(id); err != nil {
		t.Fatalf("ForceStop() error: %v", err)
	}

	// Give time for state to update.
	time.Sleep(200 * time.Millisecond)

	info, _ := mgr.Inspect(id)
	if info.State == "running" {
		t.Error("expected process to not be running after ForceStop")
	}
}

func TestProcessConfig_Script(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	dir := tmpDir(t)
	script := filepath.Join(dir, "test.sh")
	content := "#!/bin/sh\necho hello\nsleep 60\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	id, err := mgr.AddProcess(ctx, sdk.ProcessConfig{
		Name:   "script-test",
		Script: script,
	})
	if err != nil {
		t.Fatalf("AddProcess() with Script error: %v", err)
	}

	info, _ := mgr.Inspect(id)
	if info.State != "running" {
		t.Errorf("expected running, got %q", info.State)
	}
}

func TestProcessConfig_Env(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	dir := tmpDir(t)
	script := filepath.Join(dir, "env.sh")
	content := "#!/bin/sh\necho $SDK_TEST_VAR\nsleep 60\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	id, err := mgr.AddProcess(ctx, sdk.ProcessConfig{
		Name:   "env-test",
		Script: script,
		Env:    map[string]string{"SDK_TEST_VAR": "hello-env"},
	})
	if err != nil {
		t.Fatalf("AddProcess() error: %v", err)
	}

	info, _ := mgr.Inspect(id)
	if info.State != "running" {
		t.Errorf("expected running, got %q", info.State)
	}
}

func TestProcessConfig_RestartPolicy(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	id, err := mgr.AddProcess(ctx, sdk.ProcessConfig{
		Name:          "policy-test",
		Binary:        "sleep",
		Args:          []string{"60"},
		RestartPolicy: "never",
		MaxRestarts:   0,
	})
	if err != nil {
		t.Fatalf("AddProcess() error: %v", err)
	}

	info, _ := mgr.Inspect(id)
	if info.State != "running" {
		t.Errorf("expected running, got %q", info.State)
	}
}

// Runtime-specific tests — these skip if the runtime binary is missing.

func TestAddProcess_Python(t *testing.T) {
	skipIfMissing(t, "python3")
	mgr := newTestManager(t)
	ctx := context.Background()

	dir := tmpDir(t)
	script := filepath.Join(dir, "test.py")
	content := "import time\nprint('hello from python')\ntime.sleep(60)\n"
	if err := os.WriteFile(script, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	id, err := mgr.AddProcess(ctx, sdk.ProcessConfig{
		Name:    "python-test",
		Script:  script,
		Runtime: "python",
	})
	if err != nil {
		t.Fatalf("AddProcess() python error: %v", err)
	}
	if id == "" {
		t.Fatal("empty ID returned")
	}
}

func TestAddProcess_Node(t *testing.T) {
	skipIfMissing(t, "node")
	mgr := newTestManager(t)
	ctx := context.Background()

	dir := tmpDir(t)
	script := filepath.Join(dir, "test.js")
	content := "console.log('hello from node');\nsetTimeout(() => {}, 60000);\n"
	if err := os.WriteFile(script, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	id, err := mgr.AddProcess(ctx, sdk.ProcessConfig{
		Name:    "node-test",
		Script:  script,
		Runtime: "node",
	})
	if err != nil {
		t.Fatalf("AddProcess() node error: %v", err)
	}
	if id == "" {
		t.Fatal("empty ID returned")
	}
}

func TestAddProcess_Bun(t *testing.T) {
	skipIfMissing(t, "bun")
	mgr := newTestManager(t)
	ctx := context.Background()

	dir := tmpDir(t)
	script := filepath.Join(dir, "test.js")
	content := "console.log('hello from bun');\nsetTimeout(() => {}, 60000);\n"
	if err := os.WriteFile(script, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	id, err := mgr.AddProcess(ctx, sdk.ProcessConfig{
		Name:    "bun-test",
		Script:  script,
		Runtime: "bun",
	})
	if err != nil {
		t.Fatalf("AddProcess() bun error: %v", err)
	}
	if id == "" {
		t.Fatal("empty ID returned")
	}
}

func TestAddProcess_Deno(t *testing.T) {
	skipIfMissing(t, "deno")
	mgr := newTestManager(t)
	ctx := context.Background()

	dir := tmpDir(t)
	script := filepath.Join(dir, "test.ts")
	content := "console.log('hello from deno');\nsetTimeout(() => {}, 60000);\n"
	if err := os.WriteFile(script, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	id, err := mgr.AddProcess(ctx, sdk.ProcessConfig{
		Name:    "deno-test",
		Script:  script,
		Runtime: "deno",
	})
	if err != nil {
		t.Fatalf("AddProcess() deno error: %v", err)
	}
	if id == "" {
		t.Fatal("empty ID returned")
	}
}

func TestAddProcess_Ruby(t *testing.T) {
	skipIfMissing(t, "ruby")
	mgr := newTestManager(t)
	ctx := context.Background()

	dir := tmpDir(t)
	script := filepath.Join(dir, "test.rb")
	content := "puts 'hello from ruby'\nsleep 60\n"
	if err := os.WriteFile(script, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	id, err := mgr.AddProcess(ctx, sdk.ProcessConfig{
		Name:    "ruby-test",
		Script:  script,
		Runtime: "ruby",
	})
	if err != nil {
		t.Fatalf("AddProcess() ruby error: %v", err)
	}
	if id == "" {
		t.Fatal("empty ID returned")
	}
}

func TestAddProcess_PHP(t *testing.T) {
	skipIfMissing(t, "php")
	mgr := newTestManager(t)
	ctx := context.Background()

	dir := tmpDir(t)
	script := filepath.Join(dir, "test.php")
	content := "<?php\necho 'hello from php'.PHP_EOL;\nsleep(60);\n"
	if err := os.WriteFile(script, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	id, err := mgr.AddProcess(ctx, sdk.ProcessConfig{
		Name:    "php-test",
		Script:  script,
		Runtime: "php",
	})
	if err != nil {
		t.Fatalf("AddProcess() php error: %v", err)
	}
	if id == "" {
		t.Fatal("empty ID returned")
	}
}

func TestAddProcess_Go(t *testing.T) {
	skipIfMissing(t, "go")

	mgr := newTestManager(t)
	ctx := context.Background()

	dir := tmpDir(t)
	script := filepath.Join(dir, "test.go")
	content := `package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("hello from go")
	time.Sleep(60 * time.Second)
}
`
	if err := os.WriteFile(script, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	id, err := mgr.AddProcess(ctx, sdk.ProcessConfig{
		Name:    "go-test",
		Script:  script,
		Runtime: "go",
		Cwd:     dir,
	})
	if err != nil {
		t.Fatalf("AddProcess() go error: %v", err)
	}
	if id == "" {
		t.Fatal("empty ID returned")
	}
}

// Example_newManager demonstrates basic SDK usage.
func Example_newManager() {
	mgr, err := sdk.New(sdk.Config{
		LogDir: "/tmp/myapp-runix",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer mgr.Close()

	ctx := context.Background()
	id, err := mgr.AddProcess(ctx, sdk.ProcessConfig{
		Name:   "my-worker",
		Binary: "sleep",
		Args:   []string{"300"},
		Env:    map[string]string{"NODE_ENV": "production"},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Started process: %s\n", id)

	// Inspect it.
	info, _ := mgr.Inspect(id)
	fmt.Printf("PID: %d, State: %s\n", info.PID, info.State)

	// Clean up.
	_ = mgr.Stop(id, 5*time.Second)
}

// Example_saveResurrect demonstrates save and restore across sessions.
func Example_saveResurrect() {
	dir := "/tmp/myapp-state"

	// Session 1: start processes and save.
	mgr, _ := sdk.New(sdk.Config{LogDir: dir})
	ctx := context.Background()

	mgr.AddProcess(ctx, sdk.ProcessConfig{
		Name:   "api",
		Binary: "sleep",
		Args:   []string{"300"},
	})
	mgr.Save()
	mgr.Close()

	// Session 2: restore from saved state.
	mgr2, _ := sdk.New(sdk.Config{LogDir: dir})
	defer mgr2.Close()

	mgr2.Resurrect()

	for _, p := range mgr2.List() {
		fmt.Printf("Restored: %s (PID %d)\n", p.Name, p.PID)
	}
}
