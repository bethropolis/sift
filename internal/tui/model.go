package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// model is the BubbleTea state for the dual-pane picker: a foldable tree on
// the left and a live file preview on the right, with a budget footer.
type model struct {
	root   *TreeNode
	rows   []*TreeNode // cached visible rows.
	cursor int
	offset int

	height int
	width  int

	filter    string
	filtering bool

	budget int
	style  string
	glyphs Glyphs

	onCopy func([]Selection) error
	notice string

	quit bool
}

// Options configures the picker.
type Options struct {
	Budget  int
	Style   string
	UseNerd bool
	OnCopy  func([]Selection) error
}

func newModel(root *TreeNode, opts Options) model {
	glyphs := NewASCIIGlyphs()
	if opts.UseNerd {
		glyphs = NewNerdFontGlyphs()
	}
	m := model{
		root:   root,
		height: 24,
		width:  80,
		budget: opts.Budget,
		style:  opts.Style,
		glyphs: glyphs,
		onCopy: opts.OnCopy,
	}
	m.recomputeRows()
	return m
}

func (m *model) recomputeRows() {
	m.rows = m.root.VisibleRows(m.filter)
	if m.cursor > len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.clampOffset()
}

func (m *model) node() *TreeNode {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	return m.rows[m.cursor]
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height, m.width = msg.Height, msg.Width
		m.clampOffset()
	case tea.KeyMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

func (m model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filtering {
		switch msg.Type {
		case tea.KeyEsc:
			m.filtering = false
			m.recomputeRows()
		case tea.KeyEnter:
			m.filtering = false
			m.recomputeRows()
		case tea.KeyBackspace, tea.KeyDelete:
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
			}
			m.recomputeRows()
		case tea.KeyRunes:
			m.filter += string(msg.Runes)
			m.recomputeRows()
		}
		return m, nil
	}

	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyCtrlQ:
		m.quit = true
		return m, tea.Quit
	case tea.KeyEnter:
		if n := m.node(); n != nil && n.Kind == KindDir {
			n.Expanded = !n.Expanded
			m.recomputeRows()
		} else {
			m.quit = true
			return m, tea.Quit
		}
	case tea.KeyUp, tea.KeyShiftTab:
		m.move(-1)
	case tea.KeyDown, tea.KeyTab:
		m.move(1)
	case tea.KeySpace:
		if n := m.node(); n != nil {
			n.Toggle()
		}
	case tea.KeyLeft:
		m.collapseOrParent()
	case tea.KeyRight:
		if n := m.node(); n != nil && n.Kind == KindDir {
			n.Expanded = true
			m.recomputeRows()
		}
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "j":
			m.move(1)
		case "k":
			m.move(-1)
		case "h":
			m.collapseOrParent()
		case "l":
			if n := m.node(); n != nil && n.Kind == KindDir {
				n.Expanded = true
				m.recomputeRows()
			}
		case " ":
			if n := m.node(); n != nil {
				n.Toggle()
			}
		case "m":
			if n := m.node(); n != nil {
				n.CycleMode()
			}
		case "a":
			m.selectAll()
		case "s":
			m.smartSelect()
		case "/":
			m.filtering = true
			m.filter = ""
		case "y":
			m.copy()
		case "q":
			m.quit = true
			return m, tea.Quit
		}
	}
	m.clampOffset()
	return m, nil
}

func (m *model) move(delta int) {
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	m.clampOffset()
}

// collapseOrParent collapses the hovered directory; when it is already
// collapsed the cursor moves to its parent.
func (m *model) collapseOrParent() {
	n := m.node()
	if n == nil {
		return
	}
	if n.Kind == KindDir && n.Expanded {
		n.Expanded = false
		m.recomputeRows()
		return
	}
	if n.Parent != nil && n.Parent.Parent != nil {
		for i, r := range m.rows {
			if r == n.Parent {
				m.cursor = i
				break
			}
		}
	}
}

func (m *model) selectAll() {
	if m.root.SelectedCount() == m.root.FileCount() {
		m.root.ClearSelection()
	} else {
		m.root.setSelected(true)
	}
}

func (m *model) smartSelect() {
	count := m.root.SelectByRank(m.budget)
	if m.budget > 0 {
		m.notice = fmt.Sprintf("Smart select: %d files within %d tokens", count, m.budget)
	} else {
		m.notice = fmt.Sprintf("Smart select: %d files", count)
	}
}

func (m *model) copy() {
	if m.onCopy == nil {
		return
	}
	sel := m.root.Selections()
	if len(sel) == 0 {
		m.notice = "Nothing selected to copy"
		return
	}
	if err := m.onCopy(sel); err != nil {
		m.notice = "Copy failed: " + err.Error()
		return
	}
	m.notice = fmt.Sprintf("Copied %d files (%d tokens) to clipboard", len(sel), m.root.TotalActiveTokens())
}

