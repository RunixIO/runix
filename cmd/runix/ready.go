package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/runixio/runix/internal/daemon"
	"github.com/runixio/runix/pkg/types"
	"github.com/spf13/cobra"
)

func newReadyCmd() *cobra.Command {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "ready <id|name>",
		Short: "Wait for a process to be ready",
		Long:  `Block until the specified process is healthy. Useful in scripts to wait for startup.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]

			if daemonIsRunning() {
				deadline := time.Now().Add(timeout)
				for time.Now().Before(deadline) {
					resp, err := sendIPC(daemon.ActionStatus, daemon.StopPayload{Target: target})
					if err == nil && resp.Success {
						var info types.ProcessInfo
						if err := json.Unmarshal(resp.Data, &info); err == nil {
							if info.Ready {
								fmt.Fprintf(os.Stdout, "[Runix] Process %q is ready\n", info.Name)
								return nil
							}
							if info.State == types.StateStopped || info.State == types.StateCrashed || info.State == types.StateErrored {
								return fmt.Errorf("process %q is not running (state: %s)", info.Name, info.State)
							}
						}
					}
					time.Sleep(200 * time.Millisecond)
				}
				return fmt.Errorf("process %q did not become ready within %s", target, timeout)
			}

			sup, err := getSupervisor()
			if err != nil {
				return err
			}
			if err := sup.WaitUntilReady(context.Background(), target, timeout); err != nil {
				return err
			}
			proc, err := sup.Get(target)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "[Runix] Process %q is ready\n", proc.Info().Name)
			return nil
		},
	}

	cmd.Flags().DurationVar(&timeout, "timeout", 60*time.Second, "maximum time to wait")

	return cmd
}
