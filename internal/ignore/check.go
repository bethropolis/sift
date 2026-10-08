package ignore

import (
	"path/filepath"
)

// Visibility describes presentation-relevant ignore metadata. Explicit and
// custom safety exclusions remain hard filters and are not represented here.
type Visibility struct {
	Hidden       bool
	GitIgnored   bool
	ProtectedGit bool
}

// ClassifyVisibility reports hidden and repository-gitignore metadata without
// applying those matches as filters.
func (m *IgnoreMatcher) ClassifyVisibility(relativePath string, isDir bool) Visibility {
	if m == nil || m.disabled {
		return Visibility{}
	}
	path := filepath.ToSlash(relativePath)
	hidden, git := scanSegments(path)
	visibility := Visibility{Hidden: hidden, ProtectedGit: git}
	if visibility.ProtectedGit {
		return visibility
	}
	// Relative skips the library Match wrapper, which stats the path on
	// every call; the walker already knows whether the entry is a directory.
	// repoIgnored also memoizes the tier so the metadata pass, the content
	// pass, and ShouldIgnore share a single computation.
	if d := m.repoIgnored(path, isDir); d.matched {
		visibility.GitIgnored = d.ignored
	}
	return visibility
}

// ShouldIgnore checks if a file or directory should be ignored
func (m *IgnoreMatcher) ShouldIgnore(relativePath string, isDir bool) bool {
	if m == nil || m.disabled {
		return false
	}

	if relativePath == "" || relativePath == "." {
		return false // Never ignore the root itself
	}

	// One allocation-free segment scan replaces the hidden-basename check,
	// the hidden-parent walk, and the .git component scan. The .git rule
	// applies even when hidden files are visible (picker mode).
	rel := filepath.ToSlash(relativePath)
	hidden, git := scanSegments(rel)
	if git {
		return true
	}
	if m.ignoreHidden && hidden {
		return true
	}

	// Custom ignore patterns first (highest priority). A match is definitive
	// whether positive or negated: --ignore negations must win over every
	// lower tier, including the built-in defaults. Relative takes the path
	// relative to the matcher root and the walker-provided isDir, avoiding
	// the library Match wrapper's filepath.Abs + os.Stat per tier per path.
	if m.customIgnore != nil {
		if match := m.customIgnore.Relative(rel, isDir); match != nil {
			return match.Ignore()
		}
	}

	// Delegate to gitignore library for repo rules. A repo match (including a
	// negation) is definitive and takes precedence over the defaults below.
	// Memoized: this is by far the most expensive tier. The ignoreGit flag
	// gates filtering only; ClassifyVisibility still reports repo metadata
	// when the walker is configured not to filter on it.
	if m.ignoreGit {
		if d := m.repoIgnored(rel, isDir); d.matched {
			return d.ignored
		}
	}

	// Built-in default patterns: the pre-classified fast tier first, then
	// the library fallback for unclassifiable patterns. They are the
	// lowest-priority safety net and only apply when neither custom
	// patterns nor repository rules matched the path. The defaults contain
	// no negations (enforced by the usable flag), so order between the two
	// sub-tiers cannot change the outcome.
	if m.fast != nil && m.fast.usable && m.fast.match(rel, segmentBase(rel), isDir) {
		return true
	}
	if m.defaultIgnore != nil {
		if match := m.defaultIgnore.Relative(rel, isDir); match != nil {
			return match.Ignore()
		}
	}

	return false
}

// repoDecision is the memoized outcome of the repository-ignore tier for one
// path. matched distinguishes "a repo rule decided this path" from "no repo
// rule matched, fall through to the defaults".
type repoDecision struct {
	matched bool
	ignored bool
}

// repoIgnored consults the repository-ignore tier for rel, memoizing the
// answer by path. The library's repository matcher walks the path's ancestor
// directories, allocating a Clean/Split/Join per level; profiling showed it
// dominating ShouldIgnore. The picker asks about the same path from
// ClassifyVisibility (metadata and content passes) and ShouldIgnore, and the
// scan is immutable for the matcher's lifetime, so one computation serves all
// of them. sync.Map keeps concurrent walker workers lock-free.
func (m *IgnoreMatcher) repoIgnored(rel string, isDir bool) repoDecision {
	if m.repoIgnore == nil {
		return repoDecision{}
	}
	if v, ok := m.repoCache.Load(rel); ok {
		if d, ok := v.(cachedRepoDecision); ok && d.dir == isDir {
			return d.repoDecision
		}
	}
	matched, ignored := false, false
	if match := m.repoIgnore.Relative(rel, isDir); match != nil {
		matched = true
		ignored = match.Ignore()
	}
	m.repoCache.Store(rel, cachedRepoDecision{
		repoDecision: repoDecision{matched: matched, ignored: ignored},
		dir:          isDir,
	})
	return repoDecision{matched: matched, ignored: ignored}
}

// cachedRepoDecision pairs the decision with the entry type it was computed
// for, so a file and a same-named directory never share an answer.
type cachedRepoDecision struct {
	repoDecision
	dir bool
}
