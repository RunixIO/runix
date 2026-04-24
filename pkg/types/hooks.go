package types

import "time"

// HookConfig defines a single lifecycle hook command.
type HookConfig struct {
	// Command is the shell command to execute (run via sh -c).
	Command string `json:"command" yaml:"command"`
	// Timeout is the maximum duration the hook may run. Default: 30s.
	Timeout time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	// IgnoreFailure skips the hook failure blocking the lifecycle action.
	// When true, hook errors are logged but don't abort the operation.
	IgnoreFailure bool `json:"ignore_failure,omitempty" yaml:"ignore_failure,omitempty"`
}

// ProcessHooks groups lifecycle hooks by event.
// Pre-hooks can block the lifecycle action on failure.
// Post-hooks are informational — failures are logged but don't undo the action.
type ProcessHooks struct {
	PreStart        *HookConfig `json:"pre_start,omitempty" yaml:"pre_start,omitempty"`
	PostStart       *HookConfig `json:"post_start,omitempty" yaml:"post_start,omitempty"`
	PreStop         *HookConfig `json:"pre_stop,omitempty" yaml:"pre_stop,omitempty"`
	PostStop        *HookConfig `json:"post_stop,omitempty" yaml:"post_stop,omitempty"`
	PreRestart      *HookConfig `json:"pre_restart,omitempty" yaml:"pre_restart,omitempty"`
	PostRestart     *HookConfig `json:"post_restart,omitempty" yaml:"post_restart,omitempty"`
	PreReload       *HookConfig `json:"pre_reload,omitempty" yaml:"pre_reload,omitempty"`
	PostReload      *HookConfig `json:"post_reload,omitempty" yaml:"post_reload,omitempty"`
	PreHealthCheck  *HookConfig `json:"pre_healthcheck,omitempty" yaml:"pre_healthcheck,omitempty"`
	PostHealthCheck *HookConfig `json:"post_healthcheck,omitempty" yaml:"post_healthcheck,omitempty"`
}
