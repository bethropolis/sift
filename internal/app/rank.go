package app

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/lang"
	"github.com/bethropolis/sift/internal/rank"
)

// Ranker applies the unified relevance scoring to collected entries. It is
// the application-level adapter between processed FileEntries and the rank
// package, which stays free of format.FileEntry. Both the blocking collect
// path and the picker's streaming rank patch go through it.
type Ranker struct {
	rootDir string
	g       *rank.Git
	weights rank.Weights
}

// NewRanker returns a Ranker rooted at dir using the default scoring weights.
func NewRanker(rootDir string) *Ranker {
	return NewRankerWithWeights(rootDir, rank.DefaultWeights())
}

// NewRankerWithWeights returns a Ranker rooted at dir with explicit scoring
// weights. Zero weight fields fall back to the defaults.
func NewRankerWithWeights(rootDir string, weights rank.Weights) *Ranker {
	return &Ranker{rootDir: rootDir, g: rank.New(rootDir), weights: weights}
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
// to every matching entry. Fan-in centrality uses the collected content.
func (r *Ranker) Rank(files []format.FileEntry) map[string]rank.FileScoreResult {
	fanIn := r.computeFanIn(files)
	results := r.g.CalculateUnifiedScores(r.rootDir, RankParams(files, fanIn), r.weights)
	ApplyRankScores(files, results)
	sort.SliceStable(files, func(i, j int) bool {
		return files[i].RankScore > files[j].RankScore
	})
	return results
}

// RankParams maps enriched entries to the ranking boundary. FanInCount is
// fed from the reverse import fan-in index.
func RankParams(files []format.FileEntry, fanIn map[string]int) []rank.ScoringParams {
	params := make([]rank.ScoringParams, len(files))
	for i, f := range files {
		params[i] = rank.ScoringParams{
			Path:        f.Path,
			TokensFull:  f.TokensFull,
			TokensSig:   f.TokensSig,
			DidCompress: f.IsCompressed || f.SigContent != nil,
			FanInCount:  fanIn[filepath.ToSlash(f.Path)],
		}
	}
	return params
}

// computeFanIn builds a reverse import fan-in index: how many files reference
// each candidate. A file imports package paths; a candidate "internal/app"
// is counted as imported when its own path equals it or lives beneath it, so
// every file in an imported package gains centrality. Uses the language
// registry's ImportScanner so import syntax lives in internal/lang.
//
// Paths are organized as a trie so each import target increments a single
// package node and a single post-order pass distributes that centrality to
// every file at or beneath it. This is near-linear in input instead of the
// previous O(files × imports × files) triple loop.
func (r *Ranker) computeFanIn(files []format.FileEntry) map[string]int {
	moduleRoot := r.moduleRoot()
	root := &fanNode{children: map[string]*fanNode{}}
	nodeByPath := make(map[string]*fanNode, len(files))
	for _, f := range files {
		path := filepath.ToSlash(f.Path)
		node := root
		prefix := ""
		for _, seg := range strings.Split(path, "/") {
			if prefix == "" {
				prefix = seg
			} else {
				prefix += "/" + seg
			}
			child := node.children[seg]
			if child == nil {
				child = &fanNode{children: map[string]*fanNode{}, path: prefix}
				node.children[seg] = child
			}
			node = child
			// Register every node (directory or file) so import targets that
			// resolve to a package directory can be found even when no collected
			// file has that exact path.
			nodeByPath[prefix] = node
		}
		node.isFile = true
	}

	for _, f := range files {
		for _, target := range lang.Imports(filepath.ToSlash(f.Path), moduleRoot, f.Content) {
			target = strings.TrimSuffix(filepath.ToSlash(target), "/")
			if target == "" {
				continue
			}
			// A package node present in collected paths receives one hit per
			// importer; it is distributed to itself and its descendants below.
			if node, ok := nodeByPath[target]; ok {
				node.hits++
			}
		}
	}

	fanIn := make(map[string]int, len(files))
	var accumulate func(node *fanNode, running int)
	accumulate = func(node *fanNode, running int) {
		total := running + node.hits
		if node.isFile {
			fanIn[node.path] = total
		}
		for _, child := range node.children {
			accumulate(child, total)
		}
	}
	accumulate(root, 0)
	return fanIn
}

// fanNode is a single node in the trie built by computeFanIn. The trie is
// shared across files, so each import prefix exists once rather than once per
// file under it.
type fanNode struct {
	path     string
	isFile   bool
	hits     int // number of collected importers targeting exactly this node
	children map[string]*fanNode
}

// moduleRoot reads the repository's module path (e.g. the go.mod module
// line) so absolute imports can be stripped to repo-relative paths. Returns
// "" when the root has no recognizable module file or is not a Go module.
func (r *Ranker) moduleRoot() string {
	if data, err := os.ReadFile(filepath.Join(r.rootDir, "go.mod")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "module" {
				return fields[1]
			}
		}
	}
	return ""
}

// ApplyRankScores writes the computed scores back into the entries.
func ApplyRankScores(files []format.FileEntry, results map[string]rank.FileScoreResult) {
	for i := range files {
		files[i].RankScore = results[files[i].Path].Score
	}
}
