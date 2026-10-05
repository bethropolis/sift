package ignore

import (
	"strings"
	"testing"

	gitignore "github.com/denormal/go-gitignore"

	"github.com/bethropolis/sift/internal/utils"
)

// probe is one candidate path used to compare a fast rule against the
// library's fnmatch evaluation of the same pattern.
type probe struct {
	rel   string
	isDir bool
}

// probesFor derives concrete paths from a pattern: the pattern with each '*'
// replaced by a fixed stem (a direct hit), the empty-star variant (fnmatch's
// '*' matches the empty string), a nested copy, the basename alone in a
// subdirectory, and near misses that must not match.
func probesFor(pattern string) []probe {
	hit := strings.ReplaceAll(pattern, "*", "gen")
	empty := strings.ReplaceAll(pattern, "*", "")
	base := segmentBase(strings.TrimSuffix(hit, "/"))

	var out []probe
	add := func(rel string, isDir bool) {
		rel = strings.TrimPrefix(rel, "/")
		rel = strings.TrimSuffix(rel, "/")
		if rel == "" || rel == "." {
			return
		}
		out = append(out, probe{rel: rel, isDir: isDir})
	}
	add(hit, false)
	add(hit, true)
	add(empty, false)
	add("sub/"+strings.TrimPrefix(hit, "/"), true)
	add(base, false)
	add("sub/"+base, true)
	add(strings.TrimSuffix(hit, "/")+".bak", false)
	add("z"+base, true)
	return out
}

// TestClassifyOneMatchesLibrary proves every fast classification answers
// exactly what the library's fnmatch evaluation answers for the same pattern
// and path. Remainder patterns are owned by the library tier by definition,
// so they are checked trivially; the fast-classified ones are the risk.
func TestClassifyOneMatchesLibrary(t *testing.T) {
	for _, pattern := range DefaultIgnorePatterns {
		rule := classifyOne(pattern)
		lib := gitignore.New(strings.NewReader(pattern), "/base", nil)
		for _, p := range probesFor(pattern) {
			want := lib.Relative(p.rel, p.isDir) != nil
			got := want
			if rule.kind != fastRemainder {
				got = rule.match(p.rel, segmentBase(p.rel), p.isDir)
			}
			if got != want {
				t.Errorf("pattern %q probe %q (dir=%v): fast=%v library=%v (rule kind=%d value=%q dirOnly=%v)",
					pattern, p.rel, p.isDir, got, want, rule.kind, rule.value, rule.dirOnly)
			}
		}
	}
}

// TestFastSetCoversAllPatterns splits the default list into fast sets and
// the library remainder and re-checks the aggregate: for every probe, the
// fast tier OR the remainder matcher must equal the full library list.
func TestFastSetCoversAllPatterns(t *testing.T) {
	full := gitignore.New(strings.NewReader(strings.Join(DefaultIgnorePatterns, "\n")), "/base", nil)
	remainder := gitignore.New(strings.NewReader(strings.Join(fastDefaults.remainder, "\n")), "/base", nil)
	if !fastDefaults.usable {
		t.Fatal("default pattern list unexpectedly contains a negation")
	}
	for _, pattern := range DefaultIgnorePatterns {
		for _, p := range probesFor(pattern) {
			want := full.Relative(p.rel, p.isDir) != nil
			got := fastDefaults.match(p.rel, segmentBase(p.rel), p.isDir) ||
				remainder.Relative(p.rel, p.isDir) != nil
			if got != want {
				t.Errorf("aggregate: pattern %q probe %q (dir=%v): fast+remainder=%v full=%v",
					pattern, p.rel, p.isDir, got, want)
			}
		}
	}
}

// newReferenceMatcher builds a matcher that uses the production decision
// logic (same ShouldIgnore, same hidden/ignoreGit flags) but with the fast
// tier disabled and the complete default pattern list handed to the library.
// Comparing it against a normal matcher therefore isolates exactly the fast
// tier and the repo-tier memo.
func newReferenceMatcher(t testing.TB, root string, ignoreHidden, ignoreGit bool) *IgnoreMatcher {
	t.Helper()
	m := &IgnoreMatcher{
		rootDir:       root,
		ignoreHidden:  ignoreHidden,
		ignoreGit:     ignoreGit,
		recursiveMode: true,
		logger:        &utils.NoopLogger{},
	}
	repo, err := gitignore.NewRepository(root)
	if err == nil && repo != nil {
		m.repoIgnore = repo
	}
	m.defaultIgnore = gitignore.New(
		strings.NewReader(strings.Join(DefaultIgnorePatterns, "\n")), root, nil)
	return m
}

