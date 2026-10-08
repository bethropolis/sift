package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/lang"
	"github.com/bethropolis/sift/internal/rank"
	"github.com/bethropolis/sift/internal/selection"
)

// Follow directions for walking the import graph.
const (
	FollowDeps       = "deps"
	FollowDependents = "dependents"
	FollowBoth       = "both"
)

// ParseFollowDirection validates a --direction value.
func ParseFollowDirection(s string) (string, error) {
	switch s {
	case FollowDeps, FollowDependents, FollowBoth:
		return s, nil
	default:
		return "", fmt.Errorf("invalid direction %q: want deps, dependents, or both", s)
	}
}

// FollowHit is one file reached from the seed: its shortest distance in hops
// (the seed itself is 0) and the parent it was first discovered through.
type FollowHit struct {
	Path     string
	Distance int
	Via      string
}

// InvertGraph builds the importers adjacency: file -> files that import it.
// Directory targets are expected to be expanded to files first; anything left
// unexpanded inverts as-is.
func InvertGraph(graph map[string][]string) map[string][]string {
	inv := make(map[string][]string, len(graph))
	for src, targets := range graph {
		for _, t := range targets {
			t = filepath.ToSlash(t)
			inv[t] = append(inv[t], filepath.ToSlash(src))
		}
	}
	for _, v := range inv {
		sort.Strings(v)
	}
	return inv
}

// MergeGraphs unions two adjacencies over the same path space, sorting and
// de-duplicating each neighbor list so walks over the union are stable.
func MergeGraphs(a, b map[string][]string) map[string][]string {
	out := make(map[string][]string, len(a)+len(b))
	add := func(src, t string) {
		src = filepath.ToSlash(src)
		t = filepath.ToSlash(t)
		for _, e := range out[src] {
			if e == t {
				return
			}
		}
		out[src] = append(out[src], t)
	}
	for src, targets := range a {
		for _, t := range targets {
			add(src, t)
		}
	}
	for src, targets := range b {
		for _, t := range targets {
			add(src, t)
		}
	}
	for _, v := range out {
		sort.Strings(v)
	}
	return out
}

// ExpandGraphDirs rewrites directory targets to the sorted collected files
// beneath them (Go package imports name directories). Targets naming neither
// a file nor a directory with collected files are dropped.
func ExpandGraphDirs(graph map[string][]string, paths []string) map[string][]string {
	files := make(map[string]bool, len(paths))
	for _, p := range paths {
		files[filepath.ToSlash(p)] = true
	}
	out := make(map[string][]string, len(graph))
	for src, targets := range graph {
		s := filepath.ToSlash(src)
		for _, t := range targets {
			t = strings.TrimSuffix(filepath.ToSlash(t), "/")
			if t == "" {
				continue
			}
			if files[t] {
				out[s] = append(out[s], t)
				continue
			}
			prefix := t + "/"
			for p := range files {
				if strings.HasPrefix(p, prefix) {
					out[s] = append(out[s], p)
				}
			}
		}
		if len(out[s]) > 0 {
			sort.Strings(out[s])
		}
	}
	return out
}

// WalkGraph breadth-first searches the adjacency from seeds. maxDepth bounds
// the walk in hops (0 reports the seeds only); negative means unlimited.
// Output is stable: seeds in order, then by distance, then by path. Cycles
// terminate on the visited set, so every file reports its shortest distance
// and the parent that first discovered it.
func WalkGraph(graph map[string][]string, seeds []string, maxDepth int) []FollowHit {
	visited := make(map[string]bool, len(seeds))
	var hits []FollowHit
	frontier := make([]string, 0, len(seeds))
	for _, s := range seeds {
		s = filepath.ToSlash(s)
		if visited[s] {
			continue
		}
		visited[s] = true
		hits = append(hits, FollowHit{Path: s})
		frontier = append(frontier, s)
	}
	for depth := 1; len(frontier) > 0 && (maxDepth < 0 || depth <= maxDepth); depth++ {
		sort.Strings(frontier)
		var next []string
		for _, p := range frontier {
			targets := append([]string(nil), graph[p]...)
			sort.Strings(targets)
			for _, t := range targets {
				t = filepath.ToSlash(t)
				if visited[t] {
					continue
				}
				visited[t] = true
				hits = append(hits, FollowHit{Path: t, Distance: depth, Via: p})
				next = append(next, t)
			}
		}
		frontier = next
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Distance != hits[j].Distance {
			return hits[i].Distance < hits[j].Distance
		}
		return hits[i].Path < hits[j].Path
	})
	return hits
}

