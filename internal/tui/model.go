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

	// Delta modal state.
	delta         *DeltaInfo
	deltaOpen     bool
	deltaCursor   int
	deltaOffset   int
	deltaStrategy DeltaStrategy
	onDelta       func(DeltaSelection) error
	deltaDone     bool

	quit bool
}

// Options configures the picker.
type Options struct {
	Budget  int
	Style   string
	UseNerd bool
	OnCopy  func([]Selection) error

	// Delta is the git delta state shown in the delta modal (pressing d).
	// When nil the modal shows "not available" messaging. OnDelta is invoked
	// when the user confirms a delta dump.
	Delta   *DeltaInfo
	OnDelta func(DeltaSelection) error
}

func newModel(root *TreeNode, opts Options) model {
	glyphs := NewASCIIGlyphs()
	if opts.UseNerd {
		glyphs = NewNerdFontGlyphs()
	}
	m := model{
		root:    root,
		height:  24,
		width:   80,
		budget:  opts.Budget,
		style:   opts.Style,
		glyphs:  glyphs,
		onCopy:  opts.OnCopy,
		delta:   opts.Delta,
		onDelta: opts.OnDelta,
	}
	m.recomputeRows()
	return m
}

func (m *model) recomputeRows() {
	m.rows = m.root.VisibleRows(m.filter)
	if m.cursor >= len(m.rows) {
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
	if m.deltaOpen {
		return m.updateDeltaKey(msg)
	}

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
		case "d":
			m.openDelta()
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

// openDelta opens the delta modal, resetting the commit selection to include
// every commit since the last dump. Outside a git repo with a baseline it
// shows an explanatory notice instead.
func (m *model) openDelta() {
	if m.delta == nil {
		m.notice = "Delta mode needs a git repo with a recorded dump baseline"
		return
	}
	m.deltaOpen = true
	m.deltaCursor = 0
	m.deltaOffset = 0
	m.deltaStrategy = DeltaFull
	for i := range m.delta.Commits {
		m.delta.Commits[i].Checked = true
	}
}

// updateDeltaKey handles keys while the delta modal is open.
func (m model) updateDeltaKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyCtrlQ:
		m.quit = true
		return m, tea.Quit
	case tea.KeyEsc:
		m.deltaOpen = false
	case tea.KeyUp, tea.KeyShiftTab:
		m.deltaMove(-1)
	case tea.KeyDown, tea.KeyTab:
		m.deltaMove(1)
	case tea.KeyEnter:
		m.performDelta(false)
	case tea.KeySpace:
		if m.delta != nil && len(m.delta.Commits) > 0 {
			i := m.deltaCursor
			m.delta.Commits[i].Checked = !m.delta.Commits[i].Checked
		}
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "k":
			m.deltaMove(-1)
		case "j":
			m.deltaMove(1)
		case "m":
			m.deltaStrategy = (m.deltaStrategy + 1) % 2
		case "c":
			m.performDelta(true)
		case "d", "q":
			m.deltaOpen = false
		}
	}
	return m, nil
}

func (m *model) deltaMove(delta int) {
	if m.delta == nil || len(m.delta.Commits) == 0 {
		return
	}
	m.deltaCursor += delta
	if m.deltaCursor < 0 {
		m.deltaCursor = 0
	}
	if m.deltaCursor >= len(m.delta.Commits) {
		m.deltaCursor = len(m.delta.Commits) - 1
	}
}

// performDelta invokes the caller's delta handler with the current selection.
// copyMode routes output to the clipboard instead of the dump destination.
// The checked commits determine the range: the newest checked commit is the
// To boundary and the parent of the oldest checked commit the From boundary,
// so unchecking the top or bottom of the list narrows the delta.
func (m *model) performDelta(copyMode bool) {
	if m.delta == nil || m.onDelta == nil {
		m.notice = "Delta mode is unavailable"
		m.deltaOpen = false
		return
	}
	sel := DeltaSelection{
		Strategy:  m.deltaStrategy,
		From:      m.delta.FromHash,
		To:        m.delta.HeadHash,
		Clipboard: copyMode,
	}
	if from, to, ok := m.deltaRange(); ok {
		sel.From, sel.To = from, to
	}
	if err := m.onDelta(sel); err != nil {
		m.notice = "Delta failed: " + err.Error()
		m.deltaOpen = false
		return
	}
	m.deltaDone = true
	m.deltaOpen = false
	if !copyMode {
		// A delta dump replaces the picker's output, so finish the session.
		m.quit = true
	} else {
		m.notice = fmt.Sprintf("Delta copied to clipboard (%s)", strategyName(m.deltaStrategy))
	}
}

