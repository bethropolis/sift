package rank

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initRepo creates a git repo with one commit containing a.txt and b.txt.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	git("config", "user.email", "t@t")
	git("config", "user.name", "t")

	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.txt", "one")
	write("b.txt", "two")
	git("add", ".")
	git("commit", "-q", "-m", "initial")
	return dir
}

func TestNotAGitRepo(t *testing.T) {
	dir := t.TempDir()
	g := New(dir)
	if g.Available() {
		t.Fatal("temp dir unexpectedly inside a git repo")
	}
	scores := g.Score(dir, []string{"a.txt"})
	if scores["a.txt"] != ScoreBaseline {
		t.Errorf("score = %v, want baseline %v", scores["a.txt"], ScoreBaseline)
	}
}

func TestChangesForTiers(t *testing.T) {
	dir := initRepo(t)
	g := New(dir)

	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Edit a.txt but never commit it, so it stays Modified.
	write("a.txt", "changed")

	// Three commits; use targeted adds so a.txt stays dirty.
	write("c.txt", "three")
	gitCmd(t, dir, "add", "c.txt")
	gitCmd(t, dir, "commit", "-q", "-m", "add c")
	write("d.txt", "four")
	gitCmd(t, dir, "add", "d.txt")
	gitCmd(t, dir, "commit", "-q", "-m", "add d")
	write("e.txt", "five")
	gitCmd(t, dir, "add", "e.txt")
	gitCmd(t, dir, "commit", "-q", "-m", "add e")

	changes := g.ChangesFor("HEAD")
	if !changes.Modified["a.txt"] {
		t.Errorf("a.txt should be Modified (uncommitted edit)")
	}
	if !changes.Diffed["e.txt"] {
		t.Errorf("e.txt should be Diffed (latest commit)")
	}
	if !changes.Committed["d.txt"] {
		t.Errorf("d.txt should be Committed (recent history)")
	}
	if changes.Score("a.txt") != ScoreModified {
		t.Errorf("a.txt score = %v, want %v", changes.Score("a.txt"), ScoreModified)
	}
	if changes.Score("e.txt") != ScoreDiffed {
		t.Errorf("e.txt score = %v, want %v", changes.Score("e.txt"), ScoreDiffed)
	}
	if changes.Score("d.txt") != ScoreCommitted {
		t.Errorf("d.txt score = %v, want %v", changes.Score("d.txt"), ScoreCommitted)
	}
}

func TestScoreDirPrefix(t *testing.T) {
	dir := initRepo(t)
	g := New(dir)
	gitCmd(t, dir, "status", "--porcelain") // no-op sanity

	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A modified file nested under sub/ makes the whole sub/ tree relevant.
	if err := os.WriteFile(filepath.Join(dir, "sub", "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	changes := g.ChangesFor("HEAD")
	if changes.Score("sub") != ScoreModified {
		t.Errorf("sub score = %v, want %v", changes.Score("sub"), ScoreModified)
	}
	if changes.Score("sub/deep/y.txt") != ScoreModified {
		t.Errorf("nested score = %v, want %v", changes.Score("sub/deep/y.txt"), ScoreModified)
	}
}

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
