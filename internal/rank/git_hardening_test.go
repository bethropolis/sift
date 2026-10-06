package rank

import (
	"crypto/sha256"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// hashTree records a content hash of every file under dir (including .git),
// so the read-only proof can detect any byte the scan leaves behind.
func hashTree(t *testing.T, dir string) map[string][32]byte {
	t.Helper()
	out := map[string][32]byte{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		// Skip sockets and other non-regular entries without blocking.
		info, infoErr := d.Info()
		if infoErr != nil || !info.Mode().IsRegular() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil // unreadable (e.g. lock file mid-write): ignore
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		out[rel] = sha256.Sum256(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestGitLeavesTreeUntouched is the serve read-only proof at the git layer:
// a full relevance pass must leave the project directory byte-identical,
// .git included.
func TestGitLeavesTreeUntouched(t *testing.T) {
	dir := initRepo(t)
	g := New(dir)

	before := hashTree(t, dir)

	_ = g.Available()
	_ = g.ChangesFor("HEAD")
	_ = g.Score(dir, []string{"a.txt", "b.txt"})
	_, _ = g.Head()
	_ = g.GetChurnFrequency(30)
	_ = g.RawPatch("HEAD~1", "HEAD")
	_ = g.RawWorktreePatch("HEAD")
	_ = g.ChangedSinceRef("HEAD")
	_ = g.AnalyzeCommitHistory(5)

	after := hashTree(t, dir)
	if len(before) != len(after) {
		t.Fatalf("file count changed: %d -> %d", len(before), len(after))
	}
	for path, want := range before {
		if got, ok := after[path]; !ok || got != want {
			t.Fatalf("file %q changed during read-only git pass", path)
		}
	}
}

// TestHostileRefsFailClosed ensures flag-like or control-character refs
// return empty results instead of reaching git argument parsing.
func TestHostileRefsFailClosed(t *testing.T) {
	dir := initRepo(t)
	g := New(dir)

	hostile := []string{"--help", "-h", "--version", "HEAD\n evil", "a b", "", "--"}
	for _, ref := range hostile {
		if short, subject := g.Ref(ref); short != "" || subject != "" {
			t.Fatalf("Ref(%q) = (%q, %q), want empty", ref, short, subject)
		}
		if got := g.Parent(ref); got != "" {
			t.Fatalf("Parent(%q) = %q, want empty", ref, got)
		}
		if got := g.ChangedBetween(ref, "HEAD"); len(got) != 0 {
			t.Fatalf("ChangedBetween(%q) = %v, want empty", ref, got)
		}
		if got := g.RawPatch(ref, "HEAD"); got != "" {
			t.Fatalf("RawPatch(%q) returned content", ref)
		}
		if got := g.ChangedSinceRef(ref); len(got) != 0 {
			t.Fatalf("ChangedSinceRef(%q) = %v, want empty", ref, got)
		}
	}
}

// TestFsmonitorNotExecuted plants a hostile core.fsmonitor hook and verifies
// the hardened invocation never runs it.
func TestFsmonitorNotExecuted(t *testing.T) {
	dir := initRepo(t)
	marker := filepath.Join(dir, "fsmonitor-ran")
	// A repo config asking git to run `touch <marker>` as its fsmonitor.
	gitCmd(t, dir, "config", "core.fsmonitor", "touch "+marker)

	g := New(dir)
	_ = g.ChangesFor("HEAD")
	_ = g.Score(dir, []string{"a.txt"})
	_ = g.RawWorktreePatch("HEAD")

	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("hostile core.fsmonitor hook was executed")
	}
}

// TestDiffDriverNotExecuted plants a hostile diff driver via repo config +
// .gitattributes and verifies driver-free diffs never run it.
func TestDiffDriverNotExecuted(t *testing.T) {
	dir := initRepo(t)
	marker := filepath.Join(dir, "driver-ran")
	gitCmd(t, dir, "config", "diff.evil.command", "touch "+marker)
	if err := os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte("*.txt diff=evil\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := New(dir)
	_ = g.ChangesFor("HEAD")
	_ = g.RawWorktreePatch("HEAD")
	_ = g.ChangedSinceRef("HEAD")

	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("hostile diff driver was executed")
	}
}

// TestGitEnvScrubbed ensures inherited GIT_* overrides cannot redirect the
// scan at another repository: the answers must come from dir, not other.
func TestGitEnvScrubbed(t *testing.T) {
	dir := initRepo(t)
	other := t.TempDir()
	gitCmd(t, other, "init", "-q", "-b", "main")
	gitCmd(t, other, "config", "user.email", "t@t")
	gitCmd(t, other, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(other, "z.txt"), []byte("other"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, other, "add", "z.txt")
	gitCmd(t, other, "commit", "-q", "-m", "other-subject")

	t.Setenv("GIT_DIR", filepath.Join(other, ".git"))
	t.Setenv("GIT_WORK_TREE", other)

	g := New(dir)
	if !g.Available() {
		t.Fatal("Available() failed with hostile GIT_DIR in environment")
	}
	_, subject := g.Head()
	if subject != "initial" {
		t.Fatalf("Head() subject = %q, want the scanned repo's commit", subject)
	}
}
