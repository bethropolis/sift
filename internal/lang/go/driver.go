package langgo

import (
	"path/filepath"
	"strings"

	"github.com/bethropolis/sift/internal/lang/registry"
	"github.com/bethropolis/sift/internal/lang/types"
)

type goDriver struct{}

func init() { registry.Register(goDriver{}) }

func (goDriver) ID() types.ID         { return types.Go }
func (goDriver) Name() string         { return "Go" }
func (goDriver) Extensions() []string { return []string{".go"} }
func (goDriver) ShouldSkipSmart(path, filename string, content []byte) (bool, string) {
	const reason = "Matched smart language filter"
	if filename == "go.sum" {
		return true, reason
	}
	if strings.HasSuffix(filename, ".pb.go") || strings.HasSuffix(filename, "_pb.go") ||
		strings.HasSuffix(filename, "_gen.go") || strings.HasSuffix(filename, ".gen.go") {
		return true, reason
	}
	if strings.HasPrefix(filename, "mock_") || strings.HasSuffix(filename, "_mock.go") {
		return true, reason
	}
	return false, ""
}
func (goDriver) Classify(path, filename string) types.Classification {
	switch {
	case strings.HasSuffix(filename, "_test.go"):
		return types.Classification{Role: types.RoleTest, Adjustment: -0.12, Confidence: 0.98, Reason: "Go test file"}
	case strings.HasSuffix(filename, "_mock.go") || strings.HasPrefix(filename, "mock_"):
		return types.Classification{Role: types.RoleMock, Adjustment: -0.18, Confidence: 0.95, Reason: "Go mock"}
	case strings.HasSuffix(filename, ".gen.go") || strings.HasSuffix(filename, "_gen.go") || strings.HasSuffix(filename, ".pb.go"):
		return types.Classification{Role: types.RoleGenerated, Adjustment: -0.35, Confidence: 0.95, Reason: "generated Go source"}
	case filename == "main.go" || strings.HasPrefix(path, "cmd/"):
		return types.Classification{Role: types.RoleEntrypoint, Adjustment: 0.18, Confidence: 0.90, Reason: "Go entrypoint/command"}
	case strings.HasPrefix(path, "internal/"):
		return types.Classification{Role: types.RoleImpl, Adjustment: 0.04, Confidence: 0.75, Reason: "Go internal implementation"}
	}
	return types.Classification{Role: types.RoleImpl, Confidence: 0.50, Reason: "Go source"}
}

// Imports returns the import targets of a Go file as repo-relative path
// prefixes. Module-rooted imports are stripped of the module prefix; standard
// library and third-party (dotless/other-domain) imports are omitted since
// they do not resolve inside the repo.
func (goDriver) Imports(path, moduleRoot string, content []byte) []string {
	var targets []string
	forEachImportSpec(content, func(spec string) {
		target := stripModulePrefix(spec, moduleRoot)
		if target == "" {
			return
		}
		// Drop file suffix if present; a package import resolves to a dir.
		if ext := filepath.Ext(target); ext != "" {
			target = strings.TrimSuffix(target, ext)
		}
		targets = append(targets, target)
	})
	return targets
}

// ResolveImports resolves a Go file's imports to confirmed repo-relative
// paths. Go imports name packages (directories), so each target is the
// package dir itself; the caller expands directories to collected files.
func (goDriver) ResolveImports(path, moduleRoot string, content []byte, exists func(string) bool) []string {
	var out []string
	seen := map[string]bool{}
	forEachImportSpec(content, func(spec string) {
		dir := stripModulePrefix(spec, moduleRoot)
		if dir == "" {
			return
		}
		// Drop file suffix if present; a package import resolves to a dir.
		if ext := filepath.Ext(dir); ext != "" {
			dir = strings.TrimSuffix(dir, ext)
		}
		dir = filepath.ToSlash(dir)
		if seen[dir] || !exists(dir) {
			return
		}
		seen[dir] = true
		out = append(out, dir)
	})
	return out
}

// forEachImportSpec calls fn for every quoted import path in a Go file,
// handling single-line and parenthesized import blocks.
func forEachImportSpec(content []byte, fn func(spec string)) {
	lines := strings.Split(string(content), "\n")
	inBlock := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "import ("):
			inBlock = true
			line = strings.TrimPrefix(line, "import (")
		case line == ")" && inBlock:
			inBlock = false
			continue
		case strings.HasPrefix(line, "import "):
			line = strings.TrimPrefix(line, "import ")
		}
		if !inBlock && !strings.HasPrefix(line, "import ") && !strings.HasPrefix(line, "\"") {
			continue
		}
		for _, token := range splitQuoted(line) {
			if token != "" {
				fn(token)
			}
		}
	}
}

// splitQuoted extracts every double-quoted string literal from s.
func splitQuoted(s string) []string {
	var out []string
	rest := s
	for {
		start := strings.IndexByte(rest, '"')
		if start == -1 {
			return out
		}
		rest = rest[start+1:]
		end := strings.IndexByte(rest, '"')
		if end == -1 {
			return out
		}
		out = append(out, rest[:end])
		rest = rest[end+1:]
	}
}

// stripModulePrefix removes a Go module path prefix from an import spec,
// returning the repo-relative remainder. It returns "" when the spec is not
// rooted at the module (standard library, third-party, or no module).
func stripModulePrefix(spec, moduleRoot string) string {
	if moduleRoot == "" || spec == moduleRoot {
		return ""
	}
	if strings.HasPrefix(spec, moduleRoot+"/") {
		return strings.TrimPrefix(spec, moduleRoot+"/")
	}
	return ""
}
