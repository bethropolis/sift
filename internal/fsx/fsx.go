// Package fsx provides small filesystem helpers with path-containment checks.
// It exists so configuration and theme loading share one definition of "this
// path must stay inside the project root", including symlink resolution.
package fsx

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// maxReadBytes caps contained reads (config and prompt/theme files are small).
const maxReadBytes = 1 << 20

// Escapes reports whether p resolves outside root. It rejects absolute paths
// and ".." traversals lexically, then resolves symlinks so a link inside the
// root cannot point at a file outside it. Paths that do not exist yet are
// checked through their deepest existing ancestor. Unresolvable paths fail
// closed.
func Escapes(root, p string) bool {
	if p == "" {
		return false
	}
	clean := filepath.Clean(filepath.FromSlash(p))
	if filepath.IsAbs(clean) {
		return true
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return true
	}

	absRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		absRoot = filepath.Clean(root)
	}
	target := filepath.Join(absRoot, clean)
	resolved, ok := evalExisting(target)
	if !ok {
		return true
	}
	rel, err := filepath.Rel(absRoot, resolved)
	if err != nil {
		return true
	}
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// evalExisting resolves p, walking up to the deepest existing ancestor when p
// (or a component) does not exist yet. The nonexistent suffix is appended
// back after resolution. ok is false when nothing can be resolved.
func evalExisting(p string) (string, bool) {
	cur := p
	suffix := ""
	for {
		resolved, err := filepath.EvalSymlinks(cur)
		if err == nil {
			if suffix != "" {
				return filepath.Join(resolved, suffix), true
			}
			return resolved, true
		}
		if !errors.Is(err, os.ErrNotExist) {
			// ENOTDIR, permission errors, and symlink loops are not safe to
			// reason about; fail closed.
			return "", false
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", false
		}
		suffix = filepath.Join(filepath.Base(cur), suffix)
		cur = parent
	}
}

// ConfigDir returns the per-user configuration directory for app, using the
// OS-native location (os.UserConfigDir: XDG on Linux, Application Support on
// macOS, %AppData% on Windows) with XDG/~/.config as a fallback.
func ConfigDir(app string) (string, error) {
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, app), nil
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, app), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", app), nil
}

// ReadFile reads path. When rootDir is non-empty and path is relative, the
// read is confined to rootDir: symlinks and ".." cannot escape it. Absolute
// paths (trusted global config) are read as-is.
func ReadFile(rootDir, path string) ([]byte, error) {
	if rootDir == "" || filepath.IsAbs(filepath.FromSlash(path)) {
		return readCapped(path)
	}
	if Escapes(rootDir, path) {
		return nil, fmt.Errorf("path %q escapes project root %q", path, rootDir)
	}
	// os.Root enforces containment at open time, closing the check/read gap.
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	f, err := root.Open(filepath.FromSlash(path))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readCappedReader(f)
}

func readCapped(path string) ([]byte, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	f, err := root.Open(filepath.Base(path))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readCappedReader(f)
}

func readCappedReader(f *os.File) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(f, maxReadBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxReadBytes {
		return nil, fmt.Errorf("file exceeds %d bytes", maxReadBytes)
	}
	return data, nil
}
