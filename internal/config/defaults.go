package config

import (
	"time"

	"github.com/runixio/runix/pkg/types"
)

// ApplyDefaults fills in zero values with sensible defaults.
func ApplyDefaults(cfg *types.RunixConfig) {
	defaultAutoRestart := true

	if cfg.Daemon.LogLevel == "" {
		cfg.Daemon.LogLevel = "info"
	}

	if cfg.Defaults.RestartPolicy == "" {
		cfg.Defaults.RestartPolicy = types.RestartOnFailure
	}
	if cfg.Defaults.AutoRestart == nil {
		cfg.Defaults.AutoRestart = &defaultAutoRestart
	}
	if cfg.Defaults.MaxRestarts == 0 {
		cfg.Defaults.MaxRestarts = 10
	}
	if cfg.Defaults.RestartWindow == 0 {
		cfg.Defaults.RestartWindow = 60 * time.Second
	}
	if cfg.Defaults.BackoffBase == 0 {
		cfg.Defaults.BackoffBase = 1 * time.Second
	}
	if cfg.Defaults.BackoffMax == 0 {
		cfg.Defaults.BackoffMax = 60 * time.Second
	}
	if cfg.Defaults.LogMaxSize == 0 {
		cfg.Defaults.LogMaxSize = 10 * 1024 * 1024 // 10 MB
	}
	if cfg.Defaults.LogMaxAge == 0 {
		cfg.Defaults.LogMaxAge = 168 * time.Hour // 7 days
	}
	if cfg.Defaults.WatchDebounce == 0 {
		cfg.Defaults.WatchDebounce = 100 * time.Millisecond
	}

	if cfg.Web.Listen == "" {
		cfg.Web.Listen = "localhost:9615"
	}

	if cfg.MCP.Transport == "" {
		cfg.MCP.Transport = "stdio"
	}

	// Security auth defaults: disabled by default for local dev simplicity.
	if cfg.Security.Auth.Mode == "" {
		cfg.Security.Auth.Mode = types.AuthModeDisabled
	}

	for i := range cfg.Processes {
		applyProcessDefaults(&cfg.Processes[i], cfg.Defaults)
	}
}

// applyProcessDefaults fills defaults for a single process config.
func applyProcessDefaults(cfg *types.ProcessConfig, defaults types.DefaultsConfig) {
	// If runtime is empty, leave it (will be auto-detected).
	if cfg.RestartPolicy == "" {
		cfg.RestartPolicy = defaults.RestartPolicy
	}
	if cfg.AutoRestart == nil {
		cfg.AutoRestart = defaults.AutoRestart
	}
	if cfg.MaxRestarts == 0 {
		cfg.MaxRestarts = defaults.MaxRestarts
	}
	if cfg.MaxMemoryRestart == "" {
		cfg.MaxMemoryRestart = defaults.MaxMemoryRestart
	}
	if cfg.RestartDelay == 0 {
		cfg.RestartDelay = defaults.RestartDelay
	}
	if cfg.MinUptime == 0 {
		cfg.MinUptime = defaults.MinUptime
	}
	if cfg.Instances == 0 {
		cfg.Instances = 1
	}
}
