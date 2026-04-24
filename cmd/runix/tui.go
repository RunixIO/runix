package main

import (
	"encoding/json"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/runixio/runix/internal/daemon"
	"github.com/runixio/runix/internal/tui"
	"github.com/runixio/runix/pkg/types"
	"github.com/spf13/cobra"
)

func newTUICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Launch the terminal UI dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			var lister tui.ProcessLister

			// Try to populate from daemon.
			if daemonIsRunning() {
				lister = &daemonLister{}
			} else {
				// Direct mode: create a supervisor and use its process list.
				sup, err := getSupervisor()
				if err != nil {
					return fmt.Errorf("failed to create supervisor: %w", err)
				}
				procs := sup.List()
				lister = &tui.DirectLister{Procs: procs}
			}

			app := tui.NewApp(lister)
			p := tea.NewProgram(
				app,
				tea.WithAltScreen(),
				tea.WithMouseCellMotion(),
			)

			if _, err := p.Run(); err != nil {
				return fmt.Errorf("TUI error: %w", err)
			}
			return nil
		},
	}

	return cmd
}

// daemonLister fetches process info from the daemon via IPC.
type daemonLister struct{}

func (d *daemonLister) ListProcesses() ([]types.ProcessInfo, error) {
	resp, err := sendIPC(daemon.ActionList, nil)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("daemon error: %s", resp.Error)
	}
	var procs []types.ProcessInfo
	if err := json.Unmarshal(resp.Data, &procs); err != nil {
		return nil, err
	}
	return procs, nil
}

func (d *daemonLister) GetProcess(idOrName string) (types.ProcessInfo, error) {
	resp, err := sendIPC(daemon.ActionStatus, daemon.StopPayload{Target: idOrName})
	if err != nil {
		return types.ProcessInfo{}, err
	}
	if !resp.Success {
		return types.ProcessInfo{}, fmt.Errorf("daemon error: %s", resp.Error)
	}
	var info types.ProcessInfo
	if err := json.Unmarshal(resp.Data, &info); err != nil {
		return types.ProcessInfo{}, err
	}
	return info, nil
}
