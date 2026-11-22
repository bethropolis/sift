package history

import (
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// ArchiveName builds the archive filename for a project name, timestamp and
// extension. The timestamp is UTC. Unsafe characters (path separators,
// whitespace and control characters) are replaced with '-'.
func ArchiveName(name string, t time.Time, ext string) string {
	base := sanitizeName(name)
	if base == "" {
		base = "dump"
	}
	suffix := t.UTC().Format("20060102-150405")
	ext = normalizeExtension(ext)
	if ext == "" {
		return base + "-" + suffix
	}
	return base + "-" + suffix + "." + ext
}

func sanitizeName(name string) string {
	if name == "" {
		return ""
	}
	if strings.Trim(name, `/\`) == "" {
		return "root"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
			lastDash = r == '-'
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-.")
}

func normalizeExtension(ext string) string {
	ext = strings.TrimSpace(ext)
	ext = strings.TrimLeft(ext, ".")
	if ext == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range ext {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	if b.Len() == 0 {
		return "txt"
	}
	return b.String()
}

// StyleExtension maps a sift output style to an archive file extension.
func StyleExtension(style string) string {
	switch strings.ToLower(style) {
	case "markdown", "md":
		return "md"
	case "xml":
		return "xml"
	case "json":
		return "json"
	case "plain", "text", "":
		return "txt"
	default:
		return normalizeExtension(style)
	}
}

// insideDir reports whether path resolves to a location inside dir. Both dir
// and path are made absolute and symlinks are resolved. A path that does not
// exist is resolved through its nearest existing parent, so a file that a
// stale record names can still be checked safely before deletion.
func insideDir(dir, path string) bool {
	if dir == "" || path == "" {
		return false
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	resolvedDir, err := filepath.EvalSymlinks(absDir)
	if err != nil {
		return false
	}
	resolvedPath, err := resolvePath(absPath)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(resolvedDir, resolvedPath)
	if err != nil || rel == "." || filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func resolvePath(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	parent, parentErr := filepath.EvalSymlinks(filepath.Dir(path))
	if parentErr != nil {
		return "", parentErr
	}
	return filepath.Join(parent, filepath.Base(path)), nil
}

func archivePath(dumpsDir, name string, t time.Time, style string) string {
	return filepath.Join(dumpsDir, ArchiveName(name, t, StyleExtension(style)))
}
