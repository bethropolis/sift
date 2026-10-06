// Package rank scores files by git relevance so an LLM gets the most
// important context first when a token budget is in play. Scores follow the
// review plan: modified files 1.0, recently diffed 0.8, recently committed
// 0.5, everything else a low baseline.
package rank

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// Git runs git against rootDir.
//
// The repository under scan is untrusted input (notably for `sift serve`,
// which opens arbitrary projects). Every invocation is therefore hardened:
//   - commands run with the caller's context so cancellation kills the child;
//   - the environment is scrubbed of GIT_* overrides and index locks are
//     disabled (GIT_OPTIONAL_LOCKS=0), so a scan never writes .git state;
//   - system config is ignored and fsmonitor/hooks are neutralized, so a
//     hostile repo config cannot execute anything;
//   - diff invocations disable external diff drivers and textconv, which a
//     repo could otherwise point at arbitrary commands;
//   - ref arguments are validated and terminated with "--" so a hostile ref
//     cannot smuggle flags.
type Git struct {
	rootDir string
	ctx     context.Context

	// availOnce memoizes the repository probe: it forks a git subprocess,
	// and one scoring pass asks several times. Handles are constructed per
	// scan/rank pass, so the answer stays fresh where it matters.
	availOnce sync.Once
	avail     bool
}

// New returns a Git handle rooted at dir with a background context.
func New(dir string) *Git {
	return NewWithContext(context.Background(), dir)
}

// NewWithContext returns a Git handle rooted at dir whose subprocesses are
// killed when ctx is done.
func NewWithContext(ctx context.Context, dir string) *Git {
	if ctx == nil {
		ctx = context.Background()
	}
	return &Git{rootDir: dir, ctx: ctx}
}

func (g *Git) run(args ...string) string {
	out := g.runRaw(args...)
	return strings.TrimSpace(out)
}

