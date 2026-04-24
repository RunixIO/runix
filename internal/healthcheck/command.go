package healthcheck

import (
	"context"
	"os/exec"
)

func (c *Checker) checkCommand(ctx context.Context) error {
	// Intentional: healthcheck commands are user-defined from runix.yaml.
	cmd := exec.CommandContext(ctx, "sh", "-c", validateCommand(c.config.Command)) //codeql[go/command-injection]
	return cmd.Run()
}

// validateCommand strips null bytes from a command string sourced from config.
func validateCommand(cmd string) string {
	for i := range len(cmd) {
		if cmd[i] == 0 {
			// Null bytes could truncate the command unexpectedly.
			return cmd[:i]
		}
	}
	return cmd
}
