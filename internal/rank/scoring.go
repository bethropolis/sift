package rank

import (
	"strconv"
	"strings"

	"github.com/bethropolis/sift/internal/lang"
)

// FileScoreResult is the outcome of the unified scoring engine for one file.
type FileScoreResult struct {
	Score         float64
	PreferredMode string // "full", "signatures", or "skip"
	Reason        string
	Signals       ScoreSignals
}

// ScoreSignals exposes the normalized inputs to the composite score. Keeping
// these values alongside the result makes selection reports explainable and
// gives callers enough detail to tune the weights without reverse-engineering
// a single opaque score.
type ScoreSignals struct {
	Recency    float64 `json:"recency"`
	Churn      float64 `json:"churn"`
	Centrality float64 `json:"centrality"`
	Role       float64 `json:"role"`
}

// ScoringParams is the per-file input to CalculateUnifiedScores.
type ScoringParams struct {
	Path        string
	TokensFull  int
	TokensSig   int
	DidCompress bool
	FanInCount  int
}

// Weights tunes the relevance composite. Zero fields fall back to the
// defaults below, so callers can override only the knobs they care about.
// Role knowledge (what each role is worth) lives in internal/lang, not here.
type Weights struct {
	RecencyWeight    float64
	ChurnWeight      float64
	CentralityWeight float64
	RoleWeight       float64
	FullBand         float64
	SkipBand         float64
}

// DefaultWeights returns the stock scoring weights.
func DefaultWeights() Weights {
	return Weights{
		RecencyWeight:    0.40,
		ChurnWeight:      0.20,
		CentralityWeight: 0.20,
		RoleWeight:       1.0,
		FullBand:         0.55,
		SkipBand:         0.20,
	}
}

const (
	churnScale      = 10.0
	centralityScale = 5.0
	ratioFullCap    = 0.85
)

// CalculateUnifiedScores evaluates every file, combining git recency, churn
// frequency, import centrality, file role, and compression yield into one
// continuous score. The score drives both mode selection (bands above) and
// budget trimming, which sorts by it. Role weights come from the lang
// registry's classification, so the numbers exist in exactly one place.
func (g *Git) CalculateUnifiedScores(rootDir string, params []ScoringParams, w Weights) map[string]FileScoreResult {
	w.applyDefaults()
	results := make(map[string]FileScoreResult, len(params))

	var changes *Changes
	churnMap := make(map[string]int)
	if g.Available() {
		changes = g.ChangesFor("HEAD")
		churnMap = g.GetChurnFrequency(30)
	}

	for _, p := range params {
		relPath := p.Path

		// Signal 1: git recency tier.
		recencyScore := ScoreBaseline
		if changes != nil {
			recencyScore = changes.Score(relPath)
		}

		// Signal 2: churn frequency over the last 30 commits.
		churnScore := float64(churnMap[relPath]) / churnScale
		if churnScore > 1.0 {
			churnScore = 1.0
		}

		// Signal 3: import centrality, reverse fan-in.
		centralityScore := float64(p.FanInCount) / centralityScale
		if centralityScore > 1.0 {
			centralityScore = 1.0
		}

		// Signal 4: file role from the lang registry. The classification
		// carries the role adjustment; no role numbers are hardcoded here.
		classification := lang.Classify(relPath)
		roleModifier := classification.Adjustment * w.RoleWeight

		compositeScore := recencyScore*w.RecencyWeight +
			churnScore*w.ChurnWeight +
			centralityScore*w.CentralityWeight +
			roleModifier
		if compositeScore < 0.0 {
			compositeScore = 0.0
		}
		if compositeScore > 1.0 {
			compositeScore = 1.0
		}

		// Signal 5: compression yield. A ratio near 1 means signatures save
		// nothing, so full content is strictly better.
		ratio := 1.0
		if p.TokensFull > 0 {
			ratio = float64(p.TokensSig) / float64(p.TokensFull)
		}

		mode := "signatures"
		reason := "Context file (Signature compressed)"
		switch {
		case !p.DidCompress || ratio > ratioFullCap:
			mode = "full"
			reason = "Low compression yield (forced Full)"
		case recencyScore >= ScoreModified:
			mode = "full"
			reason = "Uncommitted change; full implementation context"
		case compositeScore >= w.FullBand:
			mode = "full"
			reason = "High relevance / active work (Full)"
		case compositeScore < w.SkipBand:
			mode = "skip"
			reason = "Low relevance background file"
		}

		results[relPath] = FileScoreResult{
			Score:         compositeScore,
			PreferredMode: mode,
			Reason:        reason,
			Signals: ScoreSignals{
				Recency: recencyScore, Churn: churnScore,
				Centrality: centralityScore, Role: roleModifier,
			},
		}
	}

	return results
}

// applyDefaults fills zero fields with the stock weights.
func (w *Weights) applyDefaults() {
	def := DefaultWeights()
	if w.RecencyWeight == 0 {
		w.RecencyWeight = def.RecencyWeight
	}
	if w.ChurnWeight == 0 {
		w.ChurnWeight = def.ChurnWeight
	}
	if w.CentralityWeight == 0 {
		w.CentralityWeight = def.CentralityWeight
	}
	if w.RoleWeight == 0 {
		w.RoleWeight = def.RoleWeight
	}
	if w.FullBand == 0 {
		w.FullBand = def.FullBand
	}
	if w.SkipBand == 0 {
		w.SkipBand = def.SkipBand
	}
}

// GetChurnFrequency counts how many of the last n commits touched each path.
// Empty path lines (commit separators) are ignored.
func (g *Git) GetChurnFrequency(n int) map[string]int {
	churn := make(map[string]int)
	if !g.Available() {
		return churn
	}
	for _, p := range g.runList("log", "-n", strconv.Itoa(n), "--name-only", "--format=") {
		if p = strings.TrimSpace(p); p != "" {
			churn[p]++
		}
	}
	return churn
}
