package registry

import (
	"strings"

	"github.com/bethropolis/sift/internal/lang/types"
)

// common applies language-agnostic path conventions before any language rules,
// mirroring the historical shared rules used by selection ranking.
func common(path, base, ext string) (types.Classification, bool) {
	switch {
	case strings.Contains(path, "/vendor/") || strings.HasPrefix(path, "vendor/"):
		return types.Classification{Role: types.RoleVendor, Adjustment: -0.20, Confidence: 0.95, Reason: "vendored dependency"}, true
	case strings.Contains(path, "/testdata/") || strings.HasPrefix(path, "testdata/") || strings.Contains(path, "/fixtures/"):
		return types.Classification{Role: types.RoleFixture, Adjustment: -0.10, Confidence: 0.90, Reason: "fixture/test data"}, true
	case strings.Contains(path, "/examples/") || strings.HasPrefix(path, "examples/"):
		return types.Classification{Role: types.RoleExample, Adjustment: 0.04, Confidence: 0.85, Reason: "example"}, true
	case strings.Contains(path, "/migrations/") || strings.HasPrefix(path, "migrations/"):
		return types.Classification{Role: types.RoleSchema, Adjustment: 0.10, Confidence: 0.85, Reason: "migration/schema"}, true
	case ext == ".md" || ext == ".mdx" || base == "readme" || strings.HasPrefix(base, "readme."):
		return types.Classification{Role: types.RoleDocs, Adjustment: 0.06, Confidence: 0.90, Reason: "documentation"}, true
	case ext == ".sql" || ext == ".graphql" || ext == ".gql" || ext == ".proto":
		return types.Classification{Role: types.RoleSchema, Adjustment: 0.10, Confidence: 0.90, Reason: "schema definition"}, true
	case ext == ".html" || ext == ".css" || ext == ".scss" || ext == ".vue" || ext == ".svelte" || ext == ".astro":
		return types.Classification{Role: types.RoleImpl, Adjustment: 0.02, Confidence: 0.75, Reason: "web source"}, true
	case base == "dockerfile" || base == "makefile" || base == "go.mod" || base == "cargo.toml" || base == "pyproject.toml" || base == "package.json" || base == "tsconfig.json" || strings.HasPrefix(base, "vite.config.") || strings.HasPrefix(base, "webpack.config."):
		return types.Classification{Role: types.RoleConfig, Adjustment: 0.08, Confidence: 0.90, Reason: "project configuration"}, true
	case strings.Contains(base, ".generated.") || strings.HasSuffix(base, "_generated"+ext) || strings.HasSuffix(base, "_gen"+ext):
		return types.Classification{Role: types.RoleGenerated, Adjustment: -0.35, Confidence: 0.90, Reason: "generated naming convention"}, true
	}
	return types.Classification{}, false
}
