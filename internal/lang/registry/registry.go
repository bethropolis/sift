// Package registry owns the global language lookup tables and the shared
// path-convention rules. Drivers register themselves here; the facade
// (internal/lang) is what external consumers import.
package registry

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/bethropolis/sift/internal/lang/types"
)

var (
	mu    sync.RWMutex
	byID  = map[types.ID]types.Language{}
	byExt = map[string]types.Language{}
)

// Register adds a language to the registry. Registering the same ID replaces
// the previous entry.
func Register(l types.Language) {
	mu.Lock()
	defer mu.Unlock()
	byID[l.ID()] = l
	for _, ext := range l.Extensions() {
		byExt[ext] = l
	}
}

// ByID returns the language registered under id.
func ByID(id types.ID) (types.Language, bool) {
	mu.RLock()
	defer mu.RUnlock()
	l, ok := byID[id]
	return l, ok
}

// ForPath resolves the language for a file by extension.
func ForPath(path string) (types.Language, bool) {
	ext := strings.ToLower(filepath.Ext(path))
	mu.RLock()
	defer mu.RUnlock()
	l, ok := byExt[ext]
	return l, ok
}

// Classify applies shared conventions and then the language driver rules.
func Classify(path string) types.Classification {
	norm := strings.ToLower(filepath.ToSlash(path))
	base := strings.ToLower(filepath.Base(norm))
	ext := strings.ToLower(filepath.Ext(base))

	var c types.Classification
	if cc, ok := common(norm, base, ext); ok {
		c = cc
	} else if l, ok := ForPath(norm); ok {
		c = l.Classify(norm, base)
	} else {
		c = types.Classification{Role: types.RoleUnknown}
	}
	if c.Retention == 0 {
		c.Retention = types.DefaultRetention(c.Role)
	}
	return c
}

// Imports returns the import targets referenced by a file, resolved to
// repo-relative path prefixes. The path's language driver owns the import
// syntax; paths with no registered ImportScanner contribute no fan-in.
func Imports(path, moduleRoot string, content []byte) []string {
	norm := strings.ToLower(filepath.ToSlash(path))
	if l, ok := ForPath(norm); ok {
		if sc, ok := l.(types.ImportScanner); ok {
			return sc.Imports(norm, moduleRoot, content)
		}
	}
	return nil
}

// ResolveImports returns the import targets referenced by a file as
// confirmed, existing repo-relative file paths. The path's language driver
// owns the import syntax: drivers implementing ImportResolver commit to real
// files, while all other paths fall back to a generic expander that runs the
// ImportScanner prefix output through extension/index expansion against
// exists. Nothing regresses for unsupported languages.
func ResolveImports(path, moduleRoot string, content []byte, exists func(string) bool) []string {
	norm := strings.ToLower(filepath.ToSlash(path))
	if l, ok := ForPath(norm); ok {
		if r, ok := l.(types.ImportResolver); ok {
			return r.ResolveImports(norm, moduleRoot, content, exists)
		}
		return expandPrefixes(Imports(norm, moduleRoot, content), exists)
	}
	return nil
}

// expandPrefixes turns loose ImportScanner prefixes into confirmed files:
// try the raw path, then extension fallbacks, then index/directory fallbacks.
func expandPrefixes(prefixes []string, exists func(string) bool) []string {
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		p = strings.TrimSuffix(filepath.ToSlash(p), "/")
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		if exists(p) {
			out = append(out, p)
		}
	}
	for _, prefix := range prefixes {
		prefix = strings.TrimSuffix(filepath.ToSlash(prefix), "/")
		if prefix == "" {
			continue
		}
		add(prefix)
		if filepath.Ext(prefix) == "" {
			for _, ext := range []string{".ts", ".tsx", ".js", ".jsx", ".go", ".py", ".rs", ".dart", ".zig", ".h", ".hpp"} {
				add(prefix + ext)
			}
			for _, index := range []string{"index.ts", "index.tsx", "index.js", "mod.go"} {
				add(prefix + "/" + index)
			}
		}
	}
	return out
}

// ShouldSkipSmart reports whether any registered language's smart rules skip
// the file. Every rule is applied to every path, so cross-language artifacts
// (e.g. package-lock.json) are still caught regardless of the file's own
// language, matching the historical flat rule list behavior.
func ShouldSkipSmart(path, filename string, content []byte) (bool, string) {
	mu.RLock()
	defer mu.RUnlock()
	for _, l := range byID {
		if skip, reason := l.ShouldSkipSmart(path, filename, content); skip {
			return true, reason
		}
	}
	return false, ""
}
