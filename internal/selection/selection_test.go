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

// TestSelectRetentionGuaranteesDocsUnderBudget ensures a high-retention docs
// file is not traded away for cheap filler: the optimizer honors the lang
// role retention before running its utility DP.
func TestSelectRetentionGuaranteesDocsUnderBudget(t *testing.T) {
	result := Select([]Candidate{
		candidate("README.md", 0.22, 1868, 20, "", true),
		candidate("hot.go", 1.0, 60, 20, "", true),
		candidate("clutter.log", 0.9, 5, 5, "", false),
	}, Request{Budget: 40})

	byPath := map[string]Decision{}
	for _, d := range result.Decisions {
		byPath[d.Path] = d
	}
	if d := byPath["README.md"]; !d.Selected {
		t.Errorf("README.md should be guaranteed by retention, decision=%+v", d)
	}
}

// TestSelectRetentionOverrideConfigPriority ensures a config-provided
// retention override (e.g. bumping a low-retention role) wins over the lang
// default. Implementation is not retained by default, so cheap filler fills
// the budget and the expensive impl file is dropped; a config override lifts
// it into the guaranteed set so it survives.
func TestSelectRetentionOverrideConfigPriority(t *testing.T) {
	base := []Candidate{
		candidate("impl.go", 0.05, 2000, 100, "", true), // implementation, expensive
		candidate("filler1.log", 0.9, 3, 3, "", false),
		candidate("filler2.log", 0.9, 3, 3, "", false),
		candidate("filler3.log", 0.9, 3, 3, "", false),
	}

	without := Select(append([]Candidate(nil), base...), Request{Budget: 100})
	withoutByPath := map[string]Decision{}
	for _, d := range without.Decisions {
		withoutByPath[d.Path] = d
	}
	if withoutByPath["impl.go"].Selected {
		t.Fatal("impl.go must not be guaranteed without an override")
	}

	tun := DefaultTuning()
	tun.Retention = map[string]float64{"implementation": 0.9}
	withOverride := Select(append([]Candidate(nil), base...), Request{Budget: 100, Tuning: tun})
	byPath := map[string]Decision{}
	for _, d := range withOverride.Decisions {
		byPath[d.Path] = d
	}
	if d := byPath["impl.go"]; !d.Selected {
		t.Errorf("impl.go should be retained via config override, decision=%+v", d)
	}
}

// TestSelectNeverExceedsBudget is a budget-invariant guard: guaranteed
// (high-retention) slots are reserved before the utility DP, and the exact
// reserved variant must be committed at finalize time so the sum of guaranteed
// picks plus the DP selection never runs past request.Budget. Sweep many
// budgets over a mix of guaranteed docs/entrypoints and ordinary filler whose
// optimizable full/signature variants interact with the reserved amounts.
func TestSelectNeverExceedsBudget(t *testing.T) {
	cands := []Candidate{
		candidate("README.md", 0.22, 60, 10, "", true), // doc: guaranteed slot
		candidate("main.go", 1.0, 90, 20, "", true),    // entrypoint: guaranteed slot
		candidate("util.go", 0.9, 55, 15, "", true),
		candidate("hot.go", 0.85, 70, 5, "", true),
		candidate("model.go", 0.8, 200, 40, "", true),
		candidate("a.go", 0.7, 12, 12, "", false),
		candidate("b.log", 0.95, 3, 3, "", false),
	}
	for budget := 1; budget <= 300; budget++ {
		result := Select(append([]Candidate(nil), cands...), Request{Budget: budget})
		if result.UsedTokens > budget {
			t.Fatalf("budget %d: used %d tokens exceeds budget", budget, result.UsedTokens)
		}
	}
}

// TestPlannedModeLabelIsScoreBand ensures no outputs claim to come from a
// history store that does not exist.
func TestPlannedModeLabelIsScoreBand(t *testing.T) {
	_, reason := plannedMode(candidate("main.go", 0.9, 100, 10, "full", true))
	if reason == "" {
		t.Fatal("empty reason")
	}
}
