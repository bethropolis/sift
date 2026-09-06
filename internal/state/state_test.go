package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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

func TestCanonicalRootUnifiesSpellings(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	abs, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, spelling := range []string{".", abs, filepath.Join(abs, "sub", "..")} {
		got, err := CanonicalRoot(spelling)
		if err != nil {
			t.Fatalf("CanonicalRoot(%q): %v", spelling, err)
		}
		if GetProjectKey(got) != GetProjectKey(abs) {
			t.Errorf("CanonicalRoot(%q) = %q, key mismatch with %q", spelling, got, abs)
		}
	}
}

func TestMutateStateConcurrent(t *testing.T) {
	if err := os.Setenv("XDG_CONFIG_HOME", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	// Every writer records distinct keys. A lost read-modify-write (two
	// transactions interleaving Load then Save) would silently drop keys,
	// so the final count must be exact.
	const writers = 8
	const perWriter = 25
	var wg sync.WaitGroup
	errs := make(chan error, writers*perWriter)
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				key := GetProjectKey(fmt.Sprintf("/proj/w%02d/i%02d", w, i))
				if err := MutateState(func(st *AppState) error {
					st.Record(key, "hash", "msg", 1, 100)
					return nil
				}); err != nil {
					errs <- err
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("MutateState: %v", err)
	}
	st, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Projects) != writers*perWriter {
		t.Fatalf("records = %d, want %d (lost update)", len(st.Projects), writers*perWriter)
	}
	for key, rec := range st.Projects {
		if rec.LastCommitHash != "hash" || rec.FilesDumped != 1 || rec.TokensDumped != 100 {
			t.Errorf("corrupt record for %q: %+v", key, rec)
		}
	}
}

func TestMutateStateFnErrorSkipsSave(t *testing.T) {
	if err := os.Setenv("XDG_CONFIG_HOME", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	key := GetProjectKey("/proj/rollback")
	if err := MutateState(func(st *AppState) error {
		st.Record(key, "hash", "msg", 1, 100)
		return errors.New("boom")
	}); err == nil {
		t.Fatal("MutateState swallowed fn error")
	}
	st, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := st.Get(key); ok {
		t.Error("failed transaction was persisted")
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
