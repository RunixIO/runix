package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/runixio/runix/pkg/types"
)

func TestLoadNoFile(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load with empty path returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load with empty path returned nil config")
	}
	// Should have defaults applied.
	if cfg.Daemon.LogLevel != "info" {
		t.Errorf("expected default log_level 'info', got %q", cfg.Daemon.LogLevel)
	}
	if cfg.Defaults.RestartPolicy != types.RestartOnFailure {
		t.Errorf("expected default restart_policy %q, got %q", types.RestartOnFailure, cfg.Defaults.RestartPolicy)
	}
	if cfg.Defaults.MaxRestarts != 10 {
		t.Errorf("expected default max_restarts 10, got %d", cfg.Defaults.MaxRestarts)
	}
}

func TestLoadYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runix.yaml")

	content := `daemon:
  loglevel: debug
defaults:
  restartpolicy: always
  maxrestarts: 5
deploy:
  prod:
    host: example.com
    user: deploy
    path: /srv/runix
processes:
  - name: web
    entrypoint: /bin/server
    args:
      - --port
      - "8080"
    autostart: true
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write yaml config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Daemon.LogLevel != "debug" {
		t.Errorf("expected log_level 'debug', got %q", cfg.Daemon.LogLevel)
	}
	if cfg.Defaults.RestartPolicy != types.RestartAlways {
		t.Errorf("expected restart_policy 'always', got %q", cfg.Defaults.RestartPolicy)
	}
	if cfg.Defaults.MaxRestarts != 5 {
		t.Errorf("expected max_restarts 5, got %d", cfg.Defaults.MaxRestarts)
	}
	if cfg.Deploy["prod"].Host != "example.com" {
		t.Errorf("expected deploy host example.com, got %q", cfg.Deploy["prod"].Host)
	}
	if len(cfg.Processes) != 1 {
		t.Fatalf("expected 1 process, got %d", len(cfg.Processes))
	}
	p := cfg.Processes[0]
	if p.Name != "web" {
		t.Errorf("expected process name 'web', got %q", p.Name)
	}
	if p.Entrypoint != "/bin/server" {
		t.Errorf("expected entrypoint '/bin/server', got %q", p.Entrypoint)
	}
	if len(p.Args) != 2 || p.Args[0] != "--port" || p.Args[1] != "8080" {
		t.Errorf("unexpected args: %v", p.Args)
	}
	if !p.Autostart {
		t.Error("expected autostart true")
	}
	// Cwd should be set to config file directory.
	if p.Cwd != dir {
		t.Errorf("expected cwd %q, got %q", dir, p.Cwd)
	}
}

func TestLoadJSON(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runix.json")

	content := `{
  "daemon": {
    "loglevel": "warn"
  },
  "processes": [
    {
      "name": "worker",
      "entrypoint": "/bin/worker",
      "instances": 3
    }
  ]
}`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write json config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Daemon.LogLevel != "warn" {
		t.Errorf("expected log_level 'warn', got %q", cfg.Daemon.LogLevel)
	}
	if len(cfg.Processes) != 1 {
		t.Fatalf("expected 1 process, got %d", len(cfg.Processes))
	}
	p := cfg.Processes[0]
	if p.Name != "worker" {
		t.Errorf("expected process name 'worker', got %q", p.Name)
	}
	if p.Instances != 3 {
		t.Errorf("expected instances 3, got %d", p.Instances)
	}
}

func TestApplyDefaults(t *testing.T) {
	cfg := &types.RunixConfig{}
	ApplyDefaults(cfg)

	if cfg.Daemon.LogLevel != "info" {
		t.Errorf("expected default log_level 'info', got %q", cfg.Daemon.LogLevel)
	}
	if cfg.Defaults.RestartPolicy != types.RestartOnFailure {
		t.Errorf("expected default restart_policy %q, got %q", types.RestartOnFailure, cfg.Defaults.RestartPolicy)
	}
	if cfg.Defaults.MaxRestarts != 10 {
		t.Errorf("expected default max_restarts 10, got %d", cfg.Defaults.MaxRestarts)
	}
	if cfg.Defaults.RestartWindow != 60*time.Second {
		t.Errorf("expected default restart_window 60s, got %v", cfg.Defaults.RestartWindow)
	}
	if cfg.Defaults.BackoffBase != 1*time.Second {
		t.Errorf("expected default backoff_base 1s, got %v", cfg.Defaults.BackoffBase)
	}
	if cfg.Defaults.BackoffMax != 60*time.Second {
		t.Errorf("expected default backoff_max 60s, got %v", cfg.Defaults.BackoffMax)
	}
	if cfg.Defaults.AutoRestart == nil || !*cfg.Defaults.AutoRestart {
		t.Errorf("expected default autorestart true, got %v", cfg.Defaults.AutoRestart)
	}
	if cfg.Defaults.LogMaxSize != 10*1024*1024 {
		t.Errorf("expected default log_max_size 10485760, got %d", cfg.Defaults.LogMaxSize)
	}
	if cfg.Defaults.LogMaxAge != 168*time.Hour {
		t.Errorf("expected default log_max_age 168h, got %v", cfg.Defaults.LogMaxAge)
	}
	if cfg.Defaults.WatchDebounce != 100*time.Millisecond {
		t.Errorf("expected default watch_debounce 100ms, got %v", cfg.Defaults.WatchDebounce)
	}
	if cfg.Web.Listen != "localhost:9615" {
		t.Errorf("expected default web listen 'localhost:9615', got %q", cfg.Web.Listen)
	}
	if cfg.MCP.Transport != "stdio" {
		t.Errorf("expected default mcp transport 'stdio', got %q", cfg.MCP.Transport)
	}
}

func TestApplyDefaults_ProcessRestartControls(t *testing.T) {
	autoRestart := true
	cfg := &types.RunixConfig{
		Defaults: types.DefaultsConfig{
			AutoRestart:      &autoRestart,
			RestartDelay:     3 * time.Second,
			MinUptime:        5 * time.Second,
			MaxMemoryRestart: "256MB",
		},
		Processes: []types.ProcessConfig{
			{Name: "api", Entrypoint: "/bin/api"},
		},
	}

	ApplyDefaults(cfg)

	p := cfg.Processes[0]
	if p.AutoRestart == nil || !*p.AutoRestart {
		t.Fatalf("expected process autorestart to inherit true, got %v", p.AutoRestart)
	}
	if p.RestartDelay != 3*time.Second {
		t.Fatalf("expected restart_delay 3s, got %v", p.RestartDelay)
	}
	if p.MinUptime != 5*time.Second {
		t.Fatalf("expected min_uptime 5s, got %v", p.MinUptime)
	}
	if p.MaxMemoryRestart != "256MB" {
		t.Fatalf("expected max_memory_restart 256MB, got %q", p.MaxMemoryRestart)
	}
}

func TestValidateDuplicateNames(t *testing.T) {
	cfg := &types.RunixConfig{
		Processes: []types.ProcessConfig{
			{Name: "svc", Entrypoint: "/bin/svc"},
			{Name: "svc", Entrypoint: "/bin/svc2"},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for duplicate names, got nil")
	}
}

func TestValidateValidConfig(t *testing.T) {
	cfg := &types.RunixConfig{
		Deploy: map[string]types.DeployTarget{
			"prod": {Host: "example.com", User: "deploy", Path: "/srv/runix"},
		},
		Processes: []types.ProcessConfig{
			{Name: "web", Entrypoint: "/bin/web"},
			{Name: "worker", Entrypoint: "/bin/worker"},
		},
	}

	if err := Validate(cfg); err != nil {
		t.Fatalf("expected valid config to pass, got error: %v", err)
	}
}

func TestValidateDeployRequiresFields(t *testing.T) {
	cfg := &types.RunixConfig{
		Deploy: map[string]types.DeployTarget{
			"prod": {Host: "example.com"},
		},
	}

	if err := Validate(cfg); err == nil {
		t.Fatal("expected deploy validation error, got nil")
	}
}

func TestResolveSecretsIntoProcessEnv(t *testing.T) {
	t.Setenv("DB_PASSWORD", "supersecret")
	cfg := &types.RunixConfig{
		Secrets: map[string]types.SecretRef{
			"db_password": {Type: "env", Value: "DB_PASSWORD"},
		},
		Processes: []types.ProcessConfig{
			{
				Name:       "api",
				Entrypoint: "/bin/api",
				Env: map[string]string{
					"DATABASE_URL": "postgres://user:${db_password}@localhost:5432/app",
				},
			},
		},
	}

	if err := resolveSecrets(cfg); err != nil {
		t.Fatalf("resolveSecrets returned error: %v", err)
	}

	got := cfg.Processes[0].Env["DATABASE_URL"]
	want := "postgres://user:supersecret@localhost:5432/app"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveSecretsIntoCronEnv(t *testing.T) {
	t.Setenv("API_TOKEN", "token-123")
	cfg := &types.RunixConfig{
		Secrets: map[string]types.SecretRef{
			"api_token": {Type: "env", Value: "API_TOKEN"},
		},
		Cron: []types.CronJobConfig{
			{
				Name:     "sync",
				Schedule: "0 * * * *",
				Command:  "/bin/sync",
				Env: map[string]string{
					"AUTH_HEADER": "Bearer ${api_token}",
				},
			},
		},
	}

	if err := resolveSecrets(cfg); err != nil {
		t.Fatalf("resolveSecrets returned error: %v", err)
	}

	got := cfg.Cron[0].Env["AUTH_HEADER"]
	want := "Bearer token-123"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveSecretsErrorsOnUnknownSecretReference(t *testing.T) {
	cfg := &types.RunixConfig{
		Processes: []types.ProcessConfig{
			{
				Name:       "api",
				Entrypoint: "/bin/api",
				Env: map[string]string{
					"DATABASE_URL": "postgres://user:${missing_secret}@localhost:5432/app",
				},
			},
		},
	}

	err := resolveSecrets(cfg)
	if err == nil {
		t.Fatal("expected error for unknown secret reference")
	}
	if !strings.Contains(err.Error(), `unknown secret "missing_secret"`) {
		t.Fatalf("expected unknown secret error, got %v", err)
	}
}
