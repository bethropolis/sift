package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/walker"
)

// TestSkeletonPickerExcludesLargeAndBinary verifies the picker's metadata
// skeleton drops oversized and binary files so the TUI never advertises files
// the content walk will reject, and that the skeleton and full walk agree on
// the eligible paths.
func TestSkeletonPickerExcludesLargeAndBinary(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go", "package main\n")
	if err := os.WriteFile(filepath.Join(root, "data.iso"), []byte("zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A 2 MB sparse text file exceeds the configured 1 MB cap without costing
	// disk space or reading contents.
	big := filepath.Join(root, "big.txt")
	f, err := os.OpenFile(big, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(2 * 1024 * 1024); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	cfg := config.New()
	cfg.RootDir = root
	cfg.Quiet = true
	cfg.OutputFile = "-"
	cfg.SmartFilter = true
	cfg.MaxFileSizeMB = 1
	a, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	ctx := context.Background()
	metas, skipped, err := a.SkeletonPicker(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(metas) != 1 || metas[0].Path != "main.go" {
		t.Errorf("skeleton metas = %v, want [main.go]", metas)
	}

	reasons := map[string]walker.SkippedReason{}
	for _, s := range skipped {
		reasons[s.Path] = s.Reason
	}
	if reasons["data.iso"] != walker.ReasonSkippedBinary {
		t.Errorf("data.iso reason = %q, want %q", reasons["data.iso"], walker.ReasonSkippedBinary)
	}
	if reasons["big.txt"] != walker.ReasonSkippedSizeLimit {
		t.Errorf("big.txt reason = %q, want %q", reasons["big.txt"], walker.ReasonSkippedSizeLimit)
	}

	// The content walk must agree with the skeleton on eligible paths.
	var walked []string
	_, err = a.StreamPicker(ctx, func(e format.FileEntry) error {
		walked = append(walked, e.Path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(walked) != 1 || walked[0] != "main.go" {
		t.Errorf("full walk produced %v, want [main.go]", walked)
	}
}
