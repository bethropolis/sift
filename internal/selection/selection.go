// Package selection chooses the most useful representation of collected files
// under a token budget. It is shared by the non-interactive CLI and the TUI.
package selection

import (
	"sort"

	"github.com/bethropolis/sift/internal/format"
)

// Mode is the output representation selected for a file.
type Mode string

const (
	ModeFull       Mode = "full"
	ModeSignatures Mode = "signatures"
	ModeSkip       Mode = "skip"
)

// Request controls automatic selection.
type Request struct {
	Budget int
}

// Candidate is a collected file plus the mode preference supplied by history
// or another caller.
type Candidate struct {
	File          format.FileEntry
	PreferredMode string
}

// Decision explains the automatic decision for one candidate.
type Decision struct {
	Path            string  `json:"path"`
	Mode            Mode    `json:"mode"`
	Selected        bool    `json:"selected"`
	Score           float64 `json:"score"`
	Tokens          int     `json:"tokens"`
	FullTokens      int     `json:"full_tokens"`
	SignatureTokens int     `json:"signature_tokens"`
	Reason          string  `json:"reason"`
}

// Result contains every decision and the finalized files selected for output.
type Result struct {
	Decisions  []Decision
	Selected   []format.FileEntry
	UsedTokens int
	Budget     int
}

// Select chooses a mode for every file and greedily fills the budget in score
// order. It does not mutate the input slice. A zero budget means unlimited.
func Select(candidates []Candidate, request Request) Result {
	ordered := append([]Candidate(nil), candidates...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].File.RankScore != ordered[j].File.RankScore {
			return ordered[i].File.RankScore > ordered[j].File.RankScore
		}
		return ordered[i].File.Path < ordered[j].File.Path
	})

	result := Result{Budget: request.Budget, Decisions: make([]Decision, 0, len(ordered))}
	for _, candidate := range ordered {
		file := candidate.File
		mode, reason := plannedMode(candidate)
		decision := Decision{
			Path:            file.Path,
			Mode:            mode,
			Score:           file.RankScore,
			FullTokens:      file.TokensFull,
			SignatureTokens: file.TokensSig,
			Reason:          reason,
		}

		if mode == ModeSkip {
			result.Decisions = append(result.Decisions, decision)
			continue
		}

		tokens := file.TokensFull
		if mode == ModeSignatures {
			tokens = file.TokensSig
		}
		if request.Budget > 0 && result.UsedTokens+tokens > request.Budget {
			decision.Mode = ModeSkip
			decision.Reason = "exceeds remaining token budget"
			result.Decisions = append(result.Decisions, decision)
			continue
		}

		decision.Selected = true
		decision.Tokens = tokens
		result.UsedTokens += tokens
		result.Decisions = append(result.Decisions, decision)
		result.Selected = append(result.Selected, finalize(file, mode, tokens))
	}
	return result
}

func plannedMode(candidate Candidate) (Mode, string) {
	file := candidate.File
	switch Mode(candidate.PreferredMode) {
	case ModeFull:
		return ModeFull, "history preference: full"
	case ModeSignatures:
		return ModeSignatures, "history preference: signatures"
	case ModeSkip:
		return ModeSkip, "low relevance background file"
	}
	if file.SigContent != nil && len(file.SigContent) > 0 && file.TokensSig < file.TokensFull {
		return ModeSignatures, "signature representation saves tokens"
	}
	return ModeFull, "full content selected"
}

func finalize(file format.FileEntry, mode Mode, tokens int) format.FileEntry {
	file.Tokens = tokens
	switch mode {
	case ModeSignatures:
		if file.SigContent != nil {
			file.Content = file.SigContent
			file.IsCompressed = true
		}
	case ModeFull:
		file.IsCompressed = false
	}
	return file
}
