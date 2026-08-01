package language

import "strings"

// RuleRust filters Rust lockfiles and generated bindings.
func RuleRust(path, filename string, content []byte) bool {
	if filename == "cargo.lock" {
		return true
	}
	if strings.HasSuffix(filename, "_bindings.rs") || strings.HasSuffix(filename, ".generated.rs") {
		return true
	}
	return false
}