// runRaw captures git stdout verbatim so column-aligned output like porcelain
// status survives unscathed.
func (g *Git) runRaw(args ...string) string {
	full := make([]string, 0, len(args)+4)
	full = append(full,
		"-c", "core.fsmonitor=false",
		"-c", "core.hooksPath=/dev/null",
	)
	full = append(full, args...)
	cmd := exec.CommandContext(g.ctx, "git", full...)
	cmd.Dir = g.rootDir
	cmd.Env = gitEnv()
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// runDiff runs `git diff` with external drivers disabled: a hostile repo
// config (.git/config diff.*.command, .gitattributes textconv) must never
// execute during a scan.
func (g *Git) runDiff(args ...string) string {
	full := make([]string, 0, len(args)+2)
	full = append(full, "--no-ext-diff", "--no-textconv")
	full = append(full, args...)
	return g.runRaw(append([]string{"diff"}, full...)...)
}

// runRef appends "--" so a validated ref can never be parsed as a flag.
func (g *Git) runRef(args ...string) string {
	out := g.runRaw(append(args, "--")...)
	return strings.TrimSpace(out)
}

func (g *Git) runListRef(args ...string) []string {
	out := g.runRef(args...)
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// runDiffList is runList through runDiff: driver-free name-only diffs.
func (g *Git) runDiffList(args ...string) []string {
	out := strings.TrimSpace(g.runDiff(args...))
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// cleanRef validates a user-supplied revision: it must be non-empty, must not
// look like a flag, and must not contain whitespace or control characters.
// Invalid refs yield "" and the callers' git invocations then fail closed
// (empty output), exactly as a git error surfaces today.
func cleanRef(ref string) string {
	if ref == "" || strings.HasPrefix(ref, "-") || ref == "--" {
		return ""
	}
	for _, r := range ref {
		if r <= ' ' || r == 0x7f {
			return ""
		}
	}
	return ref
}

// gitEnv returns a scrubbed environment for git subprocesses: inherited
// GIT_* overrides (which could redirect the repository, config, or objects)
// are dropped, index locks are disabled so read-only probes never write
// .git state, and system config is ignored.
func gitEnv() []string {
	kept := os.Environ()
	env := kept[:0]
	for _, kv := range kept {
		key := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			key = kv[:i]
		}
		switch key {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_NAMESPACE", "GIT_INDEX_FILE",
			"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES",
			"GIT_CONFIG", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT",
			"GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM",
			"GIT_CEILING_DIRECTORIES", "GIT_DISCOVERY_ACROSS_FILESYSTEM":
			continue
		}
		if strings.HasPrefix(key, "GIT_CONFIG_KEY_") || strings.HasPrefix(key, "GIT_CONFIG_VALUE_") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "GIT_OPTIONAL_LOCKS=0", "GIT_CONFIG_NOSYSTEM=1")
	return env
}

func (g *Git) runList(args ...string) []string {
	out := g.run(args...)
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// Available reports whether rootDir is inside a git repository. The probe is
// memoized per Git handle so a scoring pass pays for at most one fork.
func (g *Git) Available() bool {
	g.availOnce.Do(func() {
		g.avail = g.run("rev-parse", "--is-inside-work-tree") == "true"
	})
	return g.avail
}

// ChangesFor computes the relevance tiers for ref (default HEAD):
//   - Modified: uncommitted working-tree or staged changes.
//   - Diffed:   files touched between ref and its first parent.
//   - Committed: files in the latest commit at ref.
//
// The three git invocations run concurrently since they are independent.
// The tier sets are normalized once before return so Score is O(depth).
func (g *Git) ChangesFor(ref string) *Changes {
	if ref == "" {
		ref = "HEAD"
	}
	changes := NewChanges()

	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(3)
	go func() {
		defer wg.Done()
		local := map[string]bool{}
		for _, line := range strings.Split(g.runRaw("status", "--porcelain"), "\n") {
			if len(line) > 3 {
				p := strings.TrimSpace(line[3:])
				if idx := strings.Index(p, " -> "); idx != -1 {
					// Rename entry: "old.go -> new.go". Register both sides so
					// the new path is scored as modified.
					local[p[idx+4:]] = true
					local[p[:idx]] = true
				} else {
					local[p] = true
				}
			}
		}
		mu.Lock()
		for p := range local {
			changes.Modified[p] = true
		}
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		if parent := g.Parent(ref); parent != "" {
			// Root commits (a single-commit history) have no parent, so the diff
			// range "ref^..ref" is invalid; skip it rather than silently returning
			// an empty list.
			local := map[string]bool{}
			for _, p := range g.runDiffList("--name-only", parent+".."+ref) {
				if p = strings.TrimSpace(p); p != "" {
					local[p] = true
				}
			}
			mu.Lock()
			for p := range local {
				changes.Diffed[p] = true
			}
			mu.Unlock()
		}
	}()
	go func() {
		defer wg.Done()
		if cleanRef(ref) == "" {
			return
		}
		local := map[string]bool{}
		for _, p := range g.runListRef("log", "-n", "5", "--name-only", "--format=", ref) {
			if p = strings.TrimSpace(p); p != "" {
				local[p] = true
			}
		}
		mu.Lock()
		for p := range local {
			changes.Committed[p] = true
		}
		mu.Unlock()
	}()
	wg.Wait()
	changes.Normalize()
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
	short = g.runRef("rev-parse", "--short", "HEAD")
	subject = g.runRef("log", "-1", "--format=%s", "HEAD")
	return short, subject
}

// Ref returns the short hash and subject of the given ref.
func (g *Git) Ref(ref string) (short, subject string) {
	if cleanRef(ref) == "" {
		return "", ""
	}
	short = g.runRef("rev-parse", "--short", ref)
	subject = g.runRef("log", "-1", "--format=%s", ref)
	return short, subject
}

// Parent returns the short hash of ref's first parent, or "" when ref has no
// parent (e.g. the repository root commit).
func (g *Git) Parent(ref string) string {
	if cleanRef(ref) == "" {
		return ""
	}
	return g.runRef("rev-parse", "--short", ref+"^")
}

// CommitsBetween lists the commits in the range from..to, newest first, in
// short form. The range is empty when to is not an ancestor of HEAD.
func (g *Git) CommitsBetween(from, to string) []Commit {
	if cleanRef(from) == "" || cleanRef(to) == "" {
		return nil
	}
	lines := g.runListRef("log", "--format=%h%x09%H%x09%s", from+".."+to)
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
	if cleanRef(from) == "" || cleanRef(to) == "" {
		return ""
	}
	return g.runDiff(from+".."+to, "--")
}

// RawWorktreePatch returns the unified diff of uncommitted working-tree
// changes against ref (usually HEAD).
func (g *Git) RawWorktreePatch(ref string) string {
	if ref == "" {
		ref = "HEAD"
	}
	if cleanRef(ref) == "" {
		return ""
	}
	return g.runDiff(ref, "--")
}

// ChangedBetween returns the paths changed between two refs.
func (g *Git) ChangedBetween(from, to string) []string {
	set := map[string]bool{}
	if cleanRef(from) == "" || cleanRef(to) == "" {
		return nil
	}
	for _, p := range g.runDiffList("--name-only", from+".."+to, "--") {
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
	if cleanRef(ref) == "" {
		return nil
	}
	set := map[string]bool{}
	add := func(lines []string) {
		for _, p := range lines {
			if p = strings.TrimSpace(p); p != "" {
				set[p] = true
			}
		}
	}
	add(g.runDiffList("--name-only", ref, "--"))
	add(g.runDiffList("--name-only", ref+"..HEAD", "--"))
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
	// Per-commit diffs are independent git subprocesses; run them with a
	// small bounded pool so a depth-5 history costs ~1 diff latency.
	results := make([][]string, len(commits))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, c := range commits {
		wg.Add(1)
		go func(i int, hash string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = g.ChangedBetween(g.Parent(hash), hash)
		}(i, c.Hash)
	}
	wg.Wait()
	for i := len(results) - 1; i >= 0; i-- { // oldest first so newer wins.
		changed := results[i]
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
