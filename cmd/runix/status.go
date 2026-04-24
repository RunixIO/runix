package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/runixio/runix/internal/daemon"
	"github.com/runixio/runix/internal/output"
	"github.com/runixio/runix/pkg/types"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "status <id|name>",
		Short: "Show detailed status of a process",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]

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
						return printProcessStatus(info, format)
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
			return printProcessStatus(info, format)
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format: table, json")

	return cmd
}

func printProcessStatus(info types.ProcessInfo, format string) error {
	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	default:
		kv := output.NewKeyValue()
		kv.Add("ID", info.ID)
		kv.Add("Name", info.Name)
		kv.Add("Runtime", info.Runtime)
		kv.Add("State", displayState(info.State))
		kv.Add("Ready", output.EnabledSprint(info.Ready))
		kv.Add("PID", fmt.Sprintf("%d", info.PID))
		kv.Add("CPU", output.FormatCPU(info.CPUPercent))
		kv.Add("Memory", output.FormatBytes(info.MemBytes))
		kv.Add("Uptime", info.UptimeString())
		kv.Add("Restarts", fmt.Sprintf("%d", info.Restarts))
		kv.Add("Exit Code", fmt.Sprintf("%d", info.ExitCode))
		kv.Add("Created", info.CreatedAt.Format("2006-01-02 15:04:05"))
		if info.StartedAt != nil {
			kv.Add("Started", info.StartedAt.Format("2006-01-02 15:04:05"))
		}
		kv.Add("Entrypoint", info.Config.Entrypoint)
		kv.Add("Cwd", info.Config.Cwd)
		kv.Add("Restart Policy", string(info.Config.RestartPolicy))
		kv.Add("Auto Restart", output.EnabledSprint(info.Config.AutoRestart == nil || *info.Config.AutoRestart))
		kv.Add("Max Restarts", fmt.Sprintf("%d", info.Config.MaxRestarts))
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
		_, _ = fmt.Fprint(os.Stdout, kv.Render())
	}
	return nil
}
