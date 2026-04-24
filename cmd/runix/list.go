package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/runixio/runix/internal/daemon"
	"github.com/runixio/runix/internal/output"
	"github.com/runixio/runix/pkg/types"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var (
		format    string
		filter    string
		namespace string
		runtime_  string
		tag       string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all managed processes",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Try daemon IPC first (auto-starts daemon if needed).
			resp, err := sendIPC(daemon.ActionList, nil)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "[Runix] Daemon IPC failed, using direct mode: %v\n", err)
			} else if resp.Success {
				var procs []types.ProcessInfo
				if err := json.Unmarshal(resp.Data, &procs); err == nil {
					procs = filterProcesses(procs, filter, namespace, runtime_, tag)
					return printProcessList(procs, format)
				}
			}

			// Direct mode.
			sup, err := getSupervisor()
			if err != nil {
				return err
			}

			procs := sup.List()
			procs = filterProcesses(procs, filter, namespace, runtime_, tag)

			return printProcessList(procs, format)
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format: table, json, yaml")
	cmd.Flags().StringVarP(&filter, "filter", "F", "", "filter by state: running, stopped, crashed, errored, all")
	cmd.Flags().StringVarP(&namespace, "namespace", "N", "", "filter by namespace")
	cmd.Flags().StringVarP(&runtime_, "runtime", "r", "", "filter by runtime: go, python, node, bun, deno, ruby, php")
	cmd.Flags().StringVar(&tag, "tag", "", "filter by tag")

	return cmd
}

func printProcessList(procs []types.ProcessInfo, format string) error {
	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(procs)
	case "yaml":
		for _, p := range procs {
			_, _ = fmt.Fprintf(os.Stdout, "- id: %d\n  name: %s\n  runtime: %s\n  state: %s\n  ready: %t\n  pid: %d\n  restarts: %d\n  uptime: %s\n  cpu: %.1f\n  memory: %s\n",
				p.NumericID, p.Name, p.Runtime, p.State, p.Ready, p.PID, p.Restarts, p.UptimeString(), p.CPUPercent, output.FormatBytes(p.MemBytes))
		}
	default:
		printProcessTable(procs)
	}
	return nil
}

func printProcessTable(procs []types.ProcessInfo) {
	tbl := output.NewTable("ID", "NAME", "STATUS", "READY", "PID", "CPU%", "MEM", "UPTIME", "↺")
	tbl.SetAlign(0, output.AlignRight) // ID
	tbl.SetAlign(4, output.AlignRight) // PID
	tbl.SetAlign(5, output.AlignRight) // CPU%
	tbl.SetAlign(8, output.AlignRight) // RESTARTS

	for _, p := range procs {
		pid := "-"
		cpu := "-"
		mem := "-"
		uptime := "-"
		if p.State == types.StateRunning || p.State == types.StateStarting {
			pid = output.Sprintf("%d", p.PID)
			cpu = output.FormatCPU(p.CPUPercent)
			mem = output.FormatBytes(p.MemBytes)
			uptime = p.UptimeString()
			if uptime == "" {
				uptime = "-"
			}
		}
		tbl.AddRow(
			output.Sprintf("%d", p.NumericID),
			p.Name,
			displayState(p.State),
			output.EnabledSprint(p.Ready),
			pid,
			cpu,
			mem,
			uptime,
			output.Sprintf("%d", p.Restarts),
		)
	}
	_, _ = fmt.Fprint(os.Stdout, tbl.Render())
}

func filterProcesses(procs []types.ProcessInfo, filter string, namespace string, runtime_ string, tag string) []types.ProcessInfo {
	if filter == "" && namespace == "" && runtime_ == "" && tag == "" {
		return procs
	}

	var filtered []types.ProcessInfo
	for _, p := range procs {
		if filter != "" && filter != "all" {
			// Match against both internal state and display state.
			stateMatch := strings.EqualFold(string(p.State), filter) ||
				strings.EqualFold(displayState(p.State), filter)
			if !stateMatch {
				continue
			}
		}
		if namespace != "" && p.Namespace != namespace {
			continue
		}
		if runtime_ != "" && p.Runtime != runtime_ {
			continue
		}
		if tag != "" {
			found := false
			for _, t := range p.Tags {
				if t == tag {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		filtered = append(filtered, p)
	}
	return filtered
}
