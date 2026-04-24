package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/runixio/runix/internal/daemon"
	"github.com/runixio/runix/internal/output"
	"github.com/runixio/runix/internal/tui"
	"github.com/runixio/runix/pkg/types"
	"github.com/spf13/cobra"
)

func newInspectCmd() *cobra.Command {
	var (
		logLines int
		format   string
		tuiMode  bool
	)

	cmd := &cobra.Command{
		Use:   "inspect <id|name>",
		Short: "Display detailed process information",
		Long:  `Show detailed information about a managed process including configuration, environment, and recent logs.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if tuiMode {
				return runInspectTUI(args[0])
			}
			return runInspect(args[0], format, logLines)
		},
	}

	cmd.Flags().IntVar(&logLines, "logs", 20, "number of log lines to show (0 to hide)")
	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format: table, json")
	cmd.Flags().BoolVar(&tuiMode, "tui", false, "launch interactive TUI control panel")

	return cmd
}

func runInspect(target string, format string, logLines int) error {
	// Try daemon IPC.
	if daemonIsRunning() {
		resp, err := sendIPC(daemon.ActionStatus, daemon.StopPayload{
			Target: target,
		})
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "[Runix] Daemon IPC failed, using direct mode: %v\n", err)
		} else if resp.Success {
			var info types.ProcessInfo
			if err := json.Unmarshal(resp.Data, &info); err == nil {
				return printInspect(info, format, logLines)
			}
		}
	}

	// Direct mode.
	sup, err := getSupervisor()
	if err != nil {
		return err
	}

	proc, err := sup.Get(target)
	if err != nil {
		return err
	}

	info := proc.Info()
	return printInspect(info, format, logLines)
}

// runInspectTUI launches the interactive inspect TUI control panel.
func runInspectTUI(target string) error {
	var inspector tui.ProcessInspector

	if daemonIsRunning() {
		inspector = tui.NewDaemonInspector(daemonClient())
	} else {
		// Direct mode: use the DirectInspector with IPC send function.
		insp := tui.NewDirectInspector(dataDir())
		insp.SetSendFunc(func(action string, payload interface{}) (daemon.Response, error) {
			return sendIPC(action, payload)
		})
		inspector = insp
	}

	model := tui.NewInspectModel(inspector, target)
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("inspect TUI error: %w", err)
	}
	return nil
}

func printInspect(info types.ProcessInfo, format string, logLines int) error {
	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	default:
		pid := "-"
		if info.State == types.StateRunning || info.State == types.StateStarting {
			pid = fmt.Sprintf("%d", info.PID)
		}

		kv := output.NewKeyValue()
		kv.Add("Name", info.Name)
		if info.Namespace != "" {
			kv.Add("Namespace", info.Namespace)
		}
		kv.Add("ID", fmt.Sprintf("%d", info.NumericID))
		kv.Add("Internal ID", info.ID)
		kv.Add("PID", pid)
		kv.Add("Status", displayState(info.State))
		kv.Add("Ready", output.EnabledSprint(info.Ready))
		kv.Add("Uptime", info.UptimeString())
		kv.Add("Restarts", fmt.Sprintf("%d", info.Restarts))
		kv.Add("Runtime", info.Runtime)
		kv.Add("Restart Policy", string(info.Config.RestartPolicy))
		kv.Add("Auto Restart", output.EnabledSprint(info.Config.AutoRestart == nil || *info.Config.AutoRestart))
		if info.Config.RestartDelay > 0 {
			kv.Add("Restart Delay", info.Config.RestartDelay.String())
		}
		if info.Config.MinUptime > 0 {
			kv.Add("Min Uptime", info.Config.MinUptime.String())
		}
		if info.Config.MaxMemoryRestart != "" {
			kv.Add("Max Memory Restart", info.Config.MaxMemoryRestart)
		}
		if info.LastEvent != "" {
			kv.Add("Last Event", info.LastEvent)
		}
		if info.LastReason != "" {
			kv.Add("Last Reason", info.LastReason)
		}
		kv.Add("Script", info.Config.Entrypoint)
		if len(info.Config.Args) > 0 {
			kv.Add("Args", strings.Join(info.Config.Args, " "))
		}
		if info.CPUPercent > 0 || info.MemBytes > 0 {
			kv.Add("CPU", output.FormatCPU(info.CPUPercent))
			kv.Add("Memory", output.FormatBytes(info.MemBytes))
			if info.MemPercent > 0 {
				kv.Add("Memory%", fmt.Sprintf("%.1f%%", info.MemPercent))
			}
			kv.Add("Threads", fmt.Sprintf("%d", info.Threads))
			kv.Add("FDs", fmt.Sprintf("%d", info.FDs))
		}

		dd := dataDir()
		appDir := filepath.Join(dd, "apps", info.Name)
		kv.Add("Log stdout", filepath.Join(appDir, "stdout.log"))
		kv.Add("Log stderr", filepath.Join(appDir, "stderr.log"))

		if len(info.Config.Env) > 0 {
			keys := make([]string, 0, len(info.Config.Env))
			for k := range info.Config.Env {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			pairs := make([]string, 0, len(keys))
			for _, k := range keys {
				pairs = append(pairs, k+"="+info.Config.Env[k])
			}
			kv.Add("Env", strings.Join(pairs, ", "))
		}

		_, _ = fmt.Fprint(os.Stdout, kv.Render())

		// Show recent logs if requested.
		if logLines > 0 {
			stdoutPath := filepath.Join(appDir, "stdout.log")
			if _, err := os.Stat(stdoutPath); err == nil {
				_, _ = fmt.Fprintf(os.Stdout, "\n--- Last %d lines ---\n", logLines)
				_ = printLogs(stdoutPath, logLines)
			}
		}

		return nil
	}
}
