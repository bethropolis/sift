package walker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/bethropolis/sift/internal/ignore"
)

func writeTestTree(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"a.txt":           "alpha",
		"b.txt":           "bravo",
		"sub/c.md":        "charlie",
		".secret.txt":     "hidden",
		"sub/.inner/d.go": "delta",
	}
	for path, content := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func collectWalk(t *testing.T, root string, opts ...Option) []string {
	t.Helper()
	matcher, err := ignore.New(root)
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var files []string
	walkFn := func(relativePath string, content []byte, err error) error {
		mu.Lock()
		defer mu.Unlock()
		files = append(files, relativePath)
		return nil
	}

	if _, err := Walk(root, matcher, walkFn, opts...); err != nil {
		t.Fatalf("Walk returned error: %v", err)
	}
	sort.Strings(files)
	return files
}

func TestWalkSequential(t *testing.T) {
	root := t.TempDir()
	writeTestTree(t, root)

	got := collectWalk(t, root)
	want := []string{"a.txt", "b.txt", "sub/c.md"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

func TestWalkConcurrentMatchesSequential(t *testing.T) {
	root := t.TempDir()
	writeTestTree(t, root)

	sequential := collectWalk(t, root)
	concurrent := collectWalk(t, root,
		WithConcurrency(true),
		WithMaxWorkers(4),
	)

	if len(sequential) != len(concurrent) {
		t.Fatalf("sequential %v != concurrent %v", sequential, concurrent)
	}
	for i := range sequential {
		if sequential[i] != concurrent[i] {
			t.Errorf("sequential[%d]=%q, concurrent[%d]=%q", i, sequential[i], i, concurrent[i])
		}
	}
}

func TestWalkExtensionFilter(t *testing.T) {
	root := t.TempDir()
	writeTestTree(t, root)

	got := collectWalk(t, root, WithExtensions([]string{"txt"}))
	want := []string{"a.txt", "b.txt"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

func TestWalkPreReadFilterSkipsBeforeCallback(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bundle.min.js"), []byte("generated"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := collectWalk(t, root, WithPreReadFilter(func(path string) bool {
		return filepath.Base(path) == "bundle.min.js"
	}))
	if len(got) != 1 || got[0] != "keep.txt" {
		t.Fatalf("files = %v, want [keep.txt]", got)
	}
}

func TestWalkStatsIncludeReadAndSkipCounts(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skip.min.js"), []byte("generated"), 0o644); err != nil {
		t.Fatal(err)
	}

	matcher, err := ignore.New(root)
	if err != nil {
		t.Fatal(err)
	}
	var got WalkStats
	_, err = Walk(root, matcher, func(string, []byte, error) error { return nil },
		WithPreReadFilter(func(path string) bool { return filepath.Base(path) == "skip.min.js" }),
		WithStats(func(stats WalkStats) { got = stats }))
	if err != nil {
		t.Fatal(err)
	}
	if got.ProcessedFiles != 1 || got.SmartSkipped != 1 || got.BytesRead != 4 {
		t.Fatalf("stats = %+v, want one read and one smart skip", got)
	}
}

func TestWalkContextCancellation(t *testing.T) {
	root := t.TempDir()
	writeTestTree(t, root)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	matcher, err := ignore.New(root)
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	count := 0
	walkFn := func(relativePath string, content []byte, err error) error {
		mu.Lock()
		defer mu.Unlock()
		count++
		return nil
	}

	if _, err := Walk(root, matcher, walkFn, WithConcurrency(true), WithContext(ctx)); err == nil {
		t.Error("expected error from cancelled context, got nil")
	}
	if count != 0 {
		t.Errorf("processed %d files with cancelled context, want 0", count)
	}
}

func TestWalkBinaryFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a file with a NUL byte in the first 512 bytes.
	if err := os.WriteFile(filepath.Join(root, "data.bin"), []byte{0x00, 0x01, 0x02}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ok.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Binary files are skipped by default.
	got := collectWalk(t, root)
	if len(got) != 1 || got[0] != "ok.txt" {
		t.Errorf("got %v, want [ok.txt]", got)
	}

	// With WithIncludeBinary they are included.
	got = collectWalk(t, root, WithIncludeBinary(true))
	if len(got) != 2 {
		t.Errorf("got %v, want [data.bin ok.txt]", got)
	}
}

