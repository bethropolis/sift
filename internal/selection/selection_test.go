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
