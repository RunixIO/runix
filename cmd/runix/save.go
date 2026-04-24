package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/runixio/runix/internal/daemon"
	"github.com/spf13/cobra"
)

func newSaveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "save",
		Short: "Save current process list for resurrection",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Try daemon IPC.
			if daemonIsRunning() {
				resp, err := sendIPC(daemon.ActionSave, nil)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[Runix] Daemon IPC failed, using direct mode: %v\n", err)
				} else if resp.Success {
					var result struct {
						Status string `json:"status"`
					}
					if err := json.Unmarshal(resp.Data, &result); err == nil {
						fmt.Fprintln(os.Stdout, "[Runix] Process state saved")
						return nil
					}
				}
			}

			// Direct mode.
			sup, err := getSupervisor()
			if err != nil {
				return err
			}

			if err := sup.Save(); err != nil {
				return fmt.Errorf("failed to save process state: %w", err)
			}

			procs := sup.List()
			fmt.Fprintf(os.Stdout, "[Runix] Saved state for %d process(es)\n", len(procs))
			return nil
		},
	}

	return cmd
}
