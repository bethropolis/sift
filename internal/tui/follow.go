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
// direction is app.FollowDependents ("f") or app.FollowDeps ("F"). The walk
// runs in a background tea.Cmd; only the result is applied on Update.
func (m *model) follow(direction string) tea.Cmd {
	n := m.node()
	if n == nil || n.Kind != KindFile {
		return m.setNotice("Follow needs a file under the cursor")
	}
	seed := filepath.ToSlash(n.Path)
	if _, err := app.ParseFollowDirection(direction); err != nil {
		return m.setNotice("Follow: " + err.Error())
	}
	if m.genBusy {
		return m.busyNotice()
	}
	input := m.root.followInput()
	if !input.present[seed] {
		return m.setNotice(fmt.Sprintf("Follow: %s is not in the collected tree", seed))
	}
	if !lang.HasImportScanner(seed) {
		return m.setNotice(fmt.Sprintf("Follow: no import resolver for %q", filepath.Ext(seed)))
	}
	rootDir := m.projectPath
	label := fmt.Sprintf("Following %s…", seed)
	return m.startAsyncJob(label, func() tea.Msg {
		ranker := app.NewRankerWithWeights(rootDir, rank.DefaultWeights())
		graph := ranker.DependencyGraph(input.entries)
		sel, err := app.FollowCandidates(input.entries, graph, nil, seed, direction, followDepth, followFullDepth, "")
		if err != nil {
			return followJobMsg{err: err.Error()}
		}
		return followJobMsg{
			seed:       seed,
			direction:  direction,
			hits:       sel.Hits,
			modes:      sel.Modes,
			unanalyzed: sel.Unanalyzed,
		}
	})
}

// followJobMsg carries a completed background follow walk back to Update.
type followJobMsg struct {
	seed       string
	direction  string
	hits       []app.FollowHit
	modes      map[string]string
	unanalyzed int
	err        string
}

// followInput is the snapshot a background follow job needs. It is built on
// the Update goroutine so the job never reads the live tree.
type followInput struct {
	entries []format.FileEntry
	present map[string]bool
}

func (n *TreeNode) followInput() followInput {
	var files []*TreeNode
	n.collectFiles(&files)
	entries := make([]format.FileEntry, 0, len(files))
	present := make(map[string]bool, len(files))
	for _, f := range files {
		p := filepath.ToSlash(f.Path)
		entries = append(entries, format.FileEntry{Path: p, Content: f.Content})
		present[p] = true
	}
	return followInput{entries: entries, present: present}
}

// FollowFrom walks the import graph from the seed file and applies the hit
// modes to the tree (seed full, near hops full, far hops signatures,
// everything else skipped), returning a one-line summary for the notice bar.
// Files the collector skipped (hidden, git-ignored) keep their state.
func (n *TreeNode) FollowFrom(rootDir, seed, direction string) string {
	if _, err := app.ParseFollowDirection(direction); err != nil {
		return "Follow: " + err.Error()
	}
	input := n.followInput()
	if !input.present[seed] {
		return fmt.Sprintf("Follow: %s is not in the collected tree", seed)
	}
	if !lang.HasImportScanner(seed) {
		return fmt.Sprintf("Follow: no import resolver for %q", filepath.Ext(seed))
	}
	ranker := app.NewRankerWithWeights(rootDir, rank.DefaultWeights())
	graph := ranker.DependencyGraph(input.entries)
	sel, err := app.FollowCandidates(input.entries, graph, nil, seed, direction, followDepth, followFullDepth, "")
	if err != nil {
		return "Follow: " + err.Error()
	}
	return n.applyFollowResult(seed, direction, sel.Hits, sel.Modes, sel.Unanalyzed)
}

// applyFollowResult mutates the tree from a completed walk. It runs on the
// Update goroutine, never in the background job.
func (n *TreeNode) applyFollowResult(seed, direction string, hits []app.FollowHit, modes map[string]string, unanalyzed int) string {
	var files []*TreeNode
	n.collectFiles(&files)
	reasons := make(map[string]string, len(hits))
	for _, h := range hits {
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
		mode, ok := modes[p]
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
	return followSummary(seed, direction, hits, unanalyzed)
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
