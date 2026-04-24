//go:build windows

package hooks

import "os/exec"

func setProcessGroup(_ *exec.Cmd) {
	// Windows does not support process groups via Setpgid.
}
