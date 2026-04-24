package migrate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseJSONEcosystem(t *testing.T) {
	data, err := os.ReadFile("testdata/ecosystem.config.json")
	if err != nil {
		t.Fatalf("reading testdata: %v", err)
	}
	cfg, err := parseJSON(data)
	if err != nil {
		t.Fatalf("parseJSON: %v", err)
	}
	if len(cfg.Apps) != 3 {
		t.Fatalf("expected 3 apps, got %d", len(cfg.Apps))
	}

	// First app: web
	web := cfg.Apps[0]
	assertStr(t, web.Name, "web")
	assertStr(t, web.Script, "server.js")
	if web.Instances != 2 {
		t.Errorf("web.Instances = %d, want 2", web.Instances)
	}
	assertStr(t, web.ExecMode, "cluster_mode")
	if len(web.Env) != 2 {
		t.Errorf("web.Env len = %d, want 2", len(web.Env))
	}
	assertStr(t, web.Env["NODE_ENV"], "development")
	assertStr(t, web.Env["PORT"], "3000")
	if len(web.EnvProduction) != 2 {
		t.Errorf("web.EnvProduction len = %d, want 2", len(web.EnvProduction))
	}
	assertStr(t, web.EnvProduction["NODE_ENV"], "production")
	if len(web.IgnoreWatch) != 2 {
		t.Errorf("web.IgnoreWatch len = %d, want 2", len(web.IgnoreWatch))
	}
	assertStr(t, web.MaxMemoryRestart, "500M")

	// Second app: worker
	worker := cfg.Apps[1]
	assertStr(t, worker.Name, "worker")
	assertStr(t, worker.Script, "worker.js")
	assertStr(t, worker.Cwd, "/app/worker")
	assertStr(t, worker.Interpreter, "python3")
	assertStr(t, worker.CronRestart, "0 3 * * *")

	// Third app: api
	api := cfg.Apps[2]
	assertStr(t, api.Name, "api")
	assertStr(t, api.Interpreter, "bun")
	assertStr(t, api.UID, "appuser")
	assertStr(t, api.OutFile, "/var/log/api/out.log")
	if len(api.StopExitCodes) != 2 {
		t.Errorf("api.StopExitCodes len = %d, want 2", len(api.StopExitCodes))
	}
}

func TestParseYAMLEcosystem(t *testing.T) {
	data, err := os.ReadFile("testdata/ecosystem.config.yaml")
	if err != nil {
		t.Fatalf("reading testdata: %v", err)
	}
	cfg, err := parseYAML(data)
	if err != nil {
		t.Fatalf("parseYAML: %v", err)
	}
	if len(cfg.Apps) != 3 {
		t.Fatalf("expected 3 apps, got %d", len(cfg.Apps))
	}

	api := cfg.Apps[0]
	assertStr(t, api.Name, "api")
	assertStr(t, api.Script, "app.js")
	assertStr(t, api.Interpreter, "bun")
	if api.Instances != 4 {
		t.Errorf("api.Instances = %d, want 4", api.Instances)
	}

	worker := cfg.Apps[1]
	assertStr(t, worker.Name, "background-worker")
	assertStr(t, worker.Script, "worker.py")
	args := NormalizeArgs(worker.Args)
	if len(args) != 3 {
		t.Errorf("worker.Args = %v, want 3 elements", args)
	}

	staticApp := cfg.Apps[2]
	assertStr(t, staticApp.Name, "static-app")
	if staticApp.Autorestart != nil && *staticApp.Autorestart {
		t.Error("static-app autorestart should be false")
	}
}

func TestParseDump(t *testing.T) {
	data, err := os.ReadFile("testdata/dump.pm2")
	if err != nil {
		t.Fatalf("reading testdata: %v", err)
	}
	cfg, err := ParseDump(data)
	if err != nil {
		t.Fatalf("ParseDump: %v", err)
	}
	if len(cfg.Apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(cfg.Apps))
	}

	app := cfg.Apps[0]
	assertStr(t, app.Name, "my-app")
	assertStr(t, app.Script, "server.js")
	assertStr(t, app.Cwd, "/app")
	assertStr(t, app.ExecInterpreter, "node")
	if len(app.Env) != 2 {
		t.Errorf("app.Env len = %d, want 2", len(app.Env))
	}

	worker := cfg.Apps[1]
	assertStr(t, worker.Name, "worker")
	assertStr(t, worker.ExecInterpreter, "python3")
	assertStr(t, worker.CronRestart, "*/30 * * * *")
}

func TestAutoDetect(t *testing.T) {
	dir := t.TempDir()

	// No files: empty string.
	if p := AutoDetect(dir); p != "" {
		t.Errorf("AutoDetect empty dir = %q, want empty", p)
	}

	// Create ecosystem.config.json.
	if err := os.WriteFile(filepath.Join(dir, "ecosystem.config.json"), []byte(`{"apps":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	p := AutoDetect(dir)
	if filepath.Base(p) != "ecosystem.config.json" {
		t.Errorf("AutoDetect = %q, want ecosystem.config.json", filepath.Base(p))
	}
}

func TestFindDumpFile(t *testing.T) {
	// This test just checks the function doesn't crash.
	// The dump file may or may not exist on the test machine.
	_ = FindDumpFile()
}

func TestParseFileJSON(t *testing.T) {
	cfg, err := ParseFile("testdata/ecosystem.config.json")
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(cfg.Apps) != 3 {
		t.Fatalf("expected 3 apps, got %d", len(cfg.Apps))
	}
}

func TestParseFileYAML(t *testing.T) {
	cfg, err := ParseFile("testdata/ecosystem.config.yaml")
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(cfg.Apps) != 3 {
		t.Fatalf("expected 3 apps, got %d", len(cfg.Apps))
	}
}

func TestParseFileDump(t *testing.T) {
	cfg, err := ParseFile("testdata/dump.pm2")
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(cfg.Apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(cfg.Apps))
	}
}

func TestNormalizeArgs(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  []string
	}{
		{"nil", nil, nil},
		{"empty string", "", nil},
		{"simple", "hello world", []string{"hello", "world"}},
		{"quoted", `--msg "hello world"`, []string{"--msg", "hello world"}},
		{"single quoted", `--msg 'hello world'`, []string{"--msg", "hello world"}},
		{"string slice", []string{"a", "b"}, []string{"a", "b"}},
		{"interface slice", []interface{}{"x", "y"}, []string{"x", "y"}},
		{"empty interface slice", []interface{}{}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeArgs(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("NormalizeArgs(%v) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("NormalizeArgs(%v)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
