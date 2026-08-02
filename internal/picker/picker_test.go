package picker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/tui"
	"github.com/bethropolis/sift/internal/walker"
)

const fallbackSource = "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n"

// TestApplySelectionFallbackRead covers a selected file that has not streamed
// in yet: applySelection falls back to ReadEntry and still honours the mode.
func TestApplySelectionFallbackRead(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(fallbackSource), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.New()
	cfg.RootDir = dir
	cfg.Quiet = true
	cfg.OutputFile = "-"
	application, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()

	t.Run("signatures", func(t *testing.T) {
		chosen := applySelection(application, nil, []tui.Selection{{Path: "main.go", Mode: tui.ModeSignatures}})
		if len(chosen) != 1 {
			t.Fatalf("got %d chosen, want 1", len(chosen))
		}
		f := chosen[0]
		if !f.IsCompressed || !strings.Contains(string(f.Content), "func main()") {
			t.Errorf("fallback did not produce a signature view: compressed=%v content=%q", f.IsCompressed, f.Content)
		}
	})

	t.Run("full", func(t *testing.T) {
		chosen := applySelection(application, nil, []tui.Selection{{Path: "main.go", Mode: tui.ModeFull}})
		if len(chosen) != 1 || string(chosen[0].Content) != fallbackSource {
			t.Errorf("full fallback did not keep raw content: %q", chosen[0].Content)
		}
	})

	t.Run("skip", func(t *testing.T) {
		chosen := applySelection(application, nil, []tui.Selection{{Path: "main.go", Mode: tui.ModeSkip}})
		if len(chosen) != 0 {
			t.Errorf("got %d chosen for skip, want 0", len(chosen))
		}
	})
}

// TestStreamScanCancellation verifies that cancelling the context terminates
// the background scan promptly, surfaces context.Canceled on errCh, and
// closes every channel so the TUI never blocks on a cancelled pick.
func TestStreamScanCancellation(t *testing.T) {
	dir := writeSmokeTree(t)
	cfg := config.New()
	cfg.RootDir = dir
	cfg.SmartFilter = true
	cfg.Quiet = true
	application, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()

	ctx, cancel := context.WithCancel(context.Background())
	var stateMu sync.Mutex
	var collected []format.FileEntry
	var skippedMu sync.Mutex
	var skipped []walker.SkippedItem
	stream := newPickStream()

	go streamScan(ctx, application, nil, dir, nil, &stateMu, &collected, &skippedMu, &skipped, 0, 0, stream)
	cancel()

	deadline := time.After(10 * time.Second)
	sawCancel := false
	for stream.nodes != nil || stream.progress != nil || stream.errCh != nil {
		select {
		case _, ok := <-stream.nodes:
			if !ok {
				stream.nodes = nil
			}
		case _, ok := <-stream.progress:
			if !ok {
				stream.progress = nil
			}
		case e, ok := <-stream.errCh:
			if !ok {
				stream.errCh = nil
				continue
			}
			if errors.Is(e, context.Canceled) {
				sawCancel = true
			}
		case <-deadline:
			t.Fatal("streamScan did not terminate after cancellation")
		}
	}
	if !sawCancel {
		t.Error("errCh did not surface context.Canceled")
	}
}
