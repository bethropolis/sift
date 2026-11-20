package rank

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// gitIn runs git in dir with a deterministic identity and optional extra env.
func gitIn(t *testing.T, dir string, extraEnv []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
	)
	cmd.Env = append(cmd.Env, extraEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func newHistoryRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitIn(t, dir, nil, "init", "-q", "-b", "main")
	gitIn(t, dir, nil, "config", "user.email", "t@t")
	gitIn(t, dir, nil, "config", "user.name", "t")
	return dir
}

func writeAndCommit(t *testing.T, dir, name, body string, extraEnv ...string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, extraEnv, "add", name)
	gitIn(t, dir, extraEnv, "commit", "-qm", "touch "+name)
}

func TestChangesForTiersFromSingleLog(t *testing.T) {
	dir := newHistoryRepo(t)
	for i := 1; i <= 6; i++ {
		writeAndCommit(t, dir, fmt.Sprintf("f%d.go", i), fmt.Sprintf("package p // %d\n", i))
	}

	changes := New(dir).ChangesFor("HEAD")
	if !changes.Diffed["f6.go"] {
		t.Errorf("newest commit not marked Diffed: %v", changes.Diffed)
	}
	for i := 2; i <= 6; i++ {
		key := fmt.Sprintf("f%d.go", i)
		if !changes.Committed[key] {
			t.Errorf("newest-five commit missing %s: %v", key, changes.Committed)
		}
	}
	if changes.Committed["f1.go"] {
		t.Errorf("sixth-newest commit should not be Committed: %v", changes.Committed)
	}
}

func TestChangesForSingleCommit(t *testing.T) {
	dir := newHistoryRepo(t)
	writeAndCommit(t, dir, "only.go", "package p\n")

	changes := New(dir).ChangesFor("HEAD")
	if !changes.Diffed["only.go"] || !changes.Committed["only.go"] {
		t.Fatalf("root commit not scored: diffed=%v committed=%v", changes.Diffed, changes.Committed)
	}
}

func TestPorcelainStatusCodes(t *testing.T) {
	dir := newHistoryRepo(t)
	writeAndCommit(t, dir, "tracked.go", "package tracked\n")
	writeAndCommit(t, dir, "renamed.go", "package renamed\n")
	writeAndCommit(t, dir, "staged.go", "package staged\n")

	// Modified tracked file.
	if err := os.WriteFile(filepath.Join(dir, "tracked.go"), []byte("package tracked // edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Staged new file.
	if err := os.WriteFile(filepath.Join(dir, "added.go"), []byte("package added\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, nil, "add", "added.go")
	// Untracked file.
	if err := os.WriteFile(filepath.Join(dir, "untracked.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Rename.
	gitIn(t, dir, nil, "mv", "renamed.go", "moved.go")

	changes := New(dir).ChangesFor("HEAD")
	want := map[string]string{
		"tracked.go":   "M",
		"added.go":     "A",
		"untracked.go": "?",
		"moved.go":     "R",
	}
	for path, code := range want {
		if got := changes.Status[path]; got != code {
			t.Errorf("Status[%s] = %q, want %q (all=%v)", path, got, code, changes.Status)
		}
	}
	if changes.Status["renamed.go"] != "D" {
		t.Errorf("renamed source status = %q, want D", changes.Status["renamed.go"])
	}
}

func TestChurnDecaysWithAge(t *testing.T) {
	dir := newHistoryRepo(t)
	oldDate := time.Now().Add(-120 * 24 * time.Hour).Format(time.RFC3339)
	newDate := time.Now().Add(-2 * 24 * time.Hour).Format(time.RFC3339)

	writeAndCommit(t, dir, "old.go", "package p\n", "GIT_AUTHOR_DATE="+oldDate, "GIT_COMMITTER_DATE="+oldDate)
	writeAndCommit(t, dir, "new.go", "package p\n", "GIT_AUTHOR_DATE="+newDate, "GIT_COMMITTER_DATE="+newDate)

	changes := New(dir).ChangesFor("HEAD")
	if changes.Churn["old.go"] == 0 || changes.Churn["new.go"] == 0 {
		t.Fatalf("missing churn weights: %v", changes.Churn)
	}
	if changes.Churn["new.go"] <= changes.Churn["old.go"] {
		t.Fatalf("recent touch should outweigh an old one: old=%f new=%f", changes.Churn["old.go"], changes.Churn["new.go"])
	}
	for path, w := range changes.Churn {
		if w < 0 || w > 32 {
			t.Errorf("churn weight for %s out of range: %f", path, w)
		}
	}
}

func TestShallowCloneRanks(t *testing.T) {
	origin := newHistoryRepo(t)
	for i := 0; i < 3; i++ {
		writeAndCommit(t, origin, fmt.Sprintf("f%d.go", i), "package p\n")
	}
	clone := filepath.Join(t.TempDir(), "clone")
	gitIn(t, filepath.Dir(clone), nil, "clone", "-q", "--depth", "1", "file://"+origin, clone)

	changes := New(clone).ChangesFor("HEAD")
	if len(changes.Diffed) == 0 {
		t.Fatalf("shallow clone produced no diffed files: %v", changes.Diffed)
	}
}
