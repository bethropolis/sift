package lang

import "strings"

// common applies language-agnostic path conventions before any language rules,
// mirroring the historical shared rules used by selection ranking.
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
	case ext == ".html" || ext == ".css" || ext == ".scss" || ext == ".vue" || ext == ".svelte" || ext == ".astro":
		return Classification{Role: RoleImpl, Adjustment: 0.02, Confidence: 0.75, Reason: "web source"}, true
	case base == "dockerfile" || base == "makefile" || base == "go.mod" || base == "cargo.toml" || base == "pyproject.toml" || base == "package.json" || base == "tsconfig.json" || strings.HasPrefix(base, "vite.config.") || strings.HasPrefix(base, "webpack.config."):
		return Classification{Role: RoleConfig, Adjustment: 0.08, Confidence: 0.90, Reason: "project configuration"}, true
	case strings.Contains(base, ".generated.") || strings.HasSuffix(base, "_generated"+ext) || strings.HasSuffix(base, "_gen"+ext):
		return Classification{Role: RoleGenerated, Adjustment: -0.35, Confidence: 0.90, Reason: "generated naming convention"}, true
	}
	return Classification{}, false
}
