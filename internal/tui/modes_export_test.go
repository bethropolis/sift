package tui

import (
	"testing"
)

func TestModeAssignmentsAndExport(t *testing.T) {
	root := BuildTree([]Item{
		{Path: "src/app/full.go"},
		{Path: "src/lib/a.go"},
		{Path: "src/lib/b.go"},
		{Path: "docs/skip.md"},
	})
	var walk func(*TreeNode)
	walk = func(n *TreeNode) {
		switch n.Path {
		case "src/lib":
			for _, c := range n.Children {
				c.Mode = ModeSignatures
			}
		case "docs/skip.md":
			n.Mode = ModeSkip
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
	assignments := root.ModeAssignments()
	if assignments["src/lib/a.go"] != "signatures" || assignments["docs/skip.md"] != "skip" {
		t.Fatalf("assignments = %v", assignments)
	}

	m := newModel(root, Options{})
	cmd := m.exportModes()
	if cmd == nil || !m.genBusy {
		t.Fatal("exportModes did not start a clipboard job")
	}
	if m.notice == "" {
		t.Fatal("exportModes did not show a notice")
	}
}
