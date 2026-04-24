// Package sdk provides an embeddable Go SDK for Runix process management.
//
// Use this package to embed Runix's process supervisor directly into your Go
// application without requiring the CLI binary. The SDK wraps the core
// supervisor and exposes a developer-friendly API for managing process
// lifecycles.
//
// Basic usage:
//
//	mgr, err := sdk.New(sdk.Config{LogDir: "/tmp/runix"})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer mgr.Close()
//
//	id, err := mgr.AddProcess(ctx, sdk.ProcessConfig{
//	    Name:   "api",
//	    Script: "main.py",
//	    Runtime: "python",
//	})
package sdk

import (
	"time"

	"github.com/runixio/runix/pkg/types"
)

// Config configures the SDK manager.
type Config struct {
	// LogDir is the directory for process logs and state persistence.
	// Defaults to OS temp dir + "/runix" if empty.
	LogDir string

	// Defaults applied to all processes unless overridden.
	Defaults DefaultsConfig
}

// DefaultsConfig holds default values for process configuration.
type DefaultsConfig struct {
	// RestartPolicy: "always", "on-failure", or "never".
	RestartPolicy string
	// MaxRestarts is the maximum number of restarts within the window.
	MaxRestarts int
	// RestartWindow is the time window for counting restarts.
	RestWindow time.Duration
	// StopTimeout is the default grace period before force-killing.
	StopTimeout time.Duration
}

// ProcessConfig describes a process to be managed by Runix.
// Use Script for interpreted files (Python, Ruby, etc.) or Binary for
// compiled executables. Both map to the underlying entrypoint field.
type ProcessConfig struct {
	// Name is the unique process identifier (required).
	Name string

	// Script is the path to an interpreted script file.
	// Maps to the internal entrypoint. Use this for Python, Ruby, PHP, etc.
	Script string

	// Binary is the path to a compiled executable or command.
	// Maps to the internal entrypoint. Use this for Go binaries or direct commands.
	Binary string

	// Runtime specifies the language runtime: "go", "python", "node", "bun",
	// "deno", "ruby", "php", or "auto" for auto-detection. When set, the SDK
	// resolves the correct interpreter and entrypoint automatically.
	Runtime string

	// Interpreter is an explicit interpreter override (e.g. "/usr/bin/python3.11").
	// When set, the process is launched as: interpreter entrypoint [args...].
	Interpreter string

	// UseBundle wraps the command with "bundle exec" (primarily Ruby).
	UseBundle bool

	// Args are additional arguments passed to the process.
	Args []string

	// Cwd is the working directory for the process.
	Cwd string

	// Env is environment variables overlaid on the current process environment.
	Env map[string]string

	// Autostart determines if the process starts automatically on resurrect.
	Autostart bool

	// RestartPolicy: "always", "on-failure", or "never".
	RestartPolicy string

	// MaxRestarts limits automatic restarts (0 = use default).
	MaxRestarts int

	// RestartWindow is the time window for counting restarts.
	RestartWindow time.Duration

	// StopSignal is the signal sent to stop the process (default: SIGTERM).
	StopSignal string

	// StopTimeout is the grace period before force-killing (default: 5s).
	StopTimeout time.Duration

	// Watch enables file watching and auto-restart on changes.
	Watch *WatchConfig

	// HealthCheck configures health checking.
	HealthCheck *HealthCheckConfig

	// Hooks defines lifecycle hook commands.
	Hooks *HooksConfig

	// Instances is the number of copies to run (default: 1).
	Instances int

	// Namespace groups processes.
	Namespace string

	// Labels are key-value pairs for filtering.
	Labels map[string]string

	// Tags are string tags for categorization.
	Tags []string

	// DependsOn lists process names this process depends on.
	DependsOn []string

	// Priority controls startup order (lower starts first).
	Priority int
}

// WatchConfig holds file watching configuration.
type WatchConfig struct {
	Enabled  bool
	Paths    []string
	Ignore   []string
	Debounce string
}

// HealthCheckConfig configures health checking for a process.
type HealthCheckConfig struct {
	Type        string // "http", "tcp", or "command"
	URL         string
	TCPEndpoint string
	Command     string
	Interval    string
	Timeout     string
	Retries     int
	GracePeriod string
}

// HookConfig defines a single lifecycle hook command.
type HookConfig struct {
	Command       string
	Timeout       time.Duration
	IgnoreFailure bool
}

// HooksConfig groups lifecycle hooks by event.
type HooksConfig struct {
	PreStart        *HookConfig
	PostStart       *HookConfig
	PreStop         *HookConfig
	PostStop        *HookConfig
	PreRestart      *HookConfig
	PostRestart     *HookConfig
	PreReload       *HookConfig
	PostReload      *HookConfig
	PreHealthCheck  *HookConfig
	PostHealthCheck *HookConfig
}

