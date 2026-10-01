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
	case underAny(path, "test", "tests", "__test__", "__tests__"):
		return types.Classification{Role: types.RoleTest, Adjustment: -0.12, Confidence: 0.92, Reason: "test directory"}, true
	case strings.Contains(path, "/examples/") || strings.HasPrefix(path, "examples/"):
		return types.Classification{Role: types.RoleExample, Adjustment: 0.04, Confidence: 0.85, Reason: "example"}, true
	case strings.Contains(path, "/migrations/") || strings.HasPrefix(path, "migrations/"):
		return types.Classification{Role: types.RoleSchema, Adjustment: 0.10, Confidence: 0.85, Reason: "migration/schema"}, true
	case ext == ".md" || ext == ".mdx" || base == "readme" || strings.HasPrefix(base, "readme."):
		return types.Classification{Role: types.RoleDocs, Adjustment: 0.06, Confidence: 0.90, Reason: "documentation"}, true
	case ext == ".sql" || ext == ".graphql" || ext == ".gql" || ext == ".proto" || ext == ".prisma" ||
		base == "schema.prisma" || strings.HasPrefix(base, "openapi.") || strings.HasPrefix(base, "swagger."):
		return types.Classification{Role: types.RoleSchema, Adjustment: 0.10, Confidence: 0.90, Reason: "schema/API specification"}, true
	case ext == ".html" || ext == ".css" || ext == ".scss" || ext == ".vue" || ext == ".svelte" || ext == ".astro":
		return types.Classification{Role: types.RoleImpl, Adjustment: 0.02, Confidence: 0.75, Reason: "web source"}, true
	case base == "dockerfile" || base == "makefile" || base == "go.mod" || base == "go.work" ||
		base == "pubspec.yaml" || base == "analysis_options.yaml" || base == "build.zig" || base == "build.zig.zon" ||
		base == "cargo.toml" || base == "pyproject.toml" || base == "package.json" ||
		base == "tsconfig.json" || base == "pnpm-workspace.yaml" || base == "turbo.json" ||
		base == "lerna.json" || base == "nx.json" || base == "build.gradle" || base == "build.gradle.kts" ||
		base == "pom.xml" || base == "settings.gradle" || base == "settings.gradle.kts" ||
		base == "gemfile" || base == "composer.json" ||
		strings.HasPrefix(base, "vite.config.") || strings.HasPrefix(base, "webpack.config.") ||
		strings.HasPrefix(base, "next.config.") || strings.HasPrefix(base, "astro.config."):
		return types.Classification{Role: types.RoleConfig, Adjustment: 0.08, Confidence: 0.90, Reason: "project or workspace configuration"}, true
	case strings.Contains(base, ".generated.") || strings.HasSuffix(base, "_generated"+ext) || strings.HasSuffix(base, "_gen"+ext):
		return types.Classification{Role: types.RoleGenerated, Adjustment: -0.35, Confidence: 0.90, Reason: "generated naming convention"}, true
	}
	return types.Classification{}, false
}

func underAny(path string, dirs ...string) bool {
	for _, dir := range dirs {
		if strings.HasPrefix(path, dir+"/") || strings.Contains(path, "/"+dir+"/") {
			return true
		}
	}
	return false
}
