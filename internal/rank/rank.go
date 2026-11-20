// Package rank scores files by git relevance so an LLM gets the most
// important context first when a token budget is in play. Scores follow the
// review plan: modified files 1.0, recently diffed 0.8, recently committed
// 0.5, everything else a low baseline.
package rank

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
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

	// repoOnce memoizes the repository probe (is-inside-work-tree plus
	// show-prefix). One fork answers both questions; earlier code paid two.
	repoOnce sync.Once
	avail    bool
	prefix   string

	// historyMu guards the parsed commit history so ChangesFor and any
	// churn consumer share one `git log` invocation.
	historyMu sync.Mutex
	history   map[string][]historyCommit
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
	full := make([]string, 0, len(args)+6)
	full = append(full,
		"-c", "core.fsmonitor=false",
		"-c", "core.hooksPath=/dev/null",
		// Emit raw UTF-8 paths instead of C-quoted escapes; quoted paths never
		// matched walker paths and broke scoring for non-ASCII filenames.
		"-c", "core.quotepath=false",
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
// memoized per Git handle and shared with Prefix.
func (g *Git) Available() bool {
	g.repoInfo()
	return g.avail
}

// Prefix returns the scan root's path inside the repository, slash-normalized
// and without a trailing slash. It is "" when rootDir is the repository root
// (or not inside a repository).
func (g *Git) Prefix() string {
	g.repoInfo()
	return g.prefix
}

// repoInfo runs one rev-parse that answers both the repository probe and the
// scan root's prefix. Non-repository roots leave both values empty.
func (g *Git) repoInfo() {
	g.repoOnce.Do(func() {
		out := g.runRaw("rev-parse", "--is-inside-work-tree", "--show-prefix")
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		if len(lines) > 0 {
			g.avail = strings.TrimSpace(lines[0]) == "true"
		}
		if len(lines) > 1 {
			g.prefix = strings.Trim(strings.TrimSpace(lines[1]), "/")
		}
	})
}

// toRootPath maps a git-reported repository-root-relative path to one relative
// to the scan root. Paths outside the scan root are dropped: a whole-repo
// git query must not score files the walker never collected.
func (g *Git) toRootPath(p string) (string, bool) {
	p = strings.TrimSpace(filepath.ToSlash(p))
	if p == "" {
		return "", false
	}
	prefix := g.Prefix()
	if prefix == "" {
		return p, true
	}
	if p == prefix {
		return "", false
	}
	if strings.HasPrefix(p, prefix+"/") {
		return p[len(prefix)+1:], true
	}
	return "", false
}

// historyCommit is one commit parsed from the single log stream.
type historyCommit struct {
	hash  string
	when  time.Time
	files []string
}

// churnHalfLife controls time decay for the churn signal: a touch this old
// counts half as much as a fresh one.
const churnHalfLife = 30 * 24 * time.Hour

// historyFor returns up to n parsed commits for ref, running one git log and
// memoizing the full 30-commit window per handle.
func (g *Git) historyFor(ref string, n int) []historyCommit {
	if cleanRef(ref) == "" {
		return nil
	}
	g.historyMu.Lock()
	defer g.historyMu.Unlock()
	commits, ok := g.history[ref]
	if !ok {
		commits = g.readHistory(ref, 30)
		if g.history == nil {
			g.history = map[string][]historyCommit{}
		}
		g.history[ref] = commits
	}
	if n > len(commits) {
		n = len(commits)
	}
	return commits[:n]
}

// readHistory parses `git log --name-only` with commit timestamps once.
func (g *Git) readHistory(ref string, n int) []historyCommit {
	out := g.runRaw("log", "-n", strconv.Itoa(n), "--name-only", "--format=COMMIT:%H:%ct", ref)
	var commits []historyCommit
	var cur *historyCommit
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "COMMIT:") {
			parts := strings.SplitN(strings.TrimPrefix(line, "COMMIT:"), ":", 2)
			c := historyCommit{hash: parts[0]}
			if len(parts) == 2 {
				if ts, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
					c.when = time.Unix(ts, 0)
				}
			}
			commits = append(commits, c)
			cur = &commits[len(commits)-1]
			continue
		}
		if line == "" || cur == nil {
			continue
		}
		if rp, ok := g.toRootPath(line); ok {
			cur.files = append(cur.files, rp)
		}
	}
	return commits
}

// ChangesFor computes the relevance tiers and per-file status for ref
// (default HEAD):
//   - Modified: uncommitted working-tree or staged changes.
//   - Diffed:   files in ref's newest commit (equivalent to parent..ref).
//   - Committed: files in the newest five commits.
//   - Status:   single-letter porcelain status (M, A, ?, R, D).
//   - Churn:    time-decayed touch frequency over 30 commits.
//
// Status and history are independent, so they run concurrently. History is a
// single git log serving both the tiers and churn.
func (g *Git) ChangesFor(ref string) *Changes {
	if ref == "" {
		ref = "HEAD"
	}
	changes := NewChanges()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		g.applyStatus(changes)
	}()
	go func() {
		defer wg.Done()
		g.applyHistory(changes, ref)
	}()
	wg.Wait()
	changes.Normalize()
	return changes
}

// applyStatus parses `git status --porcelain`, recording both the Modified
// tier and the single-letter status used by TUI/Web UI badges.
func (g *Git) applyStatus(changes *Changes) {
	add := func(p, status string) {
		rp, ok := g.toRootPath(p)
		if !ok {
			return
		}
		changes.Modified[rp] = true
		if status != "" {
			changes.Status[rp] = status
		}
	}
	for _, line := range strings.Split(g.runRaw("status", "--porcelain"), "\n") {
		if len(line) < 4 {
			continue
		}
		code := line[:2]
		p := strings.TrimSpace(line[3:])
		if idx := strings.Index(p, " -> "); idx != -1 {
			add(p[idx+4:], "R")
			add(p[:idx], "D")
			continue
		}
		add(p, porcelainStatus(code))
	}
}

// porcelainStatus maps a two-letter porcelain code to one badge letter.
func porcelainStatus(code string) string {
	switch {
	case code == "??":
		return "?"
	case strings.ContainsRune(code, 'R'):
		return "R"
	case strings.ContainsRune(code, 'A'):
		return "A"
	case strings.ContainsRune(code, 'D'):
		return "D"
	case strings.ContainsRune(code, 'M'):
		return "M"
	case strings.ContainsRune(code, 'T'):
		return "M"
	default:
		return "?"
	}
}

// applyHistory fills Diffed, Committed, and decayed Churn from one log.
func (g *Git) applyHistory(changes *Changes, ref string) {
	commits := g.historyFor(ref, 30)
	if len(commits) == 0 {
		return
	}
	// Newest commit is equivalent to ref^..ref, including a root commit.
	for _, p := range commits[0].files {
		changes.Diffed[p] = true
	}
	for i := 0; i < len(commits) && i < 5; i++ {
		for _, p := range commits[i].files {
			changes.Committed[p] = true
		}
	}
	now := time.Now()
	for _, c := range commits {
		age := now.Sub(c.when)
		if age < 0 {
			age = 0 // clamp clock skew so weights stay in (0, 1]
		}
		weight := math.Pow(0.5, age.Seconds()/churnHalfLife.Seconds())
		for _, p := range c.files {
			changes.Churn[p] += weight
		}
	}
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
		if rp, ok := g.toRootPath(p); ok {
			set[rp] = true
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
			if rp, ok := g.toRootPath(p); ok {
				set[rp] = true
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
