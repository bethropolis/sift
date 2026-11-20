package app

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/format"
)

// DiffNeighborFiles returns the import-graph neighbors of changed files, in
// both directions, up to depth hops. Neighbors are handed back in signature
// mode so the diff remains the focus of the document. Changed files themselves
// are excluded: their content is already in the patch.
func DiffNeighborFiles(cfg *config.Config, files []format.FileEntry, changed []string, depth int) []format.FileEntry {
	if depth <= 0 || len(files) == 0 || len(changed) == 0 {
		return nil
	}
	root := cfg.RootDir
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	ranker := NewRankerWithWeights(root, WeightsFromScoring(cfg.Scoring))
	graph := ranker.DependencyGraph(files)

	byPath := make(map[string]*format.FileEntry, len(files))
	byDir := make(map[string][]string)
	for i := range files {
		p := filepath.ToSlash(files[i].Path)
		byPath[p] = &files[i]
		dir := filepath.ToSlash(filepath.Dir(p))
		byDir[dir] = append(byDir[dir], p)
	}

	// Forward and reverse adjacency, with directory-shaped targets (Go
	// package imports) expanded to every collected file beneath them.
	forward := make(map[string][]string)
	reverse := make(map[string][]string)
	addEdge := func(from, to string) {
		forward[from] = append(forward[from], to)
		reverse[to] = append(reverse[to], from)
	}
	for src, targets := range graph {
		src = filepath.ToSlash(src)
		if _, ok := byPath[src]; !ok {
			continue
		}
		for _, target := range targets {
			target = strings.TrimSuffix(filepath.ToSlash(strings.TrimSpace(target)), "/")
			if target == "" {
				continue
			}
			if _, ok := byPath[target]; ok {
				addEdge(src, target)
				continue
			}
			for _, p := range byDir[target] {
				addEdge(src, p)
			}
		}
	}

	isChanged := make(map[string]bool, len(changed))
	frontier := make([]string, 0, len(changed))
	for _, p := range changed {
		p = filepath.ToSlash(p)
		if _, ok := byPath[p]; ok && !isChanged[p] {
			isChanged[p] = true
			frontier = append(frontier, p)
		}
	}

	seen := make(map[string]bool)
	var neighbors []string
	for hop := 0; hop < depth && len(frontier) > 0; hop++ {
		var next []string
		for _, p := range frontier {
			for _, target := range forward[p] {
				if isChanged[target] || seen[target] {
					continue
				}
				seen[target] = true
				neighbors = append(neighbors, target)
				next = append(next, target)
			}
			for _, source := range reverse[p] {
				if isChanged[source] || seen[source] {
					continue
				}
				seen[source] = true
				neighbors = append(neighbors, source)
				next = append(next, source)
			}
		}
		frontier = next
	}

	sort.SliceStable(neighbors, func(i, j int) bool {
		a, b := byPath[neighbors[i]], byPath[neighbors[j]]
		if a.RankScore != b.RankScore {
			return a.RankScore > b.RankScore
		}
		return neighbors[i] < neighbors[j]
	})

	out := make([]format.FileEntry, 0, len(neighbors))
	for _, p := range neighbors {
		entry := *byPath[p]
		if len(entry.SigContent) > 0 {
			entry.Content = entry.SigContent
			entry.Tokens = entry.TokensSig
			entry.IsCompressed = true
			entry.SigContent = nil
		}
		out = append(out, entry)
	}
	return out
}
