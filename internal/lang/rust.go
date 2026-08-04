package lang

import "strings"

type rsDriver struct{}

func init() { Register(rsDriver{}) }

func (rsDriver) ID() ID               { return Rust }
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
func (rsDriver) Classify(path, filename string) Classification {
	switch {
	case strings.HasPrefix(path, "tests/") || strings.HasSuffix(filename, "_test.rs"):
		return Classification{Role: RoleTest, Adjustment: -0.12, Confidence: 0.95, Reason: "Rust test"}
	case strings.HasPrefix(path, "examples/") || filename == "main.rs":
		return Classification{Role: RoleEntrypoint, Adjustment: 0.16, Confidence: 0.90, Reason: "Rust entrypoint/example"}
	case filename == "lib.rs":
		return Classification{Role: RoleAPI, Adjustment: 0.12, Confidence: 0.95, Reason: "Rust public library API"}
	}
	return Classification{Role: RoleImpl, Confidence: 0.50, Reason: "Rust source"}
}
