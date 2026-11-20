package selection

import (
	"regexp"
	"sort"

	"github.com/bethropolis/sift/internal/format"
)

// declSymbol matches a declaration's name across the supported languages:
// func/function/def/class/struct/interface/type/enum/trait/impl/const/let/var,
// including Go method receivers.
var declSymbol = regexp.MustCompile(`(?m)^\s*(?:export\s+|pub\s+)?(?:default\s+)?(?:func|function|def|class|struct|interface|type|enum|trait|impl|const|let|var)\s+(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)`)

// maxSymbolsPerFile bounds the per-file index so one huge generated file does
// not dominate prompt matching.
const maxSymbolsPerFile = 2000

// Symbols returns the declaration names found in a file. Signature text is
// preferred when present because it is smaller and already declaration-only.
func Symbols(file format.FileEntry) []string {
	src := file.SigContent
	if len(src) == 0 {
		src = file.Content
	}
	if len(src) == 0 {
		return nil
	}
	matches := declSymbol.FindAllSubmatch(src, maxSymbolsPerFile)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		name := string(m[1])
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
