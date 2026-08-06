// Package selection chooses the most useful representation of collected files
// under a token budget. It is shared by the non-interactive CLI and the TUI.
package selection

import (
	"sort"
	"strings"
	"unicode"

	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/lang"
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
	Budget    int
	MaxStates int    // zero uses the default sparse-DP state limit.
	Prompt    string // optional task text used to improve path relevance.
}

// Signals are optional ranking inputs retained for explainable reports.
type Signals struct {
	Recency    float64 `json:"recency,omitempty"`
	Churn      float64 `json:"churn,omitempty"`
	Centrality float64 `json:"centrality,omitempty"`
	Role       float64 `json:"role,omitempty"`
}

// Candidate is a collected file plus the mode preference supplied by history
// or another caller.
type Candidate struct {
	File          format.FileEntry
	PreferredMode string
	Signals       Signals
}

// Decision explains the automatic decision for one candidate.
type Decision struct {
	Path            string  `json:"path"`
	Mode            Mode    `json:"mode"`
	Selected        bool    `json:"selected"`
	Score           float64 `json:"score"`
	Utility         float64 `json:"utility"`
	Tokens          int     `json:"tokens"`
	FullTokens      int     `json:"full_tokens"`
	SignatureTokens int     `json:"signature_tokens"`
	Role            string  `json:"role,omitempty"`
	TaskRelevance   float64 `json:"task_relevance,omitempty"`
	Signals         Signals `json:"signals,omitempty"`
	Reason          string  `json:"reason"`
}

// Result contains every decision and the finalized files selected for output.
type Result struct {
	Decisions  []Decision
	Selected   []format.FileEntry
	UsedTokens int
	Budget     int
}

// Variant is one representation choice for a candidate.
type Variant struct {
	Mode    Mode
	Tokens  int
	Utility float64
	Reason  string
}

type choice struct {
	Index int
	Mode  Mode
}

type dpState struct {
	Utility float64
	Choices []choice
}

const defaultMaxStates = 50000

// Select chooses a mode for every file. With no budget it preserves the
// established history/filter modes. With a budget it uses sparse dynamic
// programming to choose among full, signature, and skip variants, maximizing
// total utility rather than simply taking files in score order.
func Select(candidates []Candidate, request Request) Result {
	ordered := append([]Candidate(nil), candidates...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].File.RankScore != ordered[j].File.RankScore {
			return ordered[i].File.RankScore > ordered[j].File.RankScore
		}
		return ordered[i].File.Path < ordered[j].File.Path
	})

	result := Result{Budget: request.Budget, Decisions: make([]Decision, 0, len(ordered))}
	if request.Budget <= 0 {
		return selectUnlimited(ordered, result, request.Prompt)
	}

	limit := request.MaxStates
	if limit <= 0 {
		limit = defaultMaxStates
	}
	states := map[int]dpState{0: {}}
	for index, candidate := range ordered {
		next := make(map[int]dpState, len(states))
		for tokens, state := range states {
			keepState(next, tokens, state)
			for _, variant := range variants(candidate, request.Prompt) {
				newTokens := tokens + variant.Tokens
				if newTokens > request.Budget {
					continue
				}
				choices := append(append([]choice(nil), state.Choices...), choice{Index: index, Mode: variant.Mode})
				keepState(next, newTokens, dpState{
					Utility: state.Utility + variant.Utility,
					Choices: choices,
				})
			}
		}
		states = pruneStates(next, limit)
	}

	_, best := bestState(states)
	chosen := make(map[int]Mode, len(best.Choices))
	for _, choice := range best.Choices {
		chosen[choice.Index] = choice.Mode
	}
	for index, candidate := range ordered {
		if candidate.File.Hidden || candidate.File.GitIgnored {
			result.Decisions = append(result.Decisions, makeDecision(candidate, ModeSkip, 0, "hidden or git-ignored; opt-in required", request.Prompt))
			continue
		}
		mode, ok := chosen[index]
		if !ok {
			result.Decisions = append(result.Decisions, makeDecision(candidate, ModeSkip, 0, "not selected by utility optimizer", request.Prompt))
			continue
		}
		variant := findVariant(candidate, mode, request.Prompt)
		decision := makeDecision(candidate, mode, variant.Utility, variant.Reason, request.Prompt)
		decision.Selected = true
		decision.Tokens = variant.Tokens
		result.Decisions = append(result.Decisions, decision)
		result.Selected = append(result.Selected, finalize(candidate.File, mode, variant.Tokens))
		result.UsedTokens += variant.Tokens
	}
	return result
}

