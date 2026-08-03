package picker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
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
// the background scan promptly and closes every channel so the TUI never
// blocks on a cancelled pick. Any error that arrives must be context.Canceled;
// on cancellation the error send is best-effort, so it may be dropped.
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
			if !errors.Is(e, context.Canceled) {
				t.Errorf("errCh = %v, want context.Canceled", e)
			}
		case <-deadline:
			t.Fatal("streamScan did not terminate after cancellation")
		}
	}
}

// TestStreamScanCancellationNoReceiver verifies the producer terminates
// promptly when the context is cancelled and nobody is reading the channels.
// Without cancellation-aware sends, a blocked send on a full channel would
// hang the goroutine forever once the TUI stopped draining.
func TestStreamScanCancellationNoReceiver(t *testing.T) {
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

	done := make(chan struct{})
	go func() {
		defer close(done)
		streamScan(ctx, application, nil, dir, nil, &stateMu, &collected, &skippedMu, &skipped, 0, 0, stream)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("streamScan blocked after cancellation with no receiver")
	}
}

// TestRunNoEligible verifies the picker reports NoEligible when every file is
// filtered by the binary or size rules, without opening the TUI (the skeleton
// is empty, so no terminal is required).
func TestRunNoEligible(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "data.iso"), []byte("zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	big := filepath.Join(dir, "big.txt")
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
	cfg.RootDir = dir
	cfg.SmartFilter = true
	cfg.Quiet = true
	cfg.MaxFileSizeMB = 1
	application, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()

	result, err := Run(context.Background(), cfg, Env{
		App:         application,
		RecordDump:  func(string, []format.FileEntry, string) error { return nil },
		RecordDelta: func(string, string, int, int) error { return nil },
		CountTokens: func([]byte) int { return 0 },
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.NoEligible {
		t.Error("NoEligible = false, want true for a fully filtered directory")
	}
	if len(result.Selections) != 0 {
		t.Errorf("Selections = %v, want empty", result.Selections)
	}
}

// TestMergeSkippedDedups verifies the stream's skipped-item merge keeps one
// entry per path with the first-seen reason, so size and binary skips recorded
// by both the metadata and content passes are not reported twice.
func TestMergeSkippedDedups(t *testing.T) {
	dst := []walker.SkippedItem{
		{Path: "big.txt", Reason: walker.ReasonSkippedSizeLimit},
		{Path: "data.iso", Reason: walker.ReasonSkippedBinary},
	}
	src := []walker.SkippedItem{
		{Path: "big.txt", Reason: walker.ReasonSkippedSizeLimit},
		{Path: "main.go", Reason: walker.ReasonSkippedSmart},
	}
	mergeSkipped(&dst, src)

	if len(dst) != 3 {
		t.Fatalf("merged length = %d, want 3", len(dst))
	}
	byPath := map[string]walker.SkippedItem{}
	for _, s := range dst {
		byPath[s.Path] = s
	}
	if byPath["big.txt"].Reason != walker.ReasonSkippedSizeLimit {
		t.Errorf("big.txt reason = %q, want first-seen %q", byPath["big.txt"].Reason, walker.ReasonSkippedSizeLimit)
	}
	if _, ok := byPath["main.go"]; !ok {
		t.Error("main.go missing from merged skipped items")
	}
}

func TestPickerWindowTitle(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		cfg := config.New()
		cfg.RootDir = "/repo/src/myproject"
		if got := pickerWindowTitle(cfg); got != "sift | myproject" {
			t.Errorf("title = %q, want %q", got, "sift | myproject")
		}
	})

	t.Run("custom", func(t *testing.T) {
		cfg := config.New()
		cfg.RootDir = "/repo"
		cfg.WindowTitle = "My Custom Title"
		if got := pickerWindowTitle(cfg); got != "My Custom Title" {
			t.Errorf("title = %q, want custom", got)
		}
	})

	t.Run("disabled", func(t *testing.T) {
		cfg := config.New()
		cfg.RootDir = "/repo"
		cfg.NoWindowTitle = true
		if got := pickerWindowTitle(cfg); got != "" {
			t.Errorf("title = %q, want empty when disabled", got)
		}
	})

	t.Run("root-directory", func(t *testing.T) {
		cfg := config.New()
		cfg.RootDir = "/"
		if got := pickerWindowTitle(cfg); got != "sift | directory" {
			t.Errorf("title = %q, want %q", got, "sift | directory")
		}
	})

	t.Run("relative-basename", func(t *testing.T) {
		cfg := config.New()
		cfg.RootDir = "rel/proj"
		if got := pickerWindowTitle(cfg); got != "sift | proj" {
			t.Errorf("title = %q, want basename of relative root", got)
		}
	})
}

// waitForGoroutinesToSettle polls runtime.NumGoroutine until it stays at or
// below the baseline for a short window. A hard equality assertion on goroutine
// counts is brittle because the Go runtime and the test binary spawn their own
// goroutines; a bounded settle window proves no leak without false positives.
func waitForGoroutinesToSettle(t *testing.T, baseline int, budget time.Duration) {
	t.Helper()
	deadline := time.Now().Add(budget)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= baseline {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("goroutine count %d did not settle to baseline %d", runtime.NumGoroutine(), baseline)
}

// TestStreamScanLeavesNoGoroutines verifies the idle-behavior guarantee: once
// a scan completes normally, its progress goroutine and walker workers are
// gone, so the picker no longer schedules polling work.
func TestStreamScanLeavesNoGoroutines(t *testing.T) {
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

	baseline := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stateMu sync.Mutex
	var collected []format.FileEntry
	var skippedMu sync.Mutex
	var skipped []walker.SkippedItem
	stream := newPickStream()

	done := make(chan struct{})
	go func() {
		defer close(done)
		streamScan(ctx, application, nil, dir, nil, &stateMu, &collected, &skippedMu, &skipped, 0, 0, stream)
	}()

	// Drain until every channel closes, like the TUI listener does.
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
		case _, ok := <-stream.errCh:
			if !ok {
				stream.errCh = nil
			}
		}
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("streamScan did not return after completion")
	}

	waitForGoroutinesToSettle(t, baseline, 5*time.Second)
}

// TestStreamScanCancellationLeavesNoGoroutines proves cancellation tears down
// the same goroutines: workers stop consuming and the progress goroutine exits
// without leaving a polling loop behind.
func TestStreamScanCancellationLeavesNoGoroutines(t *testing.T) {
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

	baseline := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	var stateMu sync.Mutex
	var collected []format.FileEntry
	var skippedMu sync.Mutex
	var skipped []walker.SkippedItem
	stream := newPickStream()

	done := make(chan struct{})
	go func() {
		defer close(done)
		streamScan(ctx, application, nil, dir, nil, &stateMu, &collected, &skippedMu, &skipped, 0, 0, stream)
	}()

	// Cancel before draining, forcing every producer to unwind on ctx.Done.
	cancel()

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
		case _, ok := <-stream.errCh:
			if !ok {
				stream.errCh = nil
			}
		}
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("streamScan did not return after cancellation")
	}

	waitForGoroutinesToSettle(t, baseline, 5*time.Second)
}
