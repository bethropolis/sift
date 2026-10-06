package state

import (
	"os"
	"path/filepath"
	"testing"
)

// isolateConfigDir points UserConfigDir at a temp dir via XDG_CONFIG_HOME.
// os.UserConfigDir honors XDG_CONFIG_HOME on Linux.
func isolateConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}

// TestRecordRecentOrdersAndCaps verifies ordering, dedup, and the 25 cap.
func TestRecordRecentOrdersAndCaps(t *testing.T) {
	isolateConfigDir(t)

	for i := 0; i < 30; i++ {
		RecordRecent(filepath.Join("/tmp", "proj"+string(rune('a'+i))))
	}
	recents, err := ListRecents()
	if err != nil {
		t.Fatalf("ListRecents: %v", err)
	}
	if len(recents) != maxRecents {
		t.Fatalf("len = %d, want cap %d", len(recents), maxRecents)
	}
	if recents[0].Name != "proj"+string(rune('a'+29)) {
		t.Fatalf("most recent = %q, want last recorded", recents[0].Name)
	}

	// Re-recording moves to front without duplicating.
	RecordRecent(filepath.Join("/tmp", "proja"))
	recents, _ = ListRecents()
	if len(recents) != maxRecents {
		t.Fatalf("len after re-record = %d, want %d", len(recents), maxRecents)
	}
	if recents[0].Root != filepath.Join("/tmp", "proja") {
		t.Fatalf("re-recorded root not first: %q", recents[0].Root)
	}
	seen := map[string]int{}
	for _, r := range recents {
		seen[r.Root]++
	}
	for root, n := range seen {
		if n > 1 {
			t.Fatalf("duplicate recent %q x%d", root, n)
		}
	}
}

// TestRemoveRecent drops the entry.
func TestRemoveRecent(t *testing.T) {
	isolateConfigDir(t)

	RecordRecent("/tmp/keep")
	RecordRecent("/tmp/drop")
	if err := RemoveRecent("/tmp/drop"); err != nil {
		t.Fatalf("RemoveRecent: %v", err)
	}
	recents, _ := ListRecents()
	if len(recents) != 1 || recents[0].Root != "/tmp/keep" {
		t.Fatalf("recents = %v, want only /tmp/keep", recents)
	}
}

// TestRecordRecentBranch reads .git/HEAD without forking git.
func TestRecordRecentBranch(t *testing.T) {
	isolateConfigDir(t)

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref: refs/heads/feat/x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	RecordRecent(root)
	recents, _ := ListRecents()
	if len(recents) != 1 {
		t.Fatalf("recents = %v, want 1 entry", recents)
	}
	if recents[0].Branch != "feat/x" {
		t.Fatalf("branch = %q, want feat/x", recents[0].Branch)
	}
}

// TestSanitizePreferences drops hostile values.
func TestSanitizePreferences(t *testing.T) {
	p := SanitizePreferences(Preferences{
		Theme:         "../../etc",
		DefaultStyle:  "evil",
		DefaultBudget: -5,
	})
	if p.Theme != "" || p.DefaultStyle != "" || p.DefaultBudget != 0 {
		t.Fatalf("unsanitized: %+v", p)
	}

	ok := SanitizePreferences(Preferences{
		Theme:         "catppuccin-mocha",
		DefaultStyle:  "xml",
		DefaultBudget: 64000,
	})
	if ok.Theme != "catppuccin-mocha" || ok.DefaultStyle != "xml" || ok.DefaultBudget != 64000 {
		t.Fatalf("valid prefs altered: %+v", ok)
	}
}
