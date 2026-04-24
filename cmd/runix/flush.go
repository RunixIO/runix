package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newFlushCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "flush [app]",
		Short: "Flush process logs",
		Long:  `Flush (truncate) log files for processes. Does not interrupt running processes.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dd := dataDir()
			appsDir := filepath.Join(dd, "apps")

			if len(args) > 0 {
				// Flush specific app.
				appDir := filepath.Join(appsDir, args[0])
				if _, err := os.Stat(appDir); os.IsNotExist(err) {
					return fmt.Errorf("app %q not found", args[0])
				}
				if err := flushLogs(appDir); err != nil {
					return err
				}
				fmt.Fprintf(os.Stdout, "[Runix] Logs flushed for %q\n", args[0])
				return nil
			}

			// Flush all apps.
			entries, err := os.ReadDir(appsDir)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Fprintln(os.Stdout, "No apps found")
					return nil
				}
				return fmt.Errorf("failed to read apps directory: %w", err)
			}

			count := 0
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				appDir := filepath.Join(appsDir, entry.Name())
				if err := flushLogs(appDir); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to flush %q: %v\n", entry.Name(), err)
				} else {
					count++
				}
			}
			fmt.Fprintf(os.Stdout, "[Runix] Logs flushed for %d app(s)\n", count)
			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "text", "output format: text, json")

	return cmd
}

// flushLogs truncates stdout.log and stderr.log in the given app directory.
func flushLogs(appDir string) error {
	for _, logFile := range []string{"stdout.log", "stderr.log"} {
		path := filepath.Join(appDir, logFile)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return fmt.Errorf("failed to truncate %s: %w", logFile, err)
		}
		f.Close()
	}
	return nil
}
