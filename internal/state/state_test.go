package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetProjectKey(t *testing.T) {
	abs := filepath.Join("/home", "user", "code", "my-project")
	// Key is stable and cleans the path.
	if k1, k2 := GetProjectKey(abs), GetProjectKey(abs); k1 != k2 {
		t.Errorf("key not stable: %q vs %q", k1, k2)
	}
	if len(GetProjectKey(abs)) != 16 {
		t.Errorf("key length = %d, want 16", len(GetProjectKey(abs)))
	}
	if GetProjectKey("/a/b/c") == GetProjectKey("/a/b/d") {
		t.Errorf("distinct paths produced the same key")
	}
}

func TestLoadMissing(t *testing.T) {
	if err := os.Setenv("XDG_CONFIG_HOME", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	st, err := Load()
	if err != nil {
		t.Fatalf("Load() with no state file: %v", err)
	}
	if st == nil || len(st.Projects) != 0 {
		t.Errorf("expected empty state, got %+v", st)
	}
}

func TestRoundTrip(t *testing.T) {
	if err := os.Setenv("XDG_CONFIG_HOME", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	st, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	key := GetProjectKey("/some/abs/path")
	st.Record(key, "abc123", "feat: initial", 12, 3400)

	if err := st.Save(); err != nil {
		t.Fatalf("Save(): %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	rec, ok := got.Get(key)
	if !ok {
		t.Fatal("record missing after reload")
	}
	if rec.LastCommitHash != "abc123" || rec.LastCommitMsg != "feat: initial" {
		t.Errorf("record = %+v", rec)
	}
	if rec.FilesDumped != 12 || rec.TokensDumped != 3400 {
		t.Errorf("record counts = %d files, %d tokens", rec.FilesDumped, rec.TokensDumped)
	}
}

func TestCorruptState(t *testing.T) {
	dir := t.TempDir()
	if err := os.Setenv("XDG_CONFIG_HOME", dir); err != nil {
		t.Fatal(err)
	}
	path, err := StatePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{ not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Error("Load() should fail on corrupt state, got nil")
	}
}