// TestWalkOversizedFileNotProcessed verifies an oversized file is recorded as
// a size-limit skip without invoking the content callback, so the app never
// logs an expected policy skip as a processing warning.
func TestWalkOversizedFileNotProcessed(t *testing.T) {
	root := t.TempDir()
	big := filepath.Join(root, "big.iso")
	if err := os.WriteFile(big, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ok.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	matcher, err := ignore.New(root)
	if err != nil {
		t.Fatal(err)
	}

	callbacks := 0
	skipped, err := Walk(root, matcher, func(relativePath string, content []byte, cbErr error) error {
		if cbErr != nil {
			return cbErr
		}
		callbacks++
		return nil
	}, WithMaxFileSize(4))
	if err != nil {
		t.Fatalf("Walk returned error: %v", err)
	}

	if callbacks != 1 {
		t.Errorf("content callback invoked %d times, want 1 (ok.txt only)", callbacks)
	}
	var found bool
	for _, s := range skipped {
		if s.Path == "big.iso" {
			found = true
			if s.Reason != ReasonSkippedSizeLimit {
				t.Errorf("reason = %q, want %q", s.Reason, ReasonSkippedSizeLimit)
			}
		}
	}
	if !found {
		t.Errorf("big.iso not recorded as a size-limit skip: %+v", skipped)
	}
}

// TestWalkMetaSizeFilter verifies the metadata pass drops oversized files so
// the picker skeleton never advertises files the content walk will reject.
func TestWalkMetaSizeFilter(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "big.txt"), []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "small.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	matcher, err := ignore.New(root)
	if err != nil {
		t.Fatal(err)
	}
	metas, skipped, err := WalkMeta(root, matcher, WithMaxFileSize(4))
	if err != nil {
		t.Fatalf("WalkMeta returned error: %v", err)
	}

	if len(metas) != 1 || metas[0].Path != "small.txt" {
		t.Errorf("metas = %v, want [small.txt]", metas)
	}
	var found bool
	for _, s := range skipped {
		if s.Path == "big.txt" {
			found = true
			if s.Reason != ReasonSkippedSizeLimit {
				t.Errorf("reason = %q, want %q", s.Reason, ReasonSkippedSizeLimit)
			}
		}
	}
	if !found {
		t.Errorf("big.txt not in skipped items: %+v", skipped)
	}
}

// TestWalkMetaBinaryFilter verifies the metadata pass drops binary files by
// default and includes them with WithIncludeBinary(true).
func TestWalkMetaBinaryFilter(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "data.iso"), []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ok.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	matcher, err := ignore.New(root)
	if err != nil {
		t.Fatal(err)
	}

	metas, skipped, err := WalkMeta(root, matcher)
	if err != nil {
		t.Fatalf("WalkMeta returned error: %v", err)
	}
	if len(metas) != 1 || metas[0].Path != "ok.txt" {
		t.Errorf("metas = %v, want [ok.txt]", metas)
	}
	var found bool
	for _, s := range skipped {
		if s.Path == "data.iso" {
			found = true
			if s.Reason != ReasonSkippedBinary {
				t.Errorf("reason = %q, want %q", s.Reason, ReasonSkippedBinary)
			}
		}
	}
	if !found {
		t.Errorf("data.iso not in skipped items: %+v", skipped)
	}

	metas, _, err = WalkMeta(root, matcher, WithIncludeBinary(true))
	if err != nil {
		t.Fatalf("WalkMeta(include binary) returned error: %v", err)
	}
	if len(metas) != 2 {
		t.Errorf("metas with binary included = %v, want 2", metas)
	}
}