// ProcessInfo holds a snapshot of process state.
type ProcessInfo struct {
	ID         string
	NumericID  int
	Name       string
	Namespace  string
	Runtime    string
	State      string
	PID        int
	ExitCode   int
	Restarts   int
	CreatedAt  time.Time
	StartedAt  *time.Time
	FinishedAt *time.Time
	Uptime     time.Duration
	CPUPercent float64
	MemBytes   int64
	MemPercent float64
	Threads    int
	FDs        int
	Config     ProcessConfig
	Tags       []string
}

// LogOptions controls log reading behavior.
type LogOptions struct {
	// Tail shows the last N lines of the log (0 = all).
	Tail int
	// Follow streams new log entries as they are written.
	Follow bool
	// Stderr reads stderr instead of stdout.
	Stderr bool
}

// toTypesWatch converts an SDK WatchConfig to types.WatchConfig.
func (w *WatchConfig) toTypes() *types.WatchConfig {
	if w == nil {
		return nil
	}
	return &types.WatchConfig{
		Enabled:  w.Enabled,
		Paths:    w.Paths,
		Ignore:   w.Ignore,
		Debounce: w.Debounce,
	}
}

// toSDKWatch converts types.WatchConfig to SDK WatchConfig.
func toSDKWatch(tc *types.WatchConfig) *WatchConfig {
	if tc == nil {
		return nil
	}
	return &WatchConfig{
		Enabled:  tc.Enabled,
		Paths:    tc.Paths,
		Ignore:   tc.Ignore,
		Debounce: tc.Debounce,
	}
}

// toTypesHealthCheck converts an SDK HealthCheckConfig to types.HealthCheckConfig.
func (h *HealthCheckConfig) toTypes() *types.HealthCheckConfig {
	if h == nil {
		return nil
	}
	return &types.HealthCheckConfig{
		Type:        types.HealthCheckType(h.Type),
		URL:         h.URL,
		TCPEndpoint: h.TCPEndpoint,
		Command:     h.Command,
		Interval:    h.Interval,
		Timeout:     h.Timeout,
		Retries:     h.Retries,
		GracePeriod: h.GracePeriod,
	}
}

// toSDKHealthCheck converts types.HealthCheckConfig to SDK HealthCheckConfig.
func toSDKHealthCheck(tc *types.HealthCheckConfig) *HealthCheckConfig {
	if tc == nil {
		return nil
	}
	return &HealthCheckConfig{
		Type:        string(tc.Type),
		URL:         tc.URL,
		TCPEndpoint: tc.TCPEndpoint,
		Command:     tc.Command,
		Interval:    tc.Interval,
		Timeout:     tc.Timeout,
		Retries:     tc.Retries,
		GracePeriod: tc.GracePeriod,
	}
}

// toTypesHook converts an SDK HookConfig to types.HookConfig.
func (h *HookConfig) toTypes() *types.HookConfig {
	if h == nil {
		return nil
	}
	return &types.HookConfig{
		Command:       h.Command,
		Timeout:       h.Timeout,
		IgnoreFailure: h.IgnoreFailure,
	}
}

// toSDKHook converts types.HookConfig to SDK HookConfig.
func toSDKHook(tc *types.HookConfig) *HookConfig {
	if tc == nil {
		return nil
	}
	return &HookConfig{
		Command:       tc.Command,
		Timeout:       tc.Timeout,
		IgnoreFailure: tc.IgnoreFailure,
	}
}

// toTypesHooks converts SDK HooksConfig to types.ProcessHooks.
func (h *HooksConfig) toTypes() *types.ProcessHooks {
	if h == nil {
		return nil
	}
	return &types.ProcessHooks{
		PreStart:        h.PreStart.toTypes(),
		PostStart:       h.PostStart.toTypes(),
		PreStop:         h.PreStop.toTypes(),
		PostStop:        h.PostStop.toTypes(),
		PreRestart:      h.PreRestart.toTypes(),
		PostRestart:     h.PostRestart.toTypes(),
		PreReload:       h.PreReload.toTypes(),
		PostReload:      h.PostReload.toTypes(),
		PreHealthCheck:  h.PreHealthCheck.toTypes(),
		PostHealthCheck: h.PostHealthCheck.toTypes(),
	}
}

// toSDKHooks converts types.ProcessHooks to SDK HooksConfig.
func toSDKHooks(tc *types.ProcessHooks) *HooksConfig {
	if tc == nil {
		return nil
	}
	return &HooksConfig{
		PreStart:        toSDKHook(tc.PreStart),
		PostStart:       toSDKHook(tc.PostStart),
		PreStop:         toSDKHook(tc.PreStop),
		PostStop:        toSDKHook(tc.PostStop),
		PreRestart:      toSDKHook(tc.PreRestart),
		PostRestart:     toSDKHook(tc.PostRestart),
		PreReload:       toSDKHook(tc.PreReload),
		PostReload:      toSDKHook(tc.PostReload),
		PreHealthCheck:  toSDKHook(tc.PreHealthCheck),
		PostHealthCheck: toSDKHook(tc.PostHealthCheck),
	}
}
