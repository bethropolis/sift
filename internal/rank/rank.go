// Package rank scores files by git relevance so an LLM gets the most
// important context first when a token budget is in play. Scores follow the
// review plan: modified files 1.0, recently diffed 0.8, recently committed
// 0.5, everything else a low baseline.
package rank

import (
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

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
//
// The three git invocations run concurrently since they are independent.
func (g *Git) ChangesFor(ref string) *Changes {
	if ref == "" {
		ref = "HEAD"
	}
	changes := NewChanges()

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for _, line := range strings.Split(g.runRaw("status", "--porcelain"), "\n") {
			if len(line) > 3 {
				p := strings.TrimSpace(line[3:])
				if idx := strings.Index(p, " -> "); idx != -1 {
					// Rename entry: "old.go -> new.go". Register both sides so
					// the new path is scored as modified.
					changes.Modified[p[idx+4:]] = true
					changes.Modified[p[:idx]] = true
				} else {
					changes.Modified[p] = true
				}
			}
		}
	}()
	go func() {
		defer wg.Done()
		if parent := g.Parent(ref); parent != "" {
			// Root commits (a single-commit history) have no parent, so the diff
			// range "ref^..ref" is invalid; skip it rather than silently returning
			// an empty list.
			for _, p := range g.runList("diff", "--name-only", parent+".."+ref) {
				if p = strings.TrimSpace(p); p != "" {
					changes.Diffed[p] = true
				}
			}
		}
	}()
	go func() {
		defer wg.Done()
		for _, p := range g.runList("log", "-n", "5", "--name-only", "--format=", ref) {
			if p = strings.TrimSpace(p); p != "" {
				changes.Committed[p] = true
			}
		}
	}()
	wg.Wait()
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

// Commit identifies a git commit in the short form used by delta dumps.
type Commit struct {
	Short   string
	Hash    string
	Subject string
}

// Head returns the short hash and subject of the current HEAD commit.
func (g *Git) Head() (short, subject string) {
	short = g.run("rev-parse", "--short", "HEAD")
	subject = g.run("log", "-1", "--format=%s", "HEAD")
	return short, subject
}

// Ref returns the short hash and subject of the given ref.
func (g *Git) Ref(ref string) (short, subject string) {
	short = g.run("rev-parse", "--short", ref)
	subject = g.run("log", "-1", "--format=%s", ref)
	return short, subject
}

// Parent returns the short hash of ref's first parent, or "" when ref has no
// parent (e.g. the repository root commit).
func (g *Git) Parent(ref string) string {
	return g.run("rev-parse", "--short", ref+"^")
}

// CommitsBetween lists the commits in the range from..to, newest first, in
// short form. The range is empty when to is not an ancestor of HEAD.
func (g *Git) CommitsBetween(from, to string) []Commit {
	lines := g.runList("log", "--format=%h%x09%H%x09%s", from+".."+to)
	commits := make([]Commit, 0, len(lines))
	for _, l := range lines {
		short, hash, subject := splitCommit(l)
		if short == "" {
			continue
		}
		commits = append(commits, Commit{Short: short, Hash: hash, Subject: subject})
	}
	return commits
}

// splitCommit splits one git log format line into short hash, full hash, and
// subject. Returns empty short when the line is unusable.
func splitCommit(line string) (short, hash, subject string) {
	fields := strings.Split(line, "\t")
	if len(fields) < 3 || fields[0] == "" {
		return "", "", ""
	}
	return fields[0], fields[1], fields[2]
}

// RawPatch returns the unified diff between from and to.
func (g *Git) RawPatch(from, to string) string {
	return g.runRaw("diff", from+".."+to)
}

// ChangedBetween returns the paths changed between two refs.
func (g *Git) ChangedBetween(from, to string) []string {
	set := map[string]bool{}
	for _, p := range g.runList("diff", "--name-only", from+".."+to) {
		if p = strings.TrimSpace(p); p != "" {
			set[p] = true
		}
	}
	paths := make([]string, 0, len(set))
	for p := range set {
		paths = append(paths, p)
	}
	return paths
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

// AnalyzeCommitHistory inspects the last depth commits plus the working tree
// and recommends an output mode per touched file. Bulk commits (>10 files)
// yield signatures; focused commits (<=10 files) yield full content, and
// working-tree edits force full. Newer commits override older ones. Returns
// an empty map outside a git repository or on any git error, so callers can
// fall back to full content.
func (g *Git) AnalyzeCommitHistory(depth int) map[string]string {
	modes := make(map[string]string)
	if !g.Available() {
		return modes
	}
	commits := g.CommitsBetween("HEAD~"+strconv.Itoa(depth), "HEAD")
	for i := len(commits) - 1; i >= 0; i-- { // oldest first so newer wins.
		c := commits[i]
		changed := g.ChangedBetween(g.Parent(c.Hash), c.Hash)
		preferred := "full"
		if len(changed) > 10 {
			preferred = "signatures"
		}
		for _, p := range changed {
			modes[p] = preferred
		}
	}
	// Uncommitted edits are the freshest signal; their files must stay full.
	for _, p := range g.ChangedSinceRef("HEAD") {
		modes[p] = "full"
	}
	return modes
}
