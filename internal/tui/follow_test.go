package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func followFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func goFiles() map[string]string {
	return map[string]string{
		"go.mod":     "module example.com/followtui\n\ngo 1.26\n",
		"main.go":    "package main\n\nimport \"example.com/followtui/sub\"\n\nfunc main() { sub.Hi() }\n",
		"sub/sub.go": "package sub\n\nfunc Hi() {}\n",
		"other.go":   "package main\n\nfunc Other() {}\n",
	}
}

func buildFollowTree(t *testing.T, root string, files map[string]string) *TreeNode {
	t.Helper()
	var items []Item
	for name := range files {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		items = append(items, Item{Path: name, Content: content, TokensFull: len(content) * 4})
	}
	return BuildTree(items)
}

func TestFollowDependents(t *testing.T) {
	root := followFixture(t, goFiles())
	tree := buildFollowTree(t, root, goFiles())
	msg := tree.FollowFrom(root, "sub/sub.go", "dependents")
	if msg == "" {
		t.Fatal("empty summary")
	}
	modes := map[string]CompressMode{}
	var walk func(n *TreeNode)
	walk = func(n *TreeNode) {
		if n.Kind == KindFile {
			modes[n.Path] = n.Mode
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(tree)
	if modes["sub/sub.go"] != ModeFull {
		t.Errorf("seed mode = %s, want full", modes["sub/sub.go"])
	}
	if modes["main.go"] != ModeFull {
		t.Errorf("importer mode = %s, want full", modes["main.go"])
	}
	if modes["other.go"] != ModeSkip {
		t.Errorf("unrelated mode = %s, want skip", modes["other.go"])
	}
}

func TestFollowDepsDirection(t *testing.T) {
	root := followFixture(t, goFiles())
	tree := buildFollowTree(t, root, goFiles())
	tree.FollowFrom(root, "main.go", "deps")
	var mode func(path string) CompressMode
	mode = func(path string) CompressMode {
		var found CompressMode = "?"
		var walk func(n *TreeNode)
		walk = func(n *TreeNode) {
			if n.Kind == KindFile && n.Path == path {
				found = n.Mode
			}
			for _, c := range n.Children {
				walk(c)
			}
		}
		walk(tree)
		return found
	}
	if mode("sub/sub.go") != ModeFull {
		t.Errorf("dependency mode = %s, want full", mode("sub/sub.go"))
	}
	if mode("other.go") != ModeSkip {
		t.Errorf("unrelated mode = %s, want skip", mode("other.go"))
	}
	if mode("main.go") != ModeFull {
		t.Errorf("seed mode = %s, want full", mode("main.go"))
	}
}

func TestFollowRejects(t *testing.T) {
	root := followFixture(t, goFiles())
	tree := buildFollowTree(t, root, goFiles())
	for name, seed := range map[string]string{
		"missing":   "nope.go",
		"directory": "sub",
		"no scanner": "go.mod",
	} {
		// go.mod has content but no import scanner; the others fail earlier.
		// None of them may touch the selection.
		before := tree.SelectedCount()
		msg := tree.FollowFrom(root, seed, "dependents")
		if msg == "" {
			t.Errorf("%s: empty rejection", name)
		}
		if got := tree.SelectedCount(); got != before {
			t.Errorf("%s: selection changed on rejection", name)
		}
	}
}
