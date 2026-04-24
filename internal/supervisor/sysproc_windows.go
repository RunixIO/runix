//go:build windows

package supervisor

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func setProcessGroup(_ *exec.Cmd) {
	// Windows does not support process groups via Setpgid.
}

func signalProcessGroup(pid int, sig syscall.Signal) {
	// On Windows, signal the process directly (no process group support).
	if p, err := os.FindProcess(pid); err == nil {
		_ = p.Signal(sig)
	}
}

func killSignal() syscall.Signal {
	return syscall.SIGKILL
}

func defaultStopSignal() syscall.Signal {
	return syscall.SIGTERM
}

func parseSignal(s string) syscall.Signal {
	switch strings.ToUpper(s) {
	case "SIGTERM", "TERM":
		return syscall.SIGTERM
	case "SIGINT", "INT":
		return syscall.SIGINT
	case "SIGKILL", "KILL":
		return syscall.SIGKILL
	default:
		return syscall.SIGTERM
	}
}
