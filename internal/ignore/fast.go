package ignore

import "strings"

// fastRuleKind classifies one default pattern into a cheap string test that
// replaces the library's fnmatch loop for that pattern.
type fastRuleKind uint8

const (
	fastRemainder fastRuleKind = iota // not safely classifiable; library tier
	fastExactName                     // literal matched against the basename
	fastExactPath                     // literal matched against the whole relative path
	fastSuffix                        // "*lit" against the basename
	fastPrefix                        // "lit*" against the basename
	fastContains                      // "*lit*" against the basename
)

// fastRule is one classified default pattern.
type fastRule struct {
	kind    fastRuleKind
	value   string
	dirOnly bool // pattern ended in '/': directories only
}

// match reports whether rel (slash-normalized, relative to the root) with
// basename base satisfies the rule for an entry of type isDir.
func (r fastRule) match(rel, base string, isDir bool) bool {
	if r.dirOnly && !isDir {
		return false
	}
	switch r.kind {
	case fastExactName:
		return base == r.value
	case fastExactPath:
		return rel == r.value
	case fastSuffix:
		return strings.HasSuffix(base, r.value)
	case fastPrefix:
		return strings.HasPrefix(base, r.value)
	case fastContains:
		return strings.Contains(base, r.value)
	default:
		return false
	}
}

// hasGlobMeta reports whether s contains fnmatch wildcards. Literal patterns
// (no metacharacters) reduce to plain equality, which is what lets most
// default patterns skip fnmatch entirely.
func hasGlobMeta(s string) bool {
	return strings.ContainsAny(s, "*?[\\")
}

// classifyOne maps a single gitignore pattern onto a fastRule. It mirrors
// the library's name/path pattern dispatch:
//
//   - "!" negations and unclassifiable wildcards go to the library tier;
//   - a trailing '/' restricts the rule to directories;
//   - a leading '/' or an embedded '/' means the pattern is matched against
//     the whole relative path (the library's path/anchored-name matching
//     without wildcards is plain equality);
//   - otherwise the pattern is matched against the basename, with the
//     glob shapes *lit / lit* / *lit* lowering to suffix/prefix/contains.
//
// Every non-remainder classification is verified against the library by
// TestClassifyOneMatchesLibrary.
func classifyOne(pattern string) fastRule {
	p := pattern
	if p == "" || strings.HasPrefix(p, "!") {
		return fastRule{kind: fastRemainder}
	}
	dirOnly := false
	if strings.HasSuffix(p, "/") {
		if len(p) == 1 {
			return fastRule{kind: fastRemainder}
		}
		p = p[:len(p)-1]
		dirOnly = true
	}
	anchored := false
	if strings.HasPrefix(p, "/") {
		p = p[1:]
		anchored = true
		if p == "" {
			return fastRule{kind: fastRemainder}
		}
	}
	if anchored || strings.Contains(p, "/") {
		if hasGlobMeta(p) {
			return fastRule{kind: fastRemainder}
		}
		return fastRule{kind: fastExactPath, value: p, dirOnly: dirOnly}
	}
	if !hasGlobMeta(p) {
		return fastRule{kind: fastExactName, value: p, dirOnly: dirOnly}
	}
	switch {
	case strings.HasPrefix(p, "*") && strings.HasSuffix(p, "*"):
		mid := p[1 : len(p)-1]
		if mid != "" && !hasGlobMeta(mid) {
			return fastRule{kind: fastContains, value: mid, dirOnly: dirOnly}
		}
	case strings.HasPrefix(p, "*"):
		rest := p[1:]
		if rest != "" && !hasGlobMeta(rest) {
			return fastRule{kind: fastSuffix, value: rest, dirOnly: dirOnly}
		}
	case strings.HasSuffix(p, "*"):
		pre := p[:len(p)-1]
		if pre != "" && !hasGlobMeta(pre) {
			return fastRule{kind: fastPrefix, value: pre, dirOnly: dirOnly}
		}
	}
	return fastRule{kind: fastRemainder}
}

// Rule flags for the exact-match sets: a literal matches every entry type,
// while its "name/" form matches directories only. Both forms may coexist
// (the default list contains e.g. build and build/).
const (
	flagAny uint8 = 1 << iota
	flagDir
)

