package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
		if out != "-" {
			out, _ = filepath.Abs(out)
		}
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
		var metrics app.RenderMetrics
		var rendered []byte
		var renderErr error
		if historyEnabled(run.cfg) {
			metrics, rendered, renderErr = run.app.RenderEntriesWithMetricsCaptured(files, skipped, time.Since(start), err)
		} else {
			metrics, renderErr = run.app.RenderEntriesWithMetrics(files, skipped, time.Since(start), err)
		}
		if renderErr != nil {
			return fmt.Errorf("target %q: %w", run.name, renderErr)
		}
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("target %q: %w", run.name, err)
		}
		recordHistory(run.cfg, "dump", rendered, metrics)
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

	// A multi-root document is one file, so resolve its relative output
	// against the current working directory rather than the first root. The
	// same absolute path is then excluded from every root's walk.
	topCfg := *cfg
	topCfg.ResolveOutputFile()
	outputFile := topCfg.OutputFile
	if outputFile != "" && outputFile != "-" {
		if !filepath.IsAbs(outputFile) {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			outputFile = filepath.Join(cwd, outputFile)
		}
		absOut, err := filepath.Abs(outputFile)
		if err != nil {
			return err
		}
		outputFile = absOut
		topCfg.OutputFile = outputFile
		topCfg.OutputFileExplicit = true
	}

	cache := app.NewContentCacheWithDisk(cfg)
	allFiles := make([]format.FileEntry, 0)
	allSkipped := make([]walker.SkippedItem, 0)
	var archEdges []string
	start := time.Now()
	for i, root := range absRoots {
		runCfg := *cfg
		runCfg.RootDir = root
		runCfg.Roots = nil
		runCfg.Clipboard = false
		runCfg.CopyOnGenerate = false
		if outputFile == "-" {
			runCfg.OutputFile = "-"
		} else {
			runCfg.OutputFile = outputFile
			runCfg.OutputFileExplicit = true
		}
		application, err := app.NewWithCache(&runCfg, cache)
		if err != nil {
			return err
		}
		files, skipped, err := application.Collect()
		application.Close()
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		if cfg.IncludeGraph && len(files) > 0 {
			ranker := app.NewRankerWithWeights(root, app.WeightsFromScoring(cfg.Scoring))
			edges := app.ArchitectureEdges(ranker.DependencyGraph(files))
			archEdges = append(archEdges, prefixArchitectureEdges(edges, labels[i])...)
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

	topCfg.RootDir = absRoots[0]
	topCfg.Roots = nil
	top, err := app.NewWithCache(&topCfg, cache)
	if err != nil {
		return err
	}
	defer top.Close()
	if cfg.IncludeGraph {
		top.SetArchitectureGraph(mergeArchitectureEdges(archEdges))
	}
	var metrics app.RenderMetrics
	var rendered []byte
	var renderErr error
	if historyEnabled(&topCfg) {
		metrics, rendered, renderErr = top.RenderEntriesWithMetricsCaptured(allFiles, allSkipped, time.Since(start), nil)
	} else {
		metrics, renderErr = top.RenderEntriesWithMetrics(allFiles, allSkipped, time.Since(start), nil)
	}
	if renderErr != nil {
		return renderErr
	}
	recordHistory(&topCfg, "dump", rendered, metrics)
	return nil
}

// prefixArchitectureEdges adds a root label to both ends of each directory
// edge, keeping "." as the root's own namespace.
func prefixArchitectureEdges(edges []string, label string) []string {
	out := make([]string, 0, len(edges))
	for _, edge := range edges {
		from, to, ok := strings.Cut(edge, " -> ")
		if !ok {
			continue
		}
		if from == "." {
			from = label
		} else {
			from = label + "/" + from
		}
		if to == "." {
			to = label
		} else {
			to = label + "/" + to
		}
		out = append(out, from+" -> "+to)
	}
	return out
}

// mergeArchitectureEdges sorts, de-duplicates, and caps a combined graph so
// the multi-root header stays the same size as a single-root one.
func mergeArchitectureEdges(edges []string) []string {
	sort.Strings(edges)
	out := edges[:0]
	for _, edge := range edges {
		if len(out) == 0 || out[len(out)-1] != edge {
			out = append(out, edge)
		}
	}
	const maxEdges = 200
	if len(out) > maxEdges {
		out = out[:maxEdges]
	}
	return out
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
