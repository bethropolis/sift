package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/lang"
	"github.com/bethropolis/sift/internal/rank"
)

// Follow depths match the CLI defaults: two hops out, near hops in full.
const (
	followDepth     = 2
	followFullDepth = 1
)

// follow runs an import-graph walk from the cursor file and replaces the
// selection with exactly what the walk picks, like the web explorer does.
// direction is app.FollowDependents ("f") or app.FollowDeps ("F").
func (m *model) follow(direction string) tea.Cmd {
	n := m.node()
	if n == nil || n.Kind != KindFile {
		return m.setNotice("Follow needs a file under the cursor")
	}
	return m.setNotice(m.root.FollowFrom(m.projectPath, filepath.ToSlash(n.Path), direction))
}

// FollowFrom walks the import graph from the seed file and applies the hit
// modes to the tree (seed full, near hops full, far hops signatures,
// everything else skipped), returning a one-line summary for the notice bar.
// Files the collector skipped (hidden, git-ignored) keep their state.
func (n *TreeNode) FollowFrom(rootDir, seed, direction string) string {
	if _, err := app.ParseFollowDirection(direction); err != nil {
		return "Follow: " + err.Error()
	}
	var files []*TreeNode
	n.collectFiles(&files)
	entries := make([]format.FileEntry, 0, len(files))
	byPath := make(map[string]*TreeNode, len(files))
	for _, f := range files {
		p := filepath.ToSlash(f.Path)
		entries = append(entries, format.FileEntry{Path: p, Content: f.Content})
		byPath[p] = f
	}
	if _, ok := byPath[seed]; !ok {
		return fmt.Sprintf("Follow: %s is not in the collected tree", seed)
	}
	if !lang.HasImportScanner(seed) {
		return fmt.Sprintf("Follow: no import resolver for %q", filepath.Ext(seed))
	}
	ranker := app.NewRankerWithWeights(rootDir, rank.DefaultWeights())
	graph := ranker.DependencyGraph(entries)
	sel, err := app.FollowCandidates(entries, graph, nil, seed, direction, followDepth, followFullDepth, "")
	if err != nil {
		return "Follow: " + err.Error()
	}
	reasons := make(map[string]string, len(sel.Hits))
	for _, h := range sel.Hits {
		switch {
		case h.Distance == 0:
			reasons[h.Path] = "follow seed"
		case h.Via != "":
			reasons[h.Path] = fmt.Sprintf("follow hop %d via %s", h.Distance, h.Via)
		default:
			reasons[h.Path] = fmt.Sprintf("follow hop %d", h.Distance)
		}
	}
	for _, f := range files {
		p := filepath.ToSlash(f.Path)
		mode, ok := sel.Modes[p]
		if !ok {
			f.Mode, f.PreferredMode = ModeSkip, ModeSkip
			f.setSelectedSubtree(false)
			continue
		}
		if mode == "signatures" {
			f.Mode, f.PreferredMode = ModeSignatures, ModeSignatures
		} else {
			f.Mode, f.PreferredMode = ModeFull, ModeFull
		}
		f.ModeReason = reasons[p]
		f.setSelectedSubtree(true)
	}
	n.recompute()
	return followSummary(seed, direction, sel.Hits, sel.Unanalyzed)
}

// followSummary reports the walk shape: seed, direction, per-hop counts,
// and how many eligible files no resolver covers.
func followSummary(seed, direction string, hits []app.FollowHit, unanalyzed int) string {
	byHop := map[int]int{}
	for _, h := range hits {
		if h.Distance > 0 {
			byHop[h.Distance]++
		}
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Follow · %s · %s · %d %s", seed, direction, len(hits), pluralFiles(len(hits)))
	for d := 1; d <= len(byHop); d++ {
		if byHop[d] > 0 {
			fmt.Fprintf(&sb, " (hop %d: %d)", d, byHop[d])
		}
	}
	if unanalyzed > 0 {
		fmt.Fprintf(&sb, " · %d without resolver", unanalyzed)
	}
	return sb.String()
}
