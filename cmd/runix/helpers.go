package main

import (
	"encoding/json"
	"errors"
	"os"
	"unicode"

	"github.com/runixio/runix/internal/daemon"
	"github.com/runixio/runix/internal/output"
	"github.com/runixio/runix/pkg/types"
	"github.com/spf13/cobra"
)

// splitShellArgs splits a command string into arguments, respecting
// single and double quotes (quotes are stripped from results).
// E.g. `bash -c 'echo hello; sleep 30'` → ["bash", "-c", "echo hello; sleep 30"]
func splitShellArgs(s string) ([]string, error) {
	var args []string
	var buf []rune
	var quote rune // 0 = no quote, '"' or '\''
	escaped := false

	for _, r := range s {
		if escaped {
			buf = append(buf, r)
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		switch {
		case quote != 0 && r == quote:
			// Closing quote.
			quote = 0
		case quote == 0 && (r == '\'' || r == '"'):
			// Opening quote.
			quote = r
		case quote == 0 && unicode.IsSpace(r):
			if len(buf) > 0 {
				args = append(args, string(buf))
				buf = buf[:0]
			}
		default:
			buf = append(buf, r)
		}
	}

	if escaped {
		return nil, errors.New("trailing backslash in command string")
	}
	if quote != 0 {
		return nil, errors.New("unterminated quote in command string")
	}
	if len(buf) > 0 {
		args = append(args, string(buf))
	}
	return args, nil
}

// displayState returns a user-friendly, colorized status string for a process state.
func displayState(s types.ProcessState) string {
	var state string
	switch s {
	case types.StateRunning:
		state = "online"
	case types.StateStarting:
		state = "launching"
	case types.StateStopped:
		state = "stopped"
	case types.StateCrashed:
		state = "errored"
	case types.StateErrored:
		state = "errored"
	case types.StateStopping:
		state = "stopping"
	case types.StateWaiting:
		state = "waiting"
	default:
		state = string(s)
	}
	return output.StatusSprint(state)
}

// outputResult writes output in the requested format (text or json).
func outputResult(format string, v any, textFn func()) {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(v)
		return
	}
	textFn()
}

// showSpeedList prints the process list after a mutating command (like pm2's speedList).
// Silently does nothing if the daemon is not reachable.
func showSpeedList() {
	resp, err := sendIPC(daemon.ActionList, nil)
	if err != nil || !resp.Success {
		return
	}
	var procs []types.ProcessInfo
	if err := json.Unmarshal(resp.Data, &procs); err != nil {
		return
	}
	printProcessTable(procs)
}

// resolveConfigPath returns the config file path from:
// 1. --config persistent flag (set by cobra into cfgFile via StringVar)
// 2. RUNIX_CONFIG environment variable
// 3. Empty string (daemon will use its own loaded config)
func resolveConfigPath(_ *cobra.Command) string {
	if cfgFile != "" {
		return cfgFile
	}
	if env := os.Getenv("RUNIX_CONFIG"); env != "" {
		return env
	}
	return ""
}
