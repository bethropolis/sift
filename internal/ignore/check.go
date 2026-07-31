package ignore

import (
	"path/filepath"
	"strings"
)

// ShouldIgnore checks if a file or directory should be ignored
func (m *IgnoreMatcher) ShouldIgnore(relativePath string, isDir bool) bool {
	// Return early if matcher is nil or disabled
	if m == nil || m.disabled {
		return false
	}

	// Normalize empty paths
	if relativePath == "" || relativePath == "." {
		return false // Never ignore the root itself
	}

	m.logger.Debug("ignore.ShouldIgnore: Checking path: %q (isDir: %v)", relativePath, isDir)

	// Check for hidden files if ignoreHidden is enabled
	if m.ignoreHidden {
		// Check if the basename starts with a dot (more efficient than splitting)
		base := filepath.Base(relativePath)
		if strings.HasPrefix(base, ".") {
			m.logger.Debug("ignore.ShouldIgnore: Ignored %q (hidden file rule)", relativePath)
			return true
		}

		// Also check if any parent directory is hidden
		dir := filepath.Dir(relativePath)
		for dir != "." && dir != "/" && dir != "\\" {
			base = filepath.Base(dir)
			if strings.HasPrefix(base, ".") {
				m.logger.Debug("ignore.ShouldIgnore: Ignored %q (hidden dir rule)", relativePath)
				return true
			}
			dir = filepath.Dir(dir)
		}
	}

	// Special check for .git directory
	if m.ignoreGit && isPathInGitDir(relativePath, isDir) {
		m.logger.Debug("ignore.ShouldIgnore: Ignored %q (.git rule)", relativePath)
		return true
	}

	// Check custom ignore patterns first (these override repo rules)
	absPath := filepath.Join(m.rootDir, relativePath)
	if m.customIgnore != nil {
		if m.customIgnore.Ignore(absPath) {
			excluded := m.customIgnore.Include(absPath)
			if !excluded {
				m.logger.Debug("ignore.ShouldIgnore: Ignored %q (custom pattern)", relativePath)
				return true
			}
			m.logger.Debug("ignore.ShouldIgnore: Path %q excluded by custom negation rule", relativePath)
		}
	}

	// Delegate to gitignore library for repo rules
	if m.repoIgnore != nil {
		m.logger.Debug("ignore.ShouldIgnore: Checking repo rules for path %q", relativePath)

		if match := m.repoIgnore.Match(absPath); match != nil {
			m.logger.Debug("ignore.ShouldIgnore: Path %q matched repo rule (ignore=%v)", relativePath, match.Ignore())
			return match.Ignore()
		}
	} else {
		m.logger.Debug("ignore.ShouldIgnore: No repository ignore patterns loaded (m.repoIgnore is nil).", relativePath)
	}

	m.logger.Debug("ignore.ShouldIgnore: Path %q NOT ignored by any rule", relativePath)
	return false
}

// isPathInGitDir checks if a path is inside a .git directory
func isPathInGitDir(relativePath string, isDir bool) bool {
	parts := strings.Split(filepath.ToSlash(relativePath), "/")
	for i, part := range parts {
		if part == ".git" {
			// If .git is a directory component (not just a prefix of a filename)
			if isDir || i < len(parts)-1 {
				return true
			}
		}
	}
	return false
}