func selectUnlimited(candidates []Candidate, result Result, prompt string) Result {
	for _, candidate := range candidates {
		if candidate.File.Hidden || candidate.File.GitIgnored {
			result.Decisions = append(result.Decisions, makeDecision(candidate, ModeSkip, 0, "hidden or git-ignored; opt-in required", prompt))
			continue
		}
		mode, reason := plannedMode(candidate)
		if mode == ModeSkip {
			// A ranker's skip preference is only a budget signal. With no
			// budget, retain useful source roles as signatures. Tests, fixtures,
			// mocks, generated files, and vendored code remain legitimately
			// skippable even when there is no budget limit.
			classification := lang.Classify(candidate.File.Path)
			if retainWithoutBudget(classification.Role) && candidate.File.TokensSig > 0 && candidate.File.TokensSig < candidate.File.TokensFull {
				mode = ModeSignatures
				reason = "background file retained; signature representation"
			}
			if mode == ModeSkip {
				result.Decisions = append(result.Decisions, makeDecision(candidate, mode, 0, reason, prompt))
				continue
			}
		}
		variant := findVariant(candidate, mode, prompt)
		decision := makeDecision(candidate, mode, variant.Utility, reason, prompt)
		decision.Selected = true
		decision.Tokens = variant.Tokens
		result.Decisions = append(result.Decisions, decision)
		result.Selected = append(result.Selected, finalize(candidate.File, mode, variant.Tokens))
		result.UsedTokens += variant.Tokens
	}
	return result
}

func retainWithoutBudget(role lang.Role) bool {
	switch role {
	case lang.RoleTest, lang.RoleFixture, lang.RoleMock,
		lang.RoleGenerated, lang.RoleVendor:
		return false
	default:
		return true
	}
}

func makeDecision(candidate Candidate, mode Mode, utility float64, reason, prompt string) Decision {
	classification := lang.Classify(candidate.File.Path)
	relevance := taskRelevance(candidate.File.Path, prompt)
	return Decision{
		Path:            candidate.File.Path,
		Mode:            mode,
		Score:           candidate.File.RankScore,
		Utility:         utility,
		FullTokens:      candidate.File.TokensFull,
		SignatureTokens: candidate.File.TokensSig,
		Role:            string(classification.Role),
		TaskRelevance:   relevance,
		Signals:         candidate.Signals,
		Reason:          reason,
	}
}

func variants(candidate Candidate, prompt string) []Variant {
	file := candidate.File
	if file.Hidden || file.GitIgnored {
		// Hidden and Git-ignored entries are opt-in only: the optimizer never
		// offers full or signature variants for them, so they can never be
		// auto-selected or counted toward the token budget.
		return []Variant{{Mode: ModeSkip, Tokens: 0, Utility: 0, Reason: "hidden or git-ignored; opt-in required"}}
	}
	classification := lang.Classify(file.Path)
	relevance := taskRelevance(file.Path, prompt)
	base := clamp(file.RankScore+classification.Adjustment+0.20*relevance, 0.01, 1.0)
	preferenceBonus := 0.0
	switch Mode(candidate.PreferredMode) {
	case ModeFull, ModeSignatures:
		preferenceBonus = 0.05
	case ModeSkip:
		// A skip hint is soft. The file can still be selected if its other
		// signals make it useful, but it starts with lower utility.
		base *= 0.35
	}

	fullUtility := base * (1.0 + preferenceBonus)
	result := []Variant{{Mode: ModeFull, Tokens: file.TokensFull, Utility: fullUtility, Reason: "full content; " + classification.Reason}}
	if file.TokensSig > 0 && file.TokensSig < file.TokensFull {
		ratio := float64(file.TokensSig) / float64(file.TokensFull)
		quality := clamp(0.35+0.5*(1.0-ratio), 0.35, 0.85)
		sigUtility := base * quality
		if Mode(candidate.PreferredMode) == ModeSignatures {
			sigUtility *= 1.05
		}
		result = append(result, Variant{
			Mode:    ModeSignatures,
			Tokens:  file.TokensSig,
			Utility: sigUtility,
			Reason:  "signature view; " + classification.Reason,
		})
	}
	return result
}

