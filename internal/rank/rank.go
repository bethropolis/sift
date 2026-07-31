// Package rank scores files by git relevance so an LLM gets the most
// important context first when a token budget is in play. Scores follow the
// review plan: modified files 1.0, recently diffed 0.8, recently committed
// 0.5, everything else a low baseline.
package rank

import (
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// ScoreBaseline is the score for files with no git signal.
	ScoreBaseline = 0.1
	// ScoreModified is the score for files with uncommitted changes.
	ScoreModified = 1.0
	// ScoreDiffed is the score for files touched by the recent diff.
	ScoreDiffed = 0.8
	// ScoreCommitted is the score for files in the latest commit.
	ScoreCommitted = 0.5
)

// Changes holds the file sets for each git relevance tier, keyed by
// slash-separated paths relative to the repository root.
type Changes struct {
	Modified  map[string]bool
	Diffed    map[string]bool
	Committed map[string]bool
}

// NewChanges returns an empty Changes.
func NewChanges() *Changes {
	return &Changes{
		Modified:  map[string]bool{},
		Diffed:    map[string]bool{},
		Committed: map[string]bool{},
	}
}

// Score returns the highest tier score matching path.
func (c *Changes) Score(path string) float64 {
	if matches(c.Modified, path) {
		return ScoreModified
	}
	if matches(c.Diffed, path) {
		return ScoreDiffed
	}
	if matches(c.Committed, path) {
		return ScoreCommitted
	}
	return ScoreBaseline
}

// matches reports whether path equals one of the keys or lives under a keyed
// directory.
func matches(set map[string]bool, path string) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	for p := range set {
		p = filepath.ToSlash(filepath.Clean(p))
		if p == path || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}

// Git runs git against rootDir.
type Git struct {
	rootDir string
}

// New returns a Git handle rooted at dir.
func New(dir string) *Git {
	return &Git{rootDir: dir}
}

func (g *Git) run(args ...string) string {
	out := g.runRaw(args...)
	return strings.TrimSpace(out)
}

// runRaw captures git stdout verbatim so column-aligned output like porcelain
// status survives unscathed.
func (g *Git) runRaw(args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.rootDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func (g *Git) runList(args ...string) []string {
	out := g.run(args...)
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// Available reports whether rootDir is inside a git repository.
func (g *Git) Available() bool {
	return g.run("rev-parse", "--is-inside-work-tree") == "true"
}

// ChangesFor computes the relevance tiers for ref (default HEAD):
//   - Modified: uncommitted working-tree or staged changes.
//   - Diffed:   files touched between ref and its first parent.
//   - Committed: files in the latest commit at ref.
func (g *Git) ChangesFor(ref string) *Changes {
	if ref == "" {
		ref = "HEAD"
	}
	changes := NewChanges()

	for _, line := range strings.Split(g.runRaw("status", "--porcelain"), "\n") {
		if len(line) > 3 {
			changes.Modified[strings.TrimSpace(line[3:])] = true
		}
	}
	for _, p := range g.runList("diff", "--name-only", ref+"^.."+ref) {
		if p = strings.TrimSpace(p); p != "" {
			changes.Diffed[p] = true
		}
	}
	for _, p := range g.runList("log", "-n", "5", "--name-only", "--format=", ref) {
		if p = strings.TrimSpace(p); p != "" {
			changes.Committed[p] = true
		}
	}
	return changes
}

// Score returns a score for every given relative path. When the directory is
// not a git repository every path gets the baseline score.
func (g *Git) Score(rootDir string, paths []string) map[string]float64 {
	scores := make(map[string]float64, len(paths))
	if !g.Available() {
		for _, p := range paths {
			scores[p] = ScoreBaseline
		}
		return scores
	}
	changes := g.ChangesFor("HEAD")
	for _, p := range paths {
		scores[p] = changes.Score(p)
	}
	return scores
}

// ChangedSinceRef returns the paths that differ between the working tree and
// ref, including uncommitted changes and commits made after ref. With the
// default ref (HEAD) this is exactly the uncommitted working tree.
func (g *Git) ChangedSinceRef(ref string) []string {
	if ref == "" {
		ref = "HEAD"
	}
	set := map[string]bool{}
	add := func(lines []string) {
		for _, p := range lines {
			if p = strings.TrimSpace(p); p != "" {
				set[p] = true
			}
		}
	}
	add(g.runList("diff", "--name-only", ref))
	add(g.runList("diff", "--name-only", ref+"..HEAD"))
	paths := make([]string, 0, len(set))
	for p := range set {
		paths = append(paths, p)
	}
	return paths
}
