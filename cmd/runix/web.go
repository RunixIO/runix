package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/runixio/runix/internal/daemon"
	"github.com/spf13/cobra"
)

func newWebCmd() *cobra.Command {
	var (
		listen string
		open   bool
	)

	cmd := &cobra.Command{
		Use:   "web",
		Short: "Launch the web UI dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Ensure the daemon is running (auto-starts if needed).
			client, err := ensureDaemon()
			if err != nil {
				fmt.Fprintf(os.Stderr, "[Runix] Failed to start daemon: %v\n", err)
				return err
			}

			// Tell the daemon to start its web server.
			resp, err := sendIPC(daemon.ActionWebStart, daemon.WebStartPayload{
				Listen: listen,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "[Runix] Failed to start web server: %v\n", err)
				return err
			}
			if !resp.Success {
				fmt.Fprintf(os.Stderr, "[Runix] Failed to start web server: %s\n", resp.Error)
				return fmt.Errorf("web start failed: %s", resp.Error)
			}

			// Parse the response to get the address.
			var result struct {
				Status string `json:"status"`
				Addr   string `json:"addr"`
			}
			if err := json.Unmarshal(resp.Data, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			url := "http://" + result.Addr

			if result.Status == "already_running" {
				fmt.Fprintf(cmd.OutOrStdout(), "[Runix] Web UI already running at %s\n", url)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "[Runix] Web UI listening on %s\n", url)
			}

			if open {
				openBrowser(url)
			}

			// Block until Ctrl+C so the user can see the URL.
			fmt.Fprintln(cmd.OutOrStdout(), "Press Ctrl+C to exit (web server will keep running in daemon)")
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh
			fmt.Fprintln(cmd.OutOrStdout(), "\n[Runix] Exiting (web server still running in daemon)")

			_ = client
			return nil
		},
	}

	cmd.Flags().StringVar(&listen, "listen", "localhost:9615", "address to listen on")
	cmd.Flags().BoolVar(&open, "open", false, "open browser automatically")

	return cmd
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
