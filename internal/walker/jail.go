package walker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ContainPath resolves all symlinks in path and verifies the result stays
// inside root. Containment is checked on a path-separator boundary so a root
// like /home/u/code never admits /home/u/code-evil. It returns the resolved
// absolute path for opening, so the check and the subsequent read race as
// little as possible. An error means the file must be skipped, never read.
func ContainPath(root, path string) (string, error) {
	cleanRoot := filepath.Clean(root)
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("cannot resolve %q: %w", path, err)
	}
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(cleanRoot, resolved)
	}
	resolved = filepath.Clean(resolved)
	if resolved != cleanRoot && !strings.HasPrefix(resolved, cleanRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes root", path)
	}
	return resolved, nil
}

// IsSymlink reports whether path itself is a symlink, without following it.
func IsSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}
