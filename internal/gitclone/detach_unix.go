//go:build !windows

package gitclone

import (
	"os/exec"
	"syscall"
)

// detach starts git in its own session so it has no controlling terminal:
// ssh and git cannot open /dev/tty to prompt on the terminal that launched
// the server, and a repository needing a login fails instead of hanging.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
