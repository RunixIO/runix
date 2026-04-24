package config

import (
	"fmt"
	"reflect"

	"github.com/runixio/runix/pkg/types"
)

// ConfigDiff describes the differences between two configurations.
type ConfigDiff struct {
	Added    []types.ProcessConfig
	Removed  []string
	Modified []ProcessConfigDiff
}

// ProcessConfigDiff describes changes to a single process config.
type ProcessConfigDiff struct {
	Name    string
	Changed []string
}

// DiffConfigs compares old and new configurations and returns the diff.
func DiffConfigs(old, new *types.RunixConfig) ConfigDiff {
	var diff ConfigDiff

	oldByName := make(map[string]types.ProcessConfig)
	for _, p := range old.Processes {
		oldByName[p.Name] = p
	}

	newByName := make(map[string]types.ProcessConfig)
	for _, p := range new.Processes {
		newByName[p.Name] = p
	}

	// Find added and modified.
	for _, p := range new.Processes {
		if _, ok := oldByName[p.Name]; !ok {
			diff.Added = append(diff.Added, p)
		} else {
			changed := diffProcess(oldByName[p.Name], p)
			if len(changed) > 0 {
				diff.Modified = append(diff.Modified, ProcessConfigDiff{
					Name:    p.Name,
					Changed: changed,
				})
			}
		}
	}

	// Find removed.
	for _, p := range old.Processes {
		if _, ok := newByName[p.Name]; !ok {
			diff.Removed = append(diff.Removed, p.Name)
		}
	}

	return diff
}

// diffProcess compares two process configs and returns field names that changed.
func diffProcess(a, b types.ProcessConfig) []string {
	var changed []string

	if a.Entrypoint != b.Entrypoint {
		changed = append(changed, "entrypoint")
	}
	if a.Runtime != b.Runtime {
		changed = append(changed, "runtime")
	}
	if !reflect.DeepEqual(a.Args, b.Args) {
		changed = append(changed, "args")
	}
	if a.Cwd != b.Cwd {
		changed = append(changed, "cwd")
	}
	if !reflect.DeepEqual(a.Env, b.Env) {
		changed = append(changed, "env")
	}
	if a.RestartPolicy != b.RestartPolicy {
		changed = append(changed, "restart_policy")
	}
	if !reflect.DeepEqual(a.AutoRestart, b.AutoRestart) {
		changed = append(changed, "autorestart")
	}
	if a.MaxRestarts != b.MaxRestarts {
		changed = append(changed, "max_restarts")
	}
	if a.MaxMemoryRestart != b.MaxMemoryRestart {
		changed = append(changed, "max_memory_restart")
	}
	if a.RestartDelay != b.RestartDelay {
		changed = append(changed, "restart_delay")
	}
	if a.MinUptime != b.MinUptime {
		changed = append(changed, "min_uptime")
	}
	if a.Instances != b.Instances {
		changed = append(changed, "instances")
	}
	if a.Namespace != b.Namespace {
		changed = append(changed, "namespace")
	}
	if !reflect.DeepEqual(a.DependsOn, b.DependsOn) {
		changed = append(changed, "depends_on")
	}
	if a.Priority != b.Priority {
		changed = append(changed, "priority")
	}
	if a.CPUQuota != b.CPUQuota {
		changed = append(changed, "cpu_quota")
	}
	if a.MemoryLimit != b.MemoryLimit {
		changed = append(changed, "memory_limit")
	}
	if a.HealthCheckURL != b.HealthCheckURL {
		changed = append(changed, "healthcheck_url")
	}
	if a.StopSignal != b.StopSignal {
		changed = append(changed, "stop_signal")
	}
	if a.StopTimeout != b.StopTimeout {
		changed = append(changed, "stop_timeout")
	}
	if !reflect.DeepEqual(a.Tags, b.Tags) {
		changed = append(changed, "tags")
	}

	return changed
}

// String returns a human-readable summary of the diff.
func (d ConfigDiff) String() string {
	if len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Modified) == 0 {
		return "no changes"
	}

	var parts []string
	for _, p := range d.Added {
		parts = append(parts, fmt.Sprintf("+ %s", p.Name))
	}
	for _, name := range d.Removed {
		parts = append(parts, fmt.Sprintf("- %s", name))
	}
	for _, m := range d.Modified {
		parts = append(parts, fmt.Sprintf("~ %s (%s)", m.Name, fmt.Sprintf("%v", m.Changed)))
	}

	s := ""
	for _, p := range parts {
		s += p + "\n"
	}
	return s
}
