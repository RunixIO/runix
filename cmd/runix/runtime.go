package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/runixio/runix/internal/config"
	"github.com/runixio/runix/internal/daemon"
	"github.com/runixio/runix/internal/metrics"
	"github.com/runixio/runix/internal/supervisor"
	"github.com/runixio/runix/pkg/types"
	"github.com/spf13/cobra"
)

func newRuntimeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runtime",
		Short: "Run configured processes in foreground container mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath := cfgFile
			if configPath == "" {
				configPath = os.Getenv("RUNIX_CONFIG")
			}

			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}

			toStart, err := runtimeProcessConfigs(cfg)
			if err != nil {
				return err
			}

			dd := dataDir()
			if err := os.MkdirAll(dd, 0o755); err != nil {
				return fmt.Errorf("failed to create data directory: %w", err)
			}

			metricsInterval := cfg.Metrics.MetricsInterval()
			col := metrics.NewCollector()
			col.Start(metricsInterval)
			defer col.Stop()

			sup := supervisor.New(supervisor.Options{
				LogDir:           dd,
				Defaults:         cfg.Defaults,
				MetricsCollector: col,
				MetricsInterval:  metricsInterval,
			})
			defer func() { _ = sup.Shutdown() }()

			for _, procCfg := range toStart {
				proc, err := sup.AddProcess(context.Background(), procCfg)
				if err != nil {
					return fmt.Errorf("failed to start process %q: %w", procCfg.Name, err)
				}
				info := proc.Info()
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[Runix] Process %q started (id: %d, pid: %d)\n", info.Name, info.NumericID, info.PID)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "[Runix] Runtime mode active")

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			defer signal.Stop(sigCh)

			sig := <-sigCh
			log.Info().Str("signal", sig.String()).Msg("received shutdown signal")
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[Runix] Shutting down on %s\n", sig.String())

			return nil
		},
	}

	return cmd
}

func runtimeProcessConfigs(cfg *types.RunixConfig) ([]types.ProcessConfig, error) {
	if cfg == nil || len(cfg.Processes) == 0 {
		return nil, fmt.Errorf("no processes defined in config")
	}

	var toStart []types.ProcessConfig
	for _, p := range cfg.Processes {
		if p.Autostart {
			toStart = append(toStart, p)
		}
	}

	if len(toStart) == 0 {
		return nil, fmt.Errorf("no autostart processes defined in config")
	}

	daemon.SortStartOrder(toStart)
	return daemon.ExpandProcessInstances(toStart), nil
}