func (m *model) clampOffset() {
	rows := m.height - headerLines - footerLines
	if rows < 1 {
		rows = 1
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
}

func (m model) View() string {
	width := max(0, min(m.width, 160))
	leftWidth := width / 2
	rightWidth := width - leftWidth

	left := m.renderTree(leftWidth)
	right := m.renderPreview(rightWidth)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	footer := m.renderFooter(width)
	return body + "\n" + footer
}

func (m model) renderTree(width int) string {
	var b strings.Builder
	title := fmt.Sprintf(" %s Directory Tree (%d files, %d tokens) ",
		m.glyphs.FolderOpen, m.root.FileCount(), m.root.TotalActiveTokens())
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", width))
	b.WriteString("\n")

	rows := m.height - headerLines - footerLines
	if rows < 1 {
		rows = 1
	}
	end := min(len(m.rows), m.offset+rows)
	for i := m.offset; i < end; i++ {
		b.WriteString(m.treeRow(m.rows[i], width))
		b.WriteString("\n")
	}
	return lipgloss.NewStyle().Width(width).Render(b.String())
}

func (m model) treeRow(n *TreeNode, width int) string {
	depth := n.depth() - 1
	indent := strings.Repeat("  ", max(0, depth))

	var mark string
	switch n.SelectState {
	case Selected:
		mark = m.glyphs.CheckFull
	case Partial:
		mark = m.glyphs.CheckPartial
	default:
		mark = m.glyphs.CheckNone
	}

	icon := m.glyphs.File
	name := n.Name
	tokens := n.TokensSig
	if n.Kind == KindDir {
		icon = m.glyphs.FolderClosed
		if n.Expanded {
			icon = m.glyphs.FolderOpen
		}
		name += "/"
	} else if n.Mode == ModeFull {
		tokens = n.TokensFull
	}

	secret := ""
	if n.SecretCount > 0 {
		secret = " " + warningStyle.Render(m.glyphs.Warning)
	}

	left := fmt.Sprintf(" %s %s %s %s", mark, icon, indent+name, secret)
	right := fmt.Sprintf("%6d tok", tokens)
	pad := max(4, width-lipgloss.Width(left)-lipgloss.Width(right))
	row := left + strings.Repeat(" ", pad) + right

	if n == m.node() {
		return cursorStyle.Render(row)
	}
	if n.SelectState != Unselected {
		return selectedStyle.Render(row)
	}
	return row
}

func (m model) renderPreview(width int) string {
	var b strings.Builder
	n := m.node()
	if n == nil {
		return lipgloss.NewStyle().Width(width).Render(b.String())
	}

	title := " Preview "
	if n.Kind == KindFile {
		title = fmt.Sprintf(" %s Preview: %s ", m.glyphs.File, n.Path)
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", width))
	b.WriteString("\n")

	if n.Kind == KindDir {
		b.WriteString(hintStyle.Render(fmt.Sprintf("  %s %d files, %d tokens",
			m.glyphs.FolderOpen, n.FileCount(), n.TotalActiveTokens())))
	} else {
		content := n.Preview()
		lines := strings.Split(string(content), "\n")
		rows := m.height - headerLines - footerLines - 1
		if rows < 1 {
			rows = 1
		}
		for _, line := range lines {
			if len(strings.Split(b.String(), "\n")) >= rows+2 {
				break
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
		if n.SecretCount > 0 {
			b.WriteString(warningStyle.Render(fmt.Sprintf(" %s Warning: %d secret detected in this file",
				m.glyphs.Warning, n.SecretCount)))
		}
	}
	return lipgloss.NewStyle().Width(width).Render(b.String())
}

func (m model) renderFooter(width int) string {
	selected := m.root.SelectedCount()
	active := m.root.TotalActiveTokens()

	budget := ""
	if m.budget > 0 {
		pct := 0
		if active >= m.budget {
			pct = 100
		} else if active > 0 {
			pct = active * 100 / m.budget
		}
		barLen := max(1, width-60)
		filled := barLen * pct / 100
		bar := strings.Repeat("█", filled) + strings.Repeat("░", max(0, barLen-filled))
		budget = fmt.Sprintf("Budget: %d / %d %s %d%% | ", active, m.budget, bar, pct)
	}

	keys := "space toggle   m mode   a all/none   s smart   / filter   y copy   enter done   q quit"
	if m.filtering {
		keys = "/ filter: " + m.filter + "▌"
	}

	var b strings.Builder
	b.WriteString(strings.Repeat("─", width))
	b.WriteString("\n")
	if m.notice != "" {
		b.WriteString(noticeStyle.Render(m.notice))
		b.WriteString("\n")
	}
	b.WriteString(hintStyle.Render(fmt.Sprintf("%sStyle: %s | %d selected, %d tokens | %s",
		budget, m.style, selected, active, keys)))
	b.WriteString("\n")
	return b.String()
}

func (n *TreeNode) depth() int {
	d := 0
	for p := n.Parent; p != nil; p = p.Parent {
		d++
	}
	return d
}

// Preview returns the preview text for a file: its signature summary when the
// mode is signatures and one is available, otherwise the full content.
func (n *TreeNode) Preview() []byte {
	if n.Mode == ModeSignatures && len(n.SigContent) > 0 {
		return n.SigContent
	}
	return n.Content
}

const headerLines = 2
const footerLines = 2

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	hintStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	cursorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	warningStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
	noticeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
)

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
