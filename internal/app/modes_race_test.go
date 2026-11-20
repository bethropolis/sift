package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/bethropolis/sift/internal/config"
)

// TestScanModesMatcherConcurrentSafety exercises the walker's concurrent
// workers against both an empty and a populated [modes] matcher. Run with
// -race: lazy matcher construction used to race here.
func TestScanModesMatcherConcurrentSafety(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 24; i++ {
		name := fmt.Sprintf("file%02d.go", i)
		body := fmt.Sprintf("package p\n\nfunc F%d() int { return %d }\n", i, i)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# docs\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		modes config.ModesConfig
	}{
		{name: "empty"},
		{
			name: "populated",
			modes: config.ModesConfig{
				Full:       config.StringList{"file0*.go"},
				Signatures: config.StringList{"file1*.go"},
				Skip:       config.StringList{"*.md"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.New()
			cfg.RootDir = dir
			cfg.Concurrent = true
			cfg.MaxWorkers = 8
			cfg.Modes = tc.modes
			cfg.SmartFilter = false

			var wg sync.WaitGroup
			for i := 0; i < 4; i++ {
				wg.Add(2)
				go func() {
					defer wg.Done()
					if _, _, err := Scan(context.Background(), dir, cfg); err != nil {
						t.Errorf("Scan: %v", err)
					}
				}()
				go func() {
					defer wg.Done()
					if _, _, err := ScanPicker(context.Background(), dir, cfg); err != nil {
						t.Errorf("ScanPicker: %v", err)
					}
				}()
			}
			wg.Wait()
		})
	}
}
