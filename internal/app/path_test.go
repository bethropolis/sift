package app

import (
	"path/filepath"
	"testing"
)

func TestResolveWithinRoot(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "tmp", "root")

	cases := []struct {
		name string
		rel  string
		want string
		err  bool
	}{
		{"normal file", "foo.go", filepath.Join(root, "foo.go"), false},
		{"nested", "a/b/c.go", filepath.Join(root, "a", "b", "c.go"), false},
		{"dot segments", "./a/./b.go", filepath.Join(root, "a", "b.go"), false},
		{"dotdot inside", "a/../b.go", filepath.Join(root, "b.go"), false},
		{"slash separators", "a/b.go", filepath.Join(root, "a", "b.go"), false},
		{"parent escape", "../escape.go", "", true},
		{"nested parent escape", "a/../../escape.go", "", true},
		{"bare double dot", "..", "", true},
		{"absolute", "/etc/passwd", "", true},
		{"absolute inside root", "/tmp/root/foo.go", "", true},
		{"empty", "", "", true},
		{"root dot", ".", "", true},
		{"prefix collision escape", "../root2/file.go", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveWithinRoot(root, tc.rel)
			if tc.err {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
