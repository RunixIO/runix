package types

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ProcessState represents the current state of a managed process.
type ProcessState string

const (
	StateStarting ProcessState = "starting"
	StateRunning  ProcessState = "running"
	StateStopping ProcessState = "stopping"
	StateStopped  ProcessState = "stopped"
	StateCrashed  ProcessState = "crashed"
	StateErrored  ProcessState = "errored"
	StateWaiting  ProcessState = "waiting"
)

// ValidTransitions defines allowed state transitions.
var ValidTransitions = map[ProcessState][]ProcessState{
	StateStarting: {StateRunning, StateStopping, StateErrored},
	StateRunning:  {StateStopping, StateCrashed},
	StateStopping: {StateStopped},
	StateStopped:  {StateStarting},
	StateCrashed:  {StateWaiting, StateStopped, StateStarting},
	StateWaiting:  {StateStarting, StateStopped},
	StateErrored:  {StateStopped, StateStarting},
}

// RestartPolicy defines when a process should be restarted.
type RestartPolicy string

const (
	RestartAlways    RestartPolicy = "always"
	RestartOnFailure RestartPolicy = "on-failure"
	RestartNever     RestartPolicy = "never"
)

// ProcessConfig holds the configuration for a managed process.
type ProcessConfig struct {
	Name        string            `json:"name" yaml:"name"`
	Runtime     string            `json:"runtime" yaml:"runtime"`
	Entrypoint  string            `json:"entrypoint" yaml:"entrypoint"`
	Args        []string          `json:"args,omitempty" yaml:"args,omitempty"`
	Cwd         string            `json:"cwd" yaml:"cwd"`
	Env         map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	Interpreter string            `json:"interpreter,omitempty" yaml:"interpreter,omitempty"`
	// UseBundle wraps the entrypoint with "bundle exec" (primarily Ruby).
	UseBundle     bool              `json:"use_bundle,omitempty" yaml:"use_bundle,omitempty"`
	RestartPolicy RestartPolicy     `json:"restart_policy" yaml:"restart_policy"`
	AutoRestart   *bool             `json:"autorestart,omitempty" yaml:"autorestart,omitempty"`
	MaxRestarts   int               `json:"max_restarts" yaml:"max_restarts"`
	RestartWindow time.Duration     `json:"restart_window" yaml:"restart_window"`
	RestartDelay  time.Duration     `json:"restart_delay,omitempty" yaml:"restart_delay,omitempty"`
	MinUptime     time.Duration     `json:"min_uptime,omitempty" yaml:"min_uptime,omitempty"`
	Autostart     bool              `json:"autostart" yaml:"autostart"`
	Watch         *WatchConfig      `json:"watch,omitempty" yaml:"watch,omitempty"`
	Instances     int               `json:"instances,omitempty" yaml:"instances,omitempty"`
	Namespace     string            `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Labels        map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Tags          []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	// InstanceIndex is the zero-based index for multi-instance processes (e.g., api:0, api:1).
	InstanceIndex int `json:"instance_index,omitempty" yaml:"instance_index,omitempty"`
	// HealthCheckURL is an HTTP endpoint for health checking.
	HealthCheckURL string `json:"healthcheck_url,omitempty" yaml:"healthcheck_url,omitempty"`
	// CronRestart is a cron expression for scheduled restarts.
	CronRestart string `json:"cron_restart,omitempty" yaml:"cron_restart,omitempty"`
	// StopSignal is the signal sent to stop the process (default: SIGTERM).
	StopSignal string `json:"stop_signal,omitempty" yaml:"stop_signal,omitempty"`
	// StopTimeout is the grace period before force-killing (default: 5s).
	StopTimeout time.Duration `json:"stop_timeout,omitempty" yaml:"stop_timeout,omitempty"`
	// Hooks defines lifecycle hook commands for this process.
	Hooks *ProcessHooks `json:"hooks,omitempty" yaml:"hooks,omitempty"`
	// DependsOn lists process names this process depends on.
	DependsOn []string `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	// Priority controls startup order (lower starts first).
	Priority int `json:"priority,omitempty" yaml:"priority,omitempty"`
	// Extends inherits config from another named process.
	Extends string `json:"extends,omitempty" yaml:"extends,omitempty"`
	// CPUQuota limits CPU usage (e.g. "50%", "0.5").
	CPUQuota string `json:"cpu_quota,omitempty" yaml:"cpu_quota,omitempty"`
	// MemoryLimit limits memory usage (e.g. "512MB", "1GB").
	MemoryLimit string `json:"memory_limit,omitempty" yaml:"memory_limit,omitempty"`
	// MaxMemoryRestart restarts the process when memory usage exceeds this threshold.
	MaxMemoryRestart string `json:"max_memory_restart,omitempty" yaml:"max_memory_restart,omitempty"`
	// HealthCheck configures health checking for this process.
	HealthCheck *HealthCheckConfig `json:"healthcheck,omitempty" yaml:"healthcheck,omitempty"`
	// WaitReady delays the transition to running until readiness succeeds.
	WaitReady bool `json:"wait_ready,omitempty" yaml:"wait_ready,omitempty"`
	// ListenTimeout is the maximum time to wait for readiness before failing startup.
	ListenTimeout time.Duration `json:"listen_timeout,omitempty" yaml:"listen_timeout,omitempty"`
	// LogMaxFiles is the maximum number of rotated log files to keep.
	LogMaxFiles int `json:"log_max_files,omitempty" yaml:"log_max_files,omitempty"`
}

