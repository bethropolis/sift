package language

import "strings"

// RuleGo filters generated Go artifacts: lockfiles, protobuf bindings,
// codegen output, and mocks.
func RuleGo(path, filename string, content []byte) bool {
	if filename == "go.sum" {
		return true
	}
	if strings.HasSuffix(filename, ".pb.go") || strings.HasSuffix(filename, "_pb.go") {
		return true
	}
	if strings.HasSuffix(filename, "_gen.go") || strings.HasSuffix(filename, ".gen.go") {
		return true
	}
	if strings.HasPrefix(filename, "mock_") || strings.HasSuffix(filename, "_mock.go") {
		return true
	}
	return false
}
