package app

import (
	"sort"

	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/rank"
)

// Ranker applies the unified relevance scoring to collected entries. It is
// the application-level adapter between processed FileEntries and the rank
// package, which stays free of format.FileEntry. Both the blocking collect
// path and the picker's streaming rank patch go through it.
type Ranker struct {
	rootDir string
	g       *rank.Git
}

// NewRanker returns a Ranker rooted at dir.
func NewRanker(rootDir string) *Ranker {
	return &Ranker{rootDir: rootDir, g: rank.New(rootDir)}
}

// Available reports whether the root is inside a git repository.
func (r *Ranker) Available() bool {
	return r.g.Available()
}

// Rank computes unified relevance scores for files, writes them back into
// the entries, and stably sorts the slice by descending score so equal
// scores keep their original order. It returns the raw results keyed by path
// for callers that also need the preferred mode (e.g. the streaming picker
// patch). Duplicate paths collapse to a single result, which is written back
// to every matching entry.
func (r *Ranker) Rank(files []format.FileEntry) map[string]rank.FileScoreResult {
	results := r.g.CalculateUnifiedScores(r.rootDir, RankParams(files))
	ApplyRankScores(files, results)
	sort.SliceStable(files, func(i, j int) bool {
		return files[i].RankScore > files[j].RankScore
	})
	return results
}

// RankParams maps enriched entries to the ranking boundary.
func RankParams(files []format.FileEntry) []rank.ScoringParams {
	params := make([]rank.ScoringParams, len(files))
	for i, f := range files {
		params[i] = rank.ScoringParams{
			Path:        f.Path,
			TokensFull:  f.TokensFull,
			TokensSig:   f.TokensSig,
			DidCompress: f.IsCompressed || f.SigContent != nil,
		}
	}
	return params
}

// ApplyRankScores writes the computed scores back into the entries.
func ApplyRankScores(files []format.FileEntry, results map[string]rank.FileScoreResult) {
	for i := range files {
		files[i].RankScore = results[files[i].Path].Score
	}
}
