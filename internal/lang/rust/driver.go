package langrs

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bethropolis/sift/internal/lang/registry"
	"github.com/bethropolis/sift/internal/lang/types"
)

type rsDriver struct{}

func init() { registry.Register(rsDriver{}) }

func (rsDriver) ID() types.ID         { return types.Rust }
func (rsDriver) Name() string         { return "Rust" }
func (rsDriver) Extensions() []string { return []string{".rs"} }
func (rsDriver) ShouldSkipSmart(path, filename string, content []byte) (bool, string) {
	const reason = "Matched smart language filter"
	if filename == "cargo.lock" {
		return true, reason
	}
	if strings.HasSuffix(filename, "_bindings.rs") || strings.HasSuffix(filename, ".generated.rs") {
		return true, reason
	}
	return false, ""
}
func (rsDriver) Classify(path, filename string) types.Classification {
	switch {
	case strings.HasPrefix(path, "tests/") || strings.HasSuffix(filename, "_test.rs"):
		return types.Classification{Role: types.RoleTest, Adjustment: -0.12, Confidence: 0.95, Reason: "Rust test"}
	case strings.HasPrefix(path, "examples/") || filename == "main.rs" || strings.Contains(path, "/src/bin/") || strings.HasPrefix(path, "src/bin/"):
		return types.Classification{Role: types.RoleEntrypoint, Adjustment: 0.16, Confidence: 0.90, Reason: "Rust entrypoint/example"}
	case filename == "lib.rs":
		return types.Classification{Role: types.RoleAPI, Adjustment: 0.12, Confidence: 0.95, Reason: "Rust public library API"}
	}
	return types.Classification{Role: types.RoleImpl, Confidence: 0.50, Reason: "Rust source"}
}

var rsUse = regexp.MustCompile(`(?m)^\s*use\s+([a-zA-Z0-9_:]+)`)

// Imports returns the local import targets of a Rust file. Crate-rooted
// imports ("crate::a::b") resolve to repo-relative paths; bare and external
// crate imports are omitted unless they begin a local module path.
func (rsDriver) Imports(path, moduleRoot string, content []byte) []string {
	_ = path
	_ = moduleRoot
	var targets []string
	for _, m := range rsUse.FindAllSubmatch(content, -1) {
		use := string(m[1])
		if !strings.HasPrefix(use, "crate::") {
			continue
		}
		rel := strings.TrimPrefix(use, "crate::")
		if i := strings.Index(rel, "::"); i != -1 {
			rel = rel[:i]
		}
		targets = append(targets, filepath.ToSlash(rel))
	}
	return targets
}
