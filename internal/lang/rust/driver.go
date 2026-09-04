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

var rsMod = regexp.MustCompile(`(?m)^\s*(?:pub\s*(?:\([^)]*\))?\s+)?mod\s+([a-zA-Z0-9_]+)\s*;`)

// Imports returns the local import targets of a Rust file, resolved to
// repo-relative paths. Crate-rooted imports ("crate::a::b") resolve beneath
// the enclosing crate root (the directory above src/, or the repository root
// when there is no src/ ancestor). Relative imports ("super::", "self::")
// resolve against the importing file's directory, and "mod foo;" declarations
// resolve to the candidate file paths (foo.rs or foo/mod.rs). Bare imports
// are external crates and are omitted.
func (rsDriver) Imports(path, moduleRoot string, content []byte) []string {
	_ = moduleRoot
	dir := filepath.ToSlash(filepath.Dir(path))
	base := crateBaseDir(path)
	var targets []string
	for _, m := range rsUse.FindAllSubmatch(content, -1) {
		use := string(m[1])
		var from string
		var ups int
		switch {
		case strings.HasPrefix(use, "crate::"):
			from = base
			use = strings.TrimPrefix(use, "crate::")
		case strings.HasPrefix(use, "self::"):
			from = dir
			use = strings.TrimPrefix(use, "self::")
		case strings.HasPrefix(use, "super::"):
			from = dir
			for strings.HasPrefix(use, "super::") {
				use = strings.TrimPrefix(use, "super::")
				from = filepath.ToSlash(filepath.Dir(from))
				ups++
			}
			_ = ups
		default:
			continue
		}
		// Emit every prefix of the remainder so both module directories and
		// item paths contribute fan-in to their enclosing package nodes.
		segments := strings.Split(use, "::")
		for i := 1; i <= len(segments); i++ {
			if segments[0] == "" {
				break
			}
			target := filepath.ToSlash(filepath.Clean(filepath.Join(from, filepath.Join(segments[:i]...))))
			if target != "." && target != "" {
				targets = append(targets, target)
			}
		}
	}
	for _, m := range rsMod.FindAllSubmatch(content, -1) {
		mod := string(m[1])
		targets = append(targets,
			filepath.ToSlash(filepath.Join(dir, mod+".rs")),
			filepath.ToSlash(filepath.Join(dir, mod, "mod.rs")),
		)
	}
	return targets
}

// crateBaseDir returns the enclosing crate root for a repo-relative Rust
// path: the directory above the first src/ ancestor (the Cargo convention),
// or "" when the file is not under a src/ directory.
func crateBaseDir(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i, p := range parts {
		if p == "src" {
			return strings.Join(parts[:i], "/")
		}
	}
	return filepath.ToSlash(filepath.Dir(path))
}
