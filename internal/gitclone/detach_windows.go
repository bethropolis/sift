//go:build windows

package gitclone

import "os/exec"

// detach is a no-op on Windows, which has no controlling-terminal prompt
// path for ssh/git; GIT_TERMINAL_PROMPT=0 covers git's own prompts.
func detach(cmd *exec.Cmd) {}
