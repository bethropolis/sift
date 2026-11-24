package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/lang"
	"github.com/bethropolis/sift/internal/rank"
	"github.com/bethropolis/sift/internal/selection"
	"github.com/bethropolis/sift/internal/walker"
)

// selectOptions holds the flags specific to `sift select`.
type selectOptions struct {
	printSelection  bool
	selectionFormat string
	selectionOnly   bool
	includeSkipped  bool
	followSeed      string
	followDirection string
	followDepth     int
	followFullDepth int
}

// newSelectCmd automatically selects the most useful files and renders them.
func newSelectCmd(cfg *config.Config) *cobra.Command {
	opts := &selectOptions{}
	cmd := &cobra.Command{
		Use:   "select [path]",
		Short: "Automatically select the most useful files and render them",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSelect(cfg, opts, cmd, args)
		},
	}
	cmd.Flags().BoolVar(&opts.printSelection, "print-selection", false, "Print the automatic full/signatures/skip decisions")
	cmd.Flags().StringVar(&opts.selectionFormat, "selection-format", "tree", "Selection report format: tree, json, ndjson")
	cmd.Flags().BoolVar(&opts.selectionOnly, "selection-only", false, "Print the selection report without rendering a document")
	cmd.Flags().BoolVar(&opts.includeSkipped, "include-skipped", false, "Include filtered files and skip reasons in the selection report")
	cmd.Flags().StringVar(&opts.followSeed, "follow", "", "Restrict candidates to the import-graph walk from this seed file (debug what follow picks)")
	cmd.Flags().StringVar(&opts.followDirection, "follow-direction", "dependents", "Follow walk direction: deps, dependents, or both")
	cmd.Flags().IntVar(&opts.followDepth, "follow-depth", 2, "Follow walk depth in hops (-1 = unlimited, 0 = seed only)")
	cmd.Flags().IntVar(&opts.followFullDepth, "follow-full-depth", 1, "Follow walk full-render depth (-1 = all full)")
	config.RegisterFlags(cfg, cmd.Flags())
	config.RegisterHistoryFlags(cfg, cmd.Flags())
	return cmd
}

// runSelectFollow restricts the candidate set to one follow walk so the
// selection report shows exactly what follow picks: distances ride along in
// each decision's signals (visible in json/ndjson reports) and the walk
// summary prints to stderr ahead of the report.
func runSelectFollow(cfg *config.Config, opts *selectOptions, cmd *cobra.Command, files []format.FileEntry, skipped []walker.SkippedItem, graph map[string][]string, preferred map[string]rank.FileScoreResult, overhead int) (selection.Result, error) {
	direction, err := app.ParseFollowDirection(opts.followDirection)
	if err != nil {
		return selection.Result{}, err
	}
	absRoot, err := filepath.Abs(cfg.RootDir)
	if err != nil {
		return selection.Result{}, fmt.Errorf("sift select: cannot resolve root: %w", err)
	}
	seed, err := app.ResolveSeed(absRoot, opts.followSeed)
	if err != nil {
		return selection.Result{}, err
	}
	present := false
	for _, f := range files {
		if filepath.ToSlash(f.Path) == seed {
			present = true
		}
	}
	if !present {
		for _, s := range skipped {
			if filepath.ToSlash(s.Path) == seed {
				return selection.Result{}, fmt.Errorf("sift select: follow seed %q skipped: %s", opts.followSeed, strings.ToLower(string(s.Reason)))
			}
		}
		return selection.Result{}, fmt.Errorf("sift select: follow seed %q is ignored or outside the collected tree", opts.followSeed)
	}
	if !lang.HasImportScanner(seed) {
		return selection.Result{}, fmt.Errorf("sift select: no import resolver for %q; supported: %s",
			filepath.Ext(seed), strings.Join(lang.ImportExtensions(), ", "))
	}
	modeOverride := ""
	if cmd.Flags().Changed("mode") {
		modeOverride = cfg.Mode
	}
	sel, err := app.FollowCandidates(files, graph, preferred, seed, direction, opts.followDepth, opts.followFullDepth, modeOverride)
	if err != nil {
		return selection.Result{}, err
	}
	result := selection.Select(sel.Candidates, selection.Request{
		Budget:   cfg.Budget,
		Overhead: overhead,
		Prompt:   cfg.Prompt,
		Tuning:   app.TuningFromScoring(cfg.Scoring),
	})
	wantMode := modeOverride
	if wantMode == "" {
		wantMode = sel.Modes[seed]
	}
	for _, d := range result.Decisions {
		if filepath.ToSlash(d.Path) == seed && d.Selected && string(d.Mode) == wantMode {
			printFollowSummary(seed, direction, opts.followDepth, sel.Hits, sel.Unanalyzed)
			return result, nil
		}
	}
	return selection.Result{}, fmt.Errorf("sift select: follow seed %q alone exceeds the token budget (%d)", opts.followSeed, cfg.Budget)
}

