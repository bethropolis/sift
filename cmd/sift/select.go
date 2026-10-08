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

var (
	printSelection  bool
	selectionFormat string
	selectionOnly   bool
	includeSkipped  bool
	followSeed      string
	selFollowDirection string
	selFollowDepth     int
	selFollowFullDepth int
)

func init() {
	selectCmd.Flags().BoolVar(&printSelection, "print-selection", false, "Print the automatic full/signatures/skip decisions")
	selectCmd.Flags().StringVar(&selectionFormat, "selection-format", "tree", "Selection report format: tree, json, ndjson")
	selectCmd.Flags().BoolVar(&selectionOnly, "selection-only", false, "Print the selection report without rendering a document")
	selectCmd.Flags().BoolVar(&includeSkipped, "include-skipped", false, "Include filtered files and skip reasons in the selection report")
	selectCmd.Flags().StringVar(&followSeed, "follow", "", "Restrict candidates to the import-graph walk from this seed file (debug what follow picks)")
	selectCmd.Flags().StringVar(&selFollowDirection, "follow-direction", "dependents", "Follow walk direction: deps, dependents, or both")
	selectCmd.Flags().IntVar(&selFollowDepth, "follow-depth", 2, "Follow walk depth in hops (-1 = unlimited, 0 = seed only)")
	selectCmd.Flags().IntVar(&selFollowFullDepth, "follow-full-depth", 1, "Follow walk full-render depth (-1 = all full)")
	rootCmd.AddCommand(selectCmd)
	config.RegisterFlags(cfg, selectCmd.Flags())
}

var selectCmd = &cobra.Command{
	Use:   "select [path]",
	Short: "Automatically select the most useful files and render them",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSelect,
}

// runSelectFollow restricts the candidate set to one follow walk so the
// selection report shows exactly what follow picks: distances ride along in
// each decision's signals (visible in json/ndjson reports) and the walk
// summary prints to stderr ahead of the report.
func runSelectFollow(cmd *cobra.Command, files []format.FileEntry, skipped []walker.SkippedItem, graph map[string][]string, preferred map[string]rank.FileScoreResult) (selection.Result, error) {
	direction, err := app.ParseFollowDirection(selFollowDirection)
	if err != nil {
		return selection.Result{}, err
	}
	absRoot, err := filepath.Abs(cfg.RootDir)
	if err != nil {
		return selection.Result{}, fmt.Errorf("sift select: cannot resolve root: %w", err)
	}
	seed, err := app.ResolveSeed(absRoot, followSeed)
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
				return selection.Result{}, fmt.Errorf("sift select: follow seed %q skipped: %s", followSeed, strings.ToLower(string(s.Reason)))
			}
		}
		return selection.Result{}, fmt.Errorf("sift select: follow seed %q is ignored or outside the collected tree", followSeed)
	}
	if !lang.HasImportScanner(seed) {
		return selection.Result{}, fmt.Errorf("sift select: no import resolver for %q; supported: %s",
			filepath.Ext(seed), strings.Join(lang.ImportExtensions(), ", "))
	}
	modeOverride := ""
	if cmd.Flags().Changed("mode") {
		modeOverride = cfg.Mode
	}
	sel, err := app.FollowCandidates(files, graph, preferred, seed, direction, selFollowDepth, selFollowFullDepth, modeOverride)
	if err != nil {
		return selection.Result{}, err
	}
	result := selection.Select(sel.Candidates, selection.Request{
		Budget: cfg.Budget,
		Prompt: cfg.Prompt,
		Tuning: app.TuningFromScoring(cfg.Scoring),
	})
	wantMode := modeOverride
	if wantMode == "" {
		wantMode = sel.Modes[seed]
	}
	for _, d := range result.Decisions {
		if filepath.ToSlash(d.Path) == seed && d.Selected && string(d.Mode) == wantMode {
			printFollowSummary(seed, direction, sel.Hits, sel.Unanalyzed)
			return result, nil
		}
	}
	return selection.Result{}, fmt.Errorf("sift select: follow seed %q alone exceeds the token budget (%d)", followSeed, cfg.Budget)
}

func runSelect(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		cfg.RootDir = args[0]
	}
	if err := applyProfile(cmd); err != nil {
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
	if selectionOnly {
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
	if followSeed != "" {
		var err error
		result, err = runSelectFollow(cmd, files, skipped, graph, preferred)
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
			Prompt:     cfg.Prompt,
			Tuning:     app.TuningFromScoring(cfg.Scoring),
			MaxDepth:   cfg.MaxDepth,
		})
	}
	if printSelection || selectionOnly {
		writer := os.Stderr
		if selectionOnly {
			writer = os.Stdout
		}
		if err := (selection.Report{Result: result, Skipped: skipped}).Print(writer, selectionFormat, includeSkipped); err != nil {
			return fmt.Errorf("print selection: %w", err)
		}
	}
	if selectionOnly {
		return nil
	}
	if len(result.Selected) == 0 {
		return fmt.Errorf("sift select: no files selected after applying filters and token budget")
	}
	if err := application.RenderFinal(result.Selected, skipped, time.Since(start), nil); err != nil {
		return err
	}
	if err := recordDumpState(cfg.RootDir, result.Selected, "HEAD"); err != nil {
		application.LogError("Failed to record dump state: %v", err)
	}
	return nil
}
