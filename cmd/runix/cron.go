package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/runixio/runix/internal/daemon"
	"github.com/runixio/runix/internal/output"
	"github.com/runixio/runix/internal/scheduler"
	"github.com/spf13/cobra"
)

func newCronCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cron <subcommand>",
		Short: "Manage cron jobs",
	}

	// cron list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List cron jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, _ := cmd.Flags().GetString("format")

			// Try daemon IPC.
			if daemonIsRunning() {
				resp, err := sendIPC(daemon.ActionCronList, nil)
				if err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "[Runix] Daemon IPC failed, using direct mode: %v\n", err)
				} else if resp.Success {
					var jobs []scheduler.JobInfo
					if err := json.Unmarshal(resp.Data, &jobs); err == nil {
						return printCronJobs(jobs, format)
					}
				}
			}

			_, _ = fmt.Fprintln(os.Stdout, "No cron jobs configured (daemon not running)")
			return nil
		},
	}
	listCmd.Flags().StringVarP(new(string), "format", "f", "table", "output format: table, json")
	cmd.AddCommand(listCmd)

	// cron start <name>
	cmd.AddCommand(&cobra.Command{
		Use:   "start <name>",
		Short: "Enable a cron job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Try daemon IPC.
			if daemonIsRunning() {
				resp, err := sendIPC(daemon.ActionCronStart, daemon.CronPayload{
					Name: name,
				})
				if err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "[Runix] Daemon IPC failed: %v\n", err)
				} else if !resp.Success {
					return fmt.Errorf("daemon error: %s", resp.Error)
				} else {
					_, _ = fmt.Fprintf(os.Stdout, "[Runix] Cron job %q started\n", name)
					return nil
				}
			}

			return fmt.Errorf("cron management requires a running daemon")
		},
	})

	// cron stop <name>
	cmd.AddCommand(&cobra.Command{
		Use:   "stop <name>",
		Short: "Disable a cron job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Try daemon IPC.
			if daemonIsRunning() {
				resp, err := sendIPC(daemon.ActionCronStop, daemon.CronPayload{
					Name: name,
				})
				if err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "[Runix] Daemon IPC failed: %v\n", err)
				} else if !resp.Success {
					return fmt.Errorf("daemon error: %s", resp.Error)
				} else {
					_, _ = fmt.Fprintf(os.Stdout, "[Runix] Cron job %q stopped\n", name)
					return nil
				}
			}

			return fmt.Errorf("cron management requires a running daemon")
		},
	})

	// cron run <name>
	cmd.AddCommand(&cobra.Command{
		Use:   "run <name>",
		Short: "Manually trigger a cron job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Try daemon IPC.
			if daemonIsRunning() {
				resp, err := sendIPC(daemon.ActionCronRun, daemon.CronPayload{
					Name: name,
				})
				if err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "[Runix] Daemon IPC failed: %v\n", err)
				} else if !resp.Success {
					return fmt.Errorf("daemon error: %s", resp.Error)
				} else {
					_, _ = fmt.Fprintf(os.Stdout, "[Runix] Cron job %q triggered\n", name)
					return nil
				}
			}

			return fmt.Errorf("cron management requires a running daemon")
		},
	})

	return cmd
}

func printCronJobs(jobs []scheduler.JobInfo, format string) error {
	if len(jobs) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No cron jobs")
		return nil
	}

	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(jobs)
	default:
		tbl := output.NewTable("NAME", "SCHEDULE", "ENABLED", "RUNS", "LAST RUN")
		tbl.SetAlign(3, output.AlignRight) // RUNS

		for _, j := range jobs {
			lastRun := "never"
			if !j.LastRun.IsZero() {
				lastRun = j.LastRun.Format("2006-01-02 15:04:05")
			}
			tbl.AddRow(
				j.Name,
				j.Schedule,
				output.EnabledSprint(j.Enabled),
				fmt.Sprintf("%d", j.RunCount),
				lastRun,
			)
		}
		_, _ = fmt.Fprint(os.Stdout, tbl.Render())
	}
	return nil
}
