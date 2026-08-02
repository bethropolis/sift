// Package tui renders an interactive picker for the pick subcommand. It shows
// a foldable repository tree with live token tallies, per-file compression
// modes, and a file preview, and returns the user's selection.
package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Result carries the picker's outcome.
type Result struct {
	// Selections is the user's file selection and modes, empty when the
	// session ended via a delta dump or with nothing selected.
	Selections []Selection
	// DeltaDone reports that the session ended by performing a delta dump,
	// whose output replaces any picker selection.
	DeltaDone bool
}

// Run displays the picker for items and returns the picker's result. The
// picker takes over its own screen via the alternate buffer and restores the
// terminal on exit.
func Run(items []Item, opts Options) (Result, error) {
	if len(items) == 0 {
		return Result{}, nil
	}
	return runProgram(BuildTree(items), opts, Stream{})
}

// RunStreaming is the progressive variant of Run: it opens the picker
// immediately on a structure-only skeleton and merges items streamed over
// scan's channels as the background walk enriches them.
func RunStreaming(skeleton []Item, opts Options, stream Stream) (Result, error) {
	if len(skeleton) == 0 {
		return Result{}, nil
	}
	return runProgram(BuildTree(skeleton), opts, stream)
}

func runProgram(root *TreeNode, opts Options, stream Stream) (Result, error) {
	m := newModel(root, opts)
	m.stream = stream
	final, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	if err != nil {
		return Result{}, err
	}
	mm, ok := final.(model)
	if !ok {
		return Result{}, fmt.Errorf("tui: unexpected final model %T", final)
	}
	return Result{Selections: mm.root.Selections(), DeltaDone: mm.deltaDone}, nil
}
