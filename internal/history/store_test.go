package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewIDFormatAndUTC(t *testing.T) {
	loc := time.FixedZone("UTC+3", 3*60*60)
	now := time.Date(2026, 11, 22, 14, 32, 12, 0, loc)
	id := NewID(now)

	if !strings.HasPrefix(id, "20261122T113212Z-") {
		t.Fatalf("NewID = %q, want UTC timestamp prefix", id)
	}
	if len(id) != len("20261122T113212Z-")+4 {
		t.Fatalf("NewID = %q, want four-character suffix", id)
	}
	for _, r := range id[len("20261122T113212Z-"):] {
		if !strings.ContainsRune("0123456789abcdef", r) {
			t.Fatalf("NewID suffix contains non-hex character %q", r)
		}
	}
}

func TestNewIDUniqueInSameSecond(t *testing.T) {
	now := time.Date(2026, 11, 22, 14, 32, 12, 0, time.UTC)
	seen := make(map[string]bool, 2000)
	for i := 0; i < 2000; i++ {
		id := NewID(now)
		if seen[id] {
			t.Fatalf("duplicate ID in same second: %q", id)
		}
		seen[id] = true
	}
}

func TestEntryJSONTags(t *testing.T) {
	entry := Entry{
		V: SchemaVersion, ID: "id", Ts: "2026-11-22T14:32:12Z", Cmd: "dump",
		Root: "/home/user/dev/sift", Name: "sift", File: "/tmp/dump.md",
		Bytes: 12, Tokens: 34, Files: 5, Style: "markdown",
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		`"v":1`, `"id":"id"`, `"ts":"2026-11-22T14:32:12Z"`, `"cmd":"dump"`,
		`"root":"/home/user/dev/sift"`, `"name":"sift"`, `"file":"/tmp/dump.md"`,
		`"bytes":12`, `"tokens":34`, `"files":5`, `"style":"markdown"`,
	} {
		if !strings.Contains(string(data), key) {
			t.Errorf("JSON %s missing %s", data, key)
		}
	}

	var decoded Entry
	if err := json.Unmarshal([]byte(`{"v":1,"id":"x","extra":"ignored"}`), &decoded); err != nil {
		t.Fatalf("unknown fields must be ignored: %v", err)
	}
	if decoded.ID != "x" {
		t.Fatalf("decoded ID = %q", decoded.ID)
	}
}

func TestArchiveName(t *testing.T) {
	when := time.Date(2026, 11, 22, 14, 32, 12, 0, time.UTC)
	tests := []struct {
		name string
		ext  string
		want string
	}{
		{"sift", "md", "sift-20261122-143212.md"},
		{"my project", "md", "my-project-20261122-143212.md"},
		{"a/b\\c", "md", "a-b-c-20261122-143212.md"},
		{"  spaced  ", "xml", "spaced-20261122-143212.xml"},
		{"", "json", "dump-20261122-143212.json"},
		{"/", "md", "root-20261122-143212.md"},
		{"sift", ".MD", "sift-20261122-143212.md"},
		{"sift", "", "sift-20261122-143212"},
	}
	for _, tc := range tests {
		t.Run(tc.name+"/"+tc.ext, func(t *testing.T) {
			if got := ArchiveName(tc.name, when, tc.ext); got != tc.want {
				t.Fatalf("ArchiveName(%q, %q) = %q, want %q", tc.name, tc.ext, got, tc.want)
			}
		})
	}
}

func TestArchiveNameUsesUTC(t *testing.T) {
	loc := time.FixedZone("UTC-5", -5*60*60)
	when := time.Date(2026, 11, 22, 9, 32, 12, 0, loc)
	got := ArchiveName("sift", when, "md")
	if got != "sift-20261122-143212.md" {
		t.Fatalf("ArchiveName = %q, want UTC timestamp", got)
	}
}

func TestStyleExtension(t *testing.T) {
	tests := map[string]string{
		"markdown": "md",
		"MD":       "md",
		"xml":      "xml",
		"json":     "json",
		"plain":    "txt",
		"":         "txt",
		"weird!":   "weird",
	}
	for style, want := range tests {
		if got := StyleExtension(style); got != want {
			t.Errorf("StyleExtension(%q) = %q, want %q", style, got, want)
		}
	}
}

func TestInsideDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(dir, "sub", "dump.md")
	if err := os.WriteFile(child, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret")
	if err := os.WriteFile(outsideFile, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	symlinkOutside := filepath.Join(dir, "link-out")
	if err := os.Symlink(outsideFile, symlinkOutside); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	symlinkInside := filepath.Join(dir, "link-in")
	if err := os.Symlink(child, symlinkInside); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"normal child", child, true},
		{"nested directory", filepath.Join(dir, "sub"), true},
		{"the directory itself", dir, false},
		{"dotdot traversal", filepath.Join(dir, "..", filepath.Base(outside), "secret"), false},
		{"symlink outside", symlinkOutside, false},
		{"symlink inside", symlinkInside, true},
		{"missing child inside", filepath.Join(dir, "sub", "missing.md"), true},
		{"empty path", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := insideDir(dir, tc.path); got != tc.want {
				t.Fatalf("insideDir(%q, %q) = %v, want %v", dir, tc.path, got, tc.want)
			}
		})
	}
}

