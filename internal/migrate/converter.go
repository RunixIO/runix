package migrate

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/runixio/runix/pkg/types"
)

// Warning represents a migration warning for fields that could not be mapped.
type Warning struct {
	Process string // process name (empty for global)
	Field   string // PM2 field name
	Value   string // original value
	Message string // explanation
}

func (w Warning) String() string {
	prefix := ""
	if w.Process != "" {
		prefix = fmt.Sprintf("[%s] ", w.Process)
	}
	return fmt.Sprintf("[WARN] %s%s: %s", prefix, w.Field, w.Message)
}

// ConvertResult holds the conversion output.
type ConvertResult struct {
	Config   types.RunixConfig
	Warnings []Warning
}

// Convert converts a PM2 config to a Runix config.
func Convert(pm2 *PM2Config) *ConvertResult {
	result := &ConvertResult{
		Config: types.RunixConfig{
			Defaults: types.DefaultsConfig{
				RestartPolicy: types.RestartOnFailure,
			},
		},
	}

	hasBackoff := false
	for i := range pm2.Apps {
		app := &pm2.Apps[i]
		proc := convertApp(app, result)

		// First app with exp_backoff sets global default.
		if app.ExpBackoffDelay > 0 && !hasBackoff {
			result.Config.Defaults.BackoffBase = time.Duration(app.ExpBackoffDelay) * time.Millisecond
			hasBackoff = true
		}

		result.Config.Processes = append(result.Config.Processes, proc)
	}

	return result
}

// ConvertDump converts dump.pm2 entries to a Runix config.
func ConvertDump(entries []PM2DumpEntry) *ConvertResult {
	pm2 := &PM2Config{}
	for i := range entries {
		e := &entries[i]
		app := PM2AppConfig{
			Name:             e.Name,
			Script:           coalesceStr(e.Script, e.PmExecPath),
			Cwd:              coalesceStr(e.Cwd, e.PmCwd),
			Args:             e.Args,
			Env:              e.Env,
			ExecInterpreter:  e.ExecInterpreter,
			ExecMode:         e.ExecMode,
			Instances:        e.Instances,
			Watch:            e.PmWatch,
			MaxMemoryRestart: e.MaxMemoryRestart,
			MaxRestarts:      e.MaxRestarts,
			KillTimeout:      e.KillTimeout,
			CronRestart:      e.CronRestart,
			Namespace:        e.Namespace,
			NodeArgs:         e.NodeArgs,
		}
		if e.Autorestart {
			app.Autorestart = boolPtr(true)
		}
		pm2.Apps = append(pm2.Apps, app)
	}
	return Convert(pm2)
}

