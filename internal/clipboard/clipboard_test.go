package clipboard

import (
	"testing"
)

func TestCommandForWayland(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "wayland-1")

	name, _, err := commandFor()
	if err != nil {
		t.Fatal(err)
	}
	if name != "wl-copy" {
		t.Errorf("commandFor() = %q, want wl-copy", name)
	}
}

func TestCommandForNoTool(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "")

	// Only assert on the error path when neither xclip nor xsel is present,
	// so the test does not depend on the machine's installed tools.
	name, _, err := commandFor()
	if err != nil {
		if name != "" {
			t.Errorf("commandFor() returned name %q with error %v", name, err)
		}
		return
	}
	// A tool was found; that is fine too.
	t.Logf("using %q", name)
}
