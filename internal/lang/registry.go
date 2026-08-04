package lang

import (
	"path/filepath"
	"strings"
	"sync"
)

var (
	mu    sync.RWMutex
	byID  = map[ID]Language{}
	byExt = map[string]Language{}
)

// Register adds a language to the global registry. Registering the same ID
// replaces the previous entry.
func Register(l Language) {
	mu.Lock()
	defer mu.Unlock()
	byID[l.ID()] = l
	for _, ext := range l.Extensions() {
		byExt[ext] = l
	}
}

// ByID returns the language registered under id.
func ByID(id ID) (Language, bool) {
	mu.RLock()
	defer mu.RUnlock()
	l, ok := byID[id]
	return l, ok
}

// ForPath resolves the language for a file by extension.
func ForPath(path string) (Language, bool) {
	ext := strings.ToLower(filepath.Ext(path))
	mu.RLock()
	defer mu.RUnlock()
	l, ok := byExt[ext]
	return l, ok
}

// Classify applies shared conventions and then the language driver rules.
func Classify(path string) Classification {
	norm := strings.ToLower(filepath.ToSlash(path))
	base := strings.ToLower(filepath.Base(norm))
	ext := strings.ToLower(filepath.Ext(base))

	if c, ok := common(norm, base, ext); ok {
		return c
	}
	if l, ok := ForPath(norm); ok {
		return l.Classify(norm, base)
	}
	return Classification{Role: RoleUnknown}
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
