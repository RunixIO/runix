package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/runixio/runix/internal/daemon"
	"github.com/spf13/cobra"
)

func newRestartCmd() *cobra.Command {
	var parallel bool
	var format string

	cmd := &cobra.Command{
		Use:   "restart [id|name|all]",
		Short: "Restart managed processes",
		Long:  "Restart managed processes. Defaults to restarting all processes when no target is specified.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "all"
			if len(args) > 0 {
				target = args[0]
			}

			if dryRun {
				_, _ = fmt.Fprintf(os.Stdout, "[Runix] Dry run: would restart %q\n", target)
				return nil
			}

			// Try daemon IPC.
			if daemonIsRunning() {
				resp, err := sendIPC(daemon.ActionRestart, daemon.StopPayload{
					Target:   target,
					Parallel: parallel,
				})
				if err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "[Runix] Daemon IPC failed, using direct mode: %v\n", err)
				} else if !resp.Success {
					return fmt.Errorf("daemon error: %s", resp.Error)
				} else {
					_, _ = fmt.Fprintf(os.Stdout, "[Runix] Process restarted\n")
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
				if parallel {
					var mu sync.Mutex
					var wg sync.WaitGroup
					for _, p := range procs {
						wg.Add(1)
						go func(id, name string, numID int) {
							defer wg.Done()
							if err := sup.RestartProcess(context.Background(), id); err != nil {
								mu.Lock()
								_, _ = fmt.Fprintf(os.Stderr, "Failed to restart %q: %v\n", name, err)
								mu.Unlock()
							} else {
								mu.Lock()
								_, _ = fmt.Fprintf(os.Stdout, "[Runix] Process %q (id: %d) restarted\n", name, numID)
								mu.Unlock()
							}
						}(p.ID, p.Name, p.NumericID)
					}
					wg.Wait()
				} else {
					for _, p := range procs {
						if err := sup.RestartProcess(context.Background(), p.ID); err != nil {
							_, _ = fmt.Fprintf(os.Stderr, "Failed to restart %q: %v\n", p.Name, err)
						} else {
							_, _ = fmt.Fprintf(os.Stdout, "[Runix] Process %q (id: %d) restarted\n", p.Name, p.NumericID)
						}
					}
				}
				showSpeedList()
				return nil
			}

			procs, err := sup.GetGroup(target)
			if err != nil {
				return fmt.Errorf("failed to restart %q: %w", target, err)
			}
			for _, proc := range procs {
				info := proc.Info()
				if err := sup.RestartProcess(context.Background(), proc.ID); err != nil {
					return fmt.Errorf("failed to restart %q: %w", target, err)
				}
				_, _ = fmt.Fprintf(os.Stdout, "[Runix] Process %q (id: %d) restarted\n", info.Name, info.NumericID)
			}
			showSpeedList()
			return nil
		},
	}

	cmd.Flags().BoolVar(&parallel, "parallel", false, "restart all processes concurrently")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "output format: text, json")

	return cmd
}
