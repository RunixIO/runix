package main

import (
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/runixio/runix/pkg/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [config-file]",
		Short: "Validate a runix configuration file",
		Long:  `Validate a runix.yaml configuration file for errors, duplicate names, invalid runtimes, and bad paths.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine config file path.
			configPath := ""
			if len(args) > 0 {
				configPath = args[0]
			} else if cfgFile != "" {
				configPath = cfgFile
			} else {
				configPath = "runix.yaml"
			}

			// Read config file.
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				// Try .yml extension.
				if _, err2 := os.Stat(configPath + "l"); err2 == nil {
					configPath = configPath + "l"
				} else {
					return fmt.Errorf("config file %q not found", configPath)
				}
			}

			v := viper.New()
			v.SetConfigFile(configPath)
			if err := v.ReadInConfig(); err != nil {
				return fmt.Errorf("failed to read config: %w", err)
			}

			var cfg types.RunixConfig
			if err := v.Unmarshal(&cfg); err != nil {
				return fmt.Errorf("failed to parse config: %w", err)
			}

			log.Info().Str("file", configPath).Msg("validating config")

			if err := cfg.Validate(); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Validation failed: %v\n", err)
				return fmt.Errorf("config validation failed")
			}

			procs := len(cfg.Processes)
			crons := len(cfg.Cron)
			_, _ = fmt.Fprintf(os.Stdout, "Configuration valid: %d process(es), %d cron job(s)\n", procs, crons)
			return nil
		},
	}

	return cmd
}
