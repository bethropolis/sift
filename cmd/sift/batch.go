package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/pflag"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/scan"
	"github.com/bethropolis/sift/internal/walker"
)

// targetRun is one resolved [[targets]] entry ready to collect and render.
type targetRun struct {
	name string
	cfg  *config.Config
	app  *app.App
}

// prepareTargetRuns resolves every named target from base and validates that
// their output files are distinct before anything is written. The returned
// runs share one content cache.
func prepareTargetRuns(base config.Config, flags *pflag.FlagSet) ([]*targetRun, error) {
	names, err := config.TargetNames(&base)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("--all-targets: no [[targets]] are defined")
	}
	outputs := make(map[string]string, len(names))
	runs := make([]*targetRun, 0, len(names))
	for _, name := range names {
		runCfg := base
		runCfg.Target = name
		if err := config.ResolveConfig(&runCfg, flags, config.WithTarget(name)); err != nil {
			return nil, fmt.Errorf("target %q: %w", name, err)
		}
		out := runCfg.OutputFile
		if out == "" || out == "-" {
			out = "-"
		} else if !filepath.IsAbs(out) {
			baseDir := runCfg.RootDir
			if baseDir == "" {
				baseDir = "."
			}
			out = filepath.Join(baseDir, out)
		}
		out, _ = filepath.Abs(out)
		if prev, clash := outputs[out]; clash {
			return nil, fmt.Errorf("targets %q and %q write the same output %s", prev, name, out)
		}
		outputs[out] = name
		runs = append(runs, &targetRun{name: name, cfg: &runCfg})
	}

	var cache *scan.ContentCache
	if len(runs) > 0 {
		cache = app.NewContentCacheWithDisk(runs[0].cfg)
	}
	for _, run := range runs {
		application, err := app.NewWithCache(run.cfg, cache)
		if err != nil {
			closeTargetRuns(runs)
			return nil, fmt.Errorf("target %q: %w", run.name, err)
		}
		run.app = application
	}
	return runs, nil
}

func closeTargetRuns(runs []*targetRun) {
	for _, run := range runs {
		if run.app != nil {
			run.app.Close()
		}
	}
}

// renderTargetRuns collects and renders each prepared target, printing one
// concise summary line per target unless quiet.
func renderTargetRuns(runs []*targetRun) error {
	for _, run := range runs {
		start := time.Now()
		files, skipped, err := run.app.Collect()
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("target %q: %w", run.name, err)
		}
		metrics, renderErr := run.app.RenderEntriesWithMetrics(files, skipped, time.Since(start), err)
		if renderErr != nil {
			return fmt.Errorf("target %q: %w", run.name, renderErr)
		}
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("target %q: %w", run.name, err)
		}
		if recordErr := recordRenderMetrics(run.cfg.RootDir, metrics, "HEAD"); recordErr != nil {
			run.app.LogError("Failed to record dump state: %v", recordErr)
		}
		if !run.cfg.Quiet {
			out := run.cfg.OutputFile
			if out == "" {
				out = "-"
			}
			fmt.Fprintf(os.Stderr, "%s: %d files, %d tokens -> %s\n", run.name, metrics.Files, metrics.Tokens, out)
		}
	}
	return nil
}

// renderMultiRoot scans each root independently, namespaces its paths under a
// unique label, and renders the combined entry set through one document. One
// root keeps the historical unnamespaced behavior.
func renderMultiRoot(cfg *config.Config, roots []string) error {
	if len(roots) == 0 {
		return fmt.Errorf("no roots to scan")
	}
	absRoots := make([]string, len(roots))
	for i, root := range roots {
		abs, err := filepath.Abs(root)
		if err != nil {
			return err
		}
		absRoots[i] = abs
	}
	labels := uniqueRootLabels(absRoots)

	cache := app.NewContentCacheWithDisk(cfg)
	allFiles := make([]format.FileEntry, 0)
	allSkipped := make([]walker.SkippedItem, 0)
	for i, root := range absRoots {
		runCfg := *cfg
		runCfg.RootDir = root
		runCfg.Roots = nil
		runCfg.OutputFile = "-"
		runCfg.Clipboard = false
		runCfg.CopyOnGenerate = false
		application, err := app.NewWithCache(&runCfg, cache)
		if err != nil {
			return err
		}
		files, skipped, err := application.Collect()
		application.Close()
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		for j := range files {
			files[j].Path = labels[i] + "/" + filepath.ToSlash(files[j].Path)
		}
		allFiles = append(allFiles, files...)
		allSkipped = append(allSkipped, skipped...)
	}
	sort.SliceStable(allFiles, func(i, j int) bool {
		if allFiles[i].RankScore != allFiles[j].RankScore {
			return allFiles[i].RankScore > allFiles[j].RankScore
		}
		return allFiles[i].Path < allFiles[j].Path
	})

	topCfg := *cfg
	topCfg.RootDir = absRoots[0]
	topCfg.Roots = nil
	top, err := app.NewWithCache(&topCfg, cache)
	if err != nil {
		return err
	}
	defer top.Close()
	if _, err := top.RenderEntriesWithMetrics(allFiles, allSkipped, 0, nil); err != nil {
		return err
	}
	return nil
}

// uniqueRootLabels derives stable, unique display labels from root basenames,
// disambiguating same-named roots with their parent directory and then an
// ordinal.
func uniqueRootLabels(roots []string) []string {
	labels := make([]string, len(roots))
	used := map[string]bool{}
	for i, root := range roots {
		label := filepath.Base(filepath.Clean(root))
		if label == "." || label == string(filepath.Separator) || label == "" {
			label = "root"
		}
		if used[label] {
			parent := filepath.Base(filepath.Dir(filepath.Clean(root)))
			if parent != "" && parent != "." && parent != string(filepath.Separator) {
				label = parent + "-" + label
			}
		}
		candidate := label
		for n := 2; used[candidate]; n++ {
			candidate = fmt.Sprintf("%s-%d", label, n)
		}
		used[candidate] = true
		labels[i] = candidate
	}
	return labels
}
