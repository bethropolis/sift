package tui

import (
	"strings"
	"time"

	"github.com/bethropolis/sift/internal/highlight"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// clearNoticeMsg is dispatched after a timer to restore the standard footer
// status line. The id guards against a stale timer clearing a newer notice.
type clearNoticeMsg struct {
	id int
}

// PaneFocus identifies which pane currently owns keyboard input.
type PaneFocus int

const (
	FocusTree PaneFocus = iota
	FocusPreview
)

// model is the BubbleTea state for the dual-pane picker: a foldable tree on
// the left and a live file preview on the right, with a budget footer.
type model struct {
	root   *TreeNode
	rows   []*TreeNode // cached visible rows.
	cursor int
	offset int

	// focus selects the active pane. FocusTree routes arrows/j/k to the tree;
	// FocusPreview routes them to scrolling the preview text.
	focus PaneFocus

	// previewOffset is the first line shown in the preview pane, and
	// previewNode the node it belongs to. Scrolling is reset whenever the
	// cursor moves to a different node.
	previewOffset int
	previewNode   *TreeNode

	height int
	width  int

	filter         string
	filtering      bool
	showHidden     bool
	showGitIgnored bool

	budget      int
	style       string
	glyphs      Glyphs
	highlight   highlight.Options
	windowTitle string

	onCopy func([]Selection) error
	notice string
	// noticeID stamps each notice so only its own timer clears it.
	noticeID int

	// Help modal state.
	helpOpen   bool
	helpOffset int
	onGenerate func([]Selection) error

	// Delta modal state.
	delta         *DeltaInfo
	deltaOpen     bool
	deltaCursor   int
	deltaOffset   int
	deltaStrategy DeltaStrategy
	onDelta       func(DeltaSelection) error
	deltaDone     bool

	// Progressive scan state. stream is the background scan's channels;
	// nodeIndex maps relative paths to nodes for O(1) merges; scan* fields
	// feed the footer's progress line until scanDone.
	stream        Stream
	nodeIndex     map[string]*TreeNode
	scanFiles     int
	scanDirs      int
	scanProcessed int
	scanDone      bool
	// stream*Closed track which scan channels have been closed, so the
	// listener stops only once every channel is exhausted.
	streamNodesClosed    bool
	streamProgressClosed bool
	streamErrClosed      bool

	quit bool
}

// Options configures the picker.
type Options struct {
	Budget            int
	Style             string
	UseNerd           bool
	Highlight         bool
	Theme             string
	HighlightMaxBytes int
	WindowTitle       string
	OnCopy            func([]Selection) error

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
	nodeIndex := make(map[string]*TreeNode, 256)
	indexTree(root, nodeIndex)
	m := model{
		root:        root,
		height:      24,
		width:       80,
		budget:      opts.Budget,
		style:       opts.Style,
		glyphs:      glyphs,
		highlight:   highlight.Options{Enabled: opts.Highlight, Theme: highlight.Theme(opts.Theme), MaxBytes: opts.HighlightMaxBytes},
		windowTitle: sanitizeWindowTitle(opts.WindowTitle),
		onCopy:      opts.OnCopy,
		onGenerate:  opts.OnGenerate,
		delta:       opts.Delta,
		onDelta:     opts.OnDelta,
		nodeIndex:   nodeIndex,
	}
	m.recomputeRows()
	return m
}

func (m *model) recomputeRows() {
	m.rows = m.root.VisibleRowsWithOptions(m.filter, m.showHidden, m.showGitIgnored)
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

func (m model) Init() tea.Cmd {
	if m.windowTitle == "" {
		return m.listenStream()
	}
	return tea.Batch(m.listenStream(), tea.SetWindowTitle(m.windowTitle))
}

func sanitizeWindowTitle(title string) string {
	title = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == '\x1b' {
			return -1
		}
		return r
	}, title)
	return truncateString(strings.TrimSpace(title), 96)
}

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
		if m.helpOpen {
			m.scrollHelp(0)
		}
	case tea.KeyMsg:
		return m.updateKey(msg)
	case tea.MouseMsg:
		return m.updateMouse(msg)
	case NodesMsg:
		// Keep the cursor pinned to the same path while new nodes arrive.
		keepPath := ""
		if n := m.node(); n != nil {
			keepPath = n.Path
		}
		if len(msg.Remove) > 0 {
			m.removeNodes(msg.Remove)
		}
		if len(msg.Items) > 0 {
			m.upsertItems(msg.Items)
		}
		m.recomputeRows()
		if keepPath != "" {
			if idx := m.findRow(keepPath); idx >= 0 {
				m.cursor = idx
				m.clampOffset()
			} else {
				// The pinned node was removed; fall back to the first row.
				m.cursor = 0
				m.clampOffset()
			}
		}
		return m, m.listenStream()
	case ProgressMsg:
		m.scanFiles = msg.Files
		m.scanDirs = msg.Dirs
		m.scanProcessed = msg.Processed
		if msg.Done {
			m.scanDone = true
		}
		// Keep draining: removal messages may still be in flight on the nodes
		// channel after Done arrives. The listener only stops once a channel
		// closes (buffered values are drained before ok=false).
		return m, m.listenStream()
	case streamClosedMsg:
		switch msg.channel {
		case streamNodesClosed:
			m.streamNodesClosed = true
		case streamProgressClosed:
			m.streamProgressClosed = true
		case streamErrClosed:
			m.streamErrClosed = true
		}
		if m.streamNodesClosed && m.streamProgressClosed && m.streamErrClosed {
			m.scanDone = true
			return m, nil
		}
		return m, m.listenStream()
	case ErrMsg:
		m.scanDone = true
		return m, m.setNotice("Scan error: " + msg.Err.Error())
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