// TestShouldIgnoreMatchesFullLibrary runs the production decision path
// against the reference (all-library defaults, fast tier off) over the probe
// corpus plus representative fixture paths.
func TestShouldIgnoreMatchesFullLibrary(t *testing.T) {
	root := t.TempDir()
	fast, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	ref := newReferenceMatcher(t, root, true, true)

	seen := map[string]bool{}
	paths := []probe{
		{rel: "src/main.go", isDir: false},
		{rel: "node_modules/pkg/index.js", isDir: false},
		{rel: "node_modules", isDir: true},
		{rel: "build/output.o", isDir: false},
		{rel: "build", isDir: true},
		{rel: "dist", isDir: true},
		{rel: "docs/_build", isDir: true},
		{rel: "sub/docs/_build", isDir: true},
		{rel: "config/app.yaml", isDir: false},
		{rel: "pkg/mod/cache.zip", isDir: false},
		{rel: ".hidden/file.txt", isDir: false},
		{rel: ".git/objects/aa/blob", isDir: false},
		{rel: "code.pyc", isDir: false},
		{rel: "lib/code.pyc", isDir: false},
		{rel: "site", isDir: true},
		{rel: "web/site", isDir: true},
		{rel: "npm-debug.log", isDir: false},
		{rel: "logs/npm-debug.log.1", isDir: false},
		{rel: "src/app.sublime-project", isDir: false},
		{rel: "__pycache__", isDir: true},
		{rel: "deep/nested/__pycache__", isDir: true},
	}
	for _, pattern := range DefaultIgnorePatterns {
		for _, p := range probesFor(pattern) {
			key := p.rel + "\x00" + string(rune(boolToInt(p.isDir)))
			if !seen[key] {
				seen[key] = true
				paths = append(paths, p)
			}
		}
	}
	for _, p := range paths {
		got := fast.ShouldIgnore(p.rel, p.isDir)
		want := ref.ShouldIgnore(p.rel, p.isDir)
		if got != want {
			t.Errorf("ShouldIgnore(%q, dir=%v) = %v, reference = %v", p.rel, p.isDir, got, want)
		}
	}
}

// TestNewWithMissingRootDoesNotPanic guards a latent crash: when the root
// directory does not exist, the library's repository loader fails and the
// fallback matcher used to be built from a nil reader, which segfaults inside
// the lexer. New must return a usable matcher instead.
func TestNewWithMissingRootDoesNotPanic(t *testing.T) {
	m, err := New("/nonexistent-sift-root-does-not-exist")
	if err != nil {
		t.Fatalf("New on a missing root returned an error: %v", err)
	}
	if m == nil {
		t.Fatal("New returned a nil matcher for a missing root")
	}
	// The matcher must still answer decisions without panicking.
	if m.ShouldIgnore("src/main.go", false) {
		t.Error("ordinary source path unexpectedly ignored with a missing root")
	}
	if !m.ShouldIgnore("node_modules", true) {
		t.Error("node_modules should still be ignored by the default tier")
	}
}

// TestVisibilityThenIgnoreAgreeWithReference exercises the memoized repo tier
// through the picker's real call order: ClassifyVisibility first (metadata
// pass), then ShouldIgnore (content pass), for the same paths. Both must
// match the reference matcher, proving the memo does not leak state across
// the two enquiry shapes.
func TestVisibilityThenIgnoreAgreeWithReference(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		".git/config":      "git config",
		".gitignore":       "*.log\nbuild/\n!keep.log\n",
		"debug.log":        "debug",
		"keep.log":         "keep",
		"main.go":          "package main",
		"build/out.o":      "obj",
		"sub/nested.log":   "nested",
		"sub/mod/keep.log": "kept",
	})
	// Picker mode: hidden and git rules are not filters, only metadata.
	m, err := New(root, WithHiddenIgnore(false), WithGitIgnore(false))
	if err != nil {
		t.Fatal(err)
	}
	ref := newReferenceMatcher(t, root, false, false)

	paths := []probe{
		{rel: "debug.log", isDir: false},
		{rel: "keep.log", isDir: false},
		{rel: "main.go", isDir: false},
		{rel: "build", isDir: true},
		{rel: "build/out.o", isDir: false},
		{rel: "sub/nested.log", isDir: false},
		{rel: "sub/mod/keep.log", isDir: false},
		{rel: "sub", isDir: true},
		{rel: ".git/config", isDir: false},
	}
	for _, p := range paths {
		// Metadata pass, then again (repeat query must be stable).
		v1 := m.ClassifyVisibility(p.rel, p.isDir)
		v2 := m.ClassifyVisibility(p.rel, p.isDir)
		if v1 != v2 {
			t.Errorf("ClassifyVisibility(%q, dir=%v) unstable: %+v then %+v", p.rel, p.isDir, v1, v2)
		}
		// Content pass after the metadata pass: the memo must not change
		// the answer.
		got := m.ShouldIgnore(p.rel, p.isDir)
		want := ref.ShouldIgnore(p.rel, p.isDir)
		if got != want {
			t.Errorf("ShouldIgnore(%q, dir=%v) = %v, reference = %v", p.rel, p.isDir, got, want)
		}
		if p.rel == ".git/config" && !v1.ProtectedGit {
			t.Errorf(".git/config should be protected, got %+v", v1)
		}
	}
}

func boolToInt(b bool) rune {
	if b {
		return 'd'
	}
	return 'f'
}
