package app

import (
	"fmt"
	"path/filepath"
	"strings"
)

// resolveWithinRoot resolves a relative path against root, rejecting any
// candidate that would escape the root via "..", an absolute path, or a
// non-canonical form. The returned path is cleaned and absolute. relative is
// allowed to contain either slash or platform path separators.
func resolveWithinRoot(root, relative string) (string, error) {
	if relative == "" {
		return "", fmt.Errorf("empty relative path")
	}

	clean := filepath.Clean(filepath.FromSlash(relative))
	if clean == "." {
		return "", fmt.Errorf("path %q resolves to the root directory", relative)
	}
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("path %q is absolute", relative)
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes root", relative)
	}
	return filepath.Join(root, clean), nil
}
