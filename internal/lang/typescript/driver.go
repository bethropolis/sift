package langts

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bethropolis/sift/internal/lang/registry"
	"github.com/bethropolis/sift/internal/lang/types"
)

type tsDriver struct{}
type tsxDriver struct{}

func init() {
	registry.Register(tsDriver{})
	registry.Register(tsxDriver{})
}

func (tsDriver) ID() types.ID         { return types.TypeScript }
func (tsDriver) Name() string         { return "TypeScript" }
func (tsDriver) Extensions() []string { return []string{".ts"} }
func (tsDriver) ShouldSkipSmart(path, filename string, content []byte) (bool, string) {
	return false, ""
}
func (tsDriver) Classify(path, filename string) types.Classification {
	return classifyTS(path, filename)
}

func (tsxDriver) ID() types.ID         { return types.TSX }
func (tsxDriver) Name() string         { return "TSX" }
func (tsxDriver) Extensions() []string { return []string{".tsx"} }
func (tsxDriver) ShouldSkipSmart(path, filename string, content []byte) (bool, string) {
	return false, ""
}
func (tsxDriver) Classify(path, filename string) types.Classification {
	return classifyTS(path, filename)
}

func classifyTS(path, filename string) types.Classification {
	switch {
	case strings.HasSuffix(filename, ".test.ts") || strings.HasSuffix(filename, ".test.tsx") ||
		strings.HasSuffix(filename, ".spec.ts") || strings.HasSuffix(filename, ".spec.tsx") ||
		strings.Contains(path, "/__tests__/"):
		return types.Classification{Role: types.RoleTest, Adjustment: -0.12, Confidence: 0.96, Reason: "TypeScript test file"}
	case strings.HasSuffix(filename, ".d.ts"):
		return types.Classification{Role: types.RoleAPI, Adjustment: 0.12, Confidence: 0.95, Reason: "TypeScript declaration API"}
	case filename == "index.ts" || filename == "index.tsx" || filename == "main.ts" || filename == "main.tsx" ||
		filename == "app.ts" || filename == "app.tsx" || filename == "server.ts" ||
		filename == "route.ts" || filename == "route.tsx" || filename == "page.tsx" || filename == "layout.tsx" ||
		filename == "middleware.ts" || filename == "middleware.tsx" ||
		strings.HasPrefix(path, "bin/"):
		return types.Classification{Role: types.RoleEntrypoint, Adjustment: 0.16, Confidence: 0.85, Reason: "TypeScript entrypoint/route"}
	case strings.HasSuffix(filename, ".config.ts") || strings.HasSuffix(filename, ".config.mts"):
		return types.Classification{Role: types.RoleConfig, Adjustment: 0.08, Confidence: 0.90, Reason: "TypeScript configuration"}
	}
	return types.Classification{Role: types.RoleImpl, Confidence: 0.50, Reason: "TypeScript source"}
}

var (
	tsImportFrom = regexp.MustCompile(`\bimport\b[^'"]*?from\s*['"]([^'"]+)['"]`)
	tsImportSide = regexp.MustCompile(`\bimport\s*['"]([^'"]+)['"]`)
	tsImportType = regexp.MustCompile(`\bimport\s+type\s+[^'"]*?from\s*['"]([^'"]+)['"]`)
)

// Imports returns the local import targets of a TS/TSX file. Bare package
// specifiers are external and omitted; relative imports are resolved against
// the importing file's directory.
func (tsDriver) Imports(path, _ string, content []byte) []string { return tsImports(path, content) }

// Imports returns the local import targets of a TSX file. See tsDriver.
func (tsxDriver) Imports(path, _ string, content []byte) []string { return tsImports(path, content) }

func tsImports(path string, content []byte) []string {
	dir := filepath.ToSlash(filepath.Dir(path))
	var targets []string
	add := func(spec string) {
		if strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") {
			targets = append(targets, filepath.ToSlash(filepath.Clean(filepath.Join(dir, spec))))
			return
		}
		// Workspace path aliases ("@/" and "~/") resolve from the repo root.
		if rest, ok := strings.CutPrefix(spec, "@/"); ok {
			targets = append(targets, filepath.ToSlash(filepath.Clean(rest)))
			return
		}
		if rest, ok := strings.CutPrefix(spec, "~/"); ok {
			targets = append(targets, filepath.ToSlash(filepath.Clean(rest)))
		}
	}
	for _, m := range tsImportFrom.FindAllSubmatch(content, -1) {
		add(string(m[1]))
	}
	for _, m := range tsImportType.FindAllSubmatch(content, -1) {
		add(string(m[1]))
	}
	for _, m := range tsImportSide.FindAllSubmatch(content, -1) {
		add(string(m[1]))
	}
	return targets
}
