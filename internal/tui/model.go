package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/bethropolis/sift/internal/highlight"
	"github.com/bethropolis/sift/internal/selection"
	"github.com/bethropolis/sift/internal/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// genJobMsg carries the outcome of a background generate/copy job back to
// the Update loop, which must apply it on the main goroutine: model mutation
// outside Update races the Bubble Tea renderer.
type genJobMsg struct {
	text string
}

// genTickInterval is the footer refresh cadence while a generate/copy job
// runs, keeping the UI visibly alive during long renders.
const genTickInterval = 250 * time.Millisecond

// genExitWait bounds how long quitting waits for an in-flight generate/copy
// job, so a hung clipboard tool cannot wedge process exit (the output write
// itself normally completes in well under this).
const genExitWait = 30 * time.Second

// clearNoticeMsg is dispatched after a timer to restore the standard footer
// status line. The id guards against a stale timer clearing a newer notice.
type clearNoticeMsg struct {
	id int
}

// busyTickMsg advances the elapsed-time line of a running generate/copy job.
// It re-arms itself while the job is still running under the same notice id,
// so the footer shows live progress at a 250ms cadence instead of appearing
// wedged for the whole render.
type busyTickMsg struct {
	id      int
	label   string
	started time.Time
}

// PaneFocus identifies which pane currently owns keyboard input.
type PaneFocus int

const (
	FocusTree PaneFocus = iota
	FocusPreview

	headerRows = 1
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

	budget          int
	style           string
	glyphs          Glyphs
	highlight       highlight.Options
	syntaxCache     *highlight.SyntaxCache
	windowTitle     string
	projectPath     string
	styles          uiStyles
	themes          []theme.ThemePreset
	prompt          string
	selectionTuning selection.Tuning

	// genBusy guards the background generate/copy path: exactly one job may
	// be in flight. A second Y/g/y press while busy is acknowledged with a
	// notice instead of stacking another unbounded render on the same
	// selection. genDone is closed by the in-flight job when it finishes so
	// runProgram can wait for a pending write before the process exits.
	genBusy bool
	genDone chan struct{}

	onCopy         func([]Selection) error
	onCopyPrompt   func([]Selection, string) error
	onGenerateCopy func([]Selection, string) error
	notice         string
	// noticeID stamps each notice so only its own timer clears it.
	noticeID int

	// Help modal state.
	helpOpen         bool
	helpOffset       int
	onGenerate       func([]Selection) error
	onGeneratePrompt func([]Selection, string) error

	// Theme modal state.
	themeOpen     bool
	themeCursor   int
	themeOffset   int
	themeIndex    int
	onThemeChange func(string) error

	// Prompt/directive modal state.
	promptOpen   bool
	promptCursor int
	promptInput  string
	promptCustom bool

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
	// onRescan starts a fresh background scan (pressing r). rescanActive
	// guards the reconcile window; rescanBaseline/rescanSeen track which
	// pre-rescan paths the fresh walk re-reported so vanished files can be
	// pruned once every channel is exhausted.
	onRescan       func() (Stream, error)
	rescanActive   bool
	rescanBaseline map[string]bool
	rescanSeen     map[string]bool

	quit bool

	// spinnerFrame advances every ProgressMsg to animate the scan indicator.
	spinnerFrame int
}

// Options configures the picker.
type Options struct {
	Budget            int
	Style             string
	UseNerd           bool
	Highlight         bool
	Theme             string
	UITheme           string
	UserThemes        []theme.ThemePreset
	HighlightMaxBytes int
	WindowTitle       string
	ProjectPath       string
	OnThemeChange     func(string) error
	Prompt            string
	SelectionTuning   selection.Tuning
	OnCopy            func([]Selection) error
	OnCopyPrompt      func([]Selection, string) error
	OnGenerateCopy    func([]Selection, string) error

	// OnGenerate renders the current selection without exiting the picker
	// (pressing g). It mirrors OnCopy but writes the document instead of the
	// clipboard.
	OnGenerate func([]Selection) error
	// OnGeneratePrompt is the prompt-aware variant used by the TUI directive
	// builder. When nil, OnGenerate is used for backward compatibility.
	OnGeneratePrompt func([]Selection, string) error

	// Delta is the git delta state shown in the delta modal (pressing d).
	// When nil the modal shows "not available" messaging. OnDelta is invoked
	// when the user confirms a delta dump.
	Delta   *DeltaInfo
	OnDelta func(DeltaSelection) error

	// OnRescan starts a fresh background scan and returns its stream
	// (pressing r). When nil the picker reports rescan as unavailable.
	OnRescan func() (Stream, error)
}

func containsThemeID(themes []theme.ThemePreset, id string) bool {
	for _, preset := range themes {
		if preset.ID == id {
			return true
		}
	}
	return false
}

