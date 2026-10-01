// Package clipboard copies text to the system clipboard using the platform's
// native tool, avoiding X11-specific dependencies.
package clipboard

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// Copy writes data to the system clipboard.
func Copy(data []byte) error {
	name, args, err := commandFor()
	if err != nil {
		return err
	}

	cmd := exec.Command(name, args...)
	cmd.Stdin = bytes.NewReader(data)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("clipboard: %s: %v: %s", name, err, out)
	}
	return nil
}

// commandFor selects the clipboard tool for the current platform.
func commandFor() (name string, args []string, err error) {
	switch {
	case runtime.GOOS == "darwin":
		return "pbcopy", nil, nil
	case runtime.GOOS == "windows":
		return "clip", nil, nil
	case os.Getenv("WAYLAND_DISPLAY") != "":
		return "wl-copy", nil, nil
	case hasBinary("xclip"):
		return "xclip", []string{"-selection", "clipboard"}, nil
	case hasBinary("xsel"):
		return "xsel", []string{"--clipboard", "--input"}, nil
	default:
		return "", nil, errors.New("no clipboard utility found (need wl-copy, xclip, or pbcopy)")
	}
}

func hasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
