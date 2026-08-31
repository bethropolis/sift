package ignore

import (
	"path/filepath"
	"strings"
)

// Visibility describes presentation-relevant ignore metadata. Explicit and
// custom safety exclusions remain hard filters and are not represented here.
type Visibility struct {
	Hidden       bool
	GitIgnored   bool
	ProtectedGit bool
}

// ClassifyVisibility reports hidden and repository-gitignore metadata without
// applying those matches as filters.
func (m *IgnoreMatcher) ClassifyVisibility(relativePath string, isDir bool) Visibility {
	if m == nil || m.disabled {
		return Visibility{}
	}
	path := filepath.ToSlash(relativePath)
	visibility := Visibility{Hidden: isHiddenPath(path), ProtectedGit: isPathInGitDir(path)}
	if visibility.ProtectedGit || m.repoIgnore == nil {
		return visibility
	}
	if match := m.repoIgnore.Match(filepath.Join(m.rootDir, relativePath)); match != nil {
		visibility.GitIgnored = match.Ignore()
	}
	return visibility
}

func isHiddenPath(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part != "" && strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

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
	if isPathInGitDir(relativePath) {
		m.logger.Debug("ignore.ShouldIgnore: Ignored %q (.git rule)", relativePath)
		return true
	}

	// Custom ignore patterns first (highest priority). A match is definitive
	// whether positive or negated: --ignore negations must win over every
	// lower tier, including the built-in defaults.
	absPath := filepath.Join(m.rootDir, relativePath)
	if m.customIgnore != nil {
		if match := m.customIgnore.Match(absPath); match != nil {
			m.logger.Debug("ignore.ShouldIgnore: Path %q matched custom rule (ignore=%v)", relativePath, match.Ignore())
			return match.Ignore()
		}
	}

	// Delegate to gitignore library for repo rules. A repo match (including a
	// negation) is definitive and takes precedence over the defaults below.
	if m.ignoreGit && m.repoIgnore != nil {
		m.logger.Debug("ignore.ShouldIgnore: Checking repo rules for path %q", relativePath)

		if match := m.repoIgnore.Match(absPath); match != nil {
			m.logger.Debug("ignore.ShouldIgnore: Path %q matched repo rule (ignore=%v)", relativePath, match.Ignore())
			return match.Ignore()
		}
	} else {
		m.logger.Debug("ignore.ShouldIgnore: No repository ignore patterns loaded (m.repoIgnore is nil).")
	}

	// Fall back to the built-in default patterns. They are the lowest-priority
	// safety net and only apply when neither custom patterns nor repository
	// rules matched the path.
	if m.defaultIgnore != nil {
		if match := m.defaultIgnore.Match(absPath); match != nil {
			m.logger.Debug("ignore.ShouldIgnore: Path %q matched default rule (ignore=%v)", relativePath, match.Ignore())
			return match.Ignore()
		}
	}

	m.logger.Debug("ignore.ShouldIgnore: Path %q NOT ignored by any rule", relativePath)
	return false
}

// isPathInGitDir reports whether any path component is named ".git".
// This covers both ".git" directories and Git submodule/worktree markers,
// which are regular files named ".git" at the end of the path.
func isPathInGitDir(relativePath string) bool {
	for _, part := range strings.Split(filepath.ToSlash(relativePath), "/") {
		if part == ".git" {
			return true
		}
	}
	return false
}
