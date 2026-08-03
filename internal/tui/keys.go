package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// updateMouse routes mouse events to the pane under the cursor: the wheel
// moves the tree cursor on the left or scrolls the preview on the right, and
// a click switches focus to the clicked pane.
func (m model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.helpOpen {
		switch msg.Type {
		case tea.MouseWheelUp:
			m.scrollHelp(-3)
		case tea.MouseWheelDown:
			m.scrollHelp(3)
		}
		return m, nil
	}
	if m.themeOpen {
		switch msg.Type {
		case tea.MouseWheelUp:
			m.themeMove(-1)
		case tea.MouseWheelDown:
			m.themeMove(1)
		}
		return m, nil
	}
	// While a modal or filter is open, ignore pointer input entirely.
	if m.deltaOpen || m.filtering {
		return m, nil
	}

	leftWidth := m.leftPaneWidth()
	onPreview := msg.X > leftWidth

	switch msg.Type {
	case tea.MouseWheelUp:
		if onPreview {
			m.scrollPreview(-3)
		} else {
			m.move(-1)
		}
	case tea.MouseWheelDown:
		if onPreview {
			m.scrollPreview(3)
		} else {
			m.move(1)
		}
	case tea.MouseRelease:
		if onPreview {
			m.focus = FocusPreview
		} else {
			m.focus = FocusTree
		}
	}
	return m, nil
}

func (m model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.deltaOpen {
		return m.updateDeltaKey(msg)
	}

	if m.helpOpen {
		switch msg.Type {
		case tea.KeyUp:
			m.scrollHelp(-1)
		case tea.KeyDown:
			m.scrollHelp(1)
		case tea.KeyPgUp:
			m.scrollHelp(-m.helpPageSize())
		case tea.KeyPgDown:
			m.scrollHelp(m.helpPageSize())
		case tea.KeyCtrlU:
			m.scrollHelp(-m.helpPageSize() / 2)
		case tea.KeyCtrlD:
			m.scrollHelp(m.helpPageSize() / 2)
		case tea.KeyEsc, tea.KeyCtrlC, tea.KeyCtrlQ:
			m.helpOpen = false
		case tea.KeyRunes:
			if r := string(msg.Runes); r == "j" {
				m.scrollHelp(1)
			} else if r == "k" {
				m.scrollHelp(-1)
			} else if r == "?" || r == "q" {
				m.helpOpen = false
			}
		}
		return m, nil
	}

	if m.themeOpen {
		switch msg.Type {
		case tea.KeyUp, tea.KeyShiftTab:
			m.themeMove(-1)
		case tea.KeyDown, tea.KeyTab:
			m.themeMove(1)
		case tea.KeyPgUp:
			m.themeMove(-m.themePageSize())
		case tea.KeyPgDown:
			m.themeMove(m.themePageSize())
		case tea.KeyEnter, tea.KeySpace:
			m.themeOpen = false
			m.themeIndex = m.themeCursor
			m.applyTheme(ThemePresets[m.themeCursor])
		case tea.KeyEsc, tea.KeyCtrlC, tea.KeyCtrlQ:
			m.themeOpen = false
		case tea.KeyRunes:
			if r := string(msg.Runes); r == "j" {
				m.themeMove(1)
			} else if r == "k" {
				m.themeMove(-1)
			} else if r == "t" || r == "q" {
				m.themeOpen = false
			}
		}
		return m, nil
	}

	if m.filtering {
		switch msg.Type {
		case tea.KeyEsc:
			m.filtering = false
			m.filter = ""
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
	case tea.KeyTab:
		if m.focus == FocusTree {
			m.focus = FocusPreview
		} else {
			m.focus = FocusTree
		}
	case tea.KeyCtrlC, tea.KeyCtrlQ:
		m.quit = true
		return m, tea.Quit
	case tea.KeyEsc:
		if m.filter != "" {
			m.filter = ""
			m.filtering = false
			m.recomputeRows()
			return m, nil
		}
		if m.focus == FocusPreview {
			m.focus = FocusTree
			return m, nil
		}
		m.quit = true
		return m, tea.Quit
	case tea.KeyEnter:
		if n := m.node(); n != nil {
			if n.Kind == KindDir {
				n.Expanded = !n.Expanded
				m.recomputeRows()
			} else {
				n.Toggle()
			}
		}
	case tea.KeyUp, tea.KeyShiftTab:
		if m.focus == FocusPreview {
			m.scrollPreview(-1)
		} else {
			m.move(-1)
		}
	case tea.KeyDown:
		if m.focus == FocusPreview {
			m.scrollPreview(1)
		} else {
			m.move(1)
		}
	case tea.KeyPgUp:
		m.scrollPreview(-m.previewPageSize())
	case tea.KeyPgDown:
		m.scrollPreview(m.previewPageSize())
	case tea.KeyCtrlU:
		m.scrollPreview(-m.previewHalfPage())
	case tea.KeyCtrlD:
		m.scrollPreview(m.previewHalfPage())
	case tea.KeySpace:
		if n := m.node(); n != nil {
			n.Toggle()
		}
	case tea.KeyLeft:
		if m.focus == FocusPreview {
			m.focus = FocusTree
			return m, nil
		}
		m.collapseOrParent()
	case tea.KeyRight:
		if n := m.node(); n != nil && n.Kind == KindDir {
			n.Expanded = true
			m.recomputeRows()
		}
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "j":
			if m.focus == FocusPreview {
				m.scrollPreview(1)
			} else {
				m.move(1)
			}
		case "k":
			if m.focus == FocusPreview {
				m.scrollPreview(-1)
			} else {
				m.move(-1)
			}
		case "h":
			if m.focus == FocusPreview {
				m.focus = FocusTree
				return m, nil
			}
			m.collapseOrParent()
		case "l":
			if n := m.node(); n != nil && n.Kind == KindDir {
				n.Expanded = true
				m.recomputeRows()
			}
		case "J":
			m.scrollPreview(1)
		case "K":
			m.scrollPreview(-1)
		case "[":
			m.scrollPreview(-m.previewPageSize())
		case "]":
			m.scrollPreview(m.previewPageSize())
		case " ":
			if n := m.node(); n != nil {
				n.Toggle()
			}
		case "m":
			if n := m.node(); n != nil {
				m.cycleMode(n)
			}
		case "a":
			m.selectAll()
		case "s":
			m.smartSelect()
		case "/":
			m.filtering = true
			m.filter = ""
		case "y":
			return m, m.copy()
		case "d":
			m.openDelta()
		case "?":
			m.helpOpen = !m.helpOpen
		case "t":
			m.themeOpen = !m.themeOpen
			if m.themeOpen {
				m.themeCursor = m.themeIndex
				m.themeOffset = 0
			}
		case ".":
			m.showHidden = !m.showHidden
			m.recomputeRows()
		case "H":
			m.showGitIgnored = !m.showGitIgnored
			m.recomputeRows()
		case "g":
			return m, m.generate()
		case "E":
			m.root.ExpandAll()
			m.recomputeRows()
		case "C":
			m.root.CollapseAll()
			m.recomputeRows()
		case "q":
			if m.filter != "" {
				m.filter = ""
				m.filtering = false
				m.recomputeRows()
				return m, nil
			}
			m.quit = true
			return m, tea.Quit
		}
	}
	m.clampOffset()
	return m, nil
}

