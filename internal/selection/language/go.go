package language

import "strings"

func goFile(path, base string) Classification {
	switch {
	case base == "main.go" || strings.HasPrefix(path, "cmd/"):
		return Classification{Role: RoleEntrypoint, Adjustment: 0.18, Confidence: 0.90, Reason: "Go entrypoint/command"}
	case strings.HasSuffix(base, "_test.go"):
		return Classification{Role: RoleTest, Adjustment: -0.12, Confidence: 0.98, Reason: "Go test file"}
	case strings.HasSuffix(base, "_mock.go") || strings.HasPrefix(base, "mock_"):
		return Classification{Role: RoleMock, Adjustment: -0.18, Confidence: 0.95, Reason: "Go mock"}
	case strings.HasPrefix(path, "internal/"):
		return Classification{Role: RoleImpl, Adjustment: 0.04, Confidence: 0.75, Reason: "Go internal implementation"}
	case strings.HasSuffix(base, ".gen.go") || strings.HasSuffix(base, "_gen.go") || strings.HasSuffix(base, ".pb.go"):
		return Classification{Role: RoleGenerated, Adjustment: -0.35, Confidence: 0.95, Reason: "generated Go source"}
	}
	return Classification{Role: RoleImpl, Adjustment: 0.0, Confidence: 0.50, Reason: "Go source"}
}
