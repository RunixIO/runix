package main

import (
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/runixio/runix/internal/updater"
	"github.com/runixio/runix/internal/version"
	"github.com/spf13/cobra"
)

func newSelfUpdateCmd() *cobra.Command {
	var (
		checkOnly bool
		targetVer string
	)

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update Runix to the latest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check for updates.
			result, err := updater.CheckForUpdate(cmd.Context())
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error checking for updates: %v\n", err)
				return fmt.Errorf("check failed: %w", err)
			}

			fmt.Fprintf(os.Stdout, "Current: %s\n", result.CurrentVersion)
			fmt.Fprintf(os.Stdout, "Latest:  %s\n", result.LatestVersion)

			if !result.HasUpdate && targetVer == "" {
				fmt.Fprintln(os.Stdout, "Already up to date.")
				return nil
			}

			if checkOnly {
				if result.HasUpdate {
					fmt.Fprintf(os.Stdout, "Update available: %s\n", result.ReleaseURL)
				}
				return nil
			}

			ver := targetVer
			if ver == "" {
				ver = result.LatestVersion
			}

			fmt.Fprintf(os.Stdout, "Updating to %s...\n", ver)
			log.Info().Str("version", ver).Msg("starting self-update")

			if err := updater.SelfUpdate(cmd.Context(), ver); err != nil {
				fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
				return fmt.Errorf("update failed: %w", err)
			}

			fmt.Fprintf(os.Stdout, "Updated to %s successfully.\n", ver)
			return nil
		},
	}

	cmd.Flags().BoolVar(&checkOnly, "check", false, "only check for updates, do not install")
	cmd.Flags().StringVar(&targetVer, "version", "", "install a specific version")

	return cmd
}

// init ensures version package is referenced.
var _ = version.Version
