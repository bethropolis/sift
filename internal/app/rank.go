package app

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"regexp"
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

// NewRankerWithContext returns a Ranker like NewRankerWithWeights whose git
// subprocesses are killed when ctx is done.
func NewRankerWithContext(ctx context.Context, rootDir string, weights rank.Weights) *Ranker {
	return &Ranker{rootDir: rootDir, g: rank.NewWithContext(ctx, rootDir), weights: weights}
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
	results, _ := r.RankGraph(files)
	return results
}

// RankGraph ranks like Rank and additionally returns the dependency
// adjacency retained from the fan-in pass: file path → confirmed dependency
// paths. Targets are resolved files, except Go package imports which name
// directories for the caller to expand against the collected set.
func (r *Ranker) RankGraph(files []format.FileEntry) (map[string]rank.FileScoreResult, map[string][]string) {
	fanIn, graph := r.computeFanIn(files)
	results := r.g.CalculateUnifiedScores(r.rootDir, RankParams(files, fanIn), r.weights)
	ApplyRankScores(files, results)
	sort.SliceStable(files, func(i, j int) bool {
		return files[i].RankScore > files[j].RankScore
	})
	return results, graph
}

// DependencyGraph returns only the import-dependency adjacency, without
// running git scoring or touching the entries. Dependency expansion
// (ExpandChosen) discards RankGraph's scores, so routing it through
// RankGraph paid for git status/diff/log subprocesses on every generate
// just to throw the results away. It never mutates files.
func (r *Ranker) DependencyGraph(files []format.FileEntry) map[string][]string {
	_, graph := r.computeFanIn(files)
	return graph
}

