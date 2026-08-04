package lang

import "strings"

type goDriver struct{}

func init() { Register(goDriver{}) }

func (goDriver) ID() ID               { return Go }
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
func (goDriver) Classify(path, filename string) Classification {
	switch {
	case filename == "main.go" || strings.HasPrefix(path, "cmd/"):
		return Classification{Role: RoleEntrypoint, Adjustment: 0.18, Confidence: 0.90, Reason: "Go entrypoint/command"}
	case strings.HasSuffix(filename, "_test.go"):
		return Classification{Role: RoleTest, Adjustment: -0.12, Confidence: 0.98, Reason: "Go test file"}
	case strings.HasSuffix(filename, "_mock.go") || strings.HasPrefix(filename, "mock_"):
		return Classification{Role: RoleMock, Adjustment: -0.18, Confidence: 0.95, Reason: "Go mock"}
	case strings.HasPrefix(path, "internal/"):
		return Classification{Role: RoleImpl, Adjustment: 0.04, Confidence: 0.75, Reason: "Go internal implementation"}
	case strings.HasSuffix(filename, ".gen.go") || strings.HasSuffix(filename, "_gen.go") || strings.HasSuffix(filename, ".pb.go"):
		return Classification{Role: RoleGenerated, Adjustment: -0.35, Confidence: 0.95, Reason: "generated Go source"}
	}
	return Classification{Role: RoleImpl, Confidence: 0.50, Reason: "Go source"}
}
