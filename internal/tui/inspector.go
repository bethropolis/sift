package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const previewFileHeaderRows = 3

func languageLabel(path string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if ext == "" {
		return "Text"
	}
	labels := map[string]string{
		"go": "Go", "md": "Markdown", "markdown": "Markdown", "rs": "Rust",
		"ts": "TypeScript", "tsx": "TypeScript", "js": "JavaScript", "jsx": "JavaScript",
		"py": "Python", "java": "Java", "kt": "Kotlin", "cs": "C#", "c": "C",
		"cc": "C++", "cpp": "C++", "h": "C", "hpp": "C++", "rb": "Ruby",
		"swift": "Swift", "php": "PHP", "sh": "Shell", "json": "JSON", "toml": "TOML",
		"yaml": "YAML", "yml": "YAML",
	}
	if label, ok := labels[ext]; ok {
		return label
	}
	return strings.ToUpper(ext)
}

func modeLabel(mode CompressMode) string {
	switch mode {
	case ModeSignatures:
		return "SIG"
	case ModeSkip:
		return "SKIP"
	default:
		return "FULL"
	}
}

func (m model) fileMetadata(n *TreeNode) string {
	parts := []string{languageLabel(n.Path), modeLabel(n.Mode)}
	if n.TokensFull == 0 && n.ApproxTokens > 0 {
		parts = append(parts, "~"+formatTokenCount(n.ApproxTokens)+" tokens")
	} else {
		parts = append(parts, formatTokenCount(n.TokensFull)+" tokens")
	}
	if n.SecretCount > 0 {
		parts = append(parts, fmt.Sprintf("%s %d", strings.TrimSpace(m.glyphs.Warning), n.SecretCount))
	}
	if n.Hidden {
		parts = append(parts, "hidden")
	}
	if n.GitIgnored {
		parts = append(parts, "git-ignored")
	}
	return strings.Join(parts, " · ")
}

func (m model) writeFileHeader(b *strings.Builder, n *TreeNode, width int) {
	b.WriteString("\n")
	separator := m.styles.subtle.Render(" · ")
	status := m.fileModeExplanation(n)
	statusStyle := m.styles.modeFull
	switch n.Mode {
	case ModeSignatures:
		statusStyle = m.styles.modeSig
	case ModeSkip:
		statusStyle = m.styles.modeSkip
	}
	metadata := m.styles.muted.Render(m.fileMetadata(n)) + separator + statusStyle.Render(status)
	b.WriteString(ansiTruncate(metadata, width))
	b.WriteString("\n")
	b.WriteString(m.styles.subtle.Render(strings.Repeat("─", max(1, width))))
}

func (m model) fileModeExplanation(n *TreeNode) string {
	if n.Hidden {
		return "hidden · reveal to include"
	}
	if n.GitIgnored {
		return "Git-ignored · reveal to include"
	}
	switch n.Mode {
	case ModeSkip:
		if n.PreferredMode == ModeSkip && n.ModeReason != "" {
			return compactModeReason(n.ModeReason, "low relevance")
		}
		return "manually skipped"
	case ModeSignatures:
		if n.PreferredMode == ModeSignatures && n.ModeReason != "" {
			return compactModeReason(n.ModeReason, "compact context")
		}
		return "signature view"
	default:
		if n.PreferredMode == ModeFull && n.ModeReason != "" {
			return compactModeReason(n.ModeReason, "included")
		}
		return "included"
	}
}

func compactModeReason(reason, fallback string) string {
	lower := strings.ToLower(reason)
	switch {
	case strings.Contains(lower, "role excluded"):
		return "excluded by role"
	case strings.Contains(lower, "not selected by utility optimizer"):
		return "budget tradeoff"
	case strings.Contains(lower, "uncommitted change"):
		return "uncommitted change"
	case strings.Contains(lower, "low relevance"):
		return "low relevance"
	case strings.Contains(lower, "compression yield"):
		return "low signature yield"
	case strings.Contains(lower, "high relevance"), strings.Contains(lower, "active work"):
		return "active work"
	case strings.Contains(lower, "signature"):
		return "compact context"
	default:
		return fallback
	}
}

func ansiTruncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "…")
}

func (m model) directoryInspector(n *TreeNode) []string {
	selected := n.SelectedCount()
	lines := []string{
		m.styles.hint.Render(fmt.Sprintf("%d files · %d selected", n.FileCount(), selected)),
		m.styles.hint.Render(fmt.Sprintf("%s active tokens", formatTokenCount(n.TotalActiveTokens()))),
		"",
		m.styles.subtle.Render(fmt.Sprintf("Full context   %s", formatTokenCount(n.TokensFull))),
		m.styles.subtle.Render(fmt.Sprintf("Signatures     %s", formatTokenCount(n.TokensSig))),
	}
	if n.SecretCount > 0 {
		lines = append(lines, m.styles.warning.Render(fmt.Sprintf("%s %d secrets detected", strings.TrimSpace(m.glyphs.Warning), n.SecretCount)))
	}
	lines = append(lines,
		"",
		m.styles.hint.Render(m.keyBadge("Space", "Toggle selection")),
		m.styles.hint.Render(m.keyBadge("Enter", "Expand directory")),
	)
	return lines
}

func (m model) previewEmptyLines(width, rows int) []string {
	centered := func(s string) string {
		s = ansiTruncate(s, width)
		pad := max(0, (width-ansi.StringWidth(s))/2)
		return strings.Repeat(" ", pad) + s
	}
	lines := []string{
		centered(m.styles.subtle.Render("sift")),
		"",
		centered(m.styles.hint.Render("Select files to build context")),
		"",
		centered(m.styles.muted.Render(fmt.Sprintf("%d / %d selected", m.root.SelectedCount(), m.root.FileCount()))),
	}
	if m.stream.active() && !m.scanDone {
		lines = append(lines, "", centered(m.styles.hint.Render(m.spinnerChar()+" Scanning…")))
	}
	if rows < len(lines) {
		return lines[:max(0, rows)]
	}
	return lines
}
