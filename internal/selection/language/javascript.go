package language

import "strings"

func javascriptFile(path, base string) Classification {
	switch {
	case strings.Contains(path, "/__tests__/") || strings.HasPrefix(path, "__tests__/") || strings.HasSuffix(base, ".test.js") || strings.HasSuffix(base, ".spec.js"):
		return Classification{Role: RoleTest, Adjustment: -0.12, Confidence: 0.95, Reason: "JavaScript test file"}
	case base == "index.js" || base == "main.js" || strings.HasPrefix(path, "bin/"):
		return Classification{Role: RoleEntrypoint, Adjustment: 0.16, Confidence: 0.85, Reason: "JavaScript entrypoint"}
	case strings.HasSuffix(base, ".config.js") || strings.HasSuffix(base, ".config.cjs"):
		return Classification{Role: RoleConfig, Adjustment: 0.08, Confidence: 0.90, Reason: "JavaScript configuration"}
	}
	return Classification{Role: RoleImpl, Confidence: 0.50, Reason: "JavaScript source"}
}
