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
	// Tuning carries the optimizer's numeric knobs. The zero value means the
	// built-in defaults; config layers pass an explicit tuning to override.
	Tuning Tuning
}

// Tuning holds every numeric knob the utility optimizer reads. Values come
// from the config [scoring] section; the zero value resolves to the defaults
// below, so no tuning numbers are hardcoded in the DP itself.
type Tuning struct {
	BaseMin         float64
	BaseMax         float64
	RelevanceWeight float64
	PreferenceBonus float64
	SignatureBonus  float64
	SkipMultiplier  float64
	SigQualityMin   float64
	SigQualityMax   float64
	// RetentionFloor is the minimum retention value at or above which a file
	// is guaranteed a slot under budget (its best affordable variant is
	// reserved first). Files below the floor compete through the DP.
	RetentionFloor float64
	// Retention overrides the lang role defaults per role name (e.g.
	// "entrypoint"). Config takes priority over the lang default.
	Retention map[string]float64
}

// DefaultTuning returns the stock optimizer tuning.
func DefaultTuning() Tuning {
	return Tuning{
		BaseMin:         0.01,
		BaseMax:         1.0,
		RelevanceWeight: 0.20,
		PreferenceBonus: 0.05,
		SignatureBonus:  0.05,
		SkipMultiplier:  0.35,
		SigQualityMin:   0.35,
		SigQualityMax:   0.85,
		RetentionFloor:  0.15,
	}
}

// applyDefaults fills zero fields with the stock tuning.
func (t *Tuning) applyDefaults() {
	def := DefaultTuning()
	if t.BaseMin == 0 {
		t.BaseMin = def.BaseMin
	}
	if t.BaseMax == 0 {
		t.BaseMax = def.BaseMax
	}
	if t.RelevanceWeight == 0 {
		t.RelevanceWeight = def.RelevanceWeight
	}
	if t.PreferenceBonus == 0 {
		t.PreferenceBonus = def.PreferenceBonus
	}
	if t.SignatureBonus == 0 {
		t.SignatureBonus = def.SignatureBonus
	}
	if t.SkipMultiplier == 0 {
		t.SkipMultiplier = def.SkipMultiplier
	}
	if t.SigQualityMin == 0 {
		t.SigQualityMin = def.SigQualityMin
	}
	if t.SigQualityMax == 0 {
		t.SigQualityMax = def.SigQualityMax
	}
	if t.RetentionFloor == 0 {
		t.RetentionFloor = def.RetentionFloor
	}
}

