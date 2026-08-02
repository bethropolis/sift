// Package language classifies source files using common language conventions.
// Classifications are soft signals for selection; hard filtering remains the
// responsibility of the smart/binary/size filtering pipeline.
package language

import (
	"path/filepath"
	"strings"
)

// Role describes the likely purpose of a file.
type Role string

const (
	RoleUnknown    Role = "unknown"
	RoleEntrypoint Role = "entrypoint"
	RoleAPI        Role = "api"
	RoleImpl       Role = "implementation"
	RoleTest       Role = "test"
	RoleFixture    Role = "fixture"
	RoleMock       Role = "mock"
	RoleConfig     Role = "config"
	RoleSchema     Role = "schema"
	RoleDocs       Role = "docs"
	RoleGenerated  Role = "generated"
	RoleExample    Role = "example"
	RoleVendor     Role = "vendor"
)

// Classification is a soft selection signal.
type Classification struct {
	Role       Role
	Adjustment float64
	Confidence float64
	Reason     string
}

// Classify applies common and language-specific path conventions.
func Classify(path string) Classification {
	norm := strings.ToLower(filepath.ToSlash(path))
	base := strings.ToLower(filepath.Base(norm))
	ext := strings.ToLower(filepath.Ext(base))

	if c, ok := common(norm, base, ext); ok {
		return c
	}
	switch ext {
	case ".go":
		return goFile(norm, base)
	case ".js", ".jsx", ".mjs", ".cjs":
		return javascriptFile(norm, base)
	case ".ts", ".tsx":
		return typescriptFile(norm, base)
	case ".py":
		return pythonFile(norm, base)
	case ".rs":
		return rustFile(norm, base)
	case ".java":
		return javaFile(norm, base)
	case ".kt", ".kts":
		return kotlinFile(norm, base)
	case ".rb":
		return rubyFile(norm, base)
	case ".php":
		return phpFile(norm, base)
	case ".cs":
		return csharpFile(norm, base)
	case ".c", ".h":
		return cFile(norm, base)
	case ".cpp", ".cc", ".cxx", ".hpp":
		return cppFile(norm, base)
	case ".swift":
		return swiftFile(norm, base)
	}
	return Classification{Role: RoleUnknown}
}

func common(path, base, ext string) (Classification, bool) {
	switch {
	case strings.Contains(path, "/vendor/") || strings.HasPrefix(path, "vendor/"):
		return Classification{Role: RoleVendor, Adjustment: -0.20, Confidence: 0.95, Reason: "vendored dependency"}, true
	case strings.Contains(path, "/testdata/") || strings.HasPrefix(path, "testdata/") || strings.Contains(path, "/fixtures/"):
		return Classification{Role: RoleFixture, Adjustment: -0.10, Confidence: 0.90, Reason: "fixture/test data"}, true
	case strings.Contains(path, "/examples/") || strings.HasPrefix(path, "examples/"):
		return Classification{Role: RoleExample, Adjustment: 0.04, Confidence: 0.85, Reason: "example"}, true
	case strings.Contains(path, "/migrations/") || strings.HasPrefix(path, "migrations/"):
		return Classification{Role: RoleSchema, Adjustment: 0.10, Confidence: 0.85, Reason: "migration/schema"}, true
	case ext == ".md" || ext == ".mdx" || base == "readme" || strings.HasPrefix(base, "readme."):
		return Classification{Role: RoleDocs, Adjustment: 0.06, Confidence: 0.90, Reason: "documentation"}, true
	case ext == ".sql" || ext == ".graphql" || ext == ".gql" || ext == ".proto":
		return Classification{Role: RoleSchema, Adjustment: 0.10, Confidence: 0.90, Reason: "schema definition"}, true
	case base == "dockerfile" || base == "makefile" || base == "go.mod" || base == "cargo.toml" || base == "pyproject.toml":
		return Classification{Role: RoleConfig, Adjustment: 0.08, Confidence: 0.90, Reason: "project configuration"}, true
	case strings.Contains(base, ".generated.") || strings.HasSuffix(base, "_generated"+ext) || strings.HasSuffix(base, "_gen"+ext):
		return Classification{Role: RoleGenerated, Adjustment: -0.35, Confidence: 0.90, Reason: "generated naming convention"}, true
	}
	return Classification{}, false
}