// RelatedTestAffinity scores each test file by the relevance of its related
// implementation files. Same-directory source files and resolved imports are
// considered; the result is normalized to the source file's existing rank.
func RelatedTestAffinity(files []format.FileEntry, graph map[string][]string) map[string]float64 {
	byPath := make(map[string]int, len(files))
	byDir := make(map[string][]int)
	roles := make(map[string]lang.Role, len(files))
	for i, file := range files {
		path := filepath.ToSlash(file.Path)
		byPath[path] = i
		roles[path] = lang.Classify(path).Role
		dir := filepath.ToSlash(filepath.Dir(path))
		byDir[dir] = append(byDir[dir], i)
	}
	out := make(map[string]float64)
	for testIndex, test := range files {
		testPath := filepath.ToSlash(test.Path)
		if roles[testPath] != lang.RoleTest {
			continue
		}
		best := 0.0
		consider := func(index int) {
			file := files[index]
			if roles[filepath.ToSlash(file.Path)] == lang.RoleTest {
				return
			}
			if file.RankScore > best {
				best = file.RankScore
			}
		}
		for _, sourceIndex := range byDir[filepath.ToSlash(filepath.Dir(testPath))] {
			consider(sourceIndex)
		}
		for _, target := range graph[testPath] {
			target = strings.TrimSuffix(filepath.ToSlash(target), "/")
			if index, ok := byPath[target]; ok {
				consider(index)
			}
			for _, index := range byDir[target] {
				consider(index)
			}
		}
		if best > 0 {
			out[filepath.ToSlash(files[testIndex].Path)] = best
		}
	}
	return out
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
func (r *Ranker) computeFanIn(files []format.FileEntry) (map[string]int, map[string][]string) {
	moduleRoots := r.moduleRoots(files)
	root := &fanNode{children: map[string]*fanNode{}}
	nodeByPath := make(map[string]*fanNode, len(files)+1)
	// "." is the scan-root package for top-level files and for imports of the
	// root package itself.
	nodeByPath["."] = root
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

	// exists covers collected files and their ancestor directories (so Go
	// package imports confirm), keyed lowercase to match the normalized
	// paths the language drivers resolve against.
	collected := make(map[string]bool, len(files))
	for _, f := range files {
		p := strings.ToLower(filepath.ToSlash(f.Path))
		collected[p] = true
		for d := filepath.Dir(p); d != "." && d != "/"; d = filepath.Dir(d) {
			d = filepath.ToSlash(d)
			collected[d] = true
		}
	}
	graph := make(map[string][]string)

	for _, f := range files {
		filePath := filepath.ToSlash(f.Path)
		mod := resolveModuleForFile(filePath, moduleRoots)
		moduleRelative := lang.ImportsUseModuleRoot(filePath)
		// Module-relative import targets (Go packages) are mapped back onto
		// scan-root relative paths. Other drivers already emit scan-root
		// relative targets, so they pass through unchanged.
		toRoot := func(p string) (string, bool) {
			p = filepath.ToSlash(strings.TrimSpace(p))
			if p == "" {
				return "", false
			}
			if !moduleRelative {
				return p, true
			}
			if mod.moduleRel != "" {
				if p == mod.moduleRel {
					return ".", true
				}
				if !strings.HasPrefix(p, mod.moduleRel+"/") {
					return "", false // import resolves outside the scan root
				}
				return strings.TrimPrefix(p, mod.moduleRel+"/"), true
			}
			if mod.dir == "" {
				return p, true
			}
			return filepath.ToSlash(path.Join(mod.dir, p)), true
		}
		exists := func(p string) bool {
			p, ok := toRoot(p)
			return ok && collected[strings.ToLower(p)]
		}
		// One resolution pass per file: the resolved targets are confirmed
		// repo paths, so they serve both the dependency graph and fan-in. The
		// previous code parsed imports twice (Imports + ResolveImports).
		targets := lang.ResolveImports(filePath, mod.moduleName, f.Content, exists)
		mapped := make([]string, 0, len(targets))
		for _, t := range targets {
			if mappedTarget, ok := toRoot(t); ok && mappedTarget != "" {
				mappedTarget = strings.TrimSuffix(mappedTarget, "/")
				mapped = append(mapped, mappedTarget)
				// A package node present in collected paths receives one hit
				// per importer. Fan-in is summed from a file's own node plus
				// its immediate parent package below, so importing a package
				// never inflates nested subpackages.
				if node, ok := nodeByPath[mappedTarget]; ok {
					node.hits++
				}
			}
		}
		if len(mapped) > 0 {
			graph[filePath] = mapped
		}
	}

	// Fan-in is the sum of hits on a file's own node and on its immediate
	// parent package directory. Earlier versions propagated a package's hits
	// to every descendant, so importing internal/lang also counted
	// internal/lang/rust/*.
	fanIn := make(map[string]int, len(files))
	for _, f := range files {
		p := filepath.ToSlash(f.Path)
		total := 0
		if node, ok := nodeByPath[p]; ok {
			total += node.hits
		}
		if parent, ok := nodeByPath[path.Dir(p)]; ok {
			total += parent.hits
		}
		fanIn[p] = total
	}
	return fanIn, graph
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

// moduleRootInfo maps a module directory (relative to the scan root) to its
// module name. dir "" is the baseline module: either the scan root itself or
// an enclosing parent module when scanning a subdirectory. moduleRel is the
// scan root's path relative to that module root ("" when they are the same),
// used to re-base module-relative import targets onto the scan root.
type moduleRootInfo struct {
	dir        string // e.g. "" for the baseline, "services/auth" for nested
	moduleName string
	moduleRel  string // rootDir relative to the module root, e.g. "internal"
}

// moduleRoots discovers module definitions around and inside the scan root:
// an enclosing go.mod/pubspec.yaml above it, nested modules found among the
// collected files, and go.work workspace members. Scanning a subdirectory of a
// Go repository must still resolve imports against the parent module.
func (r *Ranker) moduleRoots(files []format.FileEntry) []moduleRootInfo {
	var roots []moduleRootInfo
	seen := make(map[string]bool)
	add := func(dir, moduleRel, mod string) {
		dir = normalizeModuleDir(dir)
		moduleRel = normalizeModuleDir(moduleRel)
		if mod == "" || seen[dir] {
			return
		}
		seen[dir] = true
		roots = append(roots, moduleRootInfo{dir: dir, moduleName: mod, moduleRel: moduleRel})
	}

	// 1. Enclosing module. All collected paths are relative to r.rootDir, so
	// whatever module encloses it is the baseline (""); moduleRel records how
	// far below the module root the scan starts.
	moduleDir, rootModule := findEnclosingModule(r.rootDir)
	moduleRel := ""
	if moduleDir != "" {
		if rel, err := filepath.Rel(moduleDir, r.rootDir); err == nil {
			moduleRel = normalizeModuleDir(rel)
		}
	}
	add("", moduleRel, rootModule)

	// 2. Discover any nested go.mod/pubspec.yaml from collected files.
	for _, f := range files {
		p := filepath.ToSlash(f.Path)
		base := filepath.Base(p)
		if (base == "go.mod" || base == "pubspec.yaml") && filepath.Dir(p) != "." {
			full := filepath.Join(r.rootDir, filepath.FromSlash(p))
			module := ""
			if base == "go.mod" {
				module = parseGoMod(full)
			} else {
				module = parsePubspecName(full)
			}
			add(filepath.Dir(p), "", module)
		}
	}

	// 3. Workspace members from go.work, in case their go.mod was skipped.
	// Check the scan root and the enclosing module directory.
	workFiles := []string{filepath.Join(r.rootDir, "go.work")}
	if moduleDir != "" && filepath.Clean(moduleDir) != filepath.Clean(r.rootDir) {
		workFiles = append(workFiles, filepath.Join(moduleDir, "go.work"))
	}
	for _, workFile := range workFiles {
		for _, member := range parseGoWork(workFile) {
			abs := filepath.FromSlash(member)
			if !filepath.IsAbs(abs) {
				abs = filepath.Join(filepath.Dir(workFile), abs)
			}
			rel, err := filepath.Rel(r.rootDir, abs)
			if err != nil {
				continue
			}
			rel = normalizeModuleDir(rel)
			if strings.HasPrefix(rel, "..") {
				continue
			}
			add(rel, "", parseGoMod(filepath.Join(abs, "go.mod")))
		}
	}
	return roots
}

// normalizeModuleDir cleans a module directory for prefix matching.
func normalizeModuleDir(dir string) string {
	dir = filepath.ToSlash(filepath.Clean(dir))
	if dir == "." || dir == "/" {
		return ""
	}
	return dir
}

// findEnclosingModule walks upward from start until it finds a go.mod or
// pubspec.yaml, returning that directory and its module name.
func findEnclosingModule(start string) (dir, module string) {
	cur := start
	for {
		if mod := parseGoMod(filepath.Join(cur, "go.mod")); mod != "" {
			return cur, mod
		}
		if mod := parsePubspecName(filepath.Join(cur, "pubspec.yaml")); mod != "" {
			return cur, mod
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", ""
		}
		cur = parent
	}
}

var pubspecName = regexp.MustCompile(`(?m)^\s*name\s*:\s*['"]?([A-Za-z0-9_-]+)['"]?\s*(?:#.*)?$`)

func parsePubspecName(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	match := pubspecName.FindSubmatch(data)
	if len(match) < 2 {
		return ""
	}
	return string(match[1])
}

// parseGoWork returns the workspace member directories declared by `use`
// directives in a go.work file, as slash-separated repo-relative paths.
// Single-line (`use ./services/auth`), block, and single-line block
// (`use ( ./a ./b )`) forms are all handled.
func parseGoWork(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var dirs []string
	add := func(field string) {
		field = strings.Trim(field, `"`)
		if field == "" || field == ")" || strings.HasPrefix(field, "//") {
			return
		}
		clean := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(field)), "./")
		if clean != "" && clean != "." {
			dirs = append(dirs, clean)
		}
	}
	inBlock := false
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "use (") {
			inBlock = true
			line = strings.TrimSpace(strings.TrimPrefix(line, "use ("))
			if line == "" {
				continue
			}
		} else if inBlock && line == ")" {
			inBlock = false
			continue
		} else if strings.HasPrefix(line, "use ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "use "))
		} else if !inBlock {
			continue
		}
		closes := false
		if strings.HasSuffix(line, ")") {
			closes = true
			line = strings.TrimSpace(strings.TrimSuffix(line, ")"))
		}
		for _, field := range strings.Fields(line) {
			add(field)
		}
		if closes {
			inBlock = false
		}
	}
	return dirs
}

func parseGoMod(path string) string {
	if data, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "module" {
				return fields[1]
			}
		}
	}
	return ""
}

// resolveModuleForFile returns the module a file belongs to. The longest
// matching nested module wins; the baseline module (dir "") applies otherwise.
func resolveModuleForFile(filePath string, roots []moduleRootInfo) moduleRootInfo {
	best := moduleRootInfo{}
	for _, r := range roots {
		if r.dir == "" {
			if best.moduleName == "" {
				best = r
			}
			continue
		}
		if strings.HasPrefix(filePath, r.dir+"/") || filePath == r.dir {
			if len(r.dir) > len(best.dir) {
				best = r
			}
		}
	}
	return best
}

// moduleRoot reads the repository root's module path for backwards compatibility.
func (r *Ranker) moduleRoot() string {
	return parseGoMod(filepath.Join(r.rootDir, "go.mod"))
}

// ApplyRankScores writes the computed scores back into the entries.
func ApplyRankScores(files []format.FileEntry, results map[string]rank.FileScoreResult) {
	for i := range files {
		r := results[files[i].Path]
		files[i].RankScore = r.Score
		files[i].GitStatus = r.Status
	}
}
