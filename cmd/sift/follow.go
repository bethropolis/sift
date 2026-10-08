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
	"github.com/bethropolis/sift/internal/lang"
	"github.com/bethropolis/sift/internal/selection"
)

var (
	followDirection string
	followDepth     int
	followFullDepth int
)

func init() {
	followCmd.Flags().StringVar(&followDirection, "direction", "dependents", "Which way to walk the import graph: deps, dependents, or both")
	followCmd.Flags().IntVar(&followDepth, "depth", 2, "Max import hops from the seed file (-1 = unlimited, 0 = seed only)")
	followCmd.Flags().IntVar(&followFullDepth, "full-depth", 1, "Files within N hops render in full; farther files render as signatures (-1 = all full)")
	rootCmd.AddCommand(followCmd)
	config.RegisterFlags(cfg, followCmd.Flags())
}

var followCmd = &cobra.Command{
	Use:   "follow <file>",
	Short: "Render a file plus the files connected to it through imports",
	Long: `Render the seed file plus its import-graph neighbors, transitively.

Dependents answers "what could break if I change this file". Deps answers
"what do I need to understand this file". Both walks every visited node's
imports and importers, which is the union of the two walks, not a path
restricted to one direction.`,
	Args: cobra.ExactArgs(1),
	RunE: runFollow,
}

func runFollow(cmd *cobra.Command, args []string) error {
	direction, err := app.ParseFollowDirection(followDirection)
	if err != nil {
		return err
	}
	if err := applyProfile(cmd); err != nil {
		return err
	}
	// Bounded like select: a transitive walk without a budget is a footgun.
	if cfg.Budget == 0 && !cmd.Flags().Changed("budget") {
		cfg.Budget = 50000
	}
	cfg.SmartFilter = true

	application, err := app.New(cfg)
	if err != nil {
		return err
	}
	defer application.Close()

	absRoot, err := filepath.Abs(cfg.RootDir)
	if err != nil {
		return fmt.Errorf("sift follow: cannot resolve root: %w", err)
	}
	seed, err := app.ResolveSeed(absRoot, args[0])
	if err != nil {
		return err
	}

	start := time.Now()
	files, skipped, err := application.CollectPicker()
	if err != nil {
		return err
	}
	byPath := make(map[string]bool, len(files))
	for _, f := range files {
		byPath[filepath.ToSlash(f.Path)] = true
	}
	if !byPath[seed] {
		for _, s := range skipped {
			if filepath.ToSlash(s.Path) == seed {
				return fmt.Errorf("sift follow: seed %q skipped: %s", args[0], strings.ToLower(string(s.Reason)))
			}
		}
		return fmt.Errorf("sift follow: seed %q is ignored or outside the collected tree", args[0])
	}
	if !lang.HasImportScanner(seed) {
		return fmt.Errorf("sift follow: no import resolver for %q; supported: %s",
			filepath.Ext(seed), strings.Join(lang.ImportExtensions(), ", "))
	}

	scores := app.WeightsFromScoring(cfg.Scoring)
	ranker := app.NewRankerWithWeights(absRoot, scores)
	preferred, graph := ranker.RankGraph(files)

	modeOverride := ""
	if cmd.Flags().Changed("mode") {
		modeOverride = cfg.Mode
	}
	sel, err := app.FollowCandidates(files, graph, preferred, seed, direction, followDepth, followFullDepth, modeOverride)
	if err != nil {
		return err
	}
	hits := sel.Hits
	result := selection.Select(sel.Candidates, selection.Request{
		Budget: cfg.Budget,
		Prompt: cfg.Prompt,
		Tuning: app.TuningFromScoring(cfg.Scoring),
	})

	wantMode := modeOverride
	if wantMode == "" {
		wantMode = sel.Modes[seed]
	}
	seedOK := false
	for _, d := range result.Decisions {
		if filepath.ToSlash(d.Path) == seed && d.Selected && string(d.Mode) == wantMode {
			seedOK = true
		}
	}
	if !seedOK {
		return fmt.Errorf("sift follow: seed %q alone exceeds the token budget (%d)", args[0], cfg.Budget)
	}
	if len(hits) == 1 {
		fmt.Fprintf(os.Stderr, "sift follow: no %s found for %s within depth %d; rendering the seed alone\n", direction, seed, followDepth)
	}
	if err := application.RenderFinal(result.Selected, skipped, time.Since(start), nil); err != nil {
		return err
	}
	printFollowSummary(seed, direction, hits, sel.Unanalyzed)
	return nil
}

// printFollowSummary reports the walk shape to stderr even when the document
// goes to a file: which seed and direction, per-hop file counts, and how many
// eligible files the graph is blind to.
func printFollowSummary(seed, direction string, hits []app.FollowHit, unanalyzed int) {
	byHop := map[int]int{}
	maxHop := 0
	for _, h := range hits {
		if h.Distance == 0 {
			continue
		}
		byHop[h.Distance]++
		if h.Distance > maxHop {
			maxHop = h.Distance
		}
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "follow: %s, %s, depth %d: %d file%s", seed, direction, followDepth, len(hits), plural(len(hits)))
	if maxHop > 0 {
		hops := make([]string, 0, maxHop)
		for d := 1; d <= maxHop; d++ {
			hops = append(hops, fmt.Sprintf("hop %d: %d", d, byHop[d]))
		}
		fmt.Fprintf(&sb, " (%s)", strings.Join(hops, ", "))
	}
	fmt.Fprintf(&sb, ", %d not analyzed (no resolver)\n", unanalyzed)
	fmt.Fprint(os.Stderr, sb.String())
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
