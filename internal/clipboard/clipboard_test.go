package clipboard

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// fakeHelper builds a stand-in clipboard tool so the exit policies can be
// tested without wl-clipboard/X11 being installed. mode selects the behavior:
// "linger" (daemonize, like wl-copy/xclip), "exit" (return cleanly), or
// "fail" (exit non-zero with a message).
func fakeHelper(t *testing.T, mode string) helper {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake helper uses a shell script")
	}
	dir := t.TempDir()
	script := map[string]string{
		// Read all of stdin, then sleep well past any grace window: this is
		// exactly what makes the real tools appear to hang.
		"linger": "#!/bin/sh\ncat > /dev/null\nsleep 30\n",
		"exit":   "#!/bin/sh\ncat > /dev/null\n",
		"fail":   "#!/bin/sh\ncat > /dev/null\necho 'no display' >&2\nexit 1\n",
	}[mode]
	path := filepath.Join(dir, "faketool")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return helper{name: path, grace: 500 * time.Millisecond, lingering: mode == "linger"}
}

// TestRunLingeringHelperDoesNotBlock is the regression test for the picker
// hanging: a helper that stays alive to own the selection must be reported as
// success promptly, not waited on until it dies.
func TestRunLingeringHelperDoesNotBlock(t *testing.T) {
	h := fakeHelper(t, "linger")
	payload := []byte(strings.Repeat("payload ", 5000))

	start := time.Now()
	err := run(h, payload)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("run() = %v, want success for a lingering helper", err)
	}
	// The helper sleeps 30s; we must return long before that.
	if elapsed > 10*time.Second {
		t.Errorf("run() blocked for %s; lingering helpers must not be waited on", elapsed)
	}
}

// TestRunExitingHelperSucceeds covers the pbcopy/xsel case, where waiting for
// a clean exit is correct.
func TestRunExitingHelperSucceeds(t *testing.T) {
	h := fakeHelper(t, "exit")
	if err := run(h, []byte("some document")); err != nil {
		t.Errorf("run() = %v, want nil for a cleanly exiting helper", err)
	}
}

// TestRunFailingHelperReportsError ensures a real failure (no display) is
// still surfaced rather than swallowed by the grace window.
func TestRunFailingHelperReportsError(t *testing.T) {
	h := fakeHelper(t, "fail")
	err := run(h, []byte("doc"))
	if err == nil {
		t.Fatal("run() = nil, want an error for a failing helper")
	}
	if !strings.Contains(err.Error(), "no display") {
		t.Errorf("error = %v, want it to carry the helper's stderr", err)
	}
}

// TestRunExitingHelperTimeout verifies a stuck non-lingering helper is killed
// and reported instead of blocking the caller indefinitely.
func TestRunExitingHelperTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake helper uses a shell script")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "stuck")
	// Accepts stdin then ignores it forever: an exiting helper that wedged.
	if err := os.WriteFile(path, []byte("#!/bin/sh\ncat > /dev/null\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	h := helper{name: path, grace: 300 * time.Millisecond, lingering: false}

	start := time.Now()
	err := run(h, []byte("doc"))
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("run() = nil, want a timeout error for a stuck exiting helper")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error = %v, want a timeout error", err)
	}
	if elapsed > 10*time.Second {
		t.Errorf("run() blocked for %s; a stuck helper must be killed", elapsed)
	}
}

// TestRunLargePayloadToLingeringHelper guards the pipe-buffer deadlock: the
// payload is much larger than a pipe buffer, so it can only complete if stdin
// is streamed rather than written in one blocking call.
func TestRunLargePayloadToLingeringHelper(t *testing.T) {
	h := fakeHelper(t, "linger")
	// ~8MB, far beyond the 64KB pipe buffer.
	payload := []byte(strings.Repeat("0123456789abcdef", 512*1024))

	start := time.Now()
	if err := run(h, payload); err != nil {
		t.Fatalf("run() with large payload = %v", err)
	}
	if elapsed := time.Since(start); elapsed > 15*time.Second {
		t.Errorf("large payload took %s", elapsed)
	}
}

// TestCappedBufferBoundsStderr ensures a chatty helper cannot grow memory
// without limit, and that truncation is reported in the error text.
func TestCappedBufferBoundsStderr(t *testing.T) {
	var c cappedBuffer
	c.limit = 8
	if _, err := c.Write([]byte("0123456789ABCDEF")); err != nil {
		t.Fatal(err)
	}
	if c.buf.Len() != 8 {
		t.Errorf("buffered %d bytes, want the 8-byte cap", c.buf.Len())
	}
	if !c.truncated {
		t.Error("truncated = false, want true after overwriting the cap")
	}
	if !strings.Contains(c.text(), "truncated") {
		t.Errorf("text() = %q, want it to mark truncation", c.text())
	}
}

func TestCappedBufferWriteReportsFullLength(t *testing.T) {
	// exec's copier treats a short write as an error, so Write must always
	// report len(p) consumed even when it discards the excess.
	var c cappedBuffer
	c.limit = 2
	n, err := c.Write([]byte("abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 6 {
		t.Errorf("Write returned n = %d, want 6 (all input consumed)", n)
	}
}

// TestMissingBinaryReportsError covers the case where the selected tool is
// absent, so Copy fails fast instead of hanging.
func TestMissingBinaryReportsError(t *testing.T) {
	h := helper{name: filepath.Join(t.TempDir(), "does-not-exist"), grace: time.Second}
	if err := run(h, []byte("doc")); err == nil {
		t.Error("run() = nil, want an error for a missing binary")
	}
}

func TestCommandForWayland(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "wayland-1")

	h, err := helperFor()
	if err != nil {
		// wl-copy may not be installed; the error must still be about that.
		if !strings.Contains(err.Error(), "wl-copy") {
			t.Fatalf("helperFor() error = %v, want it to name wl-copy", err)
		}
		t.Logf("wl-copy not installed here: %v", err)
		return
	}
	if h.name != "wl-copy" {
		t.Errorf("helperFor() = %q, want wl-copy", h.name)
	}
	// wl-copy daemonizes, so it must never be treated as an exiting helper and
	// must not be passed flags that force foreground blocking.
	if !h.lingering {
		t.Error("wl-copy must be marked lingering; it stays alive to own the selection")
	}
	for _, a := range h.args {
		if a == "-f" || a == "--foreground" {
			t.Errorf("wl-copy args = %v, must omit --foreground or it blocks", h.args)
		}
	}
}

func TestHelperExitPolicies(t *testing.T) {
	// Linger-capable helpers own the selection from a background process, so
	// waiting for their exit is what caused the picker to hang.
	for _, name := range []string{"wl-copy", "xclip"} {
		if name == "wl-copy" {
			t.Setenv("WAYLAND_DISPLAY", "wayland-1")
		}
		h, err := helperFor()
		if err != nil || h.name != name {
			continue // tool not installed on this machine
		}
		if !h.lingering {
			t.Errorf("%s must be marked lingering", name)
		}
	}
}

func TestCommandForNoTool(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "")

	// Only assert on the error path when neither xclip nor xsel is present,
	// so the test does not depend on the machine's installed tools.
	h, err := helperFor()
	if err != nil {
		if h.name != "" {
			t.Errorf("helperFor() returned name %q with error %v", h.name, err)
		}
		return
	}
	// A tool was found; that is fine too.
	t.Logf("using %q", h.name)
}
