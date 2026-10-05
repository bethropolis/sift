// Package selection chooses the most useful representation of collected files
// under a token budget. It is shared by the non-interactive CLI and the TUI.
package selection

import (
	"path"
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
	// TestTaskBoost raises test-file utility when the prompt is test-focused.
	TestTaskBoost float64
	// RetentionFloor is the minimum retention value at or above which a file
	// is guaranteed a slot under budget (its best affordable variant is
	// reserved first). Files below the floor compete through the DP.
	RetentionFloor float64
	// AreaDiminishing reduces utility for later files in the same area.
	AreaDiminishing float64
	// AreaBudgetShare is the default maximum fraction of the total budget one
	// area may consume when several distinct areas are present.
	AreaBudgetShare float64
	// AreaRoots maps directory prefixes to explicit area names. Longest
	// matching prefix wins; this lets repositories override inferred areas.
	AreaRoots map[string]string
	// Retention overrides the lang role defaults per role name (e.g.
	// "entrypoint"). Config takes priority over the lang default.
	Retention map[string]float64
	// SkipRoles are role names that automatic selection excludes before
	// optimization. An empty list uses the conservative built-in defaults.
	SkipRoles []string
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
		TestTaskBoost:   0.20,
		RetentionFloor:  0.15,
		AreaDiminishing: 0.65,
		AreaBudgetShare: 0.35,
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
	if t.TestTaskBoost == 0 {
		t.TestTaskBoost = def.TestTaskBoost
	}
	if t.RetentionFloor == 0 {
		t.RetentionFloor = def.RetentionFloor
	}
	if t.AreaDiminishing == 0 {
		t.AreaDiminishing = def.AreaDiminishing
	}
	if t.AreaBudgetShare == 0 {
		t.AreaBudgetShare = def.AreaBudgetShare
	}
}

// retentionFor returns the effective retention priority for a candidate,
// applying the config override on top of the lang role default.
func (t Tuning) retentionFor(candidate Candidate) float64 {
	classification := lang.Classify(candidate.File.Path)
	role := string(classification.Role)
	if v, ok := t.Retention[role]; ok {
		return v
	}
	// Protect recently changed implementation files from losing their slot
	// to stable background context under a tight budget.
	if classification.Role == lang.RoleImpl && candidate.File.RankScore >= 0.40 {
		return 0.16
	}
	return classification.Retention
}

func (t Tuning) skipRole(role lang.Role, prompt string) bool {
	if len(t.SkipRoles) == 0 {
		if role == lang.RoleTest && testFocusedPrompt(prompt) {
			return false
		}
		switch role {
		case lang.RoleTest, lang.RoleFixture, lang.RoleMock, lang.RoleGenerated, lang.RoleVendor:
			return true
		default:
			return false
		}
	}
	for _, name := range t.SkipRoles {
		if strings.EqualFold(strings.TrimSpace(name), string(role)) {
			return true
		}
	}
	return false
}

// Signals are optional ranking inputs retained for explainable reports.
type Signals struct {
	Recency      float64 `json:"recency,omitempty"`
	Churn        float64 `json:"churn,omitempty"`
	Centrality   float64 `json:"centrality,omitempty"`
	Role         float64 `json:"role,omitempty"`
	TestAffinity float64 `json:"test_affinity,omitempty"`
}

// Candidate is a collected file plus the mode preference supplied by history
// or another caller.
type Candidate struct {
	File          format.FileEntry
	PreferredMode string
	Signals       Signals
	AreaWeight    float64
	Area          string
}

