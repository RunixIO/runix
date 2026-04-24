package main

import (
	"fmt"
	"os"
	"time"

	"github.com/runixio/runix/internal/daemon"
	"github.com/spf13/cobra"
)

func newStopCmd() *cobra.Command {
	var (
		force    bool
		timeout  time.Duration
		graceful bool
		format   string
	)

	cmd := &cobra.Command{
		Use:   "stop [id|name|all]",
		Short: "Stop managed processes",
		Long:  "Stop managed processes. Defaults to stopping all processes when no target is specified.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "all"
			if len(args) > 0 {
				target = args[0]
			}

			if dryRun {
				_, _ = fmt.Fprintf(os.Stdout, "[Runix] Dry run: would stop %q\n", target)
				return nil
			}

			// Try daemon IPC.
			if daemonIsRunning() {
				resp, err := sendIPC(daemon.ActionStop, daemon.StopPayload{
					Target:   target,
					Force:    force,
					Timeout:  timeout.String(),
					Graceful: graceful,
				})
				if err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "[Runix] Daemon IPC failed, using direct mode: %v\n", err)
				} else if !resp.Success {
					return fmt.Errorf("daemon error: %s", resp.Error)
				} else {
					_, _ = fmt.Fprintf(os.Stdout, "[Runix] Process stopped\n")
					showSpeedList()
					return nil
				}
			}

			// Direct mode.
			sup, err := getSupervisor()
			if err != nil {
				return err
			}

			if target == "all" {
				procs := sup.List()
				if len(procs) == 0 {
					_, _ = fmt.Fprintln(os.Stdout, "No processes running")
					return nil
				}
				for _, p := range procs {
					if err := sup.StopProcess(p.ID, force, timeout); err != nil {
						_, _ = fmt.Fprintf(os.Stderr, "Failed to stop %q: %v\n", p.Name, err)
					} else {
						_, _ = fmt.Fprintf(os.Stdout, "[Runix] Process %q (id: %d) stopped\n", p.Name, p.NumericID)
					}
				}
				showSpeedList()
				return nil
			}

			procs, err := sup.GetGroup(target)
			if err != nil {
				return fmt.Errorf("failed to stop %q: %w", target, err)
			}
			for _, proc := range procs {
				info := proc.Info()
				if err := sup.StopProcess(proc.ID, force, timeout); err != nil {
					return fmt.Errorf("failed to stop %q: %w", target, err)
				}
				outputResult(format, map[string]any{
					"name": info.Name, "id": info.NumericID, "status": "stopped",
				}, func() {
					_, _ = fmt.Fprintf(os.Stdout, "[Runix] Process %q (id: %d) stopped\n", info.Name, info.NumericID)
				})
			}
			showSpeedList()
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "force stop (SIGKILL)")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Second, "grace period before force kill")
	cmd.Flags().BoolVar(&graceful, "graceful", false, "graceful stop")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "output format: text, json")

	return cmd
}
