package migrate

import (
	"runtime"
	"testing"
	"time"

	"github.com/runixio/runix/pkg/types"
)

func TestConvertBasic(t *testing.T) {
	pm2 := &PM2Config{
		Apps: []PM2AppConfig{
			{Name: "web", Script: "server.js"},
		},
	}
	result := Convert(pm2)
	if len(result.Config.Processes) != 1 {
		t.Fatalf("expected 1 process, got %d", len(result.Config.Processes))
	}
	p := result.Config.Processes[0]
	assertStr(t, p.Name, "web")
	assertStr(t, p.Entrypoint, "server.js")
	if p.Instances != 1 {
		t.Errorf("Instances = %d, want 1", p.Instances)
	}
	if !p.Autostart {
		t.Error("Autostart should be true")
	}
}

func TestConvertClusterMode(t *testing.T) {
	pm2 := &PM2Config{
		Apps: []PM2AppConfig{
			{Name: "api", Script: "app.js", ExecMode: "cluster_mode", Instances: 4},
		},
	}
	result := Convert(pm2)
	p := result.Config.Processes[0]
	if p.Instances != 4 {
		t.Errorf("Instances = %d, want 4", p.Instances)
	}
}

func TestConvertEnvProfiles(t *testing.T) {
	pm2 := &PM2Config{
		Apps: []PM2AppConfig{{
			Name:          "web",
			Script:        "server.js",
			Env:           map[string]string{"PORT": "3000"},
			EnvProduction: map[string]string{"PORT": "8080", "NODE_ENV": "production"},
		}},
	}
	result := Convert(pm2)
	p := result.Config.Processes[0]
	assertStr(t, p.Env["PORT"], "3000")

	prod := result.Config.Profiles["production"]["web"]
	assertStr(t, prod["PORT"], "8080")
	assertStr(t, prod["NODE_ENV"], "production")
}

func TestConvertWatchBool(t *testing.T) {
	pm2 := &PM2Config{
		Apps: []PM2AppConfig{{
			Name:   "web",
			Script: "server.js",
			Watch:  true,
		}},
	}
	result := Convert(pm2)
	w := result.Config.Processes[0].Watch
	if w == nil || !w.Enabled {
		t.Fatal("watch should be enabled")
	}
}

func TestConvertWatchArray(t *testing.T) {
	pm2 := &PM2Config{
		Apps: []PM2AppConfig{{
			Name:        "web",
			Script:      "server.js",
			Watch:       []interface{}{"src", "lib"},
			IgnoreWatch: []string{"node_modules"},
			WatchDelay:  500,
		}},
	}
	result := Convert(pm2)
	w := result.Config.Processes[0].Watch
	if w == nil || !w.Enabled {
		t.Fatal("watch should be enabled")
	}
	if len(w.Paths) != 2 {
		t.Errorf("watch.Paths len = %d, want 2", len(w.Paths))
	}
	assertStr(t, w.Paths[0], "src")
	assertStr(t, w.Paths[1], "lib")
	if len(w.Ignore) != 1 {
		t.Errorf("watch.Ignore len = %d, want 1", len(w.Ignore))
	}
	assertStr(t, w.Ignore[0], "node_modules")
	assertStr(t, w.Debounce, "500ms")
}

func TestConvertAutorestart(t *testing.T) {
	tests := []struct {
		name   string
		auto   *bool
		expect types.RestartPolicy
	}{
		{"true", boolPtr(true), types.RestartAlways},
		{"false", boolPtr(false), types.RestartNever},
		{"nil", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm2 := &PM2Config{
				Apps: []PM2AppConfig{{Name: "x", Script: "x.js", Autorestart: tt.auto}},
			}
			result := Convert(pm2)
			p := result.Config.Processes[0]
			if p.RestartPolicy != tt.expect {
				t.Errorf("RestartPolicy = %q, want %q", p.RestartPolicy, tt.expect)
			}
		})
	}
}