// Decision explains the automatic decision for one candidate.
type Decision struct {
	Path            string  `json:"path"`
	Area            string  `json:"area,omitempty"`
	Mode            Mode    `json:"mode"`
	Selected        bool    `json:"selected"`
	Score           float64 `json:"score"`
	Utility         float64 `json:"utility"`
	Tokens          int     `json:"tokens"`
	FullTokens      int     `json:"full_tokens"`
	SignatureTokens int     `json:"signature_tokens"`
	Role            string  `json:"role,omitempty"`
	RoleConfidence  float64 `json:"role_confidence,omitempty"`
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

// choiceNode is one node in a persistent, shared-tail linked list of selected
// choices. Each DP transition allocates a single node pointing back at its
// parent state's tail, so the per-state choice history is never deep-copied.
// This turns the previous O(states × files) allocation blow-up into O(1) per
// transition, which matters at the default 50k-state sparse DP.
type choiceNode struct {
	choice
	prev *choiceNode
}

type dpState struct {
	Utility float64
	count   int // number of choices in the chain (for tie-breaking)
	last    *choiceNode
}

const defaultMaxStates = 50000

// areaOf maps paths to stable architectural areas. The first two components
// identify a package or subsystem; root-level files share the root area.
func areaOf(filePath string) string {
	parts := strings.Split(strings.Trim(filePath, "/"), "/")
	if len(parts) < 2 || parts[0] == "" {
		return "."
	}
	// A file directly under a top-level directory belongs to that directory,
	// not to a one-file area named after its own basename.
	if len(parts) == 2 && path.Ext(parts[1]) != "" {
		return parts[0]
	}
	return parts[0] + "/" + parts[1]
}

func candidateArea(candidate Candidate) string {
	if candidate.Area != "" {
		return candidate.Area
	}
	return areaOf(candidate.File.Path)
}

func areaMultiplier(position int, laterWeight float64) float64 {
	if position <= 1 {
		return 1
	}
	if laterWeight <= 0 || laterWeight >= 1 {
		laterWeight = DefaultTuning().AreaDiminishing
	}
	weight := laterWeight
	for i := 2; i < position && weight > 0.20; i++ {
		weight *= laterWeight
	}
	if weight < 0.20 {
		return 0.20
	}
	return weight
}

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
	areaCounts := make(map[string]int)
	for i := range ordered {
		ordered[i].Area = areaOf(ordered[i].File.Path)
		bestPrefix := ""
		for prefix, area := range tuning.AreaRoots {
			prefix = strings.Trim(strings.ReplaceAll(prefix, "\\", "/"), "/")
			path := strings.Trim(ordered[i].File.Path, "/")
			if (path == prefix || strings.HasPrefix(path, prefix+"/")) && len(prefix) > len(bestPrefix) {
				bestPrefix = prefix
				ordered[i].Area = area
			}
		}
		area := candidateArea(ordered[i])
		areaCounts[area]++
		ordered[i].AreaWeight = areaMultiplier(areaCounts[area], tuning.AreaDiminishing)
	}

	result := Result{Budget: request.Budget, Decisions: make([]Decision, 0, len(ordered))}
	if request.Budget <= 0 {
		return selectUnlimited(ordered, result, request.Prompt, tuning)
	}

	// Reserve high-retention files before the DP, with at most two such
	// reservations per area (and one recent implementation file per area), so
	// repeated docs or changed files cannot crowd out other useful areas.
	// The exact reserved variant is stored and reused in the final pass so the
	// reserved token count (which sizes the DP budget below) is honored and the
	// selection can never exceed request.Budget. Re-picking bestAffordable
	// against the running total at finalize time would let a guaranteed file
	// upgrade to a more expensive variant than reserved, silently overrunning
	// the budget relative to the DP files already accounted for.
	reserved := make([]Variant, len(ordered))
	areaReserved := make([]bool, len(ordered))
	promptReserved := make([]bool, len(ordered))
	protected := make([]bool, len(ordered))
	protectedImplAreas := make(map[string]bool)
	protectedRoleAreas := make(map[string]int)
	remaining := request.Budget
	for index, candidate := range ordered {
		if candidate.File.Hidden || candidate.File.GitIgnored {
			continue
		}
		if tuning.skipRole(lang.Classify(candidate.File.Path).Role, request.Prompt) {
			continue
		}
		classification := lang.Classify(candidate.File.Path)
		area := candidateArea(candidate)
		if protectedRoleAreas[area] >= 2 {
			continue
		}
		if classification.Role == lang.RoleImpl && protectedImplAreas[area] {
			continue
		}
		if tuning.retentionFor(candidate) >= tuning.RetentionFloor {
			if variant := bestAffordable(candidate, remaining, tuning, request.Prompt); variant.Tokens > 0 {
				// Prefer the signature variant when full content would take
				// over half the remaining budget: one guaranteed file must
				// not starve the DP stage. Leftover slack can upgrade it
				// back to full later (see opportunisticUpgrade).
				if variant.Mode == ModeFull {
					if sig := findVariant(candidate, ModeSignatures, request.Prompt, tuning); sig.Mode == ModeSignatures && sig.Tokens > 0 && variant.Tokens > remaining/2 {
						variant = sig
					}
				}
				reserved[index] = variant
				protected[index] = true
				if classification.Role == lang.RoleImpl {
					protectedImplAreas[area] = true
				}
				protectedRoleAreas[area]++
				remaining -= variant.Tokens
			}
		}
	}

	// Strong path matches are explicit task context. Reserve them before
	// generic area coverage so the default breadth policy cannot crowd out a
	// user-focused request.
	promptOrder := make([]int, 0, len(ordered))
	for index, candidate := range ordered {
		if reserved[index].Tokens == 0 && !candidate.File.Hidden && !candidate.File.GitIgnored &&
			!tuning.skipRole(lang.Classify(candidate.File.Path).Role, request.Prompt) &&
			taskRelevance(candidate.File.Path, request.Prompt) >= 0.20 {
			promptOrder = append(promptOrder, index)
		}
	}
	sort.SliceStable(promptOrder, func(i, j int) bool {
		left, right := ordered[promptOrder[i]], ordered[promptOrder[j]]
		lr, rr := taskRelevance(left.File.Path, request.Prompt), taskRelevance(right.File.Path, request.Prompt)
		if lr != rr {
			return lr > rr
		}
		return left.File.RankScore > right.File.RankScore
	})
	for _, index := range promptOrder {
		candidate := ordered[index]
		variant := bestAffordable(candidate, remaining, tuning, request.Prompt)
		if variant.Tokens == 0 {
			continue
		}
		if variant.Mode == ModeFull && variant.Tokens > remaining/2 {
			if sig := findVariant(candidate, ModeSignatures, request.Prompt, tuning); sig.Mode == ModeSignatures && sig.Tokens > 0 && sig.Tokens <= remaining {
				variant = sig
			}
		}
		reserved[index] = variant
		promptReserved[index] = true
		protected[index] = true
		remaining -= variant.Tokens
	}

	// Reserve one affordable representative per area before the general
	// optimizer spends the remaining budget on depth. Role-based retention
	// reservations above keep their higher priority.
	coveredAreas := make(map[string]bool)
	for index, variant := range reserved {
		if variant.Tokens > 0 {
			coveredAreas[candidateArea(ordered[index])] = true
		}
	}
	for index, candidate := range ordered {
		if reserved[index].Tokens > 0 || candidate.File.Hidden || candidate.File.GitIgnored {
			continue
		}
		if tuning.skipRole(lang.Classify(candidate.File.Path).Role, request.Prompt) {
			continue
		}
		area := candidateArea(candidate)
		if coveredAreas[area] {
			continue
		}
		variant := bestAffordable(candidate, remaining, tuning, request.Prompt)
		if variant.Tokens == 0 {
			continue
		}
		if variant.Mode == ModeFull && variant.Tokens > remaining/2 {
			if sig := findVariant(candidate, ModeSignatures, request.Prompt, tuning); sig.Mode == ModeSignatures && sig.Tokens > 0 && sig.Tokens <= remaining {
				variant = sig
			}
		}
		reserved[index] = variant
		areaReserved[index] = true
		protected[index] = true
		remaining -= variant.Tokens
		coveredAreas[area] = true
	}

	limit := request.MaxStates
	if limit <= 0 {
		limit = defaultMaxStates
	}
	states := map[int]dpState{0: {}}
	for index, candidate := range ordered {
		if tuning.skipRole(lang.Classify(candidate.File.Path).Role, request.Prompt) {
			continue
		}
		if reserved[index].Tokens > 0 {
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
				node := &choiceNode{choice: choice{Index: index, Mode: variant.Mode}, prev: state.last}
				keepState(next, newTokens, dpState{
					Utility: state.Utility + variant.Utility,
					count:   state.count + 1,
					last:    node,
				})
			}
		}
		states = pruneStates(next, limit)
	}

	_, best := bestState(states)
	chosen := make(map[int]Mode, best.count)
	for node := best.last; node != nil; node = node.prev {
		chosen[node.Index] = node.Mode
	}
	for index, candidate := range ordered {
		if tuning.skipRole(lang.Classify(candidate.File.Path).Role, request.Prompt) {
			result.Decisions = append(result.Decisions, makeDecision(candidate, ModeSkip, 0, "role excluded by automatic selection policy", request.Prompt))
			continue
		}
		if candidate.File.Hidden || candidate.File.GitIgnored {
			result.Decisions = append(result.Decisions, makeDecision(candidate, ModeSkip, 0, "hidden or git-ignored; opt-in required", request.Prompt))
			continue
		}
		if reserved[index].Tokens > 0 {
			// Reuse the exact variant reserved above: its token count already
			// reduced the DP budget (remaining), so committing the same amount
			// here keeps the total at or under request.Budget.
			variant := reserved[index]
			reason := variant.Reason
			if areaReserved[index] {
				reason = "area coverage; " + reason
			} else if promptReserved[index] {
				reason = "prompt-relevant path; " + reason
			}
			decision := makeDecision(candidate, variant.Mode, variant.Utility, reason, request.Prompt)
			decision.Selected = true
			decision.Tokens = variant.Tokens
			result.Decisions = append(result.Decisions, decision)
			result.Selected = append(result.Selected, finalize(candidate.File, variant.Mode, variant.Tokens))
			result.UsedTokens += variant.Tokens
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
	opportunisticUpgrade(&result, ordered, request.Prompt, tuning)
	balanceAreaBudget(&result, ordered, request.Prompt, tuning, protected)
	return result
}

// opportunisticUpgrade spends leftover budget upgrading signature selections
// to full content, highest utility-per-token first. Reservation may hold a
// large file at signatures to leave room for the DP stage, and state pruning
// can drop an optimal full variant; when slack remains, upgrading restores
// the higher-utility representation without ever exceeding the budget.
func opportunisticUpgrade(result *Result, ordered []Candidate, prompt string, tuning Tuning) {
	leftover := result.Budget - result.UsedTokens
	if leftover <= 0 {
		return
	}
	byPath := make(map[string]int, len(ordered))
	for i := range ordered {
		byPath[ordered[i].File.Path] = i
	}
	selByPath := make(map[string]int, len(result.Selected))
	for i := range result.Selected {
		selByPath[result.Selected[i].Path] = i
	}
	type upgrade struct {
		decIdx int
		selIdx int
		candIx int
		gain   float64
		cost   int
		full   Variant
	}
	var cands []upgrade
	for di := range result.Decisions {
		d := &result.Decisions[di]
		if !d.Selected || d.Mode != ModeSignatures {
			continue
		}
		ci, ok := byPath[d.Path]
		if !ok {
			continue
		}
		si, ok := selByPath[d.Path]
		if !ok {
			continue
		}
		full := findVariant(ordered[ci], ModeFull, prompt, tuning)
		if full.Tokens <= d.Tokens {
			continue
		}
		gain := full.Utility - d.Utility
		if gain <= 0 {
			continue
		}
		cands = append(cands, upgrade{decIdx: di, selIdx: si, candIx: ci, gain: gain, cost: full.Tokens - d.Tokens, full: full})
	}
	sort.SliceStable(cands, func(i, j int) bool {
		lhs := cands[i].gain / float64(cands[i].cost)
		rhs := cands[j].gain / float64(cands[j].cost)
		if lhs != rhs {
			return lhs > rhs
		}
		return cands[i].gain > cands[j].gain
	})
	for _, u := range cands {
		if u.cost > leftover {
			continue
		}
		leftover -= u.cost
		d := &result.Decisions[u.decIdx]
		d.Mode = ModeFull
		d.Tokens = u.full.Tokens
		d.Utility = u.full.Utility
		d.Reason = "upgraded to full content with leftover budget; " + u.full.Reason
		result.Selected[u.selIdx] = finalize(ordered[u.candIx].File, ModeFull, u.full.Tokens)
		result.UsedTokens += u.cost
	}
}

// bestAffordable returns the highest-utility variant that fits within budget,
// or a zero-token skip variant when nothing fits. It prefers the most useful
// representation of a file that the remaining budget can still pay for.
func balanceAreaBudget(result *Result, ordered []Candidate, prompt string, tuning Tuning, protected []bool) {
	if result.Budget <= 0 || len(ordered) == 0 {
		return
	}
	byPath := make(map[string]int, len(ordered))
	areas := make(map[string]bool)
	areaRelevance := make(map[string]float64)
	protectedTokens := make(map[string]int)
	for i, candidate := range ordered {
		byPath[candidate.File.Path] = i
		area := candidateArea(candidate)
		if !candidate.File.Hidden && !candidate.File.GitIgnored && !tuning.skipRole(lang.Classify(candidate.File.Path).Role, prompt) {
			areas[area] = true
			if relevance := taskRelevance(candidate.File.Path, prompt); relevance > areaRelevance[area] {
				areaRelevance[area] = relevance
			}
		}
		if i < len(protected) && protected[i] {
			variant := findVariant(candidate, ModeSignatures, prompt, tuning)
			protectedTokens[area] += variant.Tokens
		}
	}
	if len(areas) < 2 {
		return
	}
	share := tuning.AreaBudgetShare
	if fair := 2.0 / float64(len(areas)); fair > share {
		share = fair
	}
	if share > 1 {
		share = 1
	}
	capTokens := int(float64(result.Budget) * share)
	areaLimit := func(area string) int {
		limit := capTokens
		// A prompt match can relax an area's ceiling in proportion to its
		// relevance, up to the full budget for an exact area-focused task.
		relevance := areaRelevance[area]
		if relevance > 0 {
			limit += int(float64(result.Budget-capTokens) * relevance)
		}
		return max(limit, protectedTokens[area])
	}
	decisionByPath := make(map[string]int, len(result.Decisions))
	for i := range result.Decisions {
		decisionByPath[result.Decisions[i].Path] = i
	}
	selected := make(map[string]format.FileEntry, len(result.Selected))
	areaTokens := make(map[string]int)
	for _, file := range result.Selected {
		selected[file.Path] = file
		areaTokens[candidateArea(ordered[byPath[file.Path]])] += file.Tokens
	}

	for area, used := range areaTokens {
		limit := areaLimit(area)
		for used > limit {
			cutPath := ""
			cutDensity := 1e9
			for path := range selected {
				index := byPath[path]
				if candidateArea(ordered[index]) != area || (index < len(protected) && protected[index]) {
					continue
				}
				d := result.Decisions[decisionByPath[path]]
				density := d.Utility / float64(max(1, d.Tokens))
				if density < cutDensity {
					cutPath, cutDensity = path, density
				}
			}
			if cutPath == "" {
				break
			}
			index := byPath[cutPath]
			file := selected[cutPath]
			d := &result.Decisions[decisionByPath[cutPath]]
			sig := findVariant(ordered[index], ModeSignatures, prompt, tuning)
			if d.Mode == ModeFull && sig.Mode == ModeSignatures && used-file.Tokens+sig.Tokens <= limit {
				used += sig.Tokens - file.Tokens
				result.UsedTokens += sig.Tokens - file.Tokens
				selected[cutPath] = finalize(ordered[index].File, ModeSignatures, sig.Tokens)
				d.Mode, d.Tokens, d.Utility = ModeSignatures, sig.Tokens, sig.Utility
				d.Reason = "signature retained to balance area budget; " + sig.Reason
			} else {
				used -= file.Tokens
				result.UsedTokens -= file.Tokens
				delete(selected, cutPath)
				d.Selected, d.Mode, d.Tokens, d.Utility = false, ModeSkip, 0, 0
				d.Reason = "area budget reached; lower marginal value than other areas"
			}
		}
		areaTokens[area] = used
	}

	// Greedily refill the released budget with the best affordable variants
	// from areas that still have room.
	for result.UsedTokens < result.Budget {
		bestIndex, bestVariant, bestDensity := -1, Variant{}, 0.0
		for i, candidate := range ordered {
			if _, ok := selected[candidate.File.Path]; ok || candidate.File.Hidden || candidate.File.GitIgnored || tuning.skipRole(lang.Classify(candidate.File.Path).Role, prompt) {
				continue
			}
			area := candidateArea(candidate)
			room := min(result.Budget-result.UsedTokens, areaLimit(area)-areaTokens[area])
			for _, variant := range variants(candidate, prompt, tuning) {
				if variant.Tokens <= 0 || variant.Tokens > room {
					continue
				}
				density := variant.Utility / float64(variant.Tokens)
				if density > bestDensity {
					bestIndex, bestVariant, bestDensity = i, variant, density
				}
			}
		}
		if bestIndex < 0 {
			break
		}
		candidate := ordered[bestIndex]
		d := &result.Decisions[decisionByPath[candidate.File.Path]]
		d.Selected, d.Mode, d.Tokens, d.Utility = true, bestVariant.Mode, bestVariant.Tokens, bestVariant.Utility
		d.Reason = "selected to fill available area budget; " + bestVariant.Reason
		selected[candidate.File.Path] = finalize(candidate.File, bestVariant.Mode, bestVariant.Tokens)
		result.UsedTokens += bestVariant.Tokens
		areaTokens[candidateArea(candidate)] += bestVariant.Tokens
	}
	result.Selected = result.Selected[:0]
	for _, candidate := range ordered {
		if file, ok := selected[candidate.File.Path]; ok {
			result.Selected = append(result.Selected, file)
		}
	}
}

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

func selectUnlimited(candidates []Candidate, result Result, prompt string, tuning Tuning) Result {
	for _, candidate := range candidates {
		if tuning.skipRole(lang.Classify(candidate.File.Path).Role, prompt) {
			result.Decisions = append(result.Decisions, makeDecision(candidate, ModeSkip, 0, "role excluded by automatic selection policy", prompt))
			continue
		}
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
			if classification.Role == lang.RoleTest && testFocusedPrompt(prompt) {
				mode = ModeSignatures
				if candidate.File.TokensSig <= 0 || candidate.File.TokensSig >= candidate.File.TokensFull {
					mode = ModeFull
				}
				reason = "test-focused task; test context included"
			}
			if retainWithoutBudget(classification.Role) && candidate.File.TokensSig > 0 && candidate.File.TokensSig < candidate.File.TokensFull {
				mode = ModeSignatures
				reason = "background file retained; signature representation"
			}
			if mode == ModeSkip {
				result.Decisions = append(result.Decisions, makeDecision(candidate, mode, 0, reason, prompt))
				continue
			}
		}
		variant := findVariant(candidate, mode, prompt, tuning)
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
		Area:            candidateArea(candidate),
		Mode:            mode,
		Score:           candidate.File.RankScore,
		Utility:         utility,
		FullTokens:      candidate.File.TokensFull,
		SignatureTokens: candidate.File.TokensSig,
		Role:            string(classification.Role),
		RoleConfidence:  classification.Confidence,
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
	testBoost := 0.0
	if classification.Role == lang.RoleTest && testFocusedPrompt(prompt) {
		testBoost = tuning.TestTaskBoost * (0.5 + clamp(candidate.Signals.TestAffinity, 0, 1))
	}
	base := clamp(file.RankScore+classification.Adjustment+tuning.RelevanceWeight*relevance+testBoost, tuning.BaseMin, tuning.BaseMax)
	preferenceBonus := 0.0
	switch Mode(candidate.PreferredMode) {
	case ModeFull, ModeSignatures:
		preferenceBonus = tuning.PreferenceBonus
	case ModeSkip:
		// A skip hint is soft. The file can still be selected if its other
		// signals make it useful, but it starts with lower utility.
		if !(classification.Role == lang.RoleTest && testFocusedPrompt(prompt)) && relevance < 0.5 {
			base *= tuning.SkipMultiplier
		}
	}

	areaWeight := candidate.AreaWeight
	if areaWeight <= 0 {
		areaWeight = 1
	}
	fullUtility := base * (1.0 + preferenceBonus) * areaWeight
	result := []Variant{{Mode: ModeFull, Tokens: file.TokensFull, Utility: fullUtility, Reason: "full content; " + classification.Reason}}
	if file.TokensSig > 0 && file.TokensSig < file.TokensFull {
		ratio := float64(file.TokensSig) / float64(file.TokensFull)
		quality := clamp(tuning.SigQualityMin+(tuning.SigQualityMax-tuning.SigQualityMin)*(1.0-ratio), tuning.SigQualityMin, tuning.SigQualityMax)
		sigUtility := base * quality * areaWeight
		if Mode(candidate.PreferredMode) == ModeSignatures {
			sigUtility *= (1.0 + tuning.SignatureBonus)
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
	// Split by camelCase, snake_case, slashes, dots, and non-alphanumeric chars
	var sb strings.Builder
	var last rune
	for _, r := range value {
		if unicode.IsUpper(r) && unicode.IsLower(last) {
			sb.WriteByte(' ')
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(unicode.ToLower(r))
		} else {
			sb.WriteByte(' ')
		}
		last = r
	}
	parts := strings.Fields(sb.String())
	filtered := parts[:0]
	for _, part := range parts {
		if len(part) >= 3 {
			filtered = append(filtered, part)
		}
	}
	return filtered
}

func testFocusedPrompt(prompt string) bool {
	for _, term := range words(prompt) {
		switch term {
		case "test", "tests", "testing", "bug", "bugfix", "regression", "coverage", "failing", "failure", "validate", "validation", "fix", "broken", "issue", "defect":
			return true
		}
	}
	return false
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
	return a.count > b.count
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
	bestTokens, hasBest := 0, false
	var best dpState
	for tokens, state := range states {
		if !hasBest || betterState(state, best) || (state.Utility == best.Utility && tokens < bestTokens) {
			hasBest, bestTokens, best = true, tokens, state
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
