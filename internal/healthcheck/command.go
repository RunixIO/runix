package healthcheck

import (
	"context"
	"os/exec"
)

func (c *Checker) checkCommand(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", c.config.Command)
	return cmd.Run()
}
