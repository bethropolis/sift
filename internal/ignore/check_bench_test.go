package ignore

import (
	"strconv"
	"testing"
)

// benchPaths are ordinary, mostly-non-matching repository paths; the ignore
// pipeline runs in full for each.
var benchPaths = []string{
	"internal/app/collect.go",
	"cmd/sift/main.go",
	"docs/assets/preview.png",
	"src/components/Button.tsx",
	"deeply/nested/tree/structure/module.rs",
	"scripts/build/helper.sh",
}

// BenchmarkShouldIgnore measures the full per-path decision: hidden and .git
// segment scans, the repository tier, and the default tier.
func BenchmarkShouldIgnore(b *testing.B) {
	m, err := New(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.ShouldIgnore(benchPaths[i%len(benchPaths)], false)
	}
}

// BenchmarkShouldIgnoreDefaultTierOnly isolates the built-in default pattern
// tier by disabling the repository rules, so the fast classifier's win is
// visible without the library's ancestor walk dominating the sample.
func BenchmarkShouldIgnoreDefaultTierOnly(b *testing.B) {
	m, err := New(b.TempDir(), WithGitIgnore(false))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.ShouldIgnore(benchPaths[i%len(benchPaths)], false)
	}
}

// BenchmarkShouldIgnoreColdUnique measures the honest per-path cost on a real
// walk: every iteration uses a path never seen before, so neither the repo
// memo nor the library's directory cache can help. This is the number that
// governs the traversal pass.
func BenchmarkShouldIgnoreColdUnique(b *testing.B) {
	m, err := New(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Deep, unique paths: the library's repo tier walks every ancestor.
		rel := "pkg/mod" + strconv.Itoa(i) + "/sub/dir/module" + strconv.Itoa(i) + ".rs"
		m.ShouldIgnore(rel, false)
	}
}

// BenchmarkShouldIgnoreColdUniqueDefaultTier isolates the default tier for
// unique paths, so the fast classifier's own win is measured without the
// repository tier.
func BenchmarkShouldIgnoreColdUniqueDefaultTier(b *testing.B) {
	m, err := New(b.TempDir(), WithGitIgnore(false))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rel := "pkg/mod" + strconv.Itoa(i) + "/sub/dir/module" + strconv.Itoa(i) + ".rs"
		m.ShouldIgnore(rel, false)
	}
}

// BenchmarkShouldIgnoreDefaultTierLibrary is the baseline for the fast
// classifier: the same decision with the fast tier disabled and the complete
// default pattern list evaluated by the library's fnmatch loop. The delta
// against BenchmarkShouldIgnoreColdUniqueDefaultTier is the classifier's win.
func BenchmarkShouldIgnoreDefaultTierLibrary(b *testing.B) {
	m := newReferenceMatcher(b, b.TempDir(), true, false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rel := "pkg/mod" + strconv.Itoa(i) + "/sub/dir/module" + strconv.Itoa(i) + ".rs"
		m.ShouldIgnore(rel, false)
	}
}

// BenchmarkClassifyVisibility measures the picker's metadata enquiry, which
// shares the memoized repository tier with ShouldIgnore.
func BenchmarkClassifyVisibility(b *testing.B) {
	m, err := New(b.TempDir(), WithHiddenIgnore(false), WithGitIgnore(false))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.ClassifyVisibility(benchPaths[i%len(benchPaths)], false)
	}
}