func newTestStore(t *testing.T, keep int, now func() time.Time) *Store {
	t.Helper()
	base := t.TempDir()
	return &Store{
		DataDir:  filepath.Join(base, "data"),
		DumpsDir: filepath.Join(base, "dumps"),
		Keep:     keep,
		Now:      now,
	}
}

func fixedNow(when time.Time) func() time.Time {
	return func() time.Time { return when }
}

func mustRecord(t *testing.T, store *Store, root, cmd string, rendered []byte) Entry {
	t.Helper()
	entry, err := store.Record(root, cmd, "markdown", rendered, 12, 3)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	return entry
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.TrimSuffix(string(data), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func TestRecordCreatesArchiveAndHistory(t *testing.T) {
	store := newTestStore(t, 20, fixedNow(time.Date(2026, 11, 22, 14, 32, 12, 0, time.UTC)))
	root := filepath.Join(t.TempDir(), "my project")
	rendered := []byte("# dump\n")
	entry := mustRecord(t, store, root, "dump", rendered)

	if entry.V != SchemaVersion || entry.Cmd != "dump" || entry.Name != "my project" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if entry.Bytes != int64(len(rendered)) || entry.Tokens != 12 || entry.Files != 3 {
		t.Fatalf("unexpected metrics: %+v", entry)
	}
	if !strings.HasPrefix(filepath.Base(entry.File), "my-project-20261122-143212.md") {
		t.Fatalf("archive name = %q", entry.File)
	}
	got, err := os.ReadFile(entry.File)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(rendered) {
		t.Fatalf("archive contents = %q, want %q", got, rendered)
	}

	for _, path := range []string{store.DataDir, store.DumpsDir} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Errorf("%s mode = %o, want 700", path, info.Mode().Perm())
		}
	}
	for _, path := range []string{entry.File, store.HistoryPath()} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("%s mode = %o, want 600", path, info.Mode().Perm())
		}
	}
	if lines := readLines(t, store.HistoryPath()); len(lines) != 1 {
		t.Fatalf("history lines = %d, want 1", len(lines))
	}
}

