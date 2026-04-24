package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/runixio/runix/internal/daemon"
	"github.com/runixio/runix/internal/metrics"
	"github.com/runixio/runix/internal/runtime"
	"github.com/runixio/runix/internal/supervisor"
	"github.com/runixio/runix/pkg/types"
	"github.com/spf13/cobra"
)

func newStartCmd() *cobra.Command {
	var (
		onlyFlag    string
		name        string
		runtimeName string
		workingDir  string
		envVars     []string
		watch       bool
		watchIgnore []string
		restartPol  string
		maxRestarts int
		instances   int
		namespace   string
		useBundle   bool
	)

	cmd := &cobra.Command{
		Use:   "start [entrypoint]",
		Short: "Start a process",
		Long: `Start a managed process. Runix will auto-detect the runtime if not specified.

With no arguments, starts all processes defined in runix.yaml (with autostart: true).
Use --only to start specific processes by name (comma-separated).

Supports Go binaries/scripts, Python scripts, Node.js/TypeScript apps, Bun apps, Deno apps, Ruby apps, and PHP apps.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// No args: start all from config file.
			if len(args) == 0 {
				configPath := resolveConfigPath(cmd)
				return startAllFromConfig(onlyFlag, configPath)
			}

			entrypoint := args[0]
			extraArgs := args[1:]

			// When the entrypoint is a quoted command string like "bun dev",
			// split it into binary + args (pm2-style).
			if len(extraArgs) == 0 && strings.Contains(entrypoint, " ") {
				parts, err := splitShellArgs(entrypoint)
				if err != nil {
					return fmt.Errorf("failed to parse command: %w", err)
				}
				if len(parts) > 1 {
					entrypoint = parts[0]
					extraArgs = parts[1:]
				}
			}

			if name == "" {
				name = filepath.Base(entrypoint)
			}
			cwd := workingDir
			if cwd == "" {
				cwd, _ = os.Getwd()
			}

			// Resolve runtime.
			resolvedRuntime := runtimeName
			detector := runtime.NewDetector()
			if resolvedRuntime == "" || resolvedRuntime == "auto" {
				// First, check if the entrypoint itself is a known runtime binary.
				// This handles cases like "bun dev", "node server.js", "python app.py".
				entryBin := filepath.Base(entrypoint)
				if _, err := detector.Get(entryBin); err == nil {
					resolvedRuntime = entryBin
					log.Debug().Str("runtime", resolvedRuntime).Msg("detected runtime from entrypoint")
				} else if _, err := exec.LookPath(entrypoint); err == nil {
					// Entrypoint is an executable on PATH (e.g. "sleep", "echo")
					// — don't inherit runtime from project files.
					resolvedRuntime = "process"
					log.Debug().Str("runtime", resolvedRuntime).Msg("detected generic process from entrypoint")
				} else {
					rt, err := detector.Detect(cwd)
					if err == nil {
						resolvedRuntime = rt.Name()
					} else {
						resolvedRuntime = "unknown"
					}
					log.Debug().Str("runtime", resolvedRuntime).Msg("auto-detected runtime from project files")
				}
			}

			// Resolve actual command via runtime adapter.
			finalEntrypoint := entrypoint
			finalArgs := extraArgs
			if rt, err := detector.Get(resolvedRuntime); err == nil {
				resolvedCmd, cmdErr := rt.StartCmd(runtime.StartOptions{
					Entrypoint: entrypoint,
					Args:       extraArgs,
					Cwd:        cwd,
					Env:        parseEnvs(envVars),
					UseBundle:  useBundle,
				})
				if cmdErr == nil && resolvedCmd != nil {
					finalEntrypoint = resolvedCmd.Path
					if len(resolvedCmd.Args) > 1 {
						finalArgs = resolvedCmd.Args[1:]
					}
				}
			}

			if dryRun {
				_, _ = fmt.Fprintf(os.Stdout, "[Runix] Dry run: would start %q\n", name)
				_, _ = fmt.Fprintf(os.Stdout, "  runtime:    %s\n", resolvedRuntime)
				_, _ = fmt.Fprintf(os.Stdout, "  entrypoint: %s\n", finalEntrypoint)
				_, _ = fmt.Fprintf(os.Stdout, "  args:       %v\n", finalArgs)
				_, _ = fmt.Fprintf(os.Stdout, "  cwd:        %s\n", cwd)
				_, _ = fmt.Fprintf(os.Stdout, "  instances:  %d\n", instances)
				return nil
			}

			cfg := types.ProcessConfig{
				Name:          name,
				Runtime:       resolvedRuntime,
				Entrypoint:    finalEntrypoint,
				Args:          finalArgs,
				Cwd:           cwd,
				Env:           parseEnvs(envVars),
				RestartPolicy: types.RestartPolicy(restartPol),
				MaxRestarts:   maxRestarts,
				Instances:     instances,
				Namespace:     namespace,
			}

			if watch {
				cfg.Watch = &types.WatchConfig{
					Enabled: true,
					Paths:   []string{cwd},
					Ignore:  watchIgnore,
				}
			}

			if cfg.Instances < 1 {
				cfg.Instances = 1
			}

			for i := 0; i < cfg.Instances; i++ {
				instanceCfg := cfg
				instanceCfg.InstanceIndex = i
				if cfg.Instances > 1 {
					instanceCfg.Name = fmt.Sprintf("%s:%d", cfg.Name, i)
				}

				// Always try daemon IPC first (auto-starts daemon if needed).
				payload := daemon.StartPayload{
					Name:          instanceCfg.Name,
					Runtime:       instanceCfg.Runtime,
					Entrypoint:    instanceCfg.Entrypoint,
					Args:          instanceCfg.Args,
					Cwd:           instanceCfg.Cwd,
					Env:           instanceCfg.Env,
					RestartPolicy: string(instanceCfg.RestartPolicy),
					MaxRestarts:   instanceCfg.MaxRestarts,
				}
				resp, err := sendIPC(daemon.ActionStart, payload)
				if err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "[Runix] Daemon IPC failed, using direct mode: %v\n", err)
				} else if !resp.Success {
					return fmt.Errorf("daemon error: %s", resp.Error)
				} else {
					var info types.ProcessInfo
					if err := json.Unmarshal(resp.Data, &info); err == nil {
						_, _ = fmt.Fprintf(os.Stdout, "[Runix] Process %q started (id: %d, pid: %d)\n", info.Name, info.NumericID, info.PID)
					}
					continue
				}

				sup, err := getSupervisor()
				if err != nil {
					return err
				}

				// Direct mode fallback.
				proc, err := sup.AddProcess(context.Background(), instanceCfg)
				if err != nil {
					return fmt.Errorf("failed to start process: %w", err)
				}

				info := proc.Info()
				_, _ = fmt.Fprintf(os.Stdout, "[Runix] Process %q started (id: %d, pid: %d)\n", info.Name, info.NumericID, info.PID)
			}

			showSpeedList()
			return nil
		},
	}

	cmd.Flags().StringVar(&onlyFlag, "only", "", "start only specific processes by name (comma-separated, requires config file)")
	cmd.Flags().StringVarP(&name, "name", "n", "", "process name (default: entrypoint filename)")
	cmd.Flags().StringVarP(&runtimeName, "runtime", "r", "", "runtime: go, python, node, bun, deno, ruby, php, auto (default: auto-detect)")
	cmd.Flags().StringVarP(&workingDir, "cwd", "d", "", "working directory (default: current)")
	cmd.Flags().StringArrayVarP(&envVars, "env", "e", nil, "environment variables (KEY=VAL, repeatable)")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "watch for file changes and auto-restart")
	cmd.Flags().StringArrayVar(&watchIgnore, "watch-ignore", nil, "patterns to ignore when watching")
	cmd.Flags().StringVar(&restartPol, "restart-policy", "", "restart policy: always, on-failure, never")
	cmd.Flags().IntVar(&maxRestarts, "max-restarts", 0, "max restarts within window (0 = use default)")
	cmd.Flags().IntVar(&maxRestarts, "max-retry", 0, "alias for --max-restarts")
	cmd.Flags().IntVar(&instances, "instances", 1, "number of instances to start")
	cmd.Flags().StringVar(&namespace, "namespace", "", "process namespace")
	cmd.Flags().BoolVar(&useBundle, "use-bundle", false, "wrap entrypoint with bundle exec (Ruby)")

	return cmd
}

// startAllFromConfig starts all processes defined in runix.yaml via daemon IPC.
func startAllFromConfig(onlyFlag string, configPath string) error {
	resp, err := sendIPC(daemon.ActionStartAll, daemon.StartAllPayload{
		Only:   onlyFlag,
		Config: configPath,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("daemon error: %s", resp.Error)
	}

	var result struct {
		Started []types.ProcessInfo `json:"started"`
		Errors  []string            `json:"errors"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	for _, info := range result.Started {
		_, _ = fmt.Fprintf(os.Stdout, "[Runix] Process %q started (id: %d, pid: %d)\n", info.Name, info.NumericID, info.PID)
	}
	for _, e := range result.Errors {
		_, _ = fmt.Fprintf(os.Stderr, "[Runix] Error: %s\n", e)
	}

	if len(result.Started) == 0 && len(result.Errors) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "[Runix] No processes to start (check runix.yaml)")
	}

	showSpeedList()
	return nil
}

func parseEnvs(envVars []string) map[string]string {
	env := make(map[string]string)
	for _, e := range envVars {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	return env
}

// cliCollector is the shared metrics collector for CLI direct mode.
// Initialized once by getSupervisor and stopped by the finalizer.
var (
	cliCollector     *metrics.Collector
	cliCollectorOnce sync.Once
)

func getSupervisor() (*supervisor.Supervisor, error) {
	// Initialize the shared collector exactly once.
	cliCollectorOnce.Do(func() {
		cliCollector = metrics.NewCollector()
		cliCollector.Start(5 * time.Second)
	})

	opts := supervisor.Options{
		LogDir: dataDir(),
		Defaults: types.DefaultsConfig{
			RestartPolicy: types.RestartOnFailure,
			MaxRestarts:   10,
		},
		MetricsCollector: cliCollector,
		MetricsInterval:  5 * time.Second,
	}

	if err := os.MkdirAll(opts.LogDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	log.Debug().Str("log_dir", opts.LogDir).Msg("creating supervisor")
	return supervisor.New(opts), nil
}
