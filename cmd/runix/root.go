package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/runixio/runix/internal/daemon"
	"github.com/runixio/runix/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	verbose bool
	debug   bool
	dryRun  bool
	noColor bool
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "runix",
		Short: "A modern process manager and application supervisor",
		Long: `Runix is a fast, lightweight process manager for Go, Python, Node.js/TypeScript, Bun, Deno, Ruby, and PHP applications.
Manage processes through CLI, TUI, Web UI, or MCP interfaces.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			setupLogging()
			output.InitColor(isatty.IsTerminal(os.Stdout.Fd()), noColor)
			return nil
		},
	}

	root.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./runix.yaml)")
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	root.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug mode with trace logging")
	root.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "show what would happen without executing")
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	root.PersistentFlags().String("data-dir", "", "data directory (default: ~/.runix)")

	// Capture root in closure so initConfig can access the actual command
	// without calling newRootCmd() again (which would reset all flag vars).
	cobra.OnInitialize(func() { initConfig(root) })

	root.AddCommand(newStartCmd())
	root.AddCommand(newStopCmd())
	root.AddCommand(newRestartCmd())
	root.AddCommand(newReloadCmd())
	root.AddCommand(newDeleteCmd())
	root.AddCommand(newListCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newLogsCmd())
	root.AddCommand(newDescribeCmd())
	root.AddCommand(newInspectCmd())
	root.AddCommand(newFlushCmd())
	root.AddCommand(newSaveCmd())
	root.AddCommand(newResurrectCmd())
	root.AddCommand(newStartupCmd())
	root.AddCommand(newSelfUpdateCmd())
	root.AddCommand(newVersionCmd())
	root.AddCommand(newTUICmd())
	root.AddCommand(newWebCmd())
	root.AddCommand(newCronCmd())
	root.AddCommand(newWatchCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newDeployCmd())
	root.AddCommand(newDaemonCmd())
	root.AddCommand(newMCPCmd())
	root.AddCommand(newRuntimeCmd())
	root.AddCommand(newValidateCmd())
	root.AddCommand(newMigrateCmd())
	root.AddCommand(newEventsCmd())
	root.AddCommand(newReadyCmd())
	root.AddCommand(newConfigReloadCmd())

	return root
}

func initConfig(root *cobra.Command) {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigName("runix")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()
	viper.SetEnvPrefix("RUNIX")

	// Bind --data-dir flag to config.
	viper.BindPFlag("daemon.data_dir", root.PersistentFlags().Lookup("data-dir"))

	if err := viper.ReadInConfig(); err != nil {
		if !isConfigNotFoundError(err) {
			fmt.Fprintf(os.Stderr, "warning: error reading config: %v\n", err)
		}
	}
}

func isConfigNotFoundError(err error) bool {
	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		return true
	}
	if _, ok := err.(*viper.ConfigFileNotFoundError); ok {
		return true
	}
	return strings.Contains(err.Error(), "Not Found") ||
		strings.Contains(err.Error(), "no such file")
}

func setupLogging() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if debug {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	} else if verbose {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	} else {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}
}

// dataDir returns the Runix data directory.
func dataDir() string {
	return daemon.ResolveDataDir(viper.GetString("daemon.data_dir"))
}

// runtimeDir returns the runtime directory for sockets and PID files.
func runtimeDir() string {
	return filepath.Join(dataDir(), "tmp")
}

// daemonClient creates an IPC client for the daemon.
// If auth is configured, credentials are sent with every request.
func daemonClient() *daemon.Client {
	socketPath := daemon.ResolveSocketPath(dataDir(), viper.GetString("daemon.socket_path"))

	// Check for auth configuration.
	authEnabled := viper.GetBool("security.auth.enabled")
	if !authEnabled {
		return daemon.NewClient(socketPath)
	}

	mode := viper.GetString("security.auth.mode")
	if mode == "token" {
		token := viper.GetString("security.auth.token")
		if token != "" {
			return daemon.NewTokenClient(socketPath, token)
		}
	}

	// Default to basic auth.
	username := viper.GetString("security.auth.username")
	password := viper.GetString("security.auth.password")
	if username != "" {
		return daemon.NewAuthenticatedClient(socketPath, username, password)
	}

	return daemon.NewClient(socketPath)
}

// daemonIsRunning checks if the daemon is alive.
func daemonIsRunning() bool {
	return daemonClient().IsAlive()
}

// ensureDaemon checks if the daemon is running, starts it if not.
func ensureDaemon() (*daemon.Client, error) {
	client := daemonClient()
	if client.IsAlive() {
		return client, nil
	}

	log.Debug().Msg("daemon not running, starting it")
	if cfgFile != "" {
		os.Setenv("RUNIX_CONFIG", cfgFile)
		defer os.Unsetenv("RUNIX_CONFIG")
	}
	if err := daemon.StartDaemon(); err != nil {
		return nil, fmt.Errorf("failed to start daemon: %w", err)
	}
	return client, nil
}

// sendIPC sends a request to the daemon, starting it if needed.
func sendIPC(action string, payload interface{}) (daemon.Response, error) {
	client, err := ensureDaemon()
	if err != nil {
		return daemon.Response{}, err
	}

	var rawPayload json.RawMessage
	if payload != nil {
		rawPayload, err = json.Marshal(payload)
		if err != nil {
			return daemon.Response{}, fmt.Errorf("failed to marshal payload: %w", err)
		}
	}

	return client.Send(context.Background(), daemon.Request{
		Action:  action,
		Payload: rawPayload,
	})
}
