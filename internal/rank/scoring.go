package rank

import (
	"path/filepath"
	"strconv"
	"strings"
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

// Mode thresholds for the continuous relevance score. High-relevance files
// (active work, entrypoints) get full content; mid-relevance files get
// signatures; low-relevance background files are skip candidates. The bands
// mirror the review plan (0.55/0.20).
const (
	scoreFullBand   = 0.55
	scoreSkipBand   = 0.20
	ratioFullCap    = 0.85
	churnScale      = 10.0
	centralityScale = 5.0
)

// CalculateUnifiedScores evaluates every file, combining git recency, churn
// frequency, import centrality, file role, and compression yield into one
// continuous score. The score drives both mode selection (bands above) and
// budget trimming, which sorts by it. FanInCount is fed by the caller; it is
// zero until the import-centrality extractor lands.
func (g *Git) CalculateUnifiedScores(rootDir string, params []ScoringParams) map[string]FileScoreResult {
	results := make(map[string]FileScoreResult, len(params))

	var changes *Changes
	churnMap := make(map[string]int)
	if g.Available() {
		changes = g.ChangesFor("HEAD")
		churnMap = g.GetChurnFrequency(30)
	}

	for _, p := range params {
		relPath := p.Path
		baseName := strings.ToLower(filepath.Base(relPath))

		// Signal 1: git recency tier (weight 0.40).
		recencyScore := ScoreBaseline
		if changes != nil {
			recencyScore = changes.Score(relPath)
		}

		// Signal 2: churn frequency over the last 30 commits (weight 0.20).
		churnScore := float64(churnMap[relPath]) / churnScale
		if churnScore > 1.0 {
			churnScore = 1.0
		}

		// Signal 3: import centrality, reverse fan-in (weight 0.20).
		centralityScore := float64(p.FanInCount) / centralityScale
		if centralityScore > 1.0 {
			centralityScore = 1.0
		}

		// Signal 4: file role modifier. Entrypoints carry the most context;
		// tests and mocks are cheap derivations of the code they exercise.
		roleModifier := 0.0
		switch {
		case strings.HasPrefix(relPath, "cmd/") || baseName == "main.go":
			roleModifier = 0.20
		case strings.HasSuffix(baseName, "_test.go") || strings.HasSuffix(baseName, ".test.ts"):
			roleModifier = -0.30
		case strings.HasPrefix(baseName, "mock_") || strings.HasSuffix(baseName, "_mock.go"):
			roleModifier = -0.40
		}

		compositeScore := recencyScore*0.40 + churnScore*0.20 + centralityScore*0.20 + roleModifier
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
		case compositeScore >= scoreFullBand:
			mode = "full"
			reason = "High relevance / active work (Full)"
		case compositeScore < scoreSkipBand:
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
