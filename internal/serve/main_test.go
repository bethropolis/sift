package serve

import (
	"os"
	"testing"
)

// TestMain points UserConfigDir at a throwaway dir: handler tests PUT
// settings (theme included), which used to clobber the developer's real
// ~/.config/sift/preferences.json on every `go test` run.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "sift-test-config")
	if err != nil {
		os.Exit(1)
	}
	os.Setenv("XDG_CONFIG_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
