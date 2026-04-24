package main

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/runixio/runix/internal/config"
	"github.com/runixio/runix/internal/daemon"
	"github.com/runixio/runix/pkg/types"
	"github.com/spf13/cobra"
)

func newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon <subcommand>",
		Short: "Manage the Runix daemon",
	}

	// daemon start
	cmd.AddCommand(&cobra.Command{
		Use:   "start",
		Short: "Start the Runix daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonIsRunning() {
				fmt.Fprintln(os.Stdout, "[Runix] Daemon is already running")
				return nil
			}
			if cfgFile != "" {
				os.Setenv("RUNIX_CONFIG", cfgFile)
			}
			if err := daemon.StartDaemon(); err != nil {
				fmt.Fprintf(os.Stderr, "[Runix] Failed to start daemon: %v\n", err)
				return err
			}
			fmt.Fprintln(os.Stdout, "[Runix] Daemon started")
			return nil
		},
	})

	// daemon stop
	cmd.AddCommand(&cobra.Command{
		Use:   "stop",
		Short: "Stop the Runix daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := daemonClient()
			if !client.IsAlive() {
				fmt.Fprintln(os.Stdout, "[Runix] Daemon is not running")
				return nil
			}

			pidFile := daemon.NewPIDFile(daemon.DefaultDataDir())
			pid, _ := pidFile.Read()
			if pid > 0 {
				proc, err := os.FindProcess(pid)
				if err == nil {
					if err := proc.Signal(syscall.SIGTERM); err != nil {
						fmt.Fprintf(os.Stderr, "[Runix] Failed to stop daemon: %v\n", err)
						return err
					}
				}
			}

			// Wait for daemon to stop.
			deadline := time.Now().Add(10 * time.Second)
			for time.Now().Before(deadline) {
				if !client.IsAlive() {
					fmt.Fprintln(os.Stdout, "[Runix] Daemon stopped")
					return nil
				}
				time.Sleep(200 * time.Millisecond)
			}
			fmt.Fprintf(os.Stderr, "[Runix] Timed out waiting for daemon to stop\n")
			return fmt.Errorf("timed out waiting for daemon to stop")
		},
	})

	// daemon status
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Check daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := daemonClient()
			if client.IsAlive() {
				pidFile := daemon.NewPIDFile(daemon.DefaultDataDir())
				pid, _ := pidFile.Read()
				socket := daemon.DefaultSocketPath()
				fmt.Fprintf(os.Stdout, "[Runix] Daemon is running (pid: %d, socket: %s)\n", pid, socket)
			} else {
				fmt.Fprintln(os.Stdout, "[Runix] Daemon is not running")
			}
			return nil
		},
	})

	// daemon restart
	cmd.AddCommand(&cobra.Command{
		Use:   "restart",
		Short: "Restart the Runix daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := daemonClient()
			if client.IsAlive() {
				pidFile := daemon.NewPIDFile(daemon.DefaultDataDir())
				pid, _ := pidFile.Read()
				if pid > 0 {
					proc, err := os.FindProcess(pid)
					if err == nil {
						_ = proc.Signal(syscall.SIGTERM)
					}
				}
				// Wait for stop.
				deadline := time.Now().Add(10 * time.Second)
				for time.Now().Before(deadline) {
					if !client.IsAlive() {
						break
					}
					time.Sleep(200 * time.Millisecond)
				}
			}

			if cfgFile != "" {
				os.Setenv("RUNIX_CONFIG", cfgFile)
			}
			if err := daemon.StartDaemon(); err != nil {
				fmt.Fprintf(os.Stderr, "[Runix] Failed to start daemon: %v\n", err)
				return err
			}
			fmt.Fprintln(os.Stdout, "[Runix] Daemon restarted")
			return nil
		},
	})

	// daemon reload
	cmd.AddCommand(&cobra.Command{
		Use:   "reload",
		Short: "Reload daemon config without stopping processes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !daemonIsRunning() {
				fmt.Fprintln(os.Stderr, "[Runix] Daemon is not running")
				return fmt.Errorf("daemon not running")
			}

			// Resolve config path to pass to daemon.
			configPath := cfgFile
			if configPath == "" {
				if env := os.Getenv("RUNIX_CONFIG"); env != "" {
					configPath = env
				}
			}
			resp, err := sendIPC(daemon.ActionConfigReload, daemon.ConfigReloadPayload{
				ConfigPath: configPath,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "[Runix] Failed to reload config: %v\n", err)
				return err
			}
			if !resp.Success {
				fmt.Fprintf(os.Stderr, "[Runix] Failed to reload config: %s\n", resp.Error)
				return fmt.Errorf("config reload failed: %s", resp.Error)
			}
			fmt.Fprintln(os.Stdout, "[Runix] Config reloaded")
			return nil
		},
	})

	// daemon run (internal - runs daemon in foreground, used when forking)
	cmd.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Run the daemon in the foreground (internal)",
		RunE: func(cmd *cobra.Command, args []string) error {
			socketPath := daemon.DefaultSocketPath()
			dataDir := daemon.DefaultDataDir()

			// Load config if available (RUNIX_CONFIG env from CLI, or search CWD).
			var cfg *types.RunixConfig
			configPath := os.Getenv("RUNIX_CONFIG")
			cfg, err := config.Load(configPath)
			if err != nil {
				cfg = &types.RunixConfig{}
				config.ApplyDefaults(cfg)
			}

			return daemon.RunDaemon(socketPath, dataDir, cfg, configPath)
		},
	})

	_ = config.Load
	return cmd
}