func TestListNewestFirstAndMissingArchives(t *testing.T) {
	base := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	store := newTestStore(t, 20, fixedNow(base))
	entries := make([]Entry, 3)
	for i := 0; i < 3; i++ {
		store.Now = fixedNow(base.Add(time.Duration(i) * time.Minute))
		entries[i] = mustRecord(t, store, filepath.Join(t.TempDir(), "p"+string(rune('a'+i))), "dump", []byte("data"))
	}
	list, err := store.List(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[0].ID != entries[2].ID || list[2].ID != entries[0].ID {
		t.Fatalf("unexpected order: %+v", list)
	}
	if err := os.Remove(entries[1].File); err != nil {
		t.Fatal(err)
	}
	list, err = store.List(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != entries[2].ID || list[1].ID != entries[0].ID {
		t.Fatalf("missing archive not omitted: %+v", list)
	}
	if limited, _ := store.List(1); len(limited) != 1 || limited[0].ID != entries[2].ID {
		t.Fatalf("limit ignored: %+v", limited)
	}
}

func TestListSkipsMalformedUnknownVersionAndOutsidePaths(t *testing.T) {
	store := newTestStore(t, 20, fixedNow(time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)))
	valid := mustRecord(t, store, filepath.Join(t.TempDir(), "valid"), "dump", []byte("data"))
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	lines := []string{
		`not json`,
		`{"v":99,"id":"future","ts":"2026-11-22T10:00:01Z","file":"` + outside + `"}`,
		`{"v":1,"id":"outside","ts":"2026-11-22T10:00:02Z","file":"` + outside + `"}`,
		`{"v":1,"id":"missing","ts":"2026-11-22T10:00:03Z","file":"` + filepath.Join(store.DumpsDir, "missing.md") + `"}`,
	}
	data, err := os.ReadFile(store.HistoryPath())
	if err != nil {
		t.Fatal(err)
	}
	all := strings.TrimSuffix(string(data), "\n") + "\n" + strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(store.HistoryPath(), []byte(all), 0o600); err != nil {
		t.Fatal(err)
	}

	list, err := store.List(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != valid.ID {
		t.Fatalf("List = %+v, want only the valid entry", list)
	}
}

func TestKeepPrunesOldestAndOnlyInsideDumpsDir(t *testing.T) {
	base := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	store := newTestStore(t, 2, fixedNow(base))
	if err := os.MkdirAll(store.DataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(store.DumpsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	unreferenced := filepath.Join(store.DumpsDir, "unreferenced.md")
	if err := os.WriteFile(unreferenced, []byte("keep me too"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Seed an old record pointing outside the dumps directory. Pruning drops
	// it from the log but must not delete the outside file.
	old := Entry{
		V: SchemaVersion, ID: "20261122T090000Z-0001", Ts: "2026-11-22T09:00:00Z",
		Cmd: "dump", Root: "/old", Name: "old", File: outside, Style: "markdown",
	}
	oldLine, _ := json.Marshal(old)
	if err := os.WriteFile(store.HistoryPath(), append(oldLine, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	first := mustRecord(t, store, filepath.Join(t.TempDir(), "first"), "dump", []byte("one"))
	store.Now = fixedNow(base.Add(time.Minute))
	second := mustRecord(t, store, filepath.Join(t.TempDir(), "second"), "dump", []byte("two"))
	store.Now = fixedNow(base.Add(2 * time.Minute))
	third := mustRecord(t, store, filepath.Join(t.TempDir(), "third"), "dump", []byte("three"))

	list, err := store.List(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != third.ID || list[1].ID != second.ID {
		t.Fatalf("retention list = %+v", list)
	}
	if _, err := os.Stat(first.File); !os.IsNotExist(err) {
		t.Fatalf("pruned archive still exists: %v", err)
	}
	for _, file := range []string{second.File, third.File} {
		if _, err := os.Stat(file); err != nil {
			t.Fatalf("retained archive missing: %v", err)
		}
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside file was deleted: %v", err)
	}
	if _, err := os.Stat(unreferenced); err != nil {
		t.Fatalf("unreferenced file was deleted: %v", err)
	}
	if lines := readLines(t, store.HistoryPath()); len(lines) != 2 {
		t.Fatalf("history lines = %d, want 2", len(lines))
	}
}

func TestClearRemovesOnlyReferencedArchives(t *testing.T) {
	store := newTestStore(t, 20, fixedNow(time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)))
	first := mustRecord(t, store, filepath.Join(t.TempDir(), "first"), "dump", []byte("one"))
	second := mustRecord(t, store, filepath.Join(t.TempDir(), "second"), "dump", []byte("two"))
	unreferenced := filepath.Join(store.DumpsDir, "unreferenced.md")
	if err := os.WriteFile(unreferenced, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(store.DumpsDir, "link-outside.md")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	extra := Entry{
		V: SchemaVersion, ID: "20261122T090000Z-0002", Ts: "2026-11-22T09:00:00Z",
		Cmd: "dump", Root: "/x", Name: "x", File: link, Style: "markdown",
	}
	extraLine, _ := json.Marshal(extra)
	data, err := os.ReadFile(store.HistoryPath())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.HistoryPath(), append(data, append(extraLine, '\n')...), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := store.Clear(); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{first.File, second.File} {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Errorf("referenced archive still exists: %s", file)
		}
	}
	if _, err := os.Stat(store.HistoryPath()); !os.IsNotExist(err) {
		t.Errorf("history log still exists")
	}
	if _, err := os.Stat(unreferenced); err != nil {
		t.Errorf("unreferenced archive was deleted: %v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Errorf("outside target was deleted: %v", err)
	}
	if _, err := os.Stat(link); err != nil {
		t.Errorf("outside symlink should be left alone: %v", err)
	}
}

func TestClearMissingHistoryIsNotAnError(t *testing.T) {
	store := newTestStore(t, 20, fixedNow(time.Now()))
	if err := store.Clear(); err != nil {
		t.Fatalf("Clear on empty store: %v", err)
	}
}

func TestRecordHandlesArchiveNameCollisions(t *testing.T) {
	store := newTestStore(t, 20, fixedNow(time.Date(2026, 11, 22, 14, 32, 12, 0, time.UTC)))
	root := filepath.Join(t.TempDir(), "same")
	first := mustRecord(t, store, root, "dump", []byte("one"))
	second := mustRecord(t, store, root, "dump", []byte("two"))
	if first.File == second.File {
		t.Fatalf("archive collision: %q", first.File)
	}
	for _, file := range []string{first.File, second.File} {
		if _, err := os.Stat(file); err != nil {
			t.Fatalf("archive missing: %v", err)
		}
	}
}

func TestRecordConcurrent(t *testing.T) {
	store := newTestStore(t, 100, fixedNow(time.Date(2026, 11, 22, 14, 32, 12, 0, time.UTC)))
	root := filepath.Join(t.TempDir(), "concurrent")

	const n = 24
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := store.Record(root, "dump", "markdown", []byte("data"), i, 1)
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Record: %v", err)
		}
	}

	list, err := store.List(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != n {
		t.Fatalf("List count = %d, want %d", len(list), n)
	}
	if lines := readLines(t, store.HistoryPath()); len(lines) != n {
		t.Fatalf("history lines = %d, want %d", len(lines), n)
	}
}