// FollowModes assigns the preferred render mode per walked path: the seed and
// everything within fullDepth hops renders in full, farther files as
// signatures. A negative fullDepth renders everything in full.
func FollowModes(hits []FollowHit, fullDepth int) map[string]string {
	modes := make(map[string]string, len(hits))
	for _, h := range hits {
		if fullDepth < 0 || h.Distance <= fullDepth {
			modes[h.Path] = "full"
		} else {
			modes[h.Path] = "signatures"
		}
	}
	return modes
}

// FollowSelection is the optimizer-ready form of a seed walk: one candidate
// per hit with distance-based mode preferences, plus the walk itself, the
// per-path modes, and the count of eligible files no resolver covers.
type FollowSelection struct {
	Candidates []selection.Candidate
	Hits       []FollowHit
	Modes      map[string]string
	Unanalyzed int
}

// FollowCandidates walks the import graph from the seed and builds the
// optimizer input, sharing one construction across the follow command, the
// select --follow debug flag, the MCP tool, and the serve endpoint. files
// are the collected entries, graph their forward adjacency, and preferred
// their rank results for signals. modeOverride ("" for distance-based
// modes) honors an explicit --mode for every hit including the seed.
func FollowCandidates(files []format.FileEntry, graph map[string][]string, preferred map[string]rank.FileScoreResult, seed, direction string, maxDepth, fullDepth int, modeOverride string) (FollowSelection, error) {
	paths := make([]string, 0, len(files))
	byEntry := make(map[string]int, len(files))
	scannerMissing := 0
	for i, f := range files {
		p := filepath.ToSlash(f.Path)
		paths = append(paths, p)
		byEntry[p] = i
		if !lang.HasImportScanner(p) {
			scannerMissing++
		}
	}
	expanded := ExpandGraphDirs(graph, paths)
	inverted := InvertGraph(expanded)
	var adj map[string][]string
	switch direction {
	case FollowDependents:
		adj = inverted
	case FollowBoth:
		adj = MergeGraphs(expanded, inverted)
	default:
		adj = expanded
	}
	hits := WalkGraph(adj, []string{seed}, maxDepth)
	modes := FollowModes(hits, fullDepth)
	if modeOverride != "" {
		for p := range modes {
			modes[p] = modeOverride
		}
	}
	testAffinity := RelatedTestAffinity(files, graph)
	candidates := make([]selection.Candidate, 0, len(hits))
	for _, h := range hits {
		f := files[byEntry[h.Path]]
		sc := preferred[f.Path]
		candidates = append(candidates, selection.Candidate{
			File:          f,
			PreferredMode: modes[h.Path],
			Signals: selection.Signals{
				Recency:      sc.Signals.Recency,
				Churn:        sc.Signals.Churn,
				Centrality:   sc.Signals.Centrality,
				Role:         sc.Signals.Role,
				TestAffinity: testAffinity[h.Path],
				Distance:     float64(h.Distance),
			},
		})
	}
	return FollowSelection{Candidates: candidates, Hits: hits, Modes: modes, Unanalyzed: scannerMissing}, nil
}

// SeedPinError reports that the seed did not survive budgeting at its
// assigned mode, so there is nothing worth rendering.
func SeedPinError(seed string, wantMode string, budget int) error {
	return fmt.Errorf("seed %q alone exceeds the token budget (%d) at %s mode", seed, budget, wantMode)
}

// ResolveSeed maps a seed argument (root-relative or absolute) to the
// repo-relative slash path of a file inside rootDir. Anything else — outside
// the root, missing, or not a regular file — errors naming the reason.
func ResolveSeed(rootDir, seed string) (string, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return "", fmt.Errorf("follow: cannot resolve root %q: %w", rootDir, err)
	}
	abs := seed
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(absRoot, seed)
	}
	abs = filepath.Clean(abs)
	rel, err := filepath.Rel(absRoot, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("follow: seed %q is outside the project root %q", seed, absRoot)
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("follow: seed %q does not exist", seed)
		}
		return "", fmt.Errorf("follow: cannot stat seed %q: %w", seed, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("follow: seed %q is not a regular file", seed)
	}
	return filepath.ToSlash(rel), nil
}
