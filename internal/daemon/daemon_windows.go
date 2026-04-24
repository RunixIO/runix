//go:build windows

package daemon

import "os/exec"

func setProcessGroup(_ *exec.Cmd) {
	// Windows does not support process groups via Setpgid.
}