func strategyName(s DeltaStrategy) string {
	if s == DeltaPatch {
		return "patch"
	}
	return "full content"
}

// deltaRange computes the effective from/to from the checked commits. The
// commits slice is newest-first, so commit[i]'s parent is commit[i+1] (or the
// recorded baseline for the oldest entry).
func (m model) deltaRange() (from, to string, ok bool) {
	if m.delta == nil {
		return "", "", false
	}
	newest := -1
	oldest := -1
	for i, c := range m.delta.Commits {
		if c.Checked {
			if newest == -1 {
				newest = i
			}
			oldest = i
		}
	}
	if newest == -1 {
		return "", "", false
	}

	to = m.delta.Commits[newest].Short
	from = m.delta.FromHash
	if oldest+1 < len(m.delta.Commits) {
		from = m.delta.Commits[oldest+1].Short
	}
	if to == "" {
		to = m.delta.HeadHash
	}
	return from, to, true
}

// footerHeight returns the number of lines the footer occupies. The join
// newline between body and footer is included, so the whole view is exactly
// height lines tall and the footer sits on the bottom row.
func (m model) footerHeight() int {
	h := footerLines
	if m.notice != "" {
		h++
	}
	return h
}

func (m *model) clampOffset() {
	bodyHeight := max(5, m.height-m.footerHeight())
	innerRows := max(1, bodyHeight-3)

	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+innerRows {
		m.offset = m.cursor - innerRows + 1
	}
}

func (m model) View() string {
	width := max(20, m.width)
	height := max(10, m.height)

	leftWidth := width * 42 / 100
	if leftWidth < 32 {
		leftWidth = 32
	}
	if leftWidth > width-35 {
		leftWidth = width - 35
	}
	if leftWidth < 20 {
		leftWidth = width / 2
	}
	rightWidth := width - leftWidth

	bodyHeight := max(5, height-m.footerHeight())

	leftBox := m.renderTreeBox(leftWidth, bodyHeight)
	rightBox := m.renderPreviewBox(rightWidth, bodyHeight)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)
	footer := m.renderFooter(width)
	view := body + "\n" + footer

	if m.deltaOpen {
		view = m.renderDeltaModal(view, width, height)
	}
	return view
}

func (m model) renderTreeBox(width, height int) string {
	boxStyle := lipgloss.NewStyle().
		Width(max(1, width-2)).
		Height(max(1, height-2)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))

	var b strings.Builder
	title := fmt.Sprintf(" Explorer (%d files, %d tok) ",
		m.root.FileCount(), m.root.TotalActiveTokens())
	if m.glyphs.FolderOpen != "" {
		title = fmt.Sprintf(" %sExplorer (%d files, %d tok) ",
			m.glyphs.FolderOpen, m.root.FileCount(), m.root.TotalActiveTokens())
	}
	b.WriteString(titleStyle.Render(title))

	innerRows := max(1, height-3)
	end := min(len(m.rows), m.offset+innerRows)
	innerWidth := max(10, width-4)

	for i := m.offset; i < end; i++ {
		b.WriteString("\n")
		b.WriteString(m.treeRow(m.rows[i], innerWidth))
	}

	return boxStyle.Render(b.String())
}

