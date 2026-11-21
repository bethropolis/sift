package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bethropolis/sift/internal/watch"
)

// TestWatchAllTargetsIgnoresDefaultProfile covers the ordering bug where
// applyProfile ran before prepareTargetRuns and leaked default_profile fields
// into targets that select another profile.
func TestWatchAllTargetsIgnoresDefaultProfile(t *testing.T) {
	root := writeTargetRepo(t, `
default_profile = "leaky"

[profiles.leaky]
prompt = "LEAKED DEFAULT PROMPT"

[profiles.clean]
style = "plain"

[[targets]]
name = "clean"
profile = "clean"
output = "target.md"
`)

	oldRunWatch := runWatch
	runWatch = func(context.Context, watch.Options) error { return nil }
	t.Cleanup(func() { runWatch = oldRunWatch })

	if _, err := runSift(t, "watch", "--all-targets", "--quiet", root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "target.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "LEAKED DEFAULT PROMPT") {
		t.Fatalf("default_profile leaked into target output:\n%s", data)
	}
}
