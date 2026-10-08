package jail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testJail(t *testing.T) (*Jail, string, string) {
	t.Helper()
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ok.txt"), []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	j, err := New([]string{root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return j, root, outside
}

func TestResolveOK(t *testing.T) {
	j, root, _ := testJail(t)
	got, err := j.Resolve(filepath.Join(root, "ok.txt"))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != filepath.Join(root, "ok.txt") {
		t.Fatalf("got %q", got)
	}
}

func TestResolveTraversal(t *testing.T) {
	j, root, outside := testJail(t)
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("s"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{
		filepath.Join(root, "..", "secret.txt"),
		filepath.Join(root, "sub", "..", "..", "secret.txt"),
		secret,
		"/etc/passwd",
		"relative/path",
		"",
		string([]byte{'a', 0, 'b'}),
		strings.Repeat("a", maxPathLen+1),
	} {
		if _, err := j.Resolve(p); err == nil {
			t.Fatalf("Resolve(%q) admitted", p)
		}
	}
}

func TestResolveSymlinks(t *testing.T) {
	j, root, outside := testJail(t)
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("s"), 0o600); err != nil {
		t.Fatal(err)
	}
	evil := filepath.Join(root, "evil.txt")
	if err := os.Symlink(secret, evil); err != nil {
		t.Skipf("no symlinks: %v", err)
	}
	if _, err := j.Resolve(evil); err == nil {
		t.Fatal("escaping symlink admitted")
	}
	inner := filepath.Join(root, "inner.txt")
	if err := os.WriteFile(inner, []byte("i"), 0o600); err != nil {
		t.Fatal(err)
	}
	good := filepath.Join(root, "good.txt")
	if err := os.Symlink(inner, good); err != nil {
		t.Fatal(err)
	}
	if _, err := j.Resolve(good); err != nil {
		t.Fatalf("inner symlink rejected: %v", err)
	}
}

// TestResolvePrefixConfusion ensures root/code never admits root/code-evil.
func TestResolvePrefixConfusion(t *testing.T) {
	base := t.TempDir()
	code := filepath.Join(base, "code")
	evil := filepath.Join(base, "code-evil")
	if err := os.MkdirAll(code, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(evil, 0o755); err != nil {
		t.Fatal(err)
	}
	j, err := New([]string{code}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := j.Resolve(filepath.Join(evil, "x")); err == nil {
		t.Fatal("prefix-confused sibling admitted")
	}
}

func TestDenylistAnyDepth(t *testing.T) {
	j, root, _ := testJail(t)
	for _, p := range []string{
		".ssh",
		".ssh/config",
		"sub/.aws/credentials",
		".config/gcloud/config",
		".docker/config.json",
		"a/b/.password-store/x",
		".netrc",
		".local/share/keyrings/login",
	} {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(full, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := j.Resolve(full); err == nil {
			t.Fatalf("denylisted path admitted: %s", p)
		}
	}
	// Benign lookalikes stay open.
	similar := filepath.Join(root, "my.ssh-config", "ok.txt")
	if err := os.MkdirAll(filepath.Dir(similar), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(similar, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := j.Resolve(similar); err != nil {
		t.Fatalf("lookalike rejected: %v", err)
	}
}

func TestNewRejectsBadRoots(t *testing.T) {
	if _, err := New(nil, nil); err == nil {
		t.Fatal("empty roots accepted")
	}
	if _, err := New([]string{filepath.Join(t.TempDir(), "nope")}, nil); err == nil {
		t.Fatal("missing root accepted")
	}
	f := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New([]string{f}, nil); err == nil {
		t.Fatal("file root accepted")
	}
}

// FuzzResolve fuzzes the resolver with hostile inputs.
func FuzzResolve(f *testing.F) {
	f.Add("../x")
	f.Add("/etc/passwd")
	f.Add(".ssh/config")
	f.Add("a/../../b")
	f.Add("%2e%2e/x")
	f.Add("....//x")
	f.Fuzz(func(t *testing.T, p string) {
		root := t.TempDir()
		j, err := New([]string{root}, nil)
		if err != nil {
			t.Fatal(err)
		}
		got, err := j.Resolve(filepath.Join(root, p))
		if err != nil {
			return
		}
		rel, relErr := filepath.Rel(root, got)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("escape admitted: %q -> %q", p, got)
		}
	})
}

func TestExtraRoots(t *testing.T) {
	j, _, outside := testJail(t)
	file := filepath.Join(outside, "f.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := j.Resolve(file); err == nil {
		t.Fatal("outside path resolved before AddRoot")
	}

	real, err := j.AddRoot(outside)
	if err != nil {
		t.Fatalf("AddRoot: %v", err)
	}
	if _, err := j.Resolve(file); err != nil {
		t.Fatalf("path under an added root: %v", err)
	}
	// The added root is exactly that directory: its parent stays closed.
	if _, err := j.Resolve(filepath.Dir(outside)); err == nil {
		t.Error("parent of an added root resolved")
	}
	// The startup roots listed in the UI are unchanged.
	if got := j.Roots(); len(got) != 1 {
		t.Errorf("Roots() = %v, want only the startup root", got)
	}
	// Adding twice is a no-op.
	if again, err := j.AddRoot(outside); err != nil || again != real {
		t.Errorf("second AddRoot = %q, %v", again, err)
	}

	j.RemoveRoot(real)
	if _, err := j.Resolve(file); err == nil {
		t.Error("path resolved after RemoveRoot")
	}
	j.RemoveRoot(real) // unknown roots are ignored
}

func TestAddRootRejectsBadInput(t *testing.T) {
	j, root, _ := testJail(t)
	if _, err := j.AddRoot(filepath.Join(root, "missing")); err == nil {
		t.Error("missing dir accepted")
	}
	if _, err := j.AddRoot(filepath.Join(root, "ok.txt")); err == nil {
		t.Error("regular file accepted as a root")
	}
}

func TestExtraRootKeepsDenylist(t *testing.T) {
	j, _, outside := testJail(t)
	if err := os.MkdirAll(filepath.Join(outside, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := j.AddRoot(outside); err != nil {
		t.Fatal(err)
	}
	if _, err := j.Resolve(filepath.Join(outside, ".ssh")); err == nil {
		t.Error("denylisted dir reachable under an added root")
	}
}

func TestExtraRootSymlinkEscape(t *testing.T) {
	j, _, outside := testJail(t)
	secretDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(secretDir, "s.txt"), []byte("s"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secretDir, filepath.Join(outside, "link")); err != nil {
		t.Skip("symlinks unavailable")
	}
	if _, err := j.AddRoot(outside); err != nil {
		t.Fatal(err)
	}
	// A hostile repo's symlink must not lead out of the checkout.
	if _, err := j.Resolve(filepath.Join(outside, "link", "s.txt")); err == nil {
		t.Error("symlink inside an added root escaped it")
	}
}