func (m model) treeRow(n *TreeNode, width int) string {
	prefix := n.TreePrefix(m.glyphs)

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
		tokens = n.TotalActiveTokens()
	} else if n.Mode == ModeFull {
		tokens = n.TokensFull
	}

	secret := ""
	if n.SecretCount > 0 {
		secret = " " + warningStyle.Render(m.glyphs.Warning)
	}

	modeStr := ""
	if n.Kind == KindFile {
		switch n.Mode {
		case ModeFull:
			modeStr = " " + modeFullStyle.Render(m.glyphs.ModeFull)
		case ModeSignatures:
			modeStr = " " + modeSigStyle.Render(m.glyphs.ModeSigns)
		case ModeSkip:
			modeStr = " " + modeSkipStyle.Render(m.glyphs.ModeSkip)
		}
	}

	treeGuide := treeGuideStyle.Render(prefix)
	left := fmt.Sprintf("%s %s %s%s%s%s", treeGuide, mark, icon, name, modeStr, secret)
	right := fmt.Sprintf("%6d tok", tokens)

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)

	if leftWidth+rightWidth > width {
		fixed := lipgloss.Width(treeGuide) + lipgloss.Width(mark) + lipgloss.Width(icon) +
			lipgloss.Width(modeStr) + lipgloss.Width(secret) + 2
		nameSpace := width - rightWidth - fixed
		if nameSpace >= 2 && len(name) > nameSpace {
			name = truncateString(name, nameSpace)
			left = fmt.Sprintf("%s %s %s%s%s%s", treeGuide, mark, icon, name, modeStr, secret)
			leftWidth = lipgloss.Width(left)
		}
	}

	pad := max(0, width-leftWidth-rightWidth)
	row := left + strings.Repeat(" ", pad) + right

	if n == m.node() {
		return cursorStyle.Render(row)
	}
	if n.SelectState != Unselected {
		return selectedStyle.Render(row)
	}
	return row
}

func (m model) renderPreviewBox(width, height int) string {
	boxStyle := lipgloss.NewStyle().
		Width(max(1, width-2)).
		Height(max(1, height-2)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1)

	var b strings.Builder
	n := m.node()

	title := " Preview "
	if n != nil && n.Kind == KindFile {
		title = fmt.Sprintf(" %sPreview: %s ", m.glyphs.File, n.Path)
	} else if n != nil && n.Kind == KindDir {
		title = fmt.Sprintf(" %sFolder: %s/ ", m.glyphs.FolderOpen, n.Path)
	}
	b.WriteString(titleStyle.Render(title))

	if n == nil {
		b.WriteString("\n")
		b.WriteString(hintStyle.Render("No file selected"))
		return boxStyle.Render(b.String())
	}

	innerWidth := max(10, width-4)

	if n.Kind == KindDir {
		innerRows := max(1, height-3)
		lines := []string{
			fmt.Sprintf("%d files, %d tokens", n.FileCount(), n.TotalActiveTokens()),
			"",
			"Press [Space] to toggle directory selection.",
			"Press [Enter] or [l] to expand/collapse.",
		}
		for i := 0; i < len(lines) && i < innerRows; i++ {
			b.WriteString("\n")
			b.WriteString(hintStyle.Render(lines[i]))
		}
	} else {
		content := n.Preview()
		lines := strings.Split(string(content), "\n")

		innerRows := max(1, height-3)
		if n.SecretCount > 0 {
			innerRows--
		}

		maxLines := min(len(lines), innerRows)
		for i := 0; i < maxLines; i++ {
			lineNo := i + 1
			lineText := lines[i]

			prefix := fmt.Sprintf("%3d │ ", lineNo)
			prefixWidth := lipgloss.Width(prefix)
			maxLen := max(1, innerWidth-prefixWidth)

			lineText = strings.ReplaceAll(lineText, "\t", "    ")
			if lipgloss.Width(lineText) > maxLen {
				lineText = truncateString(lineText, maxLen)
			}

			b.WriteString("\n")
			b.WriteString(dimStyle.Render(prefix))
			b.WriteString(lineText)
		}

		if n.SecretCount > 0 {
			b.WriteString("\n")
			b.WriteString(warningStyle.Render(fmt.Sprintf("%sWarning: %d secret(s) detected in this file",
				m.glyphs.Warning, n.SecretCount)))
		}
	}

	return boxStyle.Render(b.String())
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

	keys := "space toggle   m mode   a all/none   s smart   / filter   y copy   d delta   enter done   q quit"
	if m.filtering {
		keys = "/ filter: " + m.filter + "▌"
	}

	var b strings.Builder
	b.WriteString(strings.Repeat("─", width))
	if m.notice != "" {
		b.WriteString("\n")
		b.WriteString(noticeStyle.Render(m.notice))
	}
	b.WriteString("\n")
	b.WriteString(hintStyle.Render(fmt.Sprintf("%sStyle: %s | %d selected, %d tokens | %s",
		budget, m.style, selected, active, keys)))
	return b.String()
}

