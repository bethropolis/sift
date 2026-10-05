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
	"sync"
	"time"
)

const (
	// lingerGrace bounds how long we wait for a daemonizing helper to either
	// exit (a real failure) or settle. Helpers that outlive it are serving the
	// selection from the background, which is success.
	lingerGrace = 2 * time.Second

	// exitGrace bounds the wait for helpers that are expected to exit once
	// they have taken the data.
	exitGrace = 30 * time.Second

	// maxStderr caps how much helper output we keep for error reporting. The
	// payload travels on stdin, so this only ever holds diagnostics.
	maxStderr = 8 << 10
)

// helper describes a clipboard tool and how it behaves after the payload is
// written. Linger-capable helpers (wl-copy, xclip) hand the data off and then
// stay alive in the background to serve the selection to whatever pastes next;
// exiting helpers return as soon as the data is delivered.
type helper struct {
	name string
	args []string
	// grace is how long to wait for the process to exit before deciding.
	grace time.Duration
	// lingering helpers are expected to outlive that wait. Still running at
	// the deadline means the copy landed, not that it hung.
	lingering bool
}

// Copy writes data to the system clipboard.
func Copy(data []byte) error {
	h, err := helperFor()
	if err != nil {
		return err
	}
	return run(h, data)
}

// run feeds data to the helper and applies its exit policy. A lingering helper
// that is still alive once grace expires has succeeded: wl-copy and xclip
// deliberately daemonize so the clipboard survives after sift exits, so
// blocking on them until they exit would hang forever.
func run(h helper, data []byte) error {
	cmd := exec.Command(h.name, h.args...)
	// os/exec streams this reader into the child's stdin on its own goroutine
	// and closes it when the reader is exhausted, so a payload far larger than
	// the pipe buffer cannot deadlock here.
	cmd.Stdin = bytes.NewReader(data)
	var stderr cappedBuffer
	stderr.limit = maxStderr
	cmd.Stderr = &stderr
	// Discard stdout rather than buffering it: the payload goes to stdin, so
	// anything on stdout is incidental noise we must not accumulate for a
	// document that can be many megabytes. A nil writer connects to os.DevNull.
	cmd.Stdout = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("clipboard: %s: %w", h.name, err)
	}
	// Reap the child on a dedicated goroutine. os/exec's Wait also waits for
	// its stdin/stdout copying goroutines, so this is the only point at which
	// the stderr buffer is safe to read; a lingering helper simply parks here
	// for its lifetime instead of leaking a zombie.
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()

	timer := time.NewTimer(h.grace)
	defer timer.Stop()

	select {
	case err := <-waited:
		// The helper exited on its own. Report a real failure, otherwise done.
		if err != nil {
			return fmt.Errorf("clipboard: %s: %w%s", h.name, err, stderr.text())
		}
		return nil
	case <-timer.C:
		if !h.lingering {
			// An exiting helper that overran its grace is genuinely stuck.
			// Killing the direct child is not always enough: a grandchild may
			// still hold the stderr pipe open, so cmd.Wait can block well past
			// the kill. Reap with a bound and report regardless.
			_ = cmd.Process.Kill()
			reap := time.NewTimer(time.Second)
			defer reap.Stop()
			select {
			case <-waited:
			case <-reap.C:
			}
			return fmt.Errorf("clipboard: %s: timed out after %s%s",
				h.name, h.grace, stderr.text())
		}
		// Still alive and expected to be: the selection is now owned by the
		// background helper and outlives this process.
		return nil
	}
}

// helperFor selects the clipboard tool for the current platform and tags it
// with its exit policy. Linger flags are deliberately omitted: --foreground
// would force the blocking behavior this package exists to avoid.
func helperFor() (helper, error) {
	switch {
	case runtime.GOOS == "darwin":
		return helper{name: "pbcopy", grace: exitGrace}, nil
	case runtime.GOOS == "windows":
		return helper{name: "clip", grace: exitGrace}, nil
	case os.Getenv("WAYLAND_DISPLAY") != "":
		if !hasBinary("wl-copy") {
			return helper{}, errors.New("WAYLAND_DISPLAY is set but wl-copy is not installed")
		}
		return helper{name: "wl-copy", grace: lingerGrace, lingering: true}, nil
	case hasBinary("xclip"):
		return helper{name: "xclip", args: []string{"-selection", "clipboard"},
			grace: lingerGrace, lingering: true}, nil
	case hasBinary("xsel"):
		return helper{name: "xsel", args: []string{"--clipboard", "--input"},
			grace: exitGrace}, nil
	default:
		return helper{}, errors.New("no clipboard utility found (need wl-copy, xclip, or pbcopy)")
	}
}

// cappedBuffer collects at most limit bytes and reports whether it truncated,
// so a chatty helper cannot balloon memory during a copy. It is safe for
// concurrent use: os/exec writes to it from its own goroutine, and text() may
// be called while that goroutine is still running when we stop waiting early.
type cappedBuffer struct {
	mu        sync.Mutex
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if remaining := c.limit - c.buf.Len(); remaining > 0 {
		if len(p) <= remaining {
			c.buf.Write(p)
		} else {
			c.buf.Write(p[:remaining])
			c.truncated = true
		}
		return len(p), nil
	}
	c.truncated = true
	return len(p), nil
}

// text renders the captured output for an error message, marking truncation.
func (c *cappedBuffer) text() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.buf.Len() == 0 {
		return ""
	}
	if c.truncated {
		return fmt.Sprintf(": %s (truncated)", c.buf.String())
	}
	return fmt.Sprintf(": %s", c.buf.String())
}

func hasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
