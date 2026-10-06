package walker

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/bethropolis/sift/internal/ignore"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func walkCollect(t *testing.T, root string, opts ...Option) []string {
	t.Helper()
	matcher, err := ignore.New(root)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	_, err = Walk(root, matcher, func(rel string, content []byte, err error) error {
		if err == nil {
			got = append(got, rel)
		}
		return nil
	}, opts...)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	return got
}

func containsPath(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}

// TestContainPathBoundary ensures /root/code does not admit /root/code-evil.
func TestContainPathBoundary(t *testing.T) {
	root := t.TempDir()
	evil := root + "-evil"
	if err := os.MkdirAll(evil, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(evil, "secret.txt")
	writeFile(t, secret, "x")

	// A symlink inside root pointing at the evil sibling must be rejected.
	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ContainPath(root, link); err == nil {
		t.Fatal("ContainPath admitted a symlink escaping to a prefix-confused sibling")
	}

	// A symlink to a file inside the root is admitted and resolves.
	inner := filepath.Join(root, "inner.txt")
	writeFile(t, inner, "y")
	good := filepath.Join(root, "good.txt")
	if err := os.Symlink(inner, good); err != nil {
		t.Fatal(err)
	}
	resolved, err := ContainPath(root, good)
	if err != nil {
		t.Fatalf("ContainPath rejected an inner symlink: %v", err)
	}
	if resolved != inner {
		t.Fatalf("resolved = %q, want %q", resolved, inner)
	}
}

// TestWalkSkipsSymlinkEscape ensures the content walk never reads through an
// escaping link, while inner links keep working.
func TestWalkSkipsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.txt"), "outside")
	writeFile(t, filepath.Join(root, "real.txt"), "inside")

	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "evil.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "real.txt"), filepath.Join(root, "link.txt")); err != nil {
		t.Fatal(err)
	}

	got := walkCollect(t, root)
	if containsPath(got, "evil.txt") {
		t.Fatal("walk read through an escaping symlink")
	}
	if !containsPath(got, "link.txt") || !containsPath(got, "real.txt") {
		t.Fatalf("walk dropped inner files: %v", got)
	}

	// The escape must be reported, not silent.
	matcher, merr := ignore.New(root)
	if merr != nil {
		t.Fatal(merr)
	}
	skipped, err := Walk(root, matcher, func(rel string, content []byte, err error) error { return nil })
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	seen := false
	for _, s := range skipped {
		if s.Path == "evil.txt" && s.Reason == ReasonSkippedSymlinkEscape {
			seen = true
		}
	}
	if !seen {
		t.Fatalf("escape not reported as %q: %v", ReasonSkippedSymlinkEscape, skipped)
	}
}

// TestWalkMetaSkipsNonRegular ensures FIFOs and escaping links are not
// advertised by the skeleton walk.
func TestWalkMetaSkipsNonRegular(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "ok.txt"), "ok")
	if err := syscall.Mkfifo(filepath.Join(root, "pipe"), 0o600); err != nil {
		t.Skipf("cannot create fifo: %v", err)
	}
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "s.txt"), "s")
	if err := os.Symlink(filepath.Join(outside, "s.txt"), filepath.Join(root, "evil.txt")); err != nil {
		t.Fatal(err)
	}

	matcher, merr := ignore.New(root)
	if merr != nil {
		t.Fatal(merr)
	}
	metas, _, err := WalkMeta(root, matcher)
	if err != nil {
		t.Fatalf("WalkMeta: %v", err)
	}
	for _, m := range metas {
		if m.Path == "pipe" {
			t.Fatal("WalkMeta advertised a FIFO")
		}
		if m.Path == "evil.txt" {
			t.Fatal("WalkMeta advertised an escaping symlink")
		}
	}
	found := false
	for _, m := range metas {
		if m.Path == "ok.txt" {
			found = true
		}
	}
	if !found {
		t.Fatal("WalkMeta dropped a regular file")
	}
}

// FuzzContainPath fuzzes the resolver against traversal inputs.
func FuzzContainPath(f *testing.F) {
	root := "/home/u/code"
	f.Add("a/b.txt")
	f.Add("../evil")
	f.Add("/etc/passwd")
	f.Add("a/../../code-evil/x")
	f.Add("link")
	f.Fuzz(func(t *testing.T, p string) {
		// Must never admit an absolute escape or a prefix-confused path.
		resolved, err := ContainPath(root, filepath.Join(root, p))
		if err != nil {
			return
		}
		if resolved != root && len(resolved) > len(root) && resolved[:len(root)+1] != root+"/" {
			t.Fatalf("ContainPath admitted %q -> %q", p, resolved)
		}
	})
}
