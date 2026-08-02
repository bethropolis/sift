package language

import "strings"

func rustFile(path, base string) Classification {
	switch {
	case strings.HasPrefix(path, "tests/") || strings.HasSuffix(base, "_test.rs"):
		return Classification{Role: RoleTest, Adjustment: -0.12, Confidence: 0.95, Reason: "Rust test"}
	case strings.HasPrefix(path, "examples/") || base == "main.rs":
		return Classification{Role: RoleEntrypoint, Adjustment: 0.16, Confidence: 0.90, Reason: "Rust entrypoint/example"}
	case base == "lib.rs":
		return Classification{Role: RoleAPI, Adjustment: 0.12, Confidence: 0.95, Reason: "Rust public library API"}
	}
	return Classification{Role: RoleImpl, Confidence: 0.50, Reason: "Rust source"}
}
