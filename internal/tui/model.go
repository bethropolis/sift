package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// clearNoticeMsg is dispatched after a timer to restore the standard footer
// status line. The id guards against a stale timer clearing a newer notice.
type clearNoticeMsg struct {
	id int
}

// model is the BubbleTea state for the dual-pane picker: a foldable tree on
// the left and a live file preview on the right, with a budget footer.
type model struct {
	root   *TreeNode
	rows   []*TreeNode // cached visible rows.
	cursor int
	offset int

	// previewOffset is the first line shown in the preview pane, and
	// previewNode the node it belongs to. Scrolling is reset whenever the
	// cursor moves to a different node.
	previewOffset int
	previewNode   *TreeNode

	height int
	width  int

	filter    string
	filtering bool

	budget int
	style  string
	glyphs Glyphs

	onCopy func([]Selection) error
	notice string
	// noticeID stamps each notice so only its own timer clears it.
	noticeID int

	// Help modal state.
	helpOpen   bool
	onGenerate func([]Selection) error

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

	// OnGenerate renders the current selection without exiting the picker
	// (pressing g). It mirrors OnCopy but writes the document instead of the
	// clipboard.
	OnGenerate func([]Selection) error

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
		root:       root,
		height:     24,
		width:      80,
		budget:     opts.Budget,
		style:      opts.Style,
		glyphs:     glyphs,
		onCopy:     opts.OnCopy,
		onGenerate: opts.OnGenerate,
		delta:      opts.Delta,
		onDelta:    opts.OnDelta,
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
	m.syncPreview()
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
	case clearNoticeMsg:
		if msg.id == m.noticeID {
			m.notice = ""
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.height, m.width = msg.Height, msg.Width
		m.clampOffset()
	case tea.KeyMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

// setNotice stores a transient status message and schedules it to clear after
// 2.5s. The returned tea.Cmd must be handed to BubbleTea for the timer to run.
func (m *model) setNotice(text string) tea.Cmd {
	m.notice = text
	m.noticeID++
	id := m.noticeID
	return tea.Tick(2500*time.Millisecond, func(time.Time) tea.Msg {
		return clearNoticeMsg{id: id}
	})
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
	if m.helpOpen {
		view = m.renderHelpModal(view, width, height)
	}
	return view
}