func (m model) helpPageSize() int {
	return max(1, max(8, m.height-2)-5)
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
	m.syncPreview()
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
	m.syncPreview()
}

func (m *model) selectAll() {
	if m.root.SelectedCount() == m.root.FileCount() {
		m.root.ClearSelection()
	} else {
		m.root.setSelected(true)
	}
}

func (m *model) smartSelect() {
	// Checkboxes and the token tally react instantly; a notice here would only
	// linger redundantly.
	m.root.SelectByRank(m.budget)
}

func (m *model) copy() tea.Cmd {
	if m.onCopy == nil {
		return nil
	}
	sel := m.root.Selections()
	if len(sel) == 0 {
		return m.setNotice("Nothing selected to copy")
	}
	if err := m.onCopy(sel); err != nil {
		return m.setNotice("Copy failed: " + err.Error())
	}
	return m.setNotice(fmt.Sprintf("Copied %d files (%d tokens) to clipboard", len(sel), m.root.TotalActiveTokens()))
}

// generate renders the current selection to the output document without
// leaving the picker, so the user can keep tweaking the selection.
func (m *model) generate() tea.Cmd {
	if m.onGenerate == nil {
		return nil
	}
	sel := m.root.Selections()
	if len(sel) == 0 {
		return m.setNotice("Nothing selected to generate")
	}
	if err := m.onGenerate(sel); err != nil {
		return m.setNotice("Generate failed: " + err.Error())
	}
	return m.setNotice(fmt.Sprintf("Generated output (%d files, %d tokens)", len(sel), m.root.TotalActiveTokens()))
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

// themePageSize returns how many presets fit in the theme modal at once.
func (m model) themePageSize() int {
	return max(1, m.height-6)
}

// themeMove moves the theme selector cursor, clamping it to the preset list.
func (m *model) themeMove(delta int) {
	m.themeCursor += delta
	if m.themeCursor < 0 {
		m.themeCursor = 0
	}
	if m.themeCursor >= len(ThemePresets) {
		m.themeCursor = len(ThemePresets) - 1
	}
}