// retentionFor returns the effective retention priority for a candidate,
// applying the config override on top of the lang role default.
func (t Tuning) retentionFor(candidate Candidate) float64 {
	role := string(lang.Classify(candidate.File.Path).Role)
	if v, ok := t.Retention[role]; ok {
		return v
	}
	return lang.Classify(candidate.File.Path).Retention
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
// established score/filter modes. With a budget it reserves guaranteed slots
// for high-retention files (entrypoints, docs, config) and uses sparse dynamic
// programming for the rest, choosing among full, signature, and skip variants
// to maximize total utility rather than simply taking files in score order.
func Select(candidates []Candidate, request Request) Result {
	request.Tuning.applyDefaults()
	tuning := request.Tuning

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

	// Reserve guaranteed slots for high-retention files before the DP so an
	// important entrypoint or README is never traded away for cheap filler.
	guaranteed := make([]bool, len(ordered))
	remaining := request.Budget
	for index, candidate := range ordered {
		if candidate.File.Hidden || candidate.File.GitIgnored {
			continue
		}
		if tuning.retentionFor(candidate) >= tuning.RetentionFloor {
			if variant := bestAffordable(candidate, remaining, tuning, request.Prompt); variant.Tokens > 0 {
				guaranteed[index] = true
				remaining -= variant.Tokens
			}
		}
	}

	limit := request.MaxStates
	if limit <= 0 {
		limit = defaultMaxStates
	}
	states := map[int]dpState{0: {}}
	for index, candidate := range ordered {
		if guaranteed[index] {
			continue
		}
		next := make(map[int]dpState, len(states))
		for tokens, state := range states {
			keepState(next, tokens, state)
			for _, variant := range variants(candidate, request.Prompt, tuning) {
				newTokens := tokens + variant.Tokens
				if newTokens > remaining {
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
	used := 0
	for index, candidate := range ordered {
		if candidate.File.Hidden || candidate.File.GitIgnored {
			result.Decisions = append(result.Decisions, makeDecision(candidate, ModeSkip, 0, "hidden or git-ignored; opt-in required", request.Prompt))
			continue
		}
		if guaranteed[index] {
			variant := bestAffordable(candidate, request.Budget-used, tuning, request.Prompt)
			decision := makeDecision(candidate, variant.Mode, variant.Utility, variant.Reason, request.Prompt)
			decision.Selected = true
			decision.Tokens = variant.Tokens
			result.Decisions = append(result.Decisions, decision)
			result.Selected = append(result.Selected, finalize(candidate.File, variant.Mode, variant.Tokens))
			result.UsedTokens += variant.Tokens
			used += variant.Tokens
			continue
		}
		mode, ok := chosen[index]
		if !ok {
			result.Decisions = append(result.Decisions, makeDecision(candidate, ModeSkip, 0, "not selected by utility optimizer", request.Prompt))
			continue
		}
		variant := findVariant(candidate, mode, request.Prompt, tuning)
		decision := makeDecision(candidate, mode, variant.Utility, variant.Reason, request.Prompt)
		decision.Selected = true
		decision.Tokens = variant.Tokens
		result.Decisions = append(result.Decisions, decision)
		result.Selected = append(result.Selected, finalize(candidate.File, mode, variant.Tokens))
		result.UsedTokens += variant.Tokens
	}
	return result
}

// bestAffordable returns the highest-utility variant that fits within budget,
// or a zero-token skip variant when nothing fits. It prefers the most useful
// representation of a file that the remaining budget can still pay for.
func bestAffordable(candidate Candidate, budget int, tuning Tuning, prompt string) Variant {
	var best Variant
	for _, v := range variants(candidate, prompt, tuning) {
		if v.Tokens == 0 {
			continue
		}
		if v.Tokens > budget {
			continue
		}
		if v.Utility > best.Utility {
			best = v
		}
	}
	return best
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
		variant := findVariant(candidate, mode, prompt, DefaultTuning())
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

func variants(candidate Candidate, prompt string, tuning Tuning) []Variant {
	file := candidate.File
	if file.Hidden || file.GitIgnored {
		// Hidden and Git-ignored entries are opt-in only: the optimizer never
		// offers full or signature variants for them, so they can never be
		// auto-selected or counted toward the token budget.
		return []Variant{{Mode: ModeSkip, Tokens: 0, Utility: 0, Reason: "hidden or git-ignored; opt-in required"}}
	}
	classification := lang.Classify(file.Path)
	relevance := taskRelevance(file.Path, prompt)
	base := clamp(file.RankScore+classification.Adjustment+tuning.RelevanceWeight*relevance, tuning.BaseMin, tuning.BaseMax)
	preferenceBonus := 0.0
	switch Mode(candidate.PreferredMode) {
	case ModeFull, ModeSignatures:
		preferenceBonus = tuning.PreferenceBonus
	case ModeSkip:
		// A skip hint is soft. The file can still be selected if its other
		// signals make it useful, but it starts with lower utility.
		base *= tuning.SkipMultiplier
	}

	fullUtility := base * (1.0 + preferenceBonus)
	result := []Variant{{Mode: ModeFull, Tokens: file.TokensFull, Utility: fullUtility, Reason: "full content; " + classification.Reason}}
	if file.TokensSig > 0 && file.TokensSig < file.TokensFull {
		ratio := float64(file.TokensSig) / float64(file.TokensFull)
		quality := clamp(tuning.SigQualityMin+(tuning.SigQualityMax-tuning.SigQualityMin)*(1.0-ratio), tuning.SigQualityMin, tuning.SigQualityMax)
		sigUtility := base * quality
		if Mode(candidate.PreferredMode) == ModeSignatures {
			sigUtility *= tuning.SignatureBonus
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
		return ModeFull, "score band preference: full; " + classification.Reason
	case ModeSignatures:
		return ModeSignatures, "score band preference: signatures; " + classification.Reason
	case ModeSkip:
		return ModeSkip, "low relevance background file; " + classification.Reason
	}
	if candidate.File.TokensSig > 0 && candidate.File.TokensSig < candidate.File.TokensFull {
		return ModeSignatures, "signature representation saves tokens; " + classification.Reason
	}
	return ModeFull, "full content selected; " + classification.Reason
}

func findVariant(candidate Candidate, mode Mode, prompt string, tuning Tuning) Variant {
	for _, variant := range variants(candidate, prompt, tuning) {
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