func TestIsBinaryFile(t *testing.T) {
	dir := t.TempDir()
	// Extensionless text file: sniffed via magic numbers.
	makefile := filepath.Join(dir, "Makefile")
	if err := os.WriteFile(makefile, []byte("all:\n\tgo build\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Extensionless binary file: NUL-prefixed, sniffed as non-text.
	data := filepath.Join(dir, "data")
	if err := os.WriteFile(data, []byte{0x00, 0x01, 0x02}, 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "known text ext", path: filepath.Join(dir, "main.go"), want: false},
		{name: "known text ext uppercase", path: filepath.Join(dir, "README.TXT"), want: false},
		{name: "known binary ext", path: filepath.Join(dir, "img.png"), want: true},
		{name: "known binary ext uppercase", path: filepath.Join(dir, "ARCHIVE.ZIP"), want: true},
		{name: "extensionless text", path: makefile, want: false},
		{name: "extensionless binary", path: data, want: true},
		{name: "missing file", path: filepath.Join(dir, "nope"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsBinaryFile(tt.path); got != tt.want {
				t.Errorf("IsBinaryFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestWalkPrunesIgnoredHeavyDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "dep.js"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A heavy dir that is also covered by ignore rules is pruned at entry.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := collectWalk(t, root)
	want := []string{"a.txt"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

func TestWalkKeepsUnignoredHeavyDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "vendor"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "vendor", "dep.go"), []byte("package dep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A heavy dir that is excluded by the built-in default ignore patterns but
	// re-included by a repository rule must still be walked: repo rules (and
	// their negations) take precedence over the default fallback.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("!vendor/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := collectWalk(t, root)
	want := []string{"a.txt", "vendor/dep.go"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

func TestWalkDefaultIgnorePrunes(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "dep.js"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "debug.log"), []byte("log"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	// No .gitignore and no --ignore: files matching the built-in default
	// patterns are still pruned.
	got := collectWalk(t, root)
	want := []string{"main.go"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

func TestWalkPathFilter(t *testing.T) {
	root := t.TempDir()
	writeTestTree(t, root)

	excluded := map[string]bool{"b.txt": true, "sub/c.md": true}
	var mu sync.Mutex
	var walked []string
	walkFn := func(relativePath string, content []byte, err error) error {
		mu.Lock()
		defer mu.Unlock()
		walked = append(walked, relativePath)
		return nil
	}

	matcher, err := ignore.New(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Walk(root, matcher, walkFn, WithPathFilter(func(rel string) bool {
		return !excluded[rel]
	})); err != nil {
		t.Fatalf("Walk returned error: %v", err)
	}

	if len(walked) != 1 || walked[0] != "a.txt" {
		t.Fatalf("walked %v, want [a.txt]", walked)
	}
}

func TestWalkPathFilterAppliesToConcurrent(t *testing.T) {
	root := t.TempDir()
	writeTestTree(t, root)

	excluded := map[string]bool{"b.txt": true}
	var mu sync.Mutex
	var walked []string
	walkFn := func(relativePath string, content []byte, err error) error {
		mu.Lock()
		defer mu.Unlock()
		walked = append(walked, relativePath)
		return nil
	}

	matcher, err := ignore.New(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Walk(root, matcher, walkFn, WithConcurrency(true), WithMaxWorkers(4),
		WithPathFilter(func(rel string) bool { return !excluded[rel] })); err != nil {
		t.Fatalf("Walk returned error: %v", err)
	}
	sort.Strings(walked)
	want := []string{"a.txt", "sub/c.md"}
	if len(walked) != len(want) {
		t.Fatalf("walked %v, want %v", walked, want)
	}
	for i := range want {
		if walked[i] != want[i] {
			t.Errorf("walked %v, want %v", walked, want)
		}
	}
}

func TestWalkPrunesNewHeavyDirs(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"Pods", "DerivedData", "site-packages"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, dir, "gen.js"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("Pods/\nDerivedData/\nsite-packages/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := collectWalk(t, root)
	want := []string{"a.txt"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestWalkKeepsUnignoredNewHeavyDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Pods"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Pods", "dep.swift"), []byte("// dep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	// No ignore rule covers Pods, so it is still walked.
	got := collectWalk(t, root)
	want := []string{"Pods/dep.swift", "a.txt"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

// TestWalkCallbackErrorAbortsSequential verifies a non-nil error from the
// WalkFunc aborts traversal in the sequential path so the caller's error is
// surfaced and later files are not processed.
func TestWalkCallbackErrorAbortsSequential(t *testing.T) {
	root := t.TempDir()
	writeTestTree(t, root)

	matcher, err := ignore.New(root)
	if err != nil {
		t.Fatal(err)
	}

	var count int
	sentinel := errors.New("stop")
	_, err = Walk(root, matcher, func(string, []byte, error) error {
		count++
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("Walk error = %v, want the callback's sentinel", err)
	}
	if count == 0 {
		t.Fatal("callback was never invoked")
	}
	if count >= 3 {
		t.Errorf("callback invoked %d times, want traversal aborted after the first file", count)
	}
}

// TestWalkCallbackErrorAbortsConcurrent verifies a callback error aborts a
// concurrent walk and surfaces as the returned error, rather than being logged
// and ignored while the walk completes.
func TestWalkCallbackErrorAbortsConcurrent(t *testing.T) {
	root := t.TempDir()
	writeTestTree(t, root)

	matcher, err := ignore.New(root)
	if err != nil {
		t.Fatal(err)
	}

	sentinel := errors.New("stop now")
	_, err = Walk(root, matcher, func(string, []byte, error) error {
		return sentinel
	}, WithConcurrency(true), WithMaxWorkers(4))
	if !errors.Is(err, sentinel) {
		t.Fatalf("Walk error = %v, want the callback's sentinel", err)
	}
}

// TestWalkStatErrorCountsSkipped verifies a file that cannot be stat'd (a
// dangling symlink) is counted as a skipped file, so Processed + Skipped
// reconciles with TotalFiles instead of silently dropping the file from stats.
func TestWalkStatErrorCountsSkipped(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ok.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "does-not-exist"), filepath.Join(root, "dangling.dat")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	matcher, err := ignore.New(root)
	if err != nil {
		t.Fatal(err)
	}
	var got WalkStats
	if _, err := Walk(root, matcher, func(string, []byte, error) error { return nil },
		WithStats(func(st WalkStats) { got = st })); err != nil {
		t.Fatalf("Walk returned an unexpected error: %v", err)
	}
	if got.TotalFiles != 2 {
		t.Fatalf("TotalFiles = %d, want 2 (ok.txt + dangling symlink)", got.TotalFiles)
	}
	if got.ProcessedFiles != 1 {
		t.Errorf("ProcessedFiles = %d, want 1", got.ProcessedFiles)
	}
	if got.SkippedFiles != 1 {
		t.Errorf("SkippedFiles = %d, want 1 (stat error must be counted)", got.SkippedFiles)
	}
	if got.ProcessedFiles+got.SkippedFiles != got.TotalFiles {
		t.Errorf("Processed(%d)+Skipped(%d) != Total(%d); stats do not reconcile",
			got.ProcessedFiles, got.SkippedFiles, got.TotalFiles)
	}
}

// TestSkippedTrackerCap verifies the tracker truncates beyond its retention cap
// and reports how many events were dropped.
func TestSkippedTrackerCap(t *testing.T) {
	st := &SkippedTracker{maxItems: 3, items: make([]SkippedItem, 0, 3)}
	for i := 0; i < 10; i++ {
		st.Track("f", ReasonSkippedReadError, false)
	}
	if got := len(st.Items()); got != 3 {
		t.Fatalf("Items() len = %d, want 3 (capped)", got)
	}
	if got := st.Dropped(); got != 7 {
		t.Errorf("Dropped() = %d, want 7 over the cap", got)
	}
}

func TestWalkFollowsSymlinkFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "real.txt"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "real.txt"), filepath.Join(root, "link.txt")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	// The always-stat change must not regress symlinked files: they are
	// followed and still walked.
	got := collectWalk(t, root)
	want := []string{"link.txt", "real.txt"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
