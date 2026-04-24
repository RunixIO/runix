package main

import (
	"fmt"
	"runtime"

	"github.com/runixio/runix/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print Runix version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "Runix %s\n", version.Version)
			if verbose {
				fmt.Fprintf(cmd.OutOrStdout(), "  Build:    %s\n", version.BuildTime)
				fmt.Fprintf(cmd.OutOrStdout(), "  Go:       %s\n", runtime.Version())
				fmt.Fprintf(cmd.OutOrStdout(), "  OS/Arch:  %s/%s\n", runtime.GOOS, runtime.GOARCH)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&verbose, "verbose", false, "show build details")

	return cmd
}
