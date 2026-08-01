package language

import "strings"

// RuleJavaScript filters JS/TS lockfiles, minified bundles, and source maps.
func RuleJavaScript(path, filename string, content []byte) bool {
	if filename == "package-lock.json" || filename == "yarn.lock" || filename == "pnpm-lock.yaml" || filename == "bun.lockb" {
		return true
	}
	if strings.HasSuffix(filename, ".min.js") || strings.HasSuffix(filename, ".min.css") {
		return true
	}
	if strings.HasSuffix(filename, ".map") || strings.HasSuffix(filename, ".bundle.js") {
		return true
	}
	return false
}