func convertApp(app *PM2AppConfig, result *ConvertResult) types.ProcessConfig {
	proc := types.ProcessConfig{
		Autostart: true,
		Instances: 1,
	}

	// Entrypoint: script first, then exec.
	proc.Entrypoint = coalesceStr(app.Script, app.Exec)

	// Name: from config or derive from entrypoint.
	proc.Name = app.Name
	if proc.Name == "" && proc.Entrypoint != "" {
		proc.Name = strings.TrimSuffix(filepath.Base(proc.Entrypoint), filepath.Ext(proc.Entrypoint))
	}

	// Cwd.
	proc.Cwd = app.Cwd

	// Args.
	proc.Args = NormalizeArgs(app.Args)

	// Node args: prepend to script args.
	if nodeArgs := NormalizeArgs(app.NodeArgs); len(nodeArgs) > 0 {
		proc.Args = append(nodeArgs, proc.Args...)
		result.Warnings = append(result.Warnings, Warning{
			Process: proc.Name,
			Field:   "node_args",
			Value:   fmt.Sprintf("%v", nodeArgs),
			Message: "prepended to args; set interpreter if needed",
		})
	}

	// Env.
	if len(app.Env) > 0 {
		proc.Env = app.Env
	}

	// Env variants -> Runix profiles.
	if app.EnvProduction != nil {
		result.addProfile("production", proc.Name, app.EnvProduction)
	}
	if app.EnvStaging != nil {
		result.addProfile("staging", proc.Name, app.EnvStaging)
	}

	// Interpreter -> runtime.
	interpreter := coalesceStr(app.ExecInterpreter, app.Interpreter)
	if interpreter != "" {
		rt := interpreterToRuntime(interpreter)
		if rt != "" && rt != "unknown" {
			proc.Runtime = rt
		} else if rt == "unknown" {
			proc.Interpreter = interpreter
			result.Warnings = append(result.Warnings, Warning{
				Process: proc.Name,
				Field:   "interpreter",
				Value:   interpreter,
				Message: "set as interpreter; runtime auto-detection may work better",
			})
		}
		// rt == "" means "none"/shell — let auto-detect handle it.
	}

	// Instances + exec_mode.
	switch {
	case app.Instances < 0, strings.EqualFold(fmt.Sprintf("%v", app.Instances), "max"):
		proc.Instances = runtime.NumCPU()
		result.Warnings = append(result.Warnings, Warning{
			Process: proc.Name,
			Field:   "instances",
			Value:   fmt.Sprintf("%d (max)", app.Instances),
			Message: fmt.Sprintf("PM2 'max' mapped to %d CPUs", runtime.NumCPU()),
		})
	case app.Instances == 0 && strings.EqualFold(app.ExecMode, "cluster_mode"):
		proc.Instances = runtime.NumCPU()
		result.Warnings = append(result.Warnings, Warning{
			Process: proc.Name,
			Field:   "instances",
			Value:   "0 (cluster_mode)",
			Message: fmt.Sprintf("cluster_mode with 0 instances mapped to %d CPUs", runtime.NumCPU()),
		})
	case app.Instances > 1:
		proc.Instances = app.Instances
	default:
		proc.Instances = 1
	}

	// Watch.
	if app.Watch != nil {
		wc := &types.WatchConfig{}
		switch v := app.Watch.(type) {
		case bool:
			wc.Enabled = v
		case []interface{}:
			wc.Enabled = true
			for _, p := range v {
				if s, ok := p.(string); ok {
					wc.Paths = append(wc.Paths, s)
				}
			}
		case string:
			wc.Enabled = strings.EqualFold(v, "true")
		}
		if len(app.IgnoreWatch) > 0 {
			wc.Ignore = app.IgnoreWatch
		}
		if app.WatchDelay > 0 {
			wc.Debounce = fmt.Sprintf("%dms", app.WatchDelay)
		}
		proc.Watch = wc
	}

	// Autorestart -> restart_policy.
	if app.Autorestart != nil {
		if *app.Autorestart {
			proc.RestartPolicy = types.RestartAlways
		} else {
			proc.RestartPolicy = types.RestartNever
		}
	}

	// MaxRestarts.
	if app.MaxRestarts > 0 {
		proc.MaxRestarts = app.MaxRestarts
	}

	// Max memory restart -> memory_limit.
	if app.MaxMemoryRestart != "" {
		proc.MemoryLimit = convertMemoryFormat(app.MaxMemoryRestart)
	}

	// Kill timeout -> stop_timeout.
	if app.KillTimeout > 0 {
		proc.StopTimeout = time.Duration(app.KillTimeout) * time.Millisecond
	}

	// Cron restart.
	if app.CronRestart != "" {
		proc.CronRestart = app.CronRestart
	}

	// Namespace.
	if app.Namespace != "" && app.Namespace != "default" {
		proc.Namespace = app.Namespace
	}

	// Autostart.
	if app.Autostart != nil {
		proc.Autostart = *app.Autostart
	}

	// Unmapped field warnings.
	emitUnmappedWarnings(app, proc.Name, result)

	return proc
}

