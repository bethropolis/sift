package tui

import "sort"

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

// SelectByRank selects files in descending rank order until the cumulative
// active tokens would exceed budget (0 = unlimited). Before charging each
// file it assigns the smart mode: a history-derived preference wins, then a
// signature summary when one exists, else full content. Returns the number of
// files selected.
func (n *TreeNode) SelectByRank(budget int) int {
	var files []*TreeNode
	n.collectFiles(&files)
	sort.SliceStable(files, func(i, j int) bool {
		if files[i].RankScore != files[j].RankScore {
			return files[i].RankScore > files[j].RankScore
		}
		return files[i].Path < files[j].Path
	})

	used := 0
	count := 0
	for _, f := range files {
		switch {
		case f.PreferredMode != "":
			f.Mode = f.PreferredMode
		case len(f.SigContent) > 0:
			f.Mode = ModeSignatures
		default:
			f.Mode = ModeFull
		}
		toks := f.TokensFull
		if f.Mode == ModeSignatures {
			toks = f.TokensSig
		}
		if budget > 0 && used+toks > budget {
			continue
		}
		f.setSelectedSubtree(true)
		used += toks
		count++
	}
	n.recompute()
	return count
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
