package rank

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestUnifiedScoresNonGit exercises the pure scoring logic where git is
// unavailable: recency and churn fall to baseline/zero, so band decisions come
// from role modifiers, centrality, and the compression-yield overrides.
func TestUnifiedScoresNonGit(t *testing.T) {
	dir := t.TempDir()
	g := New(dir)
	if g.Available() {
		t.Fatal("temp dir unexpectedly inside a git repo")
	}

	params := []ScoringParams{
		{Path: "main.go", TokensFull: 100, TokensSig: 20, DidCompress: true},                 // entrypoint -> boosted above skip
		{Path: "cmd/app/main.go", TokensFull: 100, TokensSig: 20, DidCompress: true},         // cmd/ entrypoint
		{Path: "types.go", TokensFull: 100, TokensSig: 20, DidCompress: true, FanInCount: 5}, // high fan-in
		{Path: "lib_test.go", TokensFull: 100, TokensSig: 20, DidCompress: true},             // test -> clamped low
		{Path: "mock_server.go", TokensFull: 100, TokensSig: 20, DidCompress: true},          // mock -> clamped low
		{Path: "config.go", TokensFull: 100, TokensSig: 98, DidCompress: true},               // poor yield -> forced full
		{Path: "binary.dat", TokensFull: 100, TokensSig: 100, DidCompress: false},            // no compression -> full
		{Path: "plain.go", TokensFull: 100, TokensSig: 25, DidCompress: true},                // neutral baseline -> skip
	}

	results := g.CalculateUnifiedScores(dir, params)
	if len(results) != len(params) {
		t.Fatalf("got %d results, want %d", len(results), len(params))
	}

	expect := map[string]string{
		"main.go":         "signatures", // entrypoint boost clears skip band
		"cmd/app/main.go": "signatures", // cmd/ entrypoint boost
		"types.go":        "signatures", // high fan-in clears skip band
		"lib_test.go":     "skip",       // test role clamped to low
		"mock_server.go":  "skip",       // mock role clamped to low
		"config.go":       "full",       // poor yield forces full
		"binary.dat":      "full",       // no compression forces full
		"plain.go":        "skip",       // neutral baseline
	}
	for path, want := range expect {
		res, ok := results[path]
		if !ok {
			t.Errorf("missing result for %s", path)
			continue
		}
		if res.Score < 0 || res.Score > 1 {
			t.Errorf("%s: score %v out of [0,1]", path, res.Score)
		}
		if res.PreferredMode != want {
			t.Errorf("%s: mode = %q, want %q (score %v, reason %s)", path, res.PreferredMode, want, res.Score, res.Reason)
		}
	}

	// High fan-in keeps a stable file out of skip despite baseline recency.
	if res := results["types.go"]; res.Score < scoreSkipBand {
		t.Errorf("types.go score %v < skip band %v despite fan-in", res.Score, scoreSkipBand)
	}
}

// TestUnifiedScoresGitRepo confirms the composite score can reach the FULL band
// when a file is both dirty and frequently committed, and that quiet files
// stay low even in an active repo.
func TestUnifiedScoresGitRepo(t *testing.T) {
	dir := initRepo(t)
	g := New(dir)

	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// quiet.txt is committed once early on, then never touched again.
	write("quiet.txt", "q")
	gitCmd(t, dir, "add", "quiet.txt")
	gitCmd(t, dir, "commit", "-q", "-m", "add quiet")

	// Touch hot.txt across 8 commits so it dominates churn.
	for i := 0; i < 8; i++ {
		write("hot.txt", fmt.Sprintf("h%d", i))
		gitCmd(t, dir, "add", "hot.txt")
		gitCmd(t, dir, "commit", "-q", "-m", "touch hot")
	}
	// A single focused commit edits a.txt so it is the most recent diff.
	write("a.txt", "a-second")
	gitCmd(t, dir, "add", "a.txt")
	gitCmd(t, dir, "commit", "-q", "-m", "touch a")
	write("hot.txt", "h-dirty")

	params := []ScoringParams{
		{Path: "hot.txt", TokensFull: 100, TokensSig: 10, DidCompress: true},
		{Path: "a.txt", TokensFull: 100, TokensSig: 10, DidCompress: true},
		{Path: "quiet.txt", TokensFull: 100, TokensSig: 10, DidCompress: true},
	}

	results := g.CalculateUnifiedScores(dir, params)

	if res := results["hot.txt"]; res.Score < scoreFullBand || res.PreferredMode != "full" {
		t.Errorf("hot.txt = %+v, want score >= %v and mode full (dirty + high churn)", res, scoreFullBand)
	}
	// a.txt was the latest diff (0.32) with a little churn: mid band.
	if res := results["a.txt"]; res.PreferredMode != "signatures" {
		t.Errorf("a.txt = %+v, want signatures", res)
	}
	// quiet.txt has no recent git signal: low band.
	if res := results["quiet.txt"]; res.PreferredMode != "skip" {
		t.Errorf("quiet.txt = %+v, want skip", res)
	}
}

func TestGetChurnFrequency(t *testing.T) {
	dir := initRepo(t)
	g := New(dir)

	for i := 0; i < 4; i++ {
		if err := os.WriteFile(filepath.Join(dir, "churned.go"), []byte(fmt.Sprintf("c%d", i)), 0o644); err != nil {
			t.Fatal(err)
		}
		gitCmd(t, dir, "add", "churned.go")
		gitCmd(t, dir, "commit", "-q", "-m", "churn")
	}

	churn := g.GetChurnFrequency(30)
	if churn["churned.go"] != 4 {
		t.Errorf("churned.go count = %d, want 4", churn["churned.go"])
	}
	if churn["a.txt"] != 1 {
		t.Errorf("a.txt count = %d, want 1 (initial commit)", churn["a.txt"])
	}
	if churn["missing.go"] != 0 {
		t.Errorf("missing.go count = %d, want 0", churn["missing.go"])
	}
}

func TestGetChurnFrequencyNotAGitRepo(t *testing.T) {
	dir := t.TempDir()
	g := New(dir)
	if churn := g.GetChurnFrequency(30); len(churn) != 0 {
		t.Errorf("GetChurnFrequency = %v, want empty map", churn)
	}
}
