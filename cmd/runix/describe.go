package main

import (
	"github.com/spf13/cobra"
)

func newDescribeCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:    "describe <id|name>",
		Short:  "Show process details (deprecated: use inspect)",
		Long:   "Describe is deprecated. Use 'runix inspect' instead.",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInspect(args[0], format, 0)
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "text", "output format: text, json")

	return cmd
}