func runSelect(cfg *config.Config, opts *selectOptions, cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		cfg.RootDir = args[0]
	}
	if err := applyProfile(cfg, cmd); err != nil {
		return err
	}
	// `select` is the agent-context workflow, so give it a useful bounded
	// default. Preserve the documented unlimited behavior when the user passes
	// --budget 0 explicitly; profile and config budgets continue to take effect.
	if cfg.Budget == 0 && !cmd.Flags().Changed("budget") {
		cfg.Budget = 50000
	}

	// Match the TUI: automatic selection uses enriched entries, signatures,
	// and the smart filter regardless of whether the user set --smart.
	cfg.SmartFilter = true
	appConfig := cfg
	if opts.selectionOnly {
		// A report-only command must not create or truncate the default
		// codebase.md output file.
		reportConfig := *cfg
		reportConfig.OutputFile = "-"
		appConfig = &reportConfig
	}
	application, err := app.New(appConfig)
	if err != nil {
		return err
	}
	defer application.Close()

	start := time.Now()
	files, skipped, err := application.CollectPicker()
	if err != nil {
		return err
	}
	scores := app.WeightsFromScoring(cfg.Scoring)
	preferred := map[string]rank.FileScoreResult{}
	graph := map[string][]string{}
	if root, rootErr := filepath.Abs(cfg.RootDir); rootErr == nil {
		ranker := app.NewRankerWithWeights(root, scores)
		preferred, graph = ranker.RankGraph(files)
	}
	overhead := app.EstimatedDocumentOverhead(cfg, cfg.Prompt, files)
	testAffinity := app.RelatedTestAffinity(files, graph)
	candidates := make([]selection.Candidate, 0, len(files))
	for _, file := range files {
		candidates = append(candidates, selection.Candidate{
			File:          file,
			PreferredMode: preferred[file.Path].PreferredMode,
			Signals: selection.Signals{
				Recency:      preferred[file.Path].Signals.Recency,
				Churn:        preferred[file.Path].Signals.Churn,
				Centrality:   preferred[file.Path].Signals.Centrality,
				Role:         preferred[file.Path].Signals.Role,
				TestAffinity: testAffinity[filepath.ToSlash(file.Path)],
			},
		})
	}
	var result selection.Result
	if opts.followSeed != "" {
		result, err = runSelectFollow(cfg, opts, cmd, files, skipped, graph, preferred, overhead)
		if err != nil {
			return err
		}
	} else {
		result = app.SelectWithDependencies(app.DependencyRequest{
			Candidates: candidates,
			Files:      files,
			Graph:      graph,
			Preferred:  preferred,
			Budget:     cfg.Budget,
			Overhead:   overhead,
			Prompt:     cfg.Prompt,
			Tuning:     app.TuningFromScoring(cfg.Scoring),
			MaxDepth:   cfg.MaxDepth,
		})
	}
	if opts.printSelection || opts.selectionOnly {
		writer := os.Stderr
		if opts.selectionOnly {
			writer = os.Stdout
		}
		if err := (selection.Report{Result: result, Skipped: skipped}).Print(writer, opts.selectionFormat, opts.includeSkipped); err != nil {
			return fmt.Errorf("print selection: %w", err)
		}
	}
	if opts.selectionOnly {
		return nil
	}
	if len(result.Selected) == 0 {
		return fmt.Errorf("sift select: no files selected after applying filters and token budget")
	}
	var metrics app.RenderMetrics
	var rendered []byte
	if historyEnabled(cfg) {
		metrics, rendered, err = application.RenderFinalCaptured(result.Selected, skipped, time.Since(start), nil)
	} else {
		err = application.RenderFinal(result.Selected, skipped, time.Since(start), nil)
	}
	if err != nil {
		return err
	}
	recordHistory(application, "select", rendered, metrics)
	if err := recordDumpState(cfg.RootDir, result.Selected, "HEAD"); err != nil {
		application.LogError("Failed to record dump state: %v", err)
	}
	return nil
}
