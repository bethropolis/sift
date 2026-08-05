package langjs

import (
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
	case strings.HasSuffix(filename, ".config.js") || strings.HasSuffix(filename, ".config.cjs"):
		return types.Classification{Role: types.RoleConfig, Adjustment: 0.08, Confidence: 0.90, Reason: "JavaScript configuration"}
	}
	return types.Classification{Role: types.RoleImpl, Confidence: 0.50, Reason: "JavaScript source"}
}