func newModel(root *TreeNode, opts Options) model {
	themes := append([]theme.ThemePreset(nil), theme.ThemePresets...)
	for _, userTheme := range opts.UserThemes {
		if !containsThemeID(themes, userTheme.ID) {
			themes = append(themes, userTheme)
		}
	}
	glyphs := NewASCIIGlyphs()
	if opts.UseNerd {
		glyphs = NewNerdFontGlyphs()
	}
	nodeIndex := make(map[string]*TreeNode, 256)
	indexTree(root, nodeIndex)
	m := model{
		root:             root,
		height:           24,
		width:            80,
		budget:           opts.Budget,
		style:            opts.Style,
		glyphs:           glyphs,
		highlight:        highlight.Options{Enabled: opts.Highlight, Theme: highlight.Theme(opts.Theme), MaxBytes: opts.HighlightMaxBytes, Profile: currentColorProfile()},
		syntaxCache:      highlight.NewSyntaxCache(),
		windowTitle:      sanitizeWindowTitle(opts.WindowTitle),
		projectPath:      opts.ProjectPath,
		styles:           defaultStyles(),
		themes:           themes,
		themeIndex:       theme.IndexOf(themes, opts.UITheme),
		themeCursor:      theme.IndexOf(themes, opts.UITheme),
		onThemeChange:    opts.OnThemeChange,
		prompt:           opts.Prompt,
		selectionTuning:  opts.SelectionTuning,
		onCopy:           opts.OnCopy,
		onCopyPrompt:     opts.OnCopyPrompt,
		onGenerateCopy:   opts.OnGenerateCopy,
		onGenerate:       opts.OnGenerate,
		onGeneratePrompt: opts.OnGeneratePrompt,
		delta:            opts.Delta,
		onDelta:          opts.OnDelta,
		onRescan:         opts.OnRescan,
		nodeIndex:        nodeIndex,
	}
	m.applyTheme(m.themes[m.themeIndex])
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
		return tea.Batch(m.listenStream(), pollThemeCmd())
	}
	return tea.Batch(m.listenStream(), pollThemeCmd(), tea.SetWindowTitle(m.windowTitle))
}

func currentColorProfile() highlight.ColorProfile {
	switch lipgloss.ColorProfile() {
	case termenv.TrueColor:
		return highlight.ProfileTrueColor
	case termenv.ANSI256:
		return highlight.ProfileANSI256
	case termenv.ANSI:
		return highlight.ProfileANSI16
	default:
		return highlight.ProfileNone
	}
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
	case themePollMsg:
		m.syncExternalTheme(msg.id)
		return m, pollThemeCmd()
	case clearNoticeMsg:
		if msg.id == m.noticeID {
			m.notice = ""
		}
		return m, nil
	case genJobMsg:
		// Exactly one job can be in flight (genBusy), so this always
		// belongs to the running job: release it and show the outcome.
		m.genBusy = false
		m.genDone = nil
		m.notice = msg.text
		m.noticeID++
		id := m.noticeID
		return m, tea.Tick(2500*time.Millisecond, func(time.Time) tea.Msg {
			return clearNoticeMsg{id: id}
		})
	case busyTickMsg:
		if !m.genBusy {
			return m, nil
		}
		// Only refresh when no newer notice displaced the job's line;
		// keep re-arming either way so the ticker dies with the job.
		if msg.id == m.noticeID {
			elapsed := time.Since(msg.started).Round(100 * time.Millisecond)
			m.notice = fmt.Sprintf("%s · %s", msg.label, elapsed)
		}
		return m, tea.Tick(genTickInterval, func(time.Time) tea.Msg {
			return busyTickMsg{id: msg.id, label: msg.label, started: msg.started}
		})
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
		m.trackRescanSeen(msg.Items)
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
		m.spinnerFrame++
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
			return m, m.finishRescan()
		}
		return m, m.listenStream()
	case ErrMsg:
		m.scanDone = true
		m.abortRescan()
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

func (m model) headerHeight() int { return headerRows }

func (m model) bodyHeight() int {
	return max(5, max(10, m.height)-m.headerHeight()-m.footerHeight())
}

func (m model) treeViewportRows() int {
	return max(1, m.bodyHeight()-3)
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

func (m model) renderHeader(width int) string {
	if width <= 0 {
		return ""
	}
	selected := m.root.SelectedCount()
	files := m.root.FileCount()
	active := m.root.TotalActiveTokens()

	compactTokens := formatTokenCount(active)
	tokenSummary := compactTokens
	if m.budget > 0 {
		tokenSummary += "/" + formatTokenCount(m.budget)
	} else {
		tokenSummary += " tok"
	}
	rightText := fmt.Sprintf("%d/%d · %s", selected, files, tokenSummary)
	// Selection and budget are the header's essential state. Keep them at narrow
	// widths and yield space to the project path first.
	rightText = ansi.Truncate(rightText, max(1, width-5), "…")

	left := m.styles.appTitle.Render("sift")
	project := displayProjectPath(m.projectPath)
	if project != "" {
		leftRoom := width - ansi.StringWidth(rightText) - ansi.StringWidth(left) - 2
		if leftRoom >= 4 {
			path := truncateTail(project, leftRoom-2)
			left += m.styles.muted.Render("  " + path)
		}
	}
	gap := width - ansi.StringWidth(left) - ansi.StringWidth(rightText)
	if gap < 1 {
		return ansi.Truncate(left, width, "")
	}
	return left + strings.Repeat(" ", gap) + m.styles.hint.Render(rightText)
}

func (m model) View() string {
	width := max(20, m.width)
	height := max(10, m.height)

	leftWidth := m.leftPaneWidth()
	rightWidth := width - leftWidth
	bodyHeight := m.bodyHeight()

	leftBox := m.renderTreeBox(leftWidth, bodyHeight)
	rightBox := m.renderPreviewBox(rightWidth, bodyHeight)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)
	header := m.renderHeader(width)
	footer := m.renderFooter(width)
	view := strings.TrimRight(header+"\n"+body, "\n") + "\n" + footer

	if m.deltaOpen {
		view = m.renderDeltaModal(view, width, height)
	}
	if m.helpOpen {
		view = m.renderHelpModal(view, width, height)
	}
	if m.themeOpen {
		view = m.renderThemeModal(view, width, height)
	}
	if m.promptOpen {
		view = m.renderPromptModal(view, width, height)
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
