package mcp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/bethropolis/sift/internal/serve/jail"
)

// scopeRoot resolves the optional "path" tool argument against the session's
// active roots. A relative path is joined to the first root; absolute paths
// must fall inside the jail. An empty argument selects the first root.
func (s *session) scopeRoot(args map[string]any) (string, error) {
	base := ""
	if len(s.roots) > 0 {
		base = s.roots[0]
	}
	var p string
	stringArg(args, "path", &p)
	if strings.TrimSpace(p) == "" {
		return base, nil
	}
	candidate := p
	if !filepath.IsAbs(filepath.FromSlash(candidate)) {
		if base == "" {
			return "", fmt.Errorf("relative path requires an active root")
		}
		candidate = filepath.Join(base, filepath.FromSlash(candidate))
	}
	real, err := s.resolveJailed(candidate)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(real)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("path %q is not a directory", p)
	}
	return real, nil
}

// resolveFile validates a file argument and returns its path relative to root.
// Symlink escapes and credential paths are rejected by the jail.
func (s *session) resolveFile(root, p string) (string, error) {
	candidate := p
	if !filepath.IsAbs(filepath.FromSlash(candidate)) {
		candidate = filepath.Join(root, filepath.FromSlash(candidate))
	}
	real, err := s.resolveJailed(candidate)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside the active root", p)
	}
	info, err := os.Stat(real)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("path %q is not a regular file", p)
	}
	return filepath.ToSlash(rel), nil
}

func (s *session) resolveJailed(p string) (string, error) {
	if s.jail != nil {
		real, err := s.jail.Resolve(filepath.ToSlash(filepath.Clean(p)))
		if err != nil {
			return "", fmt.Errorf("path %q is outside the allowed roots", p)
		}
		return real, nil
	}
	// Tests and clients without roots fall back to plain containment.
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	for _, root := range s.roots {
		if root == "" {
			continue
		}
		if abs == root || strings.HasPrefix(abs, root+string(filepath.Separator)) {
			return abs, nil
		}
	}
	return "", fmt.Errorf("path %q is outside the active root", p)
}

// sendRootsRequest asks the client for its current roots. The request ID is a
// string so it cannot collide with client-assigned numeric IDs.
func (s *session) sendRootsRequest(enc *json.Encoder) error {
	s.rootsRequested = true
	s.nextRootsID++
	s.pendingRootsID = fmt.Sprintf("sift-roots-%d", s.nextRootsID)
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      s.pendingRootsID,
		"method":  "roots/list",
	}
	return enc.Encode(req)
}

// handleRootsResult applies a roots/list reply. Failed or empty replies keep
// the startup root so the server never becomes unusable.
func (s *session) handleRootsResult(raw json.RawMessage) error {
	s.pendingRootsID = ""
	var result RootsListResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil
	}
	var paths []string
	for _, root := range result.Roots {
		p, ok := rootURIPath(root.URI)
		if !ok {
			continue
		}
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			paths = append(paths, p)
		}
	}
	if len(paths) == 0 {
		return nil
	}
	j, err := jail.New(paths, nil)
	if err != nil {
		return nil
	}
	s.jail = j
	s.roots = j.Roots()
	return nil
}

// rootURIPath converts a file:// URI to a native absolute path on POSIX and
// Windows.
func rootURIPath(uri string) (string, bool) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return "", false
	}
	p := u.Path
	if u.Host != "" && u.Host != "localhost" {
		p = "//" + u.Host + p
	}
	if runtime.GOOS == "windows" {
		if len(p) >= 3 && p[0] == '/' && p[2] == ':' {
			p = p[1:]
		}
		p = strings.ReplaceAll(p, "/", `\`)
	}
	if p == "" {
		return "", false
	}
	return filepath.Clean(filepath.FromSlash(p)), true
}
