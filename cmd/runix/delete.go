package main

import (
	"fmt"
	"os"

	"github.com/runixio/runix/internal/daemon"
	"github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
	var force bool
	var format string

	cmd := &cobra.Command{
		Use:   "delete [id|name|all]",
		Short: "Stop and remove processes",
		Long:  "Stop and remove processes from the process table. Defaults to removing all processes when no target is specified.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "all"
			if len(args) > 0 {
				target = args[0]
			}

			// Try daemon IPC.
			if daemonIsRunning() {
				resp, err := sendIPC(daemon.ActionDelete, daemon.StopPayload{
					Target: target,
				})
				if err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "[Runix] Daemon IPC failed, using direct mode: %v\n", err)
				} else if !resp.Success {
					return fmt.Errorf("daemon error: %s", resp.Error)
				} else {
					if target == "all" {
						_, _ = fmt.Fprintf(os.Stdout, "[Runix] All processes deleted\n")
					} else {
						_, _ = fmt.Fprintf(os.Stdout, "[Runix] Process deleted\n")
					}
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
					_, _ = fmt.Fprintln(os.Stdout, "No processes to delete")
					return nil
				}
				for _, p := range procs {
					if err := sup.RemoveProcess(p.ID); err != nil {
						_, _ = fmt.Fprintf(os.Stderr, "Failed to delete %q: %v\n", p.Name, err)
					} else {
						_, _ = fmt.Fprintf(os.Stdout, "[Runix] Process %q (id: %d) deleted\n", p.Name, p.NumericID)
					}
				}
				showSpeedList()
				return nil
			}

			procs, err := sup.GetGroup(target)
			if err != nil {
				return fmt.Errorf("failed to delete %q: %w", target, err)
			}
			for _, proc := range procs {
				info := proc.Info()
				if err := sup.RemoveProcess(proc.ID); err != nil {
					return fmt.Errorf("failed to delete %q: %w", target, err)
				}
				_, _ = fmt.Fprintf(os.Stdout, "[Runix] Process %q (id: %d) deleted\n", info.Name, info.NumericID)
			}
			showSpeedList()
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "force stop before deleting")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "output format: text, json")

	return cmd
}
