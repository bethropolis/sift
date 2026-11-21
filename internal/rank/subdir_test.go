package rank

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// subdirRepo creates a repository with a nested `sub` directory and returns
// the repository root. All writes go through writeSub.
func subdirRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	git("config", "user.email", "t@t")
	git("config", "user.name", "t")
	for name, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("add", ".")
	git("commit", "-q", "-m", "initial")
	return dir
}

func TestChangedPathsAreScanRootRelative(t *testing.T) {
	repo := subdirRepo(t, map[string]string{
		"root.go":  "package root\n",
		"sub/a.go": "package sub\n",
		"sub/b.go": "package sub\n",
	})
	if err := os.WriteFile(filepath.Join(repo, "sub", "a.go"), []byte("package sub\n// changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := New(filepath.Join(repo, "sub"))
	if got := g.Prefix(); got != "sub" {
		t.Fatalf("Prefix() = %q, want sub", got)
	}
	changes := g.ChangesFor("HEAD")
	if !changes.Modified["a.go"] {
		t.Errorf("a.go not marked modified: %+v", changes.Modified)
	}
	if changes.Modified["sub/a.go"] {
		t.Errorf("repo-root-relative path leaked: %+v", changes.Modified)
	}
	if changes.Modified["root.go"] {
		t.Errorf("file outside the scan root leaked: %+v", changes.Modified)
	}

	if got := g.ChangedSinceRef("HEAD"); len(got) != 1 || got[0] != "a.go" {
		t.Errorf("ChangedSinceRef = %v, want [a.go]", got)
	}
}

func TestChangedBetweenSubdirAndQuotedNames(t *testing.T) {
	repo := subdirRepo(t, map[string]string{"sub/a.go": "package sub\n"})
	name := "sub/na me ü.go"
	if err := os.WriteFile(filepath.Join(repo, filepath.FromSlash(name)), []byte("package sub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "commit", "-q", "-m", "add spaced")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	g := New(filepath.Join(repo, "sub"))
	got := g.ChangedBetween("HEAD~1", "HEAD")
	if len(got) != 1 || got[0] != "na me ü.go" {
		t.Fatalf("ChangedBetween = %q, want [na me ü.go]", got)
	}
}

func TestRawPatchScopedToScanRoot(t *testing.T) {
	repo := subdirRepo(t, map[string]string{
		"root.go":  "package root\n",
		"sub/a.go": "package sub\n",
		"sub/b.go": "package sub\n",
	})
	if err := os.WriteFile(filepath.Join(repo, "root.go"), []byte("package root\n// root change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "sub", "a.go"), []byte("package sub\n// sub change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, repo, "add", ".")
	gitCmd(t, repo, "commit", "-q", "-m", "changes")

	g := New(filepath.Join(repo, "sub"))
	patch := g.RawPatch("HEAD~1", "HEAD")
	if !strings.Contains(patch, "a.go") {
		t.Fatalf("RawPatch missing sub file:\n%s", patch)
	}
	if strings.Contains(patch, "root.go") {
		t.Fatalf("RawPatch leaked a file outside the scan root:\n%s", patch)
	}

	if err := os.WriteFile(filepath.Join(repo, "root.go"), []byte("package root\n// dirty root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "sub", "b.go"), []byte("package sub\n// dirty sub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	worktree := g.RawWorktreePatch("HEAD")
	if !strings.Contains(worktree, "b.go") {
		t.Fatalf("RawWorktreePatch missing sub file:\n%s", worktree)
	}
	if strings.Contains(worktree, "root.go") {
		t.Fatalf("RawWorktreePatch leaked a file outside the scan root:\n%s", worktree)
	}
}

func TestChurnFrequencySubdir(t *testing.T) {
	repo := subdirRepo(t, map[string]string{
		"sub/a.go": "package sub\n",
		"sub/b.go": "package sub\n",
	})
	for i := 0; i < 2; i++ {
		body := "package sub\n// edit " + string(rune('a'+i)) + "\n"
		if err := os.WriteFile(filepath.Join(repo, "sub", "a.go"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("git", "commit", "-qam", "edit")
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git commit: %v\n%s", err, out)
		}
	}

	g := New(filepath.Join(repo, "sub"))
	churn := g.GetChurnFrequency(30)
	if churn["a.go"] == 0 {
		t.Errorf("churn for a.go = 0, want > 0: %v", churn)
	}
	if churn["sub/a.go"] != 0 {
		t.Errorf("repo-root-relative churn key leaked: %v", churn)
	}
}
