package app

import (
	"testing"

	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/rank"
)

// compressible is a picker-style entry with a signature variant.
func compressible(path string) format.FileEntry {
	return format.FileEntry{
		Path:         path,
		TokensFull:   100,
		TokensSig:    20,
		SigContent:   []byte("sig"),
		IsCompressed: false,
	}
}

// TestRankerNonGit verifies the adapter works outside a git repository: every
// entry is ranked, scores stay within [0,1], and no git is required.
func TestRankerNonGit(t *testing.T) {
	dir := t.TempDir()
	r := NewRanker(dir)
	if r.Available() {
		t.Fatal("temp dir unexpectedly inside a git repo")
	}

	files := []format.FileEntry{
		compressible("a.go"),
		compressible("main.go"), // entrypoint boost
	}
	results := r.Rank(files)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if files[0].Path != "main.go" {
		t.Errorf("top-ranked = %q, want main.go (entrypoint boost)", files[0].Path)
	}
	for _, f := range files {
		if f.RankScore < 0 || f.RankScore > 1 {
			t.Errorf("%s: RankScore %v out of [0,1]", f.Path, f.RankScore)
		}
	}
}

// TestRankerDuplicatePaths verifies duplicate paths collapse to a single
// scoring result that is written back to every matching entry.
func TestRankerDuplicatePaths(t *testing.T) {
	dir := t.TempDir()
	r := NewRanker(dir)

	files := []format.FileEntry{
		compressible("dup.go"),
		compressible("dup.go"),
		compressible("other.go"),
	}
	results := r.Rank(files)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 (duplicate paths collapse)", len(results))
	}
	if files[0].RankScore != files[1].RankScore {
		t.Errorf("duplicate entries got different scores: %v vs %v", files[0].RankScore, files[1].RankScore)
	}
	if files[2].RankScore != results["other.go"].Score {
		t.Errorf("other.go RankScore %v does not match result %v", files[2].RankScore, results["other.go"].Score)
	}
}

// TestRankerStableOrderForEqualScores verifies the stable sort keeps the
// original order when scores tie (all non-git plain files get the same
// baseline), while a boosted entrypoint moves ahead.
func TestRankerStableOrderForEqualScores(t *testing.T) {
	dir := t.TempDir()
	r := NewRanker(dir)

	files := []format.FileEntry{
		compressible("c.go"),
		compressible("main.go"),
		compressible("a.go"),
		compressible("b.go"),
	}
	r.Rank(files)

	want := []string{"main.go", "c.go", "a.go", "b.go"}
	for i, w := range want {
		if files[i].Path != w {
			t.Fatalf("order = %v, want %v", paths(files), want)
		}
	}
	// The three plain files must all have the same baseline score.
	if files[1].RankScore != files[2].RankScore || files[2].RankScore != files[3].RankScore {
		t.Errorf("tied files differ in score: %v, %v, %v", files[1].RankScore, files[2].RankScore, files[3].RankScore)
	}
}

// TestRankerYieldDecision verifies the compression-yield override surfaces
// through the adapter: a file whose signatures save nothing ranks as full.
func TestRankerYieldDecision(t *testing.T) {
	dir := t.TempDir()
	r := NewRanker(dir)

	files := []format.FileEntry{
		{Path: "config.go", TokensFull: 100, TokensSig: 98, SigContent: []byte("sig"), IsCompressed: false},
		{Path: "good.go", TokensFull: 100, TokensSig: 20, SigContent: []byte("sig"), IsCompressed: false},
	}
	results := r.Rank(files)
	if res := results["config.go"]; res.PreferredMode != "full" {
		t.Errorf("config.go mode = %q, want full (poor compression yield)", res.PreferredMode)
	}
	if res := results["good.go"]; res.PreferredMode == "full" {
		t.Errorf("good.go mode = %q, want signatures (good yield)", res.PreferredMode)
	}
}

// TestApplyRankScoresBaseline verifies picker entries outside a git
// repository get the rank baseline score.
func TestApplyRankScoresBaseline(t *testing.T) {
	dir := t.TempDir()
	files := []format.FileEntry{compressible("a.go")}
	a := &App{}
	a.applyRank(collectPicker, dir, &files)
	if files[0].RankScore != rank.ScoreBaseline {
		t.Errorf("RankScore = %v, want baseline %v", files[0].RankScore, rank.ScoreBaseline)
	}
}

// TestComputeFanIn verifies the reverse import fan-in trie: every file in an
// imported package gains one centrality hit per importer, files under nested
// subpackages inherit it, and files outside any imported package get zero.
// This locks in the behavior of the O(files × imports) trie rewrite against
// the previous O(files × imports × files) implementation.
func TestComputeFanIn(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/repo\n")

	r := NewRanker(dir)
	files := []format.FileEntry{
		{Path: "main.go", Content: []byte("package main\nimport \"example.com/repo/internal/app\"\n")},
		{Path: "cmd/tool/main.go", Content: []byte("package main\nimport \"example.com/repo/internal/app\"\n")},
		{Path: "internal/app/app.go", Content: []byte("package app\n")},
		{Path: "internal/app/store.go", Content: []byte("package app\n")},
		{Path: "internal/app/sub/extra.go", Content: []byte("package sub\n")},
		{Path: "other.go", Content: []byte("package main\n")},
	}
	fanIn := r.computeFanIn(files)

	want := map[string]int{
		"main.go":                   0,
		"cmd/tool/main.go":          0,
		"internal/app/app.go":       2, // imported by main.go and cmd/tool
		"internal/app/store.go":     2,
		"internal/app/sub/extra.go": 2, // inherits from the internal/app package node
		"other.go":                  0,
	}
	for path, wantCount := range want {
		if got := fanIn[path]; got != wantCount {
			t.Errorf("fanIn[%q] = %d, want %d", path, got, wantCount)
		}
	}
}

func paths(files []format.FileEntry) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Path
	}
	return out
}