func emitUnmappedWarnings(app *PM2AppConfig, name string, result *ConvertResult) {
	if app.UID != "" || app.GID != "" {
		result.Warnings = append(result.Warnings, Warning{
			Process: name,
			Field:   "uid/gid",
			Value:   fmt.Sprintf("uid=%s gid=%s", app.UID, app.GID),
			Message: "not supported; run the Runix daemon under the desired user",
		})
	}
	if app.OutFile != "" || app.ErrorFile != "" || app.LogFile != "" {
		result.Warnings = append(result.Warnings, Warning{
			Process: name,
			Field:   "log paths",
			Value:   fmt.Sprintf("out=%s err=%s combined=%s", app.OutFile, app.ErrorFile, app.LogFile),
			Message: "Runix manages logs internally (see data_dir/apps/<name>/)",
		})
	}
	if app.LogDateFormat != "" {
		result.Warnings = append(result.Warnings, Warning{
			Process: name,
			Field:   "log_date_format",
			Value:   app.LogDateFormat,
			Message: "not supported; use Runix structured logging",
		})
	}
	if app.MergeLogs {
		result.Warnings = append(result.Warnings, Warning{
			Process: name,
			Field:   "merge_logs",
			Value:   "true",
			Message: "not supported; Runix separates stdout/stderr by default",
		})
	}
	if app.MinUptime != nil {
		result.Warnings = append(result.Warnings, Warning{
			Process: name,
			Field:   "min_uptime",
			Value:   fmt.Sprintf("%v", app.MinUptime),
			Message: "no equivalent; consider healthcheck with grace_period",
		})
	}
	if app.ListenTimeout > 0 {
		result.Warnings = append(result.Warnings, Warning{
			Process: name,
			Field:   "listen_timeout",
			Value:   fmt.Sprintf("%dms", app.ListenTimeout),
			Message: "no equivalent; consider healthcheck with timeout",
		})
	}
	if app.WaitReady {
		result.Warnings = append(result.Warnings, Warning{
			Process: name,
			Field:   "wait_ready",
			Value:   "true",
			Message: "no equivalent in Runix",
		})
	}
	if len(app.StopExitCodes) > 0 {
		result.Warnings = append(result.Warnings, Warning{
			Process: name,
			Field:   "stop_exit_codes",
			Value:   fmt.Sprintf("%v", app.StopExitCodes),
			Message: "no equivalent in Runix",
		})
	}
}

// interpreterToRuntime maps PM2 interpreter values to Runix runtime names.
func interpreterToRuntime(interp string) string {
	lower := strings.ToLower(interp)
	switch {
	case lower == "node" || strings.HasPrefix(lower, "node"):
		return "node"
	case lower == "bun":
		return "bun"
	case lower == "deno":
		return "deno"
	case lower == "python" || lower == "python3" || lower == "python2":
		return "python"
	case lower == "ruby":
		return "ruby"
	case lower == "php":
		return "php"
	case lower == "none":
		return ""
	case lower == "bash" || lower == "sh" || lower == "/bin/bash" || lower == "/bin/sh":
		return ""
	default:
		return "unknown"
	}
}

// convertMemoryFormat converts PM2 memory format ("150M", "1G") to Runix format ("150MB", "1GB").
func convertMemoryFormat(pm2Mem string) string {
	s := strings.TrimSpace(pm2Mem)
	if len(s) == 0 {
		return ""
	}
	last := s[len(s)-1]
	switch last {
	case 'G', 'g':
		return s[:len(s)-1] + "GB"
	case 'M', 'm':
		return s[:len(s)-1] + "MB"
	case 'K', 'k':
		return s[:len(s)-1] + "KB"
	default:
		return s
	}
}

// addProfile adds environment variables to a Runix profile.
func (r *ConvertResult) addProfile(profile, processName string, env map[string]string) {
	if r.Config.Profiles == nil {
		r.Config.Profiles = make(map[string]map[string]map[string]string)
	}
	if r.Config.Profiles[profile] == nil {
		r.Config.Profiles[profile] = make(map[string]map[string]string)
	}
	r.Config.Profiles[profile][processName] = env
}
