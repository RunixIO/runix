package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectDenoJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "deno.json"), []byte(`{"tasks": {"dev": "deno run main.ts"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	d := NewDetector()
	rt, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if rt.Name() != "deno" {
		t.Errorf("Detected runtime = %q, want %q", rt.Name(), "deno")
	}
}

func TestDetectDenoJSONC(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "deno.jsonc"), []byte(`// Deno config\n{"tasks": {}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	d := NewDetector()
	rt, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if rt.Name() != "deno" {
		t.Errorf("Detected runtime = %q, want %q", rt.Name(), "deno")
	}
}

func TestDenoStartCmd(t *testing.T) {
	rt := &DenoRuntime{}
	cmd, err := rt.StartCmd(StartOptions{
		Entrypoint: "main.ts",
		Cwd:        "/app",
	})
	if err != nil {
		t.Fatalf("StartCmd() error: %v", err)
	}
	if cmd.Path != "deno" && cmd.Args[0] != "deno" {
		t.Errorf("cmd.Args = %v, want program deno", cmd.Args)
	}
	want := []string{"deno", "run", "main.ts"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("cmd.Args = %v, want %v", cmd.Args, want)
	}
	for i, v := range want {
		if cmd.Args[i] != v {
			t.Errorf("cmd.Args[%d] = %q, want %q", i, cmd.Args[i], v)
		}
	}
	if cmd.Dir != "/app" {
		t.Errorf("cmd.Dir = %q, want %q", cmd.Dir, "/app")
	}
}

func TestDenoStartCmdWithPerms(t *testing.T) {
	rt := &DenoRuntime{}
	cmd, err := rt.StartCmd(StartOptions{
		Entrypoint: "main.ts",
		Args:       []string{"--allow-net", "--allow-env", "--allow-read"},
		Cwd:        "/app",
	})
	if err != nil {
		t.Fatalf("StartCmd() error: %v", err)
	}
	want := []string{"deno", "run", "--allow-net", "--allow-env", "--allow-read", "main.ts"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("cmd.Args = %v, want %v", cmd.Args, want)
	}
	for i, v := range want {
		if cmd.Args[i] != v {
			t.Errorf("cmd.Args[%d] = %q, want %q", i, cmd.Args[i], v)
		}
	}
}

func TestDenoStartCmdInterpreter(t *testing.T) {
	rt := &DenoRuntime{}
	cmd, err := rt.StartCmd(StartOptions{
		Entrypoint:  "main.ts",
		Interpreter: "/custom/deno",
		Cwd:         "/app",
	})
	if err != nil {
		t.Fatalf("StartCmd() error: %v", err)
	}
	if cmd.Args[0] != "/custom/deno" {
		t.Errorf("cmd.Args[0] = %q, want %q", cmd.Args[0], "/custom/deno")
	}
}

func TestDenoStartCmdNoEntrypoint(t *testing.T) {
	rt := &DenoRuntime{}
	_, err := rt.StartCmd(StartOptions{
		Cwd: "/app",
	})
	if err == nil {
		t.Error("StartCmd() with empty entrypoint should return error")
	}
}

func TestDenoStartCmdWithEnv(t *testing.T) {
	rt := &DenoRuntime{}
	cmd, err := rt.StartCmd(StartOptions{
		Entrypoint: "main.ts",
		Cwd:        "/app",
		Env:        map[string]string{"PORT": "8080"},
	})
	if err != nil {
		t.Fatalf("StartCmd() error: %v", err)
	}
	found := false
	for _, e := range cmd.Env {
		if e == "PORT=8080" {
			found = true
			break
		}
	}
	if !found {
		t.Error("cmd.Env should contain PORT=8080")
	}
}
