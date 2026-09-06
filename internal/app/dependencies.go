package app

import (
	"path/filepath"
	"strings"

	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/rank"
	"github.com/bethropolis/sift/internal/selection"
)

// ExpandDependencies walks the dependency graph outward from the selected
// set with a breadth-first search. maxDepth bounds the walk (mirroring
// codegrab's --max-depth); negative means unlimited, zero disables
// expansion. A visited set breaks import cycles. Graph targets naming
// directories (Go package imports) expand to every collected file beneath
// them. Already-selected files are never returned.
func ExpandDependencies(selected, files []format.FileEntry, graph map[string][]string, maxDepth int) []format.FileEntry {
	if maxDepth == 0 {
		return nil
	}
	byPath := make(map[string]format.FileEntry, len(files))
	for _, f := range files {
		byPath[filepath.ToSlash(f.Path)] = f
	}
	visited := make(map[string]bool, len(selected))
	frontier := make([]string, 0, len(selected))
	for _, s := range selected {
		p := filepath.ToSlash(s.Path)
		visited[p] = true
		frontier = append(frontier, p)
	}
	var out []format.FileEntry
	for depth := 0; len(frontier) > 0 && (maxDepth < 0 || depth < maxDepth); depth++ {
		var next []string
		for _, p := range frontier {
			for _, t := range graph[p] {
				for _, fp := range expandTarget(t, byPath) {
					if visited[fp] {
						continue
					}
					visited[fp] = true
					out = append(out, byPath[fp])
					next = append(next, fp)
				}
			}
		}
		frontier = next
	}
	return out
}

// expandTarget resolves one graph target to collected file paths: a file
// maps to itself, a directory to every collected file beneath it.
func expandTarget(target string, byPath map[string]format.FileEntry) []string {
	if _, ok := byPath[target]; ok {
		return []string{target}
	}
	var out []string
	prefix := strings.TrimSuffix(target, "/") + "/"
	for p := range byPath {
		if strings.HasPrefix(p, prefix) {
			out = append(out, p)
		}
	}
	return out
}

// ExpandChosen appends dependency files of the chosen set as
// signature-compressed entries for render paths with a hand-picked
// selection (picker generate/quit-and-render). When budget > 0, dependency
// spend is bounded by what remains after the chosen set; non-positive
// budgets add the full closure. rootDir anchors module-root resolution.
func ExpandChosen(files, chosen []format.FileEntry, rootDir string, budget, maxDepth int) []format.FileEntry {
	if maxDepth == 0 || len(chosen) == 0 {
		return chosen
	}
	ranker := NewRankerWithWeights(rootDir, rank.DefaultWeights())
	_, graph := ranker.RankGraph(append([]format.FileEntry(nil), files...))
	deps := ExpandDependencies(chosen, files, graph, maxDepth)
	if len(deps) == 0 {
		return chosen
	}
	spent := 0
	for _, f := range chosen {
		spent += f.Tokens
	}
	out := chosen
	for _, dep := range deps {
		dep.IsCompressed = true
		if dep.SigContent != nil {
			dep.Content = dep.SigContent
			dep.Tokens = dep.TokensSig
		} else {
			dep.Tokens = dep.TokensFull
		}
		if budget > 0 && spent+dep.Tokens > budget {
			break
		}
		spent += dep.Tokens
		out = append(out, dep)
	}
	return out
}

// DependencyRequest carries one selection pass plus its dependency follow-up.
type DependencyRequest struct {
	Candidates []selection.Candidate
	Files      []format.FileEntry
	Graph      map[string][]string
	Preferred  map[string]rank.FileScoreResult
	Budget     int
	Prompt     string
	Tuning     selection.Tuning
	MaxDepth   int
}

// SelectWithDependencies runs the normal selection pass, expands the
// dependency closure of the selected set, and re-runs the optimizer over
// the union: first-pass selections keep their chosen modes as preferences,
// dependencies enter biased toward signatures (they're context, not the
// thing under review). One budget, one optimization — dependencies compete
// fairly with upgrades instead of starving on post-upgrade crumbs.
func SelectWithDependencies(req DependencyRequest) selection.Result {
	first := selection.Select(req.Candidates, selection.Request{
		Budget: req.Budget,
		Prompt: req.Prompt,
		Tuning: req.Tuning,
	})
	deps := ExpandDependencies(first.Selected, req.Files, req.Graph, req.MaxDepth)
	if len(deps) == 0 {
		return first
	}
	byPath := make(map[string]selection.Candidate, len(req.Candidates))
	for _, c := range req.Candidates {
		byPath[filepath.ToSlash(c.File.Path)] = c
	}
	// First-pass selections re-enter with their chosen modes; decisions for
	// unselected files are skipped (they stay available via dep expansion
	// when relevant).
	second := make([]selection.Candidate, 0, len(first.Selected)+len(deps))
	for _, d := range first.Decisions {
		if !d.Selected {
			continue
		}
		c, ok := byPath[filepath.ToSlash(d.Path)]
		if !ok {
			continue
		}
		c.PreferredMode = string(d.Mode)
		second = append(second, c)
	}
	for _, file := range deps {
		sc := req.Preferred[filepath.ToSlash(file.Path)]
		second = append(second, selection.Candidate{
			File:          file,
			PreferredMode: string(selection.ModeSignatures),
			Signals: selection.Signals{
				Recency:    sc.Signals.Recency,
				Churn:      sc.Signals.Churn,
				Centrality: sc.Signals.Centrality,
				Role:       sc.Signals.Role,
			},
		})
	}
	result := selection.Select(second, selection.Request{
		Budget: req.Budget,
		Prompt: req.Prompt,
		Tuning: req.Tuning,
	})
	// Re-attach first-pass verdicts for files the second pass never
	// considered, so selection reports still render the whole tree. The
	// Selected set and token accounting stay pass-2.
	seen := make(map[string]bool, len(result.Decisions))
	for _, d := range result.Decisions {
		seen[filepath.ToSlash(d.Path)] = true
	}
	for _, d := range first.Decisions {
		if !seen[filepath.ToSlash(d.Path)] {
			result.Decisions = append(result.Decisions, d)
		}
	}
	return result
}
