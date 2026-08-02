package language

import "strings"

func typescriptFile(path, base string) Classification {
	switch {
	case strings.HasSuffix(base, ".test.ts") || strings.HasSuffix(base, ".test.tsx") || strings.HasSuffix(base, ".spec.ts") || strings.HasSuffix(base, ".spec.tsx") || strings.Contains(path, "/__tests__/"):
		return Classification{Role: RoleTest, Adjustment: -0.12, Confidence: 0.96, Reason: "TypeScript test file"}
	case strings.HasSuffix(base, ".d.ts"):
		return Classification{Role: RoleAPI, Adjustment: 0.12, Confidence: 0.95, Reason: "TypeScript declaration API"}
	case base == "index.ts" || base == "index.tsx" || base == "main.ts" || base == "main.tsx" || strings.HasPrefix(path, "bin/"):
		return Classification{Role: RoleEntrypoint, Adjustment: 0.16, Confidence: 0.85, Reason: "TypeScript entrypoint"}
	case strings.HasSuffix(base, ".config.ts"):
		return Classification{Role: RoleConfig, Adjustment: 0.08, Confidence: 0.90, Reason: "TypeScript configuration"}
	}
	return Classification{Role: RoleImpl, Confidence: 0.50, Reason: "TypeScript source"}
}
