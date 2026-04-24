package main

import (
	"context"
	"fmt"
	"os"

	"github.com/runixio/runix/internal/daemon"
	"github.com/runixio/runix/internal/supervisor"
	"github.com/spf13/cobra"
)

func newReloadCmd() *cobra.Command {
	var (
		format    string
		rolling   bool
		batchSize int
		waitReady bool
		rollback  bool
	)

	cmd := &cobra.Command{
		Use:   "reload <id|name>",
		Short: "Gracefully reload a process",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]

			if dryRun {
				fmt.Fprintf(os.Stdout, "[Runix] Dry run: would reload %q\n", target)
				return nil
			}

			if rolling {
				// Try daemon IPC for rolling reload.
				if daemonIsRunning() {
					resp, err := sendIPC(daemon.ActionRollingReload, daemon.RollingReloadPayload{
						Target:            target,
						BatchSize:         batchSize,
						WaitReady:         waitReady,
						RollbackOnFailure: rollback,
					})
					if err != nil {
						fmt.Fprintf(os.Stderr, "[Runix] Daemon IPC failed, using direct mode: %v\n", err)
					} else if !resp.Success {
						return fmt.Errorf("daemon error: %s", resp.Error)
					} else {
						fmt.Fprintf(os.Stdout, "[Runix] All processes rolling-reloaded\n")
						return nil
					}
				}

				// Direct mode rolling reload.
				sup, err := getSupervisor()
				if err != nil {
					return err
				}
				names := []string{target}
				if target == "all" {
					procs := sup.List()
					names = make([]string, len(procs))
					for i, p := range procs {
						names[i] = p.Name
					}
				} else {
					names, err = sup.GetGroupNames(target)
					if err != nil {
						return err
					}
				}
				opts := supervisor.RollingReloadOptions{
					BatchSize:         batchSize,
					WaitReady:         waitReady,
					RollbackOnFailure: rollback,
				}
				if err := sup.RollingReload(context.Background(), names, opts); err != nil {
					return err
				}
				fmt.Fprintf(os.Stdout, "[Runix] All processes rolling-reloaded\n")
				return nil
			}

			// Try daemon IPC.
			if daemonIsRunning() {
				resp, err := sendIPC(daemon.ActionReload, daemon.StopPayload{
					Target: target,
				})
				if err != nil {
					fmt.Fprintf(os.Stderr, "[Runix] Daemon IPC failed, using direct mode: %v\n", err)
				} else if !resp.Success {
					return fmt.Errorf("daemon error: %s", resp.Error)
				} else {
					fmt.Fprintf(os.Stdout, "[Runix] Process reloaded\n")
					return nil
				}
			}

			// Direct mode.
			sup, err := getSupervisor()
			if err != nil {
				return err
			}
			procs, err := sup.GetGroup(target)
			if err != nil {
				return fmt.Errorf("failed to reload %q: %w", target, err)
			}
			for _, proc := range procs {
				if err := sup.ReloadProcess(context.Background(), proc.ID); err != nil {
					return fmt.Errorf("failed to reload %q: %w", target, err)
				}
			}
			fmt.Fprintf(os.Stdout, "[Runix] Process reloaded\n")
			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "text", "output format: text, json")
	cmd.Flags().BoolVar(&rolling, "rolling", false, "perform rolling reload")
	cmd.Flags().IntVar(&batchSize, "batch-size", 1, "number of processes to reload concurrently")
	cmd.Flags().BoolVar(&waitReady, "wait-ready", false, "wait for readiness between batches")
	cmd.Flags().BoolVar(&rollback, "rollback", false, "rollback on failure")

	return cmd
}
