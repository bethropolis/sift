package app

import (
	"context"
	"os"
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
	exists := func(p string) bool { return collected[strings.ToLower(filepath.ToSlash(p))] }
	graph := make(map[string][]string)

	for _, f := range files {
		filePath := filepath.ToSlash(f.Path)
		modRoot := resolveModuleForFile(filePath, moduleRoots)
		for _, target := range lang.Imports(filePath, modRoot, f.Content) {
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
		// Retain the resolved adjacency in the same pass: confirmed
		// dependency paths per file for dependency expansion. The exists
		// predicate covers collected files and their ancestor directories so
		// Go package (directory) imports confirm.
		if targets := lang.ResolveImports(filePath, modRoot, f.Content, exists); len(targets) > 0 {
			graph[filePath] = targets
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

// moduleRootInfo maps a workspace directory prefix to its module name.
type moduleRootInfo struct {
	dir        string // e.g. "" for root, "services/auth" for nested
	moduleName string
}

// moduleRoots discovers module definitions across the repository (e.g. root
// go.mod and nested go.mod files in subpackages/monorepos). Workspace
// members declared via go.work `use` directives are included even when their
// go.mod files were not collected.
func (r *Ranker) moduleRoots(files []format.FileEntry) []moduleRootInfo {
	var roots []moduleRootInfo
	seen := make(map[string]bool)
	add := func(dir, mod string) {
		if mod == "" || seen[dir] {
			return
		}
		seen[dir] = true
		roots = append(roots, moduleRootInfo{dir: dir, moduleName: mod})
	}
	// 1. Root module
	rootModule := parseGoMod(filepath.Join(r.rootDir, "go.mod"))
	if rootModule == "" {
		rootModule = parsePubspecName(filepath.Join(r.rootDir, "pubspec.yaml"))
	}
	add("", rootModule)
	// 2. Discover any nested go.mod from collected files
	for _, f := range files {
		p := filepath.ToSlash(f.Path)
		base := filepath.Base(p)
		if (base == "go.mod" || base == "pubspec.yaml") && filepath.Dir(p) != "." {
			dir := filepath.Dir(p)
			module := ""
			if base == "go.mod" {
				module = parseGoMod(filepath.Join(r.rootDir, filepath.FromSlash(p)))
			} else {
				module = parsePubspecName(filepath.Join(r.rootDir, filepath.FromSlash(p)))
			}
			add(dir, module)
		}
	}
	// 3. Workspace members from go.work, in case their go.mod was skipped
	for _, dir := range parseGoWork(filepath.Join(r.rootDir, "go.work")) {
		add(dir, parseGoMod(filepath.Join(r.rootDir, filepath.FromSlash(dir), "go.mod")))
	}
	return roots
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

func resolveModuleForFile(filePath string, roots []moduleRootInfo) string {
	bestDir := ""
	bestMod := ""
	for _, r := range roots {
		if r.dir == "" {
			if bestMod == "" {
				bestMod = r.moduleName
			}
		} else if strings.HasPrefix(filePath, r.dir+"/") {
			if len(r.dir) > len(bestDir) {
				bestDir = r.dir
				bestMod = r.moduleName
			}
		}
	}
	return bestMod
}

// moduleRoot reads the repository root's module path for backwards compatibility.
func (r *Ranker) moduleRoot() string {
	return parseGoMod(filepath.Join(r.rootDir, "go.mod"))
}

// ApplyRankScores writes the computed scores back into the entries.
func ApplyRankScores(files []format.FileEntry, results map[string]rank.FileScoreResult) {
	for i := range files {
		files[i].RankScore = results[files[i].Path].Score
	}
}
