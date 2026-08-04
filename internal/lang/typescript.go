package lang

import "strings"

type tsDriver struct{}
type tsxDriver struct{}

func init() {
	Register(tsDriver{})
	Register(tsxDriver{})
}

func (tsDriver) ID() ID               { return TypeScript }
func (tsDriver) Name() string         { return "TypeScript" }
func (tsDriver) Extensions() []string { return []string{".ts"} }
func (tsDriver) ShouldSkipSmart(path, filename string, content []byte) (bool, string) {
	return false, ""
}
func (tsDriver) Classify(path, filename string) Classification { return classifyTS(path, filename) }

func (tsxDriver) ID() ID               { return TSX }
func (tsxDriver) Name() string         { return "TSX" }
func (tsxDriver) Extensions() []string { return []string{".tsx"} }
func (tsxDriver) ShouldSkipSmart(path, filename string, content []byte) (bool, string) {
	return false, ""
}
func (tsxDriver) Classify(path, filename string) Classification { return classifyTS(path, filename) }

func classifyTS(path, filename string) Classification {
	switch {
	case strings.HasSuffix(filename, ".test.ts") || strings.HasSuffix(filename, ".test.tsx") ||
		strings.HasSuffix(filename, ".spec.ts") || strings.HasSuffix(filename, ".spec.tsx") ||
		strings.Contains(path, "/__tests__/"):
		return Classification{Role: RoleTest, Adjustment: -0.12, Confidence: 0.96, Reason: "TypeScript test file"}
	case strings.HasSuffix(filename, ".d.ts"):
		return Classification{Role: RoleAPI, Adjustment: 0.12, Confidence: 0.95, Reason: "TypeScript declaration API"}
	case filename == "index.ts" || filename == "index.tsx" || filename == "main.ts" || filename == "main.tsx" || strings.HasPrefix(path, "bin/"):
		return Classification{Role: RoleEntrypoint, Adjustment: 0.16, Confidence: 0.85, Reason: "TypeScript entrypoint"}
	case strings.HasSuffix(filename, ".config.ts"):
		return Classification{Role: RoleConfig, Adjustment: 0.08, Confidence: 0.90, Reason: "TypeScript configuration"}
	}
	return Classification{Role: RoleImpl, Confidence: 0.50, Reason: "TypeScript source"}
}
