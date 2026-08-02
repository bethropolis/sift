package tui

import (
	"path/filepath"

	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/selection"
)

// Toggle flips the node's selection. Files flip between selected and not;
// directories recursively select or deselect all descendants. Selection
// bubbles up to parents as Partial/Selected.
func (n *TreeNode) Toggle() {
	switch n.SelectState {
	case Selected, Partial:
		n.setSelected(false)
	default:
		n.setSelected(true)
	}
}

// setSelected sets the subtree to the given selection state.
func (n *TreeNode) setSelected(on bool) {
	n.setSelectedSubtree(on)
	n.recompute()
	n.bubbleUp()
}

func (n *TreeNode) setSelectedSubtree(on bool) {
	if on {
		n.SelectState = Selected
	} else {
		n.SelectState = Unselected
	}
	for _, c := range n.Children {
		c.setSelectedSubtree(on)
	}
}

// bubbleUp recomputes the SelectState of every ancestor.
func (n *TreeNode) bubbleUp() {
	for p := n.Parent; p != nil; p = p.Parent {
		p.recompute()
	}
}

// CycleMode advances the effective mode FULL -> SIGS -> SKIP -> FULL and
// applies it to the whole subtree, so a directory set to a mode propagates to
// every file beneath it.
func (n *TreeNode) CycleMode() {
	var next CompressMode
	switch n.Mode {
	case ModeFull:
		next = ModeSignatures
	case ModeSignatures:
		next = ModeSkip
	default:
		next = ModeFull
	}
	n.applyMode(next)
	n.recompute()
	n.bubbleUp()
}

// applyMode sets the mode on n and every descendant.
func (n *TreeNode) applyMode(m CompressMode) {
	n.Mode = m
	for _, c := range n.Children {
		c.applyMode(m)
	}
}

// TotalActiveTokens returns the cached active tokens for n in O(1) time.
func (n *TreeNode) TotalActiveTokens() int {
	return n.ActiveTokens
}

// Selection pairs a selected path with its output mode.
type Selection struct {
	Path string
	Mode CompressMode
}

// Selections returns the chosen files in presentation order, each with its
// active mode. Files under skipped directories are omitted.
func (n *TreeNode) Selections() []Selection {
	var out []Selection
	n.collectSelections(&out)
	return out
}

func (n *TreeNode) collectSelections(out *[]Selection) {
	for _, c := range n.Children {
		if c.SelectState == Unselected || c.Mode == ModeSkip {
			continue
		}
		if c.Kind == KindFile {
			*out = append(*out, Selection{Path: c.Path, Mode: c.Mode})
			continue
		}
		c.collectSelections(out)
	}
}

// FileCount returns the cached file count under n in O(1) time.
func (n *TreeNode) FileCount() int {
	return n.FileCountVal
}

// SelectedCount returns the cached selected file count under n in O(1) time.
func (n *TreeNode) SelectedCount() int {
	return n.SelectedCountVal
}

// SelectByRank delegates smart selection to the shared utility optimizer.
// Budget zero means unlimited; otherwise the optimizer considers full,
// signature, and skipped variants together. Returns the selected file count.
func (n *TreeNode) SelectByRank(budget int) int {
	var files []*TreeNode
	n.collectFiles(&files)
	candidates := make([]selection.Candidate, 0, len(files))
	byPath := make(map[string]*TreeNode, len(files))
	for _, f := range files {
		candidates = append(candidates, selection.Candidate{
			File: format.FileEntry{
				Path:       filepath.ToSlash(f.Path),
				Content:    f.Content,
				SigContent: f.SigContent,
				TokensFull: f.TokensFull,
				TokensSig:  f.TokensSig,
				RankScore:  f.RankScore,
			},
			PreferredMode: string(f.PreferredMode),
		})
		byPath[filepath.ToSlash(f.Path)] = f
	}
	result := selection.Select(candidates, selection.Request{Budget: budget})
	n.ClearSelection()
	for _, decision := range result.Decisions {
		f := byPath[decision.Path]
		if f == nil {
			continue
		}
		switch decision.Mode {
		case selection.ModeSignatures:
			f.Mode = ModeSignatures
		case selection.ModeSkip:
			f.Mode = ModeSkip
		default:
			f.Mode = ModeFull
		}
		if decision.Selected {
			f.setSelectedSubtree(true)
		}
	}
	n.recompute()
	return len(result.Selected)
}

// ClearSelection deselects every file.
func (n *TreeNode) ClearSelection() {
	for _, c := range n.Children {
		c.setSelectedSubtree(false)
	}
	n.recompute()
}

func (n *TreeNode) collectFiles(out *[]*TreeNode) {
	for _, c := range n.Children {
		if c.Kind == KindFile {
			*out = append(*out, c)
			continue
		}
		c.collectFiles(out)
	}
}
