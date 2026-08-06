package selection

import (
	"testing"

	"github.com/bethropolis/sift/internal/format"
)

func candidate(path string, score float64, full, sig int, preferred string, hasSig bool) Candidate {
	file := format.FileEntry{
		Path:       path,
		Content:    []byte("full"),
		TokensFull: full,
		TokensSig:  sig,
		RankScore:  score,
	}
	if hasSig {
		file.SigContent = []byte("sig")
	}
	return Candidate{File: file, PreferredMode: preferred}
}

func hiddenCandidate(path string, hidden, gitIgnored bool) Candidate {
	file := format.FileEntry{
		Path:       path,
		Content:    []byte("full"),
		SigContent: []byte("sig"),
		TokensFull: 50,
		TokensSig:  10,
		RankScore:  1.0,
		Hidden:     hidden,
		GitIgnored: gitIgnored,
	}
	return Candidate{File: file, PreferredMode: "full"}
}

func TestSelectSkipsHiddenAndGitIgnored(t *testing.T) {
	result := Select([]Candidate{
		hiddenCandidate(".config/settings", true, false),
		hiddenCandidate("ignored.log", false, true),
		candidate("app.go", 0.5, 20, 5, "", true),
	}, Request{Budget: 100})

	if len(result.Selected) != 1 || result.Selected[0].Path != "app.go" {
		t.Fatalf("selected = %+v, want only app.go", result.Selected)
	}
	if result.UsedTokens != 20 {
		t.Errorf("used = %d, want 20 (hidden/ignored excluded)", result.UsedTokens)
	}
	byPath := map[string]Decision{}
	for _, d := range result.Decisions {
		byPath[d.Path] = d
	}
	for _, path := range []string{".config/settings", "ignored.log"} {
		if d := byPath[path]; d.Mode != ModeSkip || d.Selected {
			t.Errorf("%s decision = %+v, want skipped and unselected", path, d)
		}
	}
}

func TestSelectUnlimitedSkipsHiddenAndGitIgnored(t *testing.T) {
	result := Select([]Candidate{
		hiddenCandidate(".config/settings", true, false),
		hiddenCandidate("ignored.log", false, true),
	}, Request{})

	if len(result.Selected) != 0 {
		t.Fatalf("selected %d files, want 0", len(result.Selected))
	}
	for _, d := range result.Decisions {
		if d.Mode != ModeSkip || d.Selected {
			t.Errorf("%s decision = %+v, want skipped and unselected", d.Path, d)
		}
	}
}

func TestSelectUsesModesAndBudget(t *testing.T) {
	result := Select([]Candidate{
		candidate("low.go", 0.1, 20, 5, "skip", true),
		candidate("hot.go", 1.0, 100, 10, "signatures", true),
		candidate("small.go", 0.5, 25, 25, "", false),
	}, Request{Budget: 35})

	if len(result.Selected) != 2 {
		t.Fatalf("selected %d files, want 2", len(result.Selected))
	}
	if result.UsedTokens != 35 {
		t.Fatalf("used %d tokens, want 35", result.UsedTokens)
	}
	byPath := map[string]Decision{}
	for _, d := range result.Decisions {
		byPath[d.Path] = d
	}
	if d := byPath["low.go"]; d.Mode != ModeSkip || d.Selected {
		t.Errorf("low.go decision = %+v, want skipped", d)
	}
	if d := byPath["hot.go"]; d.Mode != ModeSignatures || !d.Selected || d.Tokens != 10 {
		t.Errorf("hot.go decision = %+v, want selected signatures at 10 tokens", d)
	}
	if d := byPath["small.go"]; d.Mode != ModeFull || !d.Selected || d.Tokens != 25 {
		t.Errorf("small.go decision = %+v, want selected full at 25 tokens", d)
	}
}

func TestSelectDoesNotMutateInput(t *testing.T) {
	files := []Candidate{candidate("a.go", 1, 10, 5, "signatures", true)}
	original := files[0].File.Content
	result := Select(files, Request{})
	if string(files[0].File.Content) != string(original) {
		t.Error("input candidate was mutated")
	}
	if len(result.Selected) != 1 || string(result.Selected[0].Content) != "sig" {
		t.Errorf("selected file was not finalized to signatures: %+v", result.Selected)
	}
}

func TestUnlimitedSelectionTreatsSkipAsSoftPreference(t *testing.T) {
	result := Select([]Candidate{
		candidate("implementation.go", 0.05, 100, 20, "skip", true),
	}, Request{})
	if len(result.Selected) != 1 {
		t.Fatalf("selected %d files, want 1", len(result.Selected))
	}
	if result.Decisions[0].Mode != ModeSignatures {
		t.Fatalf("mode = %q, want signatures", result.Decisions[0].Mode)
	}
}

func TestUnlimitedSelectionKeepsTestFilesSkippable(t *testing.T) {
	result := Select([]Candidate{
		candidate("implementation_test.go", 0.05, 100, 20, "skip", true),
	}, Request{})
	if len(result.Selected) != 0 || result.Decisions[0].Mode != ModeSkip {
		t.Fatalf("test decision = %+v, want skipped", result.Decisions[0])
	}
}

func TestSelectBeatsGreedyFullFile(t *testing.T) {
	result := Select([]Candidate{
		candidate("large.go", 1.0, 100, 60, "", true),
		candidate("related.go", 0.8, 40, 20, "", true),
	}, Request{Budget: 100})

	if len(result.Selected) != 2 || result.UsedTokens != 100 {
		t.Fatalf("selected %d files using %d tokens, want two files using 100", len(result.Selected), result.UsedTokens)
	}
	byPath := map[string]Decision{}
	for _, d := range result.Decisions {
		byPath[d.Path] = d
	}
	if byPath["large.go"].Mode != ModeSignatures {
		t.Errorf("large.go mode = %q, want signatures to make room for related.go", byPath["large.go"].Mode)
	}
	if !byPath["related.go"].Selected {
		t.Error("related.go was not selected by the optimized combination")
	}
}
