package watch

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunFiresOnChange(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(target, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int64
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, Options{
			Root:     dir,
			Interval: 50 * time.Millisecond,
			OnChange: func() error {
				calls.Add(1)
				return nil
			},
		})
	}()

	// Give the watcher time to register directories.
	time.Sleep(200 * time.Millisecond)
	if err := os.WriteFile(target, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if calls.Load() == 0 {
		t.Fatal("OnChange never fired after file modification")
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("Run did not exit after context cancel")
	}
}

func TestRunIgnoresFilteredEvents(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "skip.txt")
	if err := os.WriteFile(target, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int64
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = Run(ctx, Options{
			Root:     dir,
			Interval: 30 * time.Millisecond,
			Ignore: func(path string) bool {
				return filepath.Base(path) == "skip.txt"
			},
			OnChange: func() error {
				calls.Add(1)
				return nil
			},
		})
	}()

	time.Sleep(150 * time.Millisecond)
	if err := os.WriteFile(target, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if calls.Load() != 0 {
		t.Errorf("OnChange fired %d times for ignored file", calls.Load())
	}
}