// leftPaneWidth returns the rendered width of the explorer pane. The mouse
// hit-test for pane focus must agree with this, so both View and updateMouse
// derive the boundary from the same helper.
func (m model) leftPaneWidth() int {
	width := max(20, m.width)
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
	return leftWidth
}

func (m model) treeViewportRows() int {
	bodyHeight := max(5, m.height-m.footerHeight())
	return max(1, bodyHeight-3)
}

func (m *model) clampTreeOffset() {
	if len(m.rows) == 0 {
		m.cursor, m.offset = 0, 0
		return
	}
	visible := m.treeViewportRows()
	maxOffset := max(0, len(m.rows)-visible)
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
	if m.offset < 0 {
		m.offset = 0
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m model) View() string {
	width := max(20, m.width)
	height := max(10, m.height)

	leftWidth := m.leftPaneWidth()
	rightWidth := width - leftWidth

	bodyHeight := max(5, height-m.footerHeight())

	leftBox := m.renderTreeBox(leftWidth, bodyHeight)
	rightBox := m.renderPreviewBox(rightWidth, bodyHeight)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)
	footer := m.renderFooter(width)
	// Lipgloss may leave a terminal newline when a fixed-height box is
	// rendered. Normalize it before adding the single body/footer separator so
	// the footer stays on the final terminal row.
	view := strings.TrimRight(body, "\n") + "\n" + footer

	if m.deltaOpen {
		view = m.renderDeltaModal(view, width, height)
	}
	if m.helpOpen {
		view = m.renderHelpModal(view, width, height)
	}
	return clampViewHeight(view, width, height)
}

// clampViewHeight keeps every rendered frame at the terminal height. Bubble
// Tea's line-diff renderer relies on a stable frame size; an oversized pane
// or modal can otherwise leave stale borders and titles behind after rapid
// input.
func clampViewHeight(view string, width, height int) string {
	view = strings.TrimRight(view, "\n")
	lines := strings.Split(view, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i, line := range lines {
		if ansi.StringWidth(line) > width {
			lines[i] = ansi.Truncate(line, width, "")
		}
	}
	return strings.Join(lines, "\n")
}