// fastEntry pairs a glob-literal with its directory-only flag.
type fastEntry struct {
	value   string
	dirOnly bool
}

// defaultFastSet is the DefaultIgnorePatterns split into O(1)/O(k) string
// tests plus the patterns that still need the library's fnmatch loop.
type defaultFastSet struct {
	exactNames map[string]uint8
	exactPaths map[string]uint8
	suffixes   []fastEntry
	prefixes   []fastEntry
	contains   []fastEntry
	remainder  []string
	// usable is false when a negation appears in the pattern list: fast
	// matches could then override a later "!pattern", so the whole fast
	// tier is disabled and every pattern stays with the library.
	usable bool
}

// buildDefaultFastSet classifies patterns once at package init.
func buildDefaultFastSet(patterns []string) *defaultFastSet {
	s := &defaultFastSet{
		exactNames: make(map[string]uint8, len(patterns)),
		exactPaths: make(map[string]uint8, len(patterns)),
		usable:     true,
	}
	for _, p := range patterns {
		if strings.HasPrefix(p, "!") {
			// A negation makes match order significant; keep everything in
			// the library tier so reverse-order semantics stay intact.
			s.usable = false
		}
		rule := classifyOne(p)
		if rule.kind == fastRemainder {
			s.remainder = append(s.remainder, p)
			continue
		}
		if !s.usable {
			continue // negation seen; the library fallback list covers all
		}
		flags := flagAny
		if rule.dirOnly {
			flags = flagDir
		}
		switch rule.kind {
		case fastExactName:
			s.exactNames[rule.value] |= flags
		case fastExactPath:
			s.exactPaths[rule.value] |= flags
		case fastSuffix:
			s.suffixes = append(s.suffixes, fastEntry{rule.value, rule.dirOnly})
		case fastPrefix:
			s.prefixes = append(s.prefixes, fastEntry{rule.value, rule.dirOnly})
		case fastContains:
			s.contains = append(s.contains, fastEntry{rule.value, rule.dirOnly})
		}
	}
	if !s.usable {
		// Fall back to the complete list so the library sees every pattern.
		s.remainder = append([]string(nil), patterns...)
	}
	return s
}

// match runs the fast string tiers against one path. A false result means
// "the fast tiers did not decide", never "not ignored": the caller must then
// consult the library remainder.
func (s *defaultFastSet) match(rel, base string, isDir bool) bool {
	if f := s.exactNames[base]; f != 0 {
		if f&flagAny != 0 || (isDir && f&flagDir != 0) {
			return true
		}
	}
	if f := s.exactPaths[rel]; f != 0 {
		if f&flagAny != 0 || (isDir && f&flagDir != 0) {
			return true
		}
	}
	for _, e := range s.suffixes {
		if (!e.dirOnly || isDir) && strings.HasSuffix(base, e.value) {
			return true
		}
	}
	for _, e := range s.prefixes {
		if (!e.dirOnly || isDir) && strings.HasPrefix(base, e.value) {
			return true
		}
	}
	for _, e := range s.contains {
		if (!e.dirOnly || isDir) && strings.Contains(base, e.value) {
			return true
		}
	}
	return false
}

// fastDefaults is the classified form of DefaultIgnorePatterns. It is a
// package-level var so the pattern list is classified exactly once; Go's
// dependency-ordered initialization guarantees DefaultIgnorePatterns is
// populated first.
var fastDefaults = buildDefaultFastSet(DefaultIgnorePatterns)

// segmentBase returns the last slash-separated component without allocating.
func segmentBase(rel string) string {
	if i := strings.LastIndexByte(rel, '/'); i >= 0 {
		return rel[i+1:]
	}
	return rel
}

// scanSegments walks path's slash-separated segments without allocating and
// reports whether any segment starts with '.' (hidden) or equals ".git". It
// replaces the split-based hidden-parent walk and git-dir scan that used to
// run (allocating) for every directory entry.
func scanSegments(path string) (hidden, git bool) {
	for i := 0; i < len(path); {
		seg := path[i:]
		j := strings.IndexByte(seg, '/')
		if j >= 0 {
			seg = seg[:j]
		}
		if seg != "" {
			if seg[0] == '.' {
				hidden = true
			}
			if seg == ".git" {
				git = true
			}
		}
		if j < 0 {
			break
		}
		i += j + 1
	}
	return hidden, git
}
