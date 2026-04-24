package main

import (
	"fmt"
	"os"
	"time"

	"github.com/runixio/runix/internal/daemon"
	"github.com/runixio/runix/internal/watcher"
	"github.com/spf13/cobra"
)

func newWatchCmd() *cobra.Command {
	var (
		paths    []string
		ignore   []string
		debounce string
	)

	cmd := &cobra.Command{
		Use:   "watch <id|name>",
		Short: "Enable file watching for a process",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			debounceDur, _ := time.ParseDuration(debounce)

			// Try daemon IPC.
			if daemonIsRunning() {
				resp, err := sendIPC(daemon.ActionReload, daemon.StopPayload{
					Target: target,
				})
				if err != nil {
					fmt.Fprintf(os.Stderr, "[Runix] Daemon IPC failed, using direct mode: %v\n", err)
				} else if resp.Success {
					fmt.Fprintf(os.Stdout, "[Runix] Watching process %q (via daemon)\n", target)
					return nil
				}
			}

			// Direct mode: get the process and attach a watcher.
			sup, err := getSupervisor()
			if err != nil {
				return err
			}

			proc, err := sup.Get(target)
			if err != nil {
				return err
			}

			info := proc.Info()
			watchPaths := paths
			if len(watchPaths) == 0 {
				watchPaths = []string{info.Config.Cwd}
			}

			watchIgnore := ignore
			if len(watchIgnore) == 0 {
				watchIgnore = []string{
					".git", "node_modules", "__pycache__", "*.pyc",
					".DS_Store", "vendor", "dist", "build", "bin",
				}
			}

			if debounceDur == 0 {
				debounceDur = 100 * time.Millisecond
			}

			w, err := watcher.New(watchPaths, watchIgnore, debounceDur)
			if err != nil {
				return fmt.Errorf("failed to create watcher: %w", err)
			}

			// Handler restarts the process on file changes.
			handler := func(changedPaths []string) {
				for _, p := range changedPaths {
					fmt.Fprintf(os.Stderr, "[Runix] File changed: %s\n", p)
				}
				ctx := cmd.Context()
				if err := sup.RestartProcess(ctx, info.ID); err != nil {
					fmt.Fprintf(os.Stderr, "[Runix] Restart failed: %v\n", err)
				} else {
					fmt.Fprintf(os.Stdout, "[Runix] Process %q restarted\n", info.Name)
				}
			}

			if err := w.Start(handler); err != nil {
				return fmt.Errorf("failed to start watcher: %w", err)
			}

			fmt.Fprintf(os.Stdout, "[Runix] Watching %v for process %q (Ctrl+C to stop)\n", watchPaths, info.Name)

			// Block until interrupted.
			select {}
		},
	}

	cmd.Flags().StringArrayVar(&paths, "paths", nil, "paths to watch")
	cmd.Flags().StringArrayVar(&ignore, "ignore", nil, "patterns to ignore")
	cmd.Flags().StringVar(&debounce, "debounce", "100ms", "debounce duration")

	return cmd
}
