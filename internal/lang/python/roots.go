package langpy

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var pySourceRoots sync.Map // absolute root -> []string

var pyWhereRe = regexp.MustCompile(`(?m)^\s*where\s*=\s*\[([^\]]*)\]`)
var pyQuotedRe = regexp.MustCompile(`"([^"]+)"|'([^']+)'`)

// pythonSourceRoots returns the repo-relative source roots configured by
// pyproject.toml (setuptools/hatch package discovery), falling back to "src"
// when that directory exists.
func pythonSourceRoots(root string) []string {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	if cached, ok := pySourceRoots.Load(abs); ok {
		return cached.([]string)
	}
	r, err := os.OpenRoot(abs)
	if err != nil {
		return nil
	}
	defer r.Close()

	seen := map[string]bool{}
	var roots []string
	add := func(p string) {
		p = strings.TrimPrefix(filepath.ToSlash(filepath.Clean(filepath.FromSlash(p))), "./")
		if p == "." || p == "" || seen[p] || !filepath.IsLocal(filepath.FromSlash(p)) {
			return
		}
		seen[p] = true
		roots = append(roots, p)
	}
	if info, err := r.Stat("src"); err == nil && info.IsDir() {
		add("src")
	}
	if data, err := r.ReadFile("pyproject.toml"); err == nil {
		for _, m := range pyWhereRe.FindAllSubmatch(data, -1) {
			for _, q := range pyQuotedRe.FindAllSubmatch(m[1], -1) {
				value := string(q[1])
				if value == "" {
					value = string(q[2])
				}
				add(value)
			}
		}
	}
	pySourceRoots.Store(abs, roots)
	return roots
}

// ResolveImportsInRoot resolves Python imports, including absolute imports
// rooted at a src/ or pyproject-configured package directory.
func (pyDriver) ResolveImportsInRoot(path, root, moduleRoot string, content []byte, exists func(string) bool) []string {
	prefixes := pyDriver{}.Imports(path, moduleRoot, content)
	sourceRoots := pythonSourceRoots(root)
	out := make([]string, 0, len(prefixes))
	seen := map[string]bool{}
	add := func(p string) {
		for _, cand := range pyCandidates(p) {
			if !seen[cand] && exists(cand) {
				seen[cand] = true
				out = append(out, cand)
			}
		}
	}
	relative := relativeTargets(path, content)
	for _, prefix := range prefixes {
		add(prefix)
		// Relative imports already carry their directory; only absolute
		// imports need the source-root variants.
		if relative[prefix] {
			continue
		}
		for _, sr := range sourceRoots {
			add(filepath.ToSlash(filepath.Join(sr, prefix)))
		}
	}
	return out
}

// relativeTargets returns the prefixes produced by relative "from .x import"
// statements, which must not be rebased onto src/.
func relativeTargets(path string, content []byte) map[string]bool {
	dir := filepath.ToSlash(filepath.Dir(path))
	out := map[string]bool{}
	for _, m := range pyFrom.FindAllSubmatch(content, -1) {
		raw := string(m[1])
		if !strings.HasPrefix(raw, ".") {
			continue
		}
		dots := 0
		for dots < len(raw) && raw[dots] == '.' {
			dots++
		}
		cur := dir
		for i := 1; i < dots; i++ {
			cur = filepath.ToSlash(filepath.Dir(cur))
		}
		target := filepath.ToSlash(filepath.Clean(filepath.Join(cur, strings.ReplaceAll(raw[dots:], ".", "/"))))
		if target != "." && target != "" {
			out[target] = true
		}
	}
	return out
}

func pyCandidates(base string) []string {
	base = strings.TrimSuffix(filepath.ToSlash(base), "/")
	return []string{
		base,
		base + ".py",
		base + ".pyi",
		filepath.ToSlash(filepath.Join(base, "__init__.py")),
	}
}
