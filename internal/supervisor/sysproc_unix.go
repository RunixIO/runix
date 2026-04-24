//go:build !windows

package supervisor

import (
	"os/exec"
	"strings"
	"syscall"
)

func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func signalProcessGroup(pid int, sig syscall.Signal) {
	_ = syscall.Kill(-pid, sig)
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
	case "SIGQUIT", "QUIT":
		return syscall.SIGQUIT
	case "SIGUSR1", "USR1":
		return syscall.SIGUSR1
	case "SIGUSR2", "USR2":
		return syscall.SIGUSR2
	case "SIGKILL", "KILL":
		return syscall.SIGKILL
	default:
		return syscall.SIGTERM
	}
}
