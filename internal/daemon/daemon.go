package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/runixio/runix/internal/auth"
	"github.com/runixio/runix/internal/mcp"
	"github.com/runixio/runix/internal/metrics"
	"github.com/runixio/runix/internal/supervisor"
	"github.com/runixio/runix/pkg/types"
)

// DefaultSocketPath returns the default Unix socket path.
func DefaultSocketPath() string {
	return filepath.Join(DefaultDataDir(), "tmp", "runix.sock")
}

// DefaultDataDir returns the default data directory.
// Always uses $HOME/.runix/ (creates it on first access if needed).
func DefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	return filepath.Join(home, ".runix")
}

// ResolveDataDir returns cfgDir if non-empty, otherwise DefaultDataDir().
func ResolveDataDir(cfgDir string) string {
	if cfgDir != "" {
		return cfgDir
	}
	return DefaultDataDir()
}

// ResolveSocketPath returns cfgSocket if non-empty, otherwise <dataDir>/tmp/runix.sock.
func ResolveSocketPath(dataDir, cfgSocket string) string {
	if cfgSocket != "" {
		return cfgSocket
	}
	return filepath.Join(dataDir, "tmp", "runix.sock")
}

// StartDaemon forks the daemon process and waits for it to become ready.
func StartDaemon() error {
	bin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	cmd := exec.Command(bin, "daemon", "run")
	cmd.Stdout = nil // daemon manages its own logging
	cmd.Stderr = nil // daemon manages its own logging
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Pass the caller's config file path to the daemon so it loads the same config.
	if cfgPath := os.Getenv("RUNIX_CONFIG"); cfgPath != "" {
		cmd.Env = append(os.Environ(), "RUNIX_CONFIG="+cfgPath)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Release the process handle immediately — the daemon outlives the caller.
	_ = cmd.Process.Release()

	log.Debug().Int("pid", cmd.Process.Pid).Msg("daemon process started")

	// Wait for the socket to become available.
	socketPath := DefaultSocketPath()
	client := NewClient(socketPath)
	defer client.Close()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if client.IsAlive() {
			log.Debug().Msg("daemon is ready")
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timed out waiting for daemon to become ready")
}

// RunDaemon runs the daemon in the current process (blocks).
func RunDaemon(socketPath string, dataDir string, cfg *types.RunixConfig, configPath string) error {
	// Create data directory structure.
	if err := os.MkdirAll(filepath.Join(dataDir, "apps"), 0o755); err != nil {
		return fmt.Errorf("failed to create apps directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "state"), 0o755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "tmp"), 0o755); err != nil {
		return fmt.Errorf("failed to create tmp directory: %w", err)
	}

	// Create metrics collector — always running so all processes are tracked.
	metricsInterval := cfg.Metrics.MetricsInterval()
	col := metrics.NewCollector()
	col.Start(metricsInterval)

	// Create supervisor with options from config.
	sup := supervisor.New(supervisor.Options{
		LogDir:           dataDir,
		Defaults:         cfg.Defaults,
		MetricsCollector: col,
		MetricsInterval:  metricsInterval,
	})

	// PID file in tmp directory.
	pidDir := filepath.Join(dataDir, "tmp")

	// Create authenticator from security config.
	authenticator, err := auth.NewAuthenticator(cfg.Security.Auth)
	if err != nil {
		return fmt.Errorf("failed to create authenticator: %w", err)
	}

	// Create server.
	srv := NewServer(sup, socketPath, pidDir, authenticator, cfg, configPath)

	log.Info().
		Str("socket", socketPath).
		Str("data_dir", dataDir).
		Msg("starting daemon")

	// Start MCP HTTP server if enabled.
	var mcpCancel context.CancelFunc
	if cfg.MCP.Enabled && cfg.MCP.Transport == "http" {
		mcpSrv := mcp.NewMCPServer(sup, col)
		mcpSrv.SetAuth(authenticator)
		mcpCfg := cfg.MCP
		if mcpCfg.Listen == "" {
			mcpCfg.Listen = "localhost:8090"
		}
		mcpCtx, cancel := context.WithCancel(context.Background())
		mcpCancel = cancel
		go func() {
			if err := mcpSrv.Start(mcpCtx, mcpCfg); err != nil {
				log.Error().Err(err).Msg("MCP HTTP server error")
			}
		}()
		log.Info().Str("addr", mcpCfg.Listen).Msg("MCP HTTP server started alongside daemon")
	}

	// Auto-start processes from config (autostart: true, respecting depends_on/priority).
	startConfigProcesses(sup, cfg)

	// Blocks until shutdown.
	err = srv.Start(context.Background())

	// Clean up MCP and metrics on shutdown.
	if mcpCancel != nil {
		mcpCancel()
	}
	col.Stop()

	return err
}

// startConfigProcesses starts all autostart processes from the config file.
// Called once during daemon startup, after the server is ready.
func startConfigProcesses(sup *supervisor.Supervisor, cfg *types.RunixConfig) {
	if cfg == nil || len(cfg.Processes) == 0 {
		return
	}

	var toStart []types.ProcessConfig
	for _, p := range cfg.Processes {
		if p.Autostart {
			toStart = append(toStart, p)
		}
	}

	if len(toStart) == 0 {
		return
	}

	// Sort by priority then topological order (same as handleStartAll).
	SortStartOrder(toStart)
	toStart = ExpandProcessInstances(toStart)

	for _, p := range toStart {
		if _, err := sup.AddProcess(context.Background(), p); err != nil {
			log.Error().Err(err).Str("process", p.Name).Msg("failed to auto-start process")
		} else {
			log.Info().Str("process", p.Name).Msg("auto-started process")
		}
	}
}
