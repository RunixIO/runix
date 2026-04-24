package config

import (
	"fmt"

	"github.com/runixio/runix/pkg/types"
)

// ResolveExtends deep-merges process configs that use the `extends` field.
// Child values override parent. Slices are replaced, not appended.
func ResolveExtends(configs []types.ProcessConfig) ([]types.ProcessConfig, error) {
	byName := make(map[string]*types.ProcessConfig)
	for i := range configs {
		byName[configs[i].Name] = &configs[i]
	}

	// Track resolution to detect cycles.
	resolving := make(map[string]bool)

	var resolve func(cfg *types.ProcessConfig) error
	resolve = func(cfg *types.ProcessConfig) error {
		if cfg.Extends == "" {
			return nil
		}

		if resolving[cfg.Name] {
			return fmt.Errorf("circular extends: %q extends %q which creates a cycle", cfg.Name, cfg.Extends)
		}
		resolving[cfg.Name] = true
		defer func() { resolving[cfg.Name] = false }()

		parent, ok := byName[cfg.Extends]
		if !ok {
			return fmt.Errorf("process %q extends %q which does not exist", cfg.Name, cfg.Extends)
		}

		// Resolve parent first (chain).
		if err := resolve(parent); err != nil {
			return err
		}

		// Merge: start with parent, overlay child.
		merged := *parent // copy parent

		// Override with child non-zero values.
		if cfg.Name != "" {
			merged.Name = cfg.Name
		}
		if cfg.Runtime != "" {
			merged.Runtime = cfg.Runtime
		}
		if cfg.Entrypoint != "" {
			merged.Entrypoint = cfg.Entrypoint
		}
		if len(cfg.Args) > 0 {
			merged.Args = cfg.Args
		}
		if cfg.Cwd != "" {
			merged.Cwd = cfg.Cwd
		}
		if len(cfg.Env) > 0 {
			if merged.Env == nil {
				merged.Env = make(map[string]string)
			}
			for k, v := range cfg.Env {
				merged.Env[k] = v
			}
		}
		if cfg.RestartPolicy != "" {
			merged.RestartPolicy = cfg.RestartPolicy
		}
		if cfg.AutoRestart != nil {
			merged.AutoRestart = cfg.AutoRestart
		}
		if cfg.MaxRestarts != 0 {
			merged.MaxRestarts = cfg.MaxRestarts
		}
		if cfg.MaxMemoryRestart != "" {
			merged.MaxMemoryRestart = cfg.MaxMemoryRestart
		}
		if cfg.RestartDelay != 0 {
			merged.RestartDelay = cfg.RestartDelay
		}
		if cfg.MinUptime != 0 {
			merged.MinUptime = cfg.MinUptime
		}
		if cfg.Instances != 0 {
			merged.Instances = cfg.Instances
		}
		if cfg.Namespace != "" {
			merged.Namespace = cfg.Namespace
		}
		if len(cfg.Tags) > 0 {
			merged.Tags = cfg.Tags
		}
		if cfg.Watch != nil {
			merged.Watch = cfg.Watch
		}
		if cfg.Hooks != nil {
			merged.Hooks = cfg.Hooks
		}
		if cfg.HealthCheck != nil {
			merged.HealthCheck = cfg.HealthCheck
		}
		if cfg.StopSignal != "" {
			merged.StopSignal = cfg.StopSignal
		}
		if cfg.StopTimeout != 0 {
			merged.StopTimeout = cfg.StopTimeout
		}
		if len(cfg.DependsOn) > 0 {
			merged.DependsOn = cfg.DependsOn
		}
		if cfg.Priority != 0 {
			merged.Priority = cfg.Priority
		}
		if cfg.CPUQuota != "" {
			merged.CPUQuota = cfg.CPUQuota
		}
		if cfg.MemoryLimit != "" {
			merged.MemoryLimit = cfg.MemoryLimit
		}

		// Clear the extends field to avoid re-resolution.
		merged.Extends = ""
		*cfg = merged
		return nil
	}

	result := make([]types.ProcessConfig, len(configs))
	for i := range configs {
		result[i] = configs[i]
		if err := resolve(&result[i]); err != nil {
			return nil, err
		}
	}
	return result, nil
}
