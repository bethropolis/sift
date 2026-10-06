// Package jail resolves every client-supplied path against the configured
// roots. There is exactly one resolution function; every handler uses it, so
// path traversal, symlink escape, and denylist bypass have a single choke
// point to audit.
package jail

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// maxPathLen bounds client paths before any filesystem access.
const maxPathLen = 4096

// defaultDeny lists credential locations that are never browsable and never
// openable as projects, matched at any depth below a root.
var defaultDeny = []string{
	".ssh",
	".gnupg",
	".aws",
	".kube",
	".config/gcloud",
	".password-store",
	".local/share/keyrings",
	".docker/config.json",
	".netrc",
}

// Jail is a resolved, immutable filesystem jail.
type Jail struct {
	roots []string
	deny  []string
}

// New builds a jail from the configured roots (each resolved with
// EvalSymlinks at startup) plus extra denylist entries. Roots must exist
// and be directories.
func New(roots []string, extraDeny []string) (*Jail, error) {
	if len(roots) == 0 {
		return nil, fmt.Errorf("no allowed roots")
	}
	resolved := make([]string, 0, len(roots))
	for _, r := range roots {
		abs, err := filepath.Abs(r)
		if err != nil {
			return nil, fmt.Errorf("bad root %q: %w", r, err)
		}
		real, err := filepath.EvalSymlinks(abs)
		if err != nil {
			return nil, fmt.Errorf("bad root %q: %w", r, err)
		}
		info, err := os.Stat(real)
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("root %q is not a directory", r)
		}
		resolved = append(resolved, filepath.Clean(real))
	}
	deny := append(append([]string{}, defaultDeny...), extraDeny...)
	clean := deny[:0]
	for _, d := range deny {
		d = filepath.ToSlash(filepath.Clean(filepath.FromSlash(d)))
		if d == "" || d == "." {
			continue
		}
		clean = append(clean, d)
	}
	return &Jail{roots: resolved, deny: clean}, nil
}

// Roots returns the resolved allowed roots for banners and /api/meta.
func (j *Jail) Roots() []string {
	out := make([]string, len(j.roots))
	copy(out, j.roots)
	return out
}

// Resolve maps a client path to an absolute filesystem path inside the jail.
// It requires an absolute path, rejects NUL bytes and overlong input,
// resolves symlinks, verifies containment on a separator boundary, and
// applies the denylist. Failures are generic: callers must answer 403
// without revealing whether the path exists.
func (j *Jail) Resolve(clientPath string) (string, error) {
	if len(clientPath) == 0 || len(clientPath) > maxPathLen {
		return "", fmt.Errorf("bad path")
	}
	if strings.IndexByte(clientPath, 0) >= 0 {
		return "", fmt.Errorf("bad path")
	}
	if !filepath.IsAbs(filepath.FromSlash(clientPath)) {
		return "", fmt.Errorf("bad path")
	}
	clean := filepath.Clean(clientPath)
	real, err := filepath.EvalSymlinks(clean)
	if err != nil {
		// Missing paths fail the same way as escapes: a generic error.
		// (Browsing a not-yet-existing path is not supported in v1.)
		return "", fmt.Errorf("bad path")
	}
	real = filepath.Clean(real)
	root, ok := j.containingRoot(real)
	if !ok {
		return "", fmt.Errorf("bad path")
	}
	rel, err := filepath.Rel(root, real)
	if err != nil {
		return "", fmt.Errorf("bad path")
	}
	if j.denied(filepath.ToSlash(rel)) {
		return "", fmt.Errorf("bad path")
	}
	return real, nil
}

// containingRoot returns the jail root containing abs, if any.
func (j *Jail) containingRoot(abs string) (string, bool) {
	for _, root := range j.roots {
		if abs == root || strings.HasPrefix(abs, root+string(filepath.Separator)) {
			return root, true
		}
	}
	return "", false
}

// denied reports whether the denylist matches at any depth of a
// root-relative slash path: a denylist entry matches when its segments
// appear contiguously anywhere in the path ("sub/.ssh/x" matches ".ssh",
// "sub/.docker/config.json" matches ".docker/config.json"). Segment-exact
// matching keeps lookalikes like "my.ssh-config" open.
func (j *Jail) denied(rel string) bool {
	segs := strings.Split(rel, "/")
	for _, d := range j.deny {
		ds := strings.Split(d, "/")
		for i := 0; i+len(ds) <= len(segs); i++ {
			match := true
			for k := range ds {
				if segs[i+k] != ds[k] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

// OpenRoot opens the resolved project root for contained reads, so symlinks
// inside the project cannot escape it even between check and open.
func OpenRoot(root string) (*os.Root, error) {
	return os.OpenRoot(root)
}
