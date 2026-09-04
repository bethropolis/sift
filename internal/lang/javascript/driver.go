package langjs

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bethropolis/sift/internal/lang/registry"
	"github.com/bethropolis/sift/internal/lang/types"
)

type jsDriver struct{}

func init() { registry.Register(jsDriver{}) }

func (jsDriver) ID() types.ID         { return types.JavaScript }
func (jsDriver) Name() string         { return "JavaScript" }
func (jsDriver) Extensions() []string { return []string{".js", ".jsx", ".mjs", ".cjs"} }
func (jsDriver) ShouldSkipSmart(path, filename string, content []byte) (bool, string) {
	const reason = "Matched smart language filter"
	if filename == "package-lock.json" || filename == "yarn.lock" || filename == "pnpm-lock.yaml" || filename == "bun.lockb" {
		return true, reason
	}
	if strings.HasSuffix(filename, ".min.js") || strings.HasSuffix(filename, ".min.css") ||
		strings.HasSuffix(filename, ".map") || strings.HasSuffix(filename, ".bundle.js") {
		return true, reason
	}
	return false, ""
}
func (jsDriver) Classify(path, filename string) types.Classification {
	switch {
	case strings.Contains(path, "/__tests__/") || strings.HasPrefix(path, "__tests__/") ||
		strings.HasSuffix(filename, ".test.js") || strings.HasSuffix(filename, ".spec.js"):
		return types.Classification{Role: types.RoleTest, Adjustment: -0.12, Confidence: 0.95, Reason: "JavaScript test file"}
	case filename == "index.js" || filename == "main.js" || strings.HasPrefix(path, "bin/"):
		return types.Classification{Role: types.RoleEntrypoint, Adjustment: 0.16, Confidence: 0.85, Reason: "JavaScript entrypoint"}
	case strings.HasSuffix(filename, ".config.js") || strings.HasSuffix(filename, ".config.cjs") || strings.HasSuffix(filename, ".config.mjs"):
		return types.Classification{Role: types.RoleConfig, Adjustment: 0.08, Confidence: 0.90, Reason: "JavaScript configuration"}
	}
	return types.Classification{Role: types.RoleImpl, Confidence: 0.50, Reason: "JavaScript source"}
}

var (
	jsImportFrom = regexp.MustCompile(`\bimport\b[^'"]*?from\s*['"]([^'"]+)['"]`)
	jsImportSide = regexp.MustCompile(`\bimport\s*['"]([^'"]+)['"]`)
	jsRequire    = regexp.MustCompile(`\brequire\s*\(\s*['"]([^'"]+)['"]\s*\)`)
)

// Imports returns the local import targets of a JS/JSX file. Bare package
// specifiers (no leading ".") are external and omitted; relative imports are
// resolved against the importing file's directory.
func (jsDriver) Imports(path, _ string, content []byte) []string {
	dir := filepath.ToSlash(filepath.Dir(path))
	var targets []string
	add := func(spec string) {
		if !strings.HasPrefix(spec, "./") && !strings.HasPrefix(spec, "../") {
			return
		}
		targets = append(targets, filepath.ToSlash(filepath.Clean(filepath.Join(dir, spec))))
	}
	for _, m := range jsImportFrom.FindAllSubmatch(content, -1) {
		add(string(m[1]))
	}
	for _, m := range jsImportSide.FindAllSubmatch(content, -1) {
		add(string(m[1]))
	}
	for _, m := range jsRequire.FindAllSubmatch(content, -1) {
		add(string(m[1]))
	}
	return targets
}