func TestConvertInterpreterToRuntime(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"node", "node"},
		{"node20", "node"},
		{"bun", "bun"},
		{"deno", "deno"},
		{"python", "python"},
		{"python3", "python"},
		{"ruby", "ruby"},
		{"php", "php"},
		{"none", ""},
		{"bash", ""},
		{"/bin/sh", ""},
		{"/usr/local/bin/custom", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := interpreterToRuntime(tt.input)
			if got != tt.want {
				t.Errorf("interpreterToRuntime(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestConvertMemoryFormat(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"150M", "150MB"},
		{"1G", "1GB"},
		{"512m", "512MB"},
		{"2g", "2GB"},
		{"500K", "500KB"},
		{"", ""},
		{"1024", "1024"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := convertMemoryFormat(tt.input)
			if got != tt.want {
				t.Errorf("convertMemoryFormat(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestConvertKillTimeout(t *testing.T) {
	pm2 := &PM2Config{
		Apps: []PM2AppConfig{{
			Name:        "web",
			Script:      "server.js",
			KillTimeout: 5000,
		}},
	}
	result := Convert(pm2)
	p := result.Config.Processes[0]
	if p.StopTimeout != 5*time.Second {
		t.Errorf("StopTimeout = %v, want 5s", p.StopTimeout)
	}
}

func TestConvertCronRestart(t *testing.T) {
	pm2 := &PM2Config{
		Apps: []PM2AppConfig{{
			Name:        "web",
			Script:      "server.js",
			CronRestart: "0 3 * * *",
		}},
	}
	result := Convert(pm2)
	assertStr(t, result.Config.Processes[0].CronRestart, "0 3 * * *")
}

func TestConvertNamespace(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		expect    string
	}{
		{"custom", "backend", "backend"},
		{"default", "default", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm2 := &PM2Config{
				Apps: []PM2AppConfig{{
					Name: "web", Script: "server.js", Namespace: tt.namespace,
				}},
			}
			result := Convert(pm2)
			assertStr(t, result.Config.Processes[0].Namespace, tt.expect)
		})
	}
}

func TestConvertUnmappedWarnings(t *testing.T) {
	pm2 := &PM2Config{
		Apps: []PM2AppConfig{{
			Name:          "web",
			Script:        "server.js",
			UID:           "appuser",
			OutFile:       "/var/log/out.log",
			ErrorFile:     "/var/log/err.log",
			WaitReady:     true,
			MergeLogs:     true,
			ListenTimeout: 3000,
		}},
	}
	result := Convert(pm2)

	warningFields := make(map[string]bool)
	for _, w := range result.Warnings {
		warningFields[w.Field] = true
	}

	expected := []string{"uid/gid", "log paths", "merge_logs", "listen_timeout", "wait_ready"}
	for _, f := range expected {
		if !warningFields[f] {
			t.Errorf("missing warning for field %q", f)
		}
	}
}

func TestConvertMaxInstances(t *testing.T) {
	pm2 := &PM2Config{
		Apps: []PM2AppConfig{{
			Name:      "web",
			Script:    "server.js",
			Instances: -1,
		}},
	}
	result := Convert(pm2)
	p := result.Config.Processes[0]
	if p.Instances != runtime.NumCPU() {
		t.Errorf("Instances = %d, want %d", p.Instances, runtime.NumCPU())
	}

	// Should have a warning about max mapping.
	found := false
	for _, w := range result.Warnings {
		if w.Field == "instances" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning about instances mapping")
	}
}

func TestConvertExpBackoff(t *testing.T) {
	pm2 := &PM2Config{
		Apps: []PM2AppConfig{{
			Name:            "web",
			Script:          "server.js",
			ExpBackoffDelay: 200,
		}},
	}
	result := Convert(pm2)
	if result.Config.Defaults.BackoffBase != 200*time.Millisecond {
		t.Errorf("BackoffBase = %v, want 200ms", result.Config.Defaults.BackoffBase)
	}
}

func TestConvertArgsAsString(t *testing.T) {
	pm2 := &PM2Config{
		Apps: []PM2AppConfig{{
			Name:   "worker",
			Script: "worker.js",
			Args:   "--queue high --verbose",
		}},
	}
	result := Convert(pm2)
	p := result.Config.Processes[0]
	if len(p.Args) != 3 {
		t.Fatalf("Args = %v, want 3 elements", p.Args)
	}
	assertStr(t, p.Args[0], "--queue")
	assertStr(t, p.Args[1], "high")
	assertStr(t, p.Args[2], "--verbose")
}

func TestConvertNodeArgs(t *testing.T) {
	pm2 := &PM2Config{
		Apps: []PM2AppConfig{{
			Name:     "web",
			Script:   "server.js",
			NodeArgs: []string{"--inspect", "--max-old-space-size=4096"},
		}},
	}
	result := Convert(pm2)
	p := result.Config.Processes[0]
	if len(p.Args) != 2 {
		t.Fatalf("Args = %v, want 2 elements", p.Args)
	}
	assertStr(t, p.Args[0], "--inspect")
	assertStr(t, p.Args[1], "--max-old-space-size=4096")

	// Should have warning about node_args.
	found := false
	for _, w := range result.Warnings {
		if w.Field == "node_args" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning about node_args")
	}
}

func TestConvertExecField(t *testing.T) {
	pm2 := &PM2Config{
		Apps: []PM2AppConfig{{
			Name: "cmd",
			Exec: "custom-binary",
		}},
	}
	result := Convert(pm2)
	assertStr(t, result.Config.Processes[0].Entrypoint, "custom-binary")
}

func TestConvertEmptyConfig(t *testing.T) {
	pm2 := &PM2Config{}
	result := Convert(pm2)
	if len(result.Config.Processes) != 0 {
		t.Errorf("expected 0 processes, got %d", len(result.Config.Processes))
	}
}

func TestConvertStagingProfile(t *testing.T) {
	pm2 := &PM2Config{
		Apps: []PM2AppConfig{{
			Name:       "web",
			Script:     "server.js",
			EnvStaging: map[string]string{"PORT": "4000"},
		}},
	}
	result := Convert(pm2)
	staging := result.Config.Profiles["staging"]["web"]
	if staging == nil {
		t.Fatal("staging profile should exist")
	}
	assertStr(t, staging["PORT"], "4000")
}

func TestConvertNoScript(t *testing.T) {
	pm2 := &PM2Config{
		Apps: []PM2AppConfig{{Name: "empty"}},
	}
	result := Convert(pm2)
	p := result.Config.Processes[0]
	assertStr(t, p.Name, "empty")
	assertStr(t, p.Entrypoint, "")
}

func TestConvertFullEcosystem(t *testing.T) {
	pm2 := &PM2Config{
		Apps: []PM2AppConfig{
			{
				Name:             "web",
				Script:           "server.js",
				Instances:        2,
				ExecMode:         "cluster_mode",
				Env:              map[string]string{"PORT": "3000"},
				EnvProduction:    map[string]string{"PORT": "8080"},
				Watch:            true,
				IgnoreWatch:      []string{"node_modules"},
				Autorestart:      boolPtr(true),
				MaxMemoryRestart: "500M",
				KillTimeout:      3000,
				CronRestart:      "0 3 * * *",
				Namespace:        "frontend",
			},
			{
				Name:        "worker",
				Script:      "worker.js",
				Interpreter: "python3",
				Autorestart: boolPtr(false),
			},
		},
	}

	result := Convert(pm2)

	if len(result.Config.Processes) != 2 {
		t.Fatalf("expected 2 processes, got %d", len(result.Config.Processes))
	}

	web := result.Config.Processes[0]
	assertStr(t, web.Name, "web")
	assertStr(t, web.Entrypoint, "server.js")
	assertStr(t, web.Runtime, "")
	if web.Instances != 2 {
		t.Errorf("web.Instances = %d, want 2", web.Instances)
	}
	if web.RestartPolicy != types.RestartAlways {
		t.Errorf("web.RestartPolicy = %q, want always", web.RestartPolicy)
	}
	assertStr(t, web.MemoryLimit, "500MB")
	if web.StopTimeout != 3*time.Second {
		t.Errorf("web.StopTimeout = %v, want 3s", web.StopTimeout)
	}
	assertStr(t, web.CronRestart, "0 3 * * *")
	assertStr(t, web.Namespace, "frontend")
	if web.Watch == nil || !web.Watch.Enabled {
		t.Error("web watch should be enabled")
	}

	worker := result.Config.Processes[1]
	assertStr(t, worker.Name, "worker")
	assertStr(t, worker.Runtime, "python")
	if worker.RestartPolicy != types.RestartNever {
		t.Errorf("worker.RestartPolicy = %q, want never", worker.RestartPolicy)
	}

	// Profile should be set.
	prod := result.Config.Profiles["production"]["web"]
	if prod == nil {
		t.Fatal("production profile should exist for web")
	}
	assertStr(t, prod["PORT"], "8080")
}

func TestConvertDumpEntries(t *testing.T) {
	entries := []PM2DumpEntry{
		{
			Name:             "my-app",
			PmExecPath:       "/app/server.js",
			PmCwd:            "/app",
			ExecInterpreter:  "node",
			Autorestart:      true,
			Env:              map[string]string{"PORT": "3000"},
			MaxMemoryRestart: "1G",
		},
	}
	result := ConvertDump(entries)
	if len(result.Config.Processes) != 1 {
		t.Fatalf("expected 1 process, got %d", len(result.Config.Processes))
	}
	p := result.Config.Processes[0]
	assertStr(t, p.Name, "my-app")
	assertStr(t, p.Entrypoint, "/app/server.js")
	assertStr(t, p.Cwd, "/app")
	assertStr(t, p.Runtime, "node")
	assertStr(t, p.MemoryLimit, "1GB")
	if p.RestartPolicy != types.RestartAlways {
		t.Errorf("RestartPolicy = %q, want always", p.RestartPolicy)
	}
}

func TestWarningString(t *testing.T) {
	w := Warning{Process: "web", Field: "uid/gid", Message: "not supported"}
	s := w.String()
	if !contains(s, "[web]") || !contains(s, "uid/gid") || !contains(s, "not supported") {
		t.Errorf("Warning.String() = %q", s)
	}

	w2 := Warning{Field: "deploy", Message: "no equivalent"}
	s2 := w2.String()
	if contains(s2, "[]") {
		t.Errorf("global warning should not have empty process prefix: %q", s2)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func assertStr(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConvertInterpreterUnknown(t *testing.T) {
	pm2 := &PM2Config{
		Apps: []PM2AppConfig{{
			Name:        "web",
			Script:      "server.js",
			Interpreter: "/usr/local/bin/custom-runtime",
		}},
	}
	result := Convert(pm2)
	p := result.Config.Processes[0]
	assertStr(t, p.Runtime, "")
	assertStr(t, p.Interpreter, "/usr/local/bin/custom-runtime")

	// Should have warning about unknown interpreter.
	found := false
	for _, w := range result.Warnings {
		if w.Field == "interpreter" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning about unknown interpreter")
	}
}

func TestConvertMultipleApps(t *testing.T) {
	pm2 := &PM2Config{
		Apps: []PM2AppConfig{
			{Name: "a", Script: "a.js", Instances: 2},
			{Name: "b", Script: "b.py", Interpreter: "python3"},
			{Name: "c", Script: "c.rb", Interpreter: "ruby", Instances: 1},
		},
	}
	result := Convert(pm2)
	if len(result.Config.Processes) != 3 {
		t.Fatalf("expected 3 processes, got %d", len(result.Config.Processes))
	}
	assertStr(t, result.Config.Processes[0].Name, "a")
	assertStr(t, result.Config.Processes[1].Runtime, "python")
	assertStr(t, result.Config.Processes[2].Runtime, "ruby")
}
