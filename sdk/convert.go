package sdk

import (
	"fmt"
	"os"

	"github.com/runixio/runix/internal/runtime"
	"github.com/runixio/runix/pkg/types"
)

// toTypesConfig converts an SDK ProcessConfig into the internal types.ProcessConfig.
// It resolves the runtime adapter when Runtime is set, deriving the correct
// interpreter and entrypoint automatically.
func toTypesConfig(cfg ProcessConfig) (types.ProcessConfig, error) {
	// Resolve entrypoint: Script and Binary both map to Entrypoint.
	entrypoint := cfg.Script
	if entrypoint == "" {
		entrypoint = cfg.Binary
	}

	// Resolve runtime if specified.
	resolvedRuntime := cfg.Runtime
	interpreter := cfg.Interpreter
	finalEntrypoint := entrypoint
	finalArgs := cfg.Args

	if resolvedRuntime != "" && resolvedRuntime != "auto" {
		detector := runtime.NewDetector()
		rt, err := detector.Get(resolvedRuntime)
		if err != nil {
			return types.ProcessConfig{}, fmt.Errorf("sdk: unknown runtime %q: %w", resolvedRuntime, err)
		}

		// Resolve working directory for detection.
		cwd := cfg.Cwd
		if cwd == "" {
			cwd, _ = os.Getwd()
		}

		resolvedCmd, cmdErr := rt.StartCmd(runtime.StartOptions{
			Entrypoint:  entrypoint,
			Args:        cfg.Args,
			Cwd:         cwd,
			Env:         cfg.Env,
			Interpreter: cfg.Interpreter,
			UseBundle:   cfg.UseBundle,
		})
		if cmdErr == nil && resolvedCmd != nil {
			finalEntrypoint = resolvedCmd.Path
			if len(resolvedCmd.Args) > 1 {
				finalArgs = resolvedCmd.Args[1:]
			}
		}
	}

	if resolvedRuntime == "" || resolvedRuntime == "auto" {
		// Auto-detect from cwd.
		cwd := cfg.Cwd
		if cwd == "" {
			cwd, _ = os.Getwd()
		}
		detector := runtime.NewDetector()
		if rt, err := detector.Detect(cwd); err == nil {
			resolvedRuntime = rt.Name()
			if entrypoint != "" {
				resolvedCmd, cmdErr := rt.StartCmd(runtime.StartOptions{
					Entrypoint:  entrypoint,
					Args:        cfg.Args,
					Cwd:         cwd,
					Env:         cfg.Env,
					Interpreter: cfg.Interpreter,
					UseBundle:   cfg.UseBundle,
				})
				if cmdErr == nil && resolvedCmd != nil {
					finalEntrypoint = resolvedCmd.Path
					if len(resolvedCmd.Args) > 1 {
						finalArgs = resolvedCmd.Args[1:]
					}
				}
			}
		} else {
			resolvedRuntime = "unknown"
		}
	}

	// If user specified an explicit interpreter but no runtime resolution happened,
	// the supervisor's buildArgs() will handle it at start time.
	if interpreter != "" && finalEntrypoint == "" {
		finalEntrypoint = entrypoint
	}

	return types.ProcessConfig{
		Name:          cfg.Name,
		Runtime:       resolvedRuntime,
		Entrypoint:    finalEntrypoint,
		Args:          finalArgs,
		Cwd:           cfg.Cwd,
		Env:           cfg.Env,
		Interpreter:   interpreter,
		UseBundle:     cfg.UseBundle,
		RestartPolicy: types.RestartPolicy(cfg.RestartPolicy),
		MaxRestarts:   cfg.MaxRestarts,
		RestartWindow: cfg.RestartWindow,
		Autostart:     cfg.Autostart,
		Watch:         cfg.Watch.toTypes(),
		Instances:     cfg.Instances,
		Namespace:     cfg.Namespace,
		Labels:        cfg.Labels,
		Tags:          cfg.Tags,
		StopSignal:    cfg.StopSignal,
		StopTimeout:   cfg.StopTimeout,
		Hooks:         cfg.Hooks.toTypes(),
		DependsOn:     cfg.DependsOn,
		Priority:      cfg.Priority,
		HealthCheck:   cfg.HealthCheck.toTypes(),
	}, nil
}

// toSDKProcessInfo converts internal types.ProcessInfo to SDK ProcessInfo.
func toSDKProcessInfo(info types.ProcessInfo) ProcessInfo {
	return ProcessInfo{
		ID:         info.ID,
		NumericID:  info.NumericID,
		Name:       info.Name,
		Namespace:  info.Namespace,
		Runtime:    info.Runtime,
		State:      string(info.State),
		PID:        info.PID,
		ExitCode:   info.ExitCode,
		Restarts:   info.Restarts,
		CreatedAt:  info.CreatedAt,
		StartedAt:  info.StartedAt,
		FinishedAt: info.FinishedAt,
		Uptime:     info.Uptime,
		CPUPercent: info.CPUPercent,
		MemBytes:   info.MemBytes,
		MemPercent: info.MemPercent,
		Threads:    info.Threads,
		FDs:        info.FDs,
		Tags:       info.Tags,
		Config:     toSDKProcessConfig(info.Config),
	}
}

// toSDKProcessConfig converts internal types.ProcessConfig to SDK ProcessConfig.
func toSDKProcessConfig(tc types.ProcessConfig) ProcessConfig {
	return ProcessConfig{
		Name:          tc.Name,
		Runtime:       tc.Runtime,
		Script:        tc.Entrypoint,
		Binary:        tc.Entrypoint,
		Interpreter:   tc.Interpreter,
		UseBundle:     tc.UseBundle,
		Args:          tc.Args,
		Cwd:           tc.Cwd,
		Env:           tc.Env,
		Autostart:     tc.Autostart,
		RestartPolicy: string(tc.RestartPolicy),
		MaxRestarts:   tc.MaxRestarts,
		RestartWindow: tc.RestartWindow,
		StopSignal:    tc.StopSignal,
		StopTimeout:   tc.StopTimeout,
		Watch:         toSDKWatch(tc.Watch),
		HealthCheck:   toSDKHealthCheck(tc.HealthCheck),
		Hooks:         toSDKHooks(tc.Hooks),
		Instances:     tc.Instances,
		Namespace:     tc.Namespace,
		Labels:        tc.Labels,
		Tags:          tc.Tags,
		DependsOn:     tc.DependsOn,
		Priority:      tc.Priority,
	}
}

// toTypesDefaults converts SDK DefaultsConfig to types.DefaultsConfig.
func toTypesDefaults(d DefaultsConfig) types.DefaultsConfig {
	return types.DefaultsConfig{
		RestartPolicy: types.RestartPolicy(d.RestartPolicy),
		MaxRestarts:   d.MaxRestarts,
		RestartWindow: d.RestWindow,
	}
}