// Validate checks the process config for errors.
func (c *ProcessConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("process name is required")
	}
	if c.Entrypoint == "" {
		return fmt.Errorf("entrypoint is required for process %q", c.Name)
	}
	if c.RestartPolicy != "" &&
		c.RestartPolicy != RestartAlways &&
		c.RestartPolicy != RestartOnFailure &&
		c.RestartPolicy != RestartNever {
		return fmt.Errorf("invalid restart_policy %q for process %q", c.RestartPolicy, c.Name)
	}
	if c.RestartDelay < 0 {
		return fmt.Errorf("restart_delay must be >= 0 for process %q", c.Name)
	}
	if c.MinUptime < 0 {
		return fmt.Errorf("min_uptime must be >= 0 for process %q", c.Name)
	}
	if c.ListenTimeout < 0 {
		return fmt.Errorf("listen_timeout must be >= 0 for process %q", c.Name)
	}
	if c.MaxMemoryRestart != "" {
		if _, err := ParseMemorySize(c.MaxMemoryRestart); err != nil {
			return fmt.Errorf("invalid max_memory_restart for process %q: %w", c.Name, err)
		}
	}
	return nil
}

// ParseMemorySize parses a memory size like "512MB", "1GB", "1024KB", or bytes.
func ParseMemorySize(s string) (int64, error) {
	orig := s
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return 0, fmt.Errorf("memory size is empty")
	}

	mult := int64(1)
	switch {
	case strings.HasSuffix(s, "GB"):
		mult = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "GB")
	case strings.HasSuffix(s, "MB"):
		mult = 1024 * 1024
		s = strings.TrimSuffix(s, "MB")
	case strings.HasSuffix(s, "KB"):
		mult = 1024
		s = strings.TrimSuffix(s, "KB")
	case strings.HasSuffix(s, "B"):
		s = strings.TrimSuffix(s, "B")
	}

	val, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %q: %w", orig, err)
	}
	if val <= 0 {
		return 0, fmt.Errorf("memory size must be > 0")
	}
	return val * mult, nil
}

// FullName returns the fully qualified process name including namespace and
// instance index. Examples: "api", "backend/api", "api:0", "backend/api:1".
func (c *ProcessConfig) FullName() string {
	name := c.Name
	if c.Namespace != "" {
		name = c.Namespace + "/" + name
	}
	if c.Instances > 1 && c.InstanceIndex >= 0 {
		name = fmt.Sprintf("%s:%d", name, c.InstanceIndex)
	}
	return name
}

// ProcessInfo holds runtime information about a managed process.
type ProcessInfo struct {
	ID            string        `json:"id"`
	NumericID     int           `json:"numeric_id"`
	Name          string        `json:"name"`
	Namespace     string        `json:"namespace,omitempty"`
	InstanceIndex int           `json:"instance_index,omitempty"`
	Runtime       string        `json:"runtime"`
	State         ProcessState  `json:"state"`
	Ready         bool          `json:"ready"`
	PID           int           `json:"pid"`
	ExitCode      int           `json:"exit_code,omitempty"`
	Restarts      int           `json:"restarts"`
	CreatedAt     time.Time     `json:"created_at"`
	StartedAt     *time.Time    `json:"started_at,omitempty"`
	FinishedAt    *time.Time    `json:"finished_at,omitempty"`
	Config        ProcessConfig `json:"config"`
	LastEvent     string        `json:"last_event,omitempty"`
	LastReason    string        `json:"last_reason,omitempty"`
	CPUPercent    float64       `json:"cpu_percent"`
	MemBytes      int64         `json:"memory_bytes"`
	MemPercent    float64       `json:"mem_percent"`
	Threads       int           `json:"threads"`
	FDs           int           `json:"fds"`
	Tags          []string      `json:"tags,omitempty"`
	Uptime        time.Duration `json:"uptime,omitempty"`
}

// UptimeString returns a human-readable uptime.
func (p *ProcessInfo) UptimeString() string {
	if p.StartedAt == nil {
		return ""
	}
	d := time.Since(*p.StartedAt)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
