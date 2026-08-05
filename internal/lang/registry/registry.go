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

	if c, ok := common(norm, base, ext); ok {
		return c
	}
	if l, ok := ForPath(norm); ok {
		return l.Classify(norm, base)
	}
	return types.Classification{Role: types.RoleUnknown}
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
