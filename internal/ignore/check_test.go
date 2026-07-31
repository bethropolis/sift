package ignore

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFiles creates the given files (relative paths) under root.
func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for path, content := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestShouldIgnoreHidden(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"visible.txt":           "visible",
		".hidden":               "hidden",
		"src/main.go":           "package main",
		"src/.secret/inner.txt": "secret",
	})

	m, err := New(root)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		path  string
		isDir bool
		want  bool
	}{
		{name: "visible file", path: "visible.txt", want: false},
		{name: "visible nested file", path: "src/main.go", want: false},
		{name: "hidden file", path: ".hidden", want: true},
		{name: "hidden dir", path: ".secret", isDir: true, want: true},
		{name: "file in hidden dir", path: "src/.secret/inner.txt", want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := m.ShouldIgnore(tc.path, tc.isDir); got != tc.want {
				t.Errorf("ShouldIgnore(%q, %v) = %v, want %v", tc.path, tc.isDir, got, tc.want)
			}
		})
	}
}

func TestShouldIgnoreGit(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		".git/config": "git config",
		".git/HEAD":   "ref: refs/heads/main",
		".gitignore":  "*.log\n",
		"debug.log":   "log",
		"main.go":     "package main",
	})

	m, err := New(root, WithHiddenIgnore(false), WithGitIgnore(true))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		path  string
		isDir bool
		want  bool
	}{
		{name: ".git dir", path: ".git", isDir: true, want: true},
		{name: "file inside .git", path: ".git/config", want: true},
		{name: "nested .git", path: "vendor/.git/HEAD", want: true},
		{name: ".gitignore file itself", path: ".gitignore", want: false},
		{name: "unrelated file", path: "main.go", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := m.ShouldIgnore(tc.path, tc.isDir); got != tc.want {
				t.Errorf("ShouldIgnore(%q, %v) = %v, want %v", tc.path, tc.isDir, got, tc.want)
			}
		})
	}
}

func TestShouldIgnoreCustomRules(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"debug.log":     "log",
		"important.log": "important",
		"main.go":       "package main",
	})

	m, err := New(root, WithCustomRules([]string{"*.log", "!important.log"}))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "matched by pattern", path: "debug.log", want: true},
		{name: "matched but negated", path: "important.log", want: false},
		{name: "not matched", path: "main.go", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := m.ShouldIgnore(tc.path, false); got != tc.want {
				t.Errorf("ShouldIgnore(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestShouldIgnoreRepoRules(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		".gitignore":    "*.log\n!important.log\n",
		"debug.log":     "log",
		"important.log": "important",
		"main.go":       "package main",
	})

	m, err := New(root)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "matched by repo rule", path: "debug.log", want: true},
		{name: "negated by repo rule", path: "important.log", want: false},
		{name: "not matched", path: "main.go", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := m.ShouldIgnore(tc.path, false); got != tc.want {
				t.Errorf("ShouldIgnore(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestShouldIgnoreDisabled(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		".hidden": "hidden",
	})

	m, err := New(root, WithDisabled(true))
	if err != nil {
		t.Fatal(err)
	}

	if m.ShouldIgnore(".hidden", false) {
		t.Error("ShouldIgnore(.hidden) = true, want false for disabled matcher")
	}
}
