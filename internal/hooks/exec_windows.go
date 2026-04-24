//go:build windows

package hooks

import (
	"os"
	"os/exec"
)

func setProcessGroup(_ *exec.Cmd) {
	// Windows does not support process groups via Setpgid.
}

func killProcessGroup(proc *os.Process) error {
	return proc.Kill()
}
