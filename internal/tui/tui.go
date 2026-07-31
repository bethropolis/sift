// Package tui renders an interactive picker for the pick subcommand. It shows
// a foldable repository tree with live token tallies, per-file compression
// modes, and a file preview, and returns the user's selection.
package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Run displays the picker for items and returns the selected files with their
// output modes. An empty selection yields an empty slice. The picker takes
// over its own screen via the alternate buffer and restores the terminal on
// exit.
func Run(items []Item, opts Options) ([]Selection, error) {
	if len(items) == 0 {
		return nil, nil
	}
	root := BuildTree(items)
	m := newModel(root, opts)
	final, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	if err != nil {
		return nil, err
	}
	mm, ok := final.(model)
	if !ok {
		return nil, fmt.Errorf("tui: unexpected final model %T", final)
	}
	return mm.root.Selections(), nil
}