func taskRelevance(path, prompt string) float64 {
	terms := words(prompt)
	if len(terms) == 0 {
		return 0
	}
	pathWords := words(path)
	hits := 0
	for _, term := range terms {
		for _, part := range pathWords {
			if part == term || strings.Contains(part, term) || strings.Contains(term, part) {
				hits++
				break
			}
		}
	}
	return float64(hits) / float64(len(terms))
}

func words(value string) []string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return ' '
	}, value)
	parts := strings.Fields(value)
	filtered := parts[:0]
	for _, part := range parts {
		if len(part) >= 3 {
			filtered = append(filtered, part)
		}
	}
	return filtered
}

func plannedMode(candidate Candidate) (Mode, string) {
	classification := lang.Classify(candidate.File.Path)
	switch Mode(candidate.PreferredMode) {
	case ModeFull:
		return ModeFull, "history preference: full; " + classification.Reason
	case ModeSignatures:
		return ModeSignatures, "history preference: signatures; " + classification.Reason
	case ModeSkip:
		return ModeSkip, "low relevance background file; " + classification.Reason
	}
	if candidate.File.TokensSig > 0 && candidate.File.TokensSig < candidate.File.TokensFull {
		return ModeSignatures, "signature representation saves tokens; " + classification.Reason
	}
	return ModeFull, "full content selected; " + classification.Reason
}

func findVariant(candidate Candidate, mode Mode, prompt string) Variant {
	for _, variant := range variants(candidate, prompt) {
		if variant.Mode == mode {
			return variant
		}
	}
	return Variant{Mode: ModeFull, Tokens: candidate.File.TokensFull, Utility: 0, Reason: "full content fallback"}
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

func keepState(states map[int]dpState, tokens int, candidate dpState) {
	current, ok := states[tokens]
	if !ok || betterState(candidate, current) {
		states[tokens] = candidate
	}
}

func betterState(a, b dpState) bool {
	if a.Utility != b.Utility {
		return a.Utility > b.Utility
	}
	return len(a.Choices) > len(b.Choices)
}

func pruneStates(states map[int]dpState, limit int) map[int]dpState {
	// Remove dominated states: a state using fewer tokens and providing at
	// least as much utility can always replace the dominated state.
	tokens := make([]int, 0, len(states))
	for token := range states {
		tokens = append(tokens, token)
	}
	sort.Ints(tokens)
	pruned := make(map[int]dpState, len(states))
	bestUtility := -1.0
	for _, token := range tokens {
		state := states[token]
		if state.Utility > bestUtility {
			pruned[token] = state
			bestUtility = state.Utility
		}
	}
	if len(pruned) <= limit {
		return pruned
	}
	keys := make([]int, 0, len(pruned))
	for token := range pruned {
		keys = append(keys, token)
	}
	sort.Slice(keys, func(i, j int) bool {
		return pruned[keys[i]].Utility > pruned[keys[j]].Utility
	})
	limited := make(map[int]dpState, limit)
	for _, token := range keys[:limit] {
		limited[token] = pruned[token]
	}
	return limited
}

func bestState(states map[int]dpState) (int, dpState) {
	bestTokens := 0
	best := states[0]
	for tokens, state := range states {
		if betterState(state, best) || (state.Utility == best.Utility && tokens < bestTokens) {
			bestTokens, best = tokens, state
		}
	}
	return bestTokens, best
}

func clamp(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
