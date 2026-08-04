package lang

import "strings"

type jsDriver struct{}

func init() { Register(jsDriver{}) }

func (jsDriver) ID() ID               { return JavaScript }
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
func (jsDriver) Classify(path, filename string) Classification {
	switch {
	case strings.Contains(path, "/__tests__/") || strings.HasPrefix(path, "__tests__/") ||
		strings.HasSuffix(filename, ".test.js") || strings.HasSuffix(filename, ".spec.js"):
		return Classification{Role: RoleTest, Adjustment: -0.12, Confidence: 0.95, Reason: "JavaScript test file"}
	case filename == "index.js" || filename == "main.js" || strings.HasPrefix(path, "bin/"):
		return Classification{Role: RoleEntrypoint, Adjustment: 0.16, Confidence: 0.85, Reason: "JavaScript entrypoint"}
	case strings.HasSuffix(filename, ".config.js") || strings.HasSuffix(filename, ".config.cjs"):
		return Classification{Role: RoleConfig, Adjustment: 0.08, Confidence: 0.90, Reason: "JavaScript configuration"}
	}
	return Classification{Role: RoleImpl, Confidence: 0.50, Reason: "JavaScript source"}
}
