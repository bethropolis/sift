package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/rank"
	"github.com/bethropolis/sift/internal/selection"
)

var (
	printSelection  bool
	selectionFormat string
	selectionOnly   bool
	includeSkipped  bool
)

func init() {
	selectCmd.Flags().BoolVar(&printSelection, "print-selection", false, "Print the automatic full/signatures/skip decisions")
	selectCmd.Flags().StringVar(&selectionFormat, "selection-format", "tree", "Selection report format: tree, json, ndjson")
	selectCmd.Flags().BoolVar(&selectionOnly, "selection-only", false, "Print the selection report without rendering a document")
	selectCmd.Flags().BoolVar(&includeSkipped, "include-skipped", false, "Include filtered files and skip reasons in the selection report")
	rootCmd.AddCommand(selectCmd)
	config.RegisterFlags(cfg, selectCmd.Flags())
}

var selectCmd = &cobra.Command{
	Use:   "select [path]",
	Short: "Automatically select the most useful files and render them",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSelect,
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
	result := app.SelectWithDependencies(app.DependencyRequest{
		Candidates: candidates,
		Files:      files,
		Graph:      graph,
		Preferred:  preferred,
		Budget:     cfg.Budget,
		Prompt:     cfg.Prompt,
		Tuning:     app.TuningFromScoring(cfg.Scoring),
		MaxDepth:   cfg.MaxDepth,
	})
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