// renderDeltaModal overlays the delta selection modal centered on the view.
func (m model) renderDeltaModal(view string, width, height int) string {
	modalWidth := min(width, 64)
	if modalWidth < 30 {
		modalWidth = 30
	}

	var b strings.Builder
	b.WriteString(" Incremental Context / Delta Mode ")
	b.WriteString("\n")

	if m.delta == nil {
		b.WriteString(hintStyle.Render("Not available outside a git repository."))
		return m.overlay(view, boxStyle(modalWidth).Render(b.String()), width, height)
	}

	b.WriteString(fmt.Sprintf("Project: %s", m.delta.RootDir))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Last Dumped: %s %s", m.delta.FromHash, truncateString(m.delta.FromMsg, modalWidth-30)))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("HEAD:        %s %s", m.delta.HeadHash, truncateString(m.delta.HeadMsg, modalWidth-30)))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("Commits since last dump:"))
	b.WriteString("\n")

	// Scrollable commit list, newest first.
	innerRows := max(1, height-14)
	end := min(len(m.delta.Commits), m.deltaOffset+innerRows)
	for i := m.deltaOffset; i < end; i++ {
		c := m.delta.Commits[i]
		mark := " [ ] "
		if c.Checked {
			mark = " [x] "
		}
		line := fmt.Sprintf("%s %s %s", mark, c.Short, c.Subject)
		line = truncateString(line, modalWidth-4)
		if i == m.deltaCursor {
			b.WriteString(cursorStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Delta output mode:"))
	b.WriteString("\n")

	modeFull := "( ) "
	if m.deltaStrategy == DeltaFull {
		modeFull = "(*) "
	}
	modePatch := "( ) "
	if m.deltaStrategy == DeltaPatch {
		modePatch = "(*) "
	}
	b.WriteString(fmt.Sprintf("%sFull content of modified files (%d files, %d tokens)", modeFull, len(m.delta.Files), m.delta.FilesToken))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%sGit unified patch diff (%d lines, %d tokens)", modePatch, m.delta.PatchLines, m.delta.PatchToken))
	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render("[Enter] perform delta dump | [c] copy | [Esc] cancel"))

	modal := boxStyle(modalWidth).Render(b.String())
	return m.overlay(view, modal, width, height)
}

// boxStyle returns a rounded bordered box of the given width.
func boxStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(max(1, width-2)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))
}

// overlay centers sub over view, blanking the area behind it.
func (m model) overlay(view, sub string, width, height int) string {
	subHeight := lipgloss.Height(sub)
	subWidth := lipgloss.Width(sub)
	top := max(0, (height-subHeight)/2)
	if top > height-1 {
		top = height - 1
	}
	left := max(0, (width-subWidth)/2)
	right := max(0, width-subWidth-left)

	viewLines := strings.Split(view, "\n")
	subLines := strings.Split(sub, "\n")

	for i := 0; i < len(subLines) && top+i < len(viewLines); i++ {
		viewLines[top+i] = strings.Repeat(" ", left) + subLines[i] + strings.Repeat(" ", right)
	}
	return strings.Join(viewLines, "\n")
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

func truncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen == 1 {
		return "…"
	}
	return string(runes[:maxLen-1]) + "…"
}

const headerLines = 2
const footerLines = 2

var (
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	hintStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	treeGuideStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("239"))
	cursorStyle    = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("236")).Foreground(lipgloss.Color("15"))
	selectedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	warningStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
	noticeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))

	modeFullStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // Green
	modeSigStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // Yellow
	modeSkipStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))  // Red
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
