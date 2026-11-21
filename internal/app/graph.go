package app

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/bethropolis/sift/internal/format"
)

// maxArchitectureEdges bounds the header so the graph's token cost stays
// predictable even on very large repositories.
const maxArchitectureEdges = 200

// ArchitectureEdges aggregates a file-level import graph into a sorted,
// deduplicated directory-level adjacency list written as "from -> to".
// Self-edges and targets that are not part of the collected tree's
// directories are omitted.
func ArchitectureEdges(graph map[string][]string) []string {
	if len(graph) == 0 {
		return nil
	}
	dirs := make(map[string]bool)
	for src := range graph {
		dirs[filepath.ToSlash(filepath.Dir(src))] = true
	}
	// Import targets can be package directories that never import anything
	// themselves, so they have no source key. Register extension-less
	// targets as directories too.
	for _, targets := range graph {
		for _, target := range targets {
			target = strings.TrimSuffix(filepath.ToSlash(strings.TrimSpace(target)), "/")
			if target != "" && filepath.Ext(target) == "" {
				dirs[target] = true
			}
		}
	}

	edges := make(map[string]map[string]bool)
	add := func(from, to string) {
		if from == "" || to == "" || from == to {
			return
		}
		set := edges[from]
		if set == nil {
			set = make(map[string]bool)
			edges[from] = set
		}
		set[to] = true
	}

	for src, targets := range graph {
		from := filepath.ToSlash(filepath.Dir(src))
		for _, target := range targets {
			target = strings.TrimSuffix(filepath.ToSlash(strings.TrimSpace(target)), "/")
			if target == "" {
				continue
			}
			to := target
			if !dirs[to] {
				to = filepath.ToSlash(filepath.Dir(target))
			}
			if !dirs[to] {
				continue
			}
			add(from, to)
		}
	}

	lines := make([]string, 0, len(edges))
	for from, tos := range edges {
		tosList := make([]string, 0, len(tos))
		for to := range tos {
			tosList = append(tosList, to)
		}
		sort.Strings(tosList)
		for _, to := range tosList {
			lines = append(lines, from+" -> "+to)
		}
	}
	sort.Strings(lines)
	if len(lines) > maxArchitectureEdges {
		lines = lines[:maxArchitectureEdges]
	}
	return lines
}

// SetArchitectureGraph installs a caller-computed directory graph and prevents
// prepareArchitecture from replacing it. Multi-root dumps use this after
// prefixing each root's edges with its namespace label.
func (a *App) SetArchitectureGraph(lines []string) {
	a.archGraph = append([]string(nil), lines...)
	a.archGraphManual = true
}

// prepareArchitecture builds the optional directory graph for the files that
// are about to be rendered. It only parses imports when the feature is on.
func (a *App) prepareArchitecture(files []format.FileEntry) {
	if a.archGraphManual {
		return
	}
	a.archGraph = nil
	if !a.cfg.IncludeGraph || len(files) == 0 {
		return
	}
	root := a.cfg.RootDir
	if abs, err := a.absRoot(); err == nil {
		root = abs
	}
	ranker := NewRankerWithWeights(root, WeightsFromScoring(a.cfg.Scoring))
	a.archGraph = ArchitectureEdges(ranker.DependencyGraph(files))
}
