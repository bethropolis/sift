package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.deltaOpen {
		return m.updateDeltaKey(msg)
	}

	if m.helpOpen {
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC, tea.KeyCtrlQ:
			m.helpOpen = false
		case tea.KeyRunes:
			if r := string(msg.Runes); r == "?" || r == "q" {
				m.helpOpen = false
			}
		}
		return m, nil
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
	case tea.KeyCtrlC, tea.KeyCtrlQ, tea.KeyEsc:
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
		m.move(-1)
	case tea.KeyDown, tea.KeyTab:
		m.move(1)
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
		case "?":
			m.helpOpen = !m.helpOpen
		case "g":
			m.generate()
		case "E":
			m.root.ExpandAll()
			m.recomputeRows()
		case "C":
			m.root.CollapseAll()
			m.recomputeRows()
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

// generate renders the current selection to the output document without
// leaving the picker, so the user can keep tweaking the selection.
func (m *model) generate() {
	if m.onGenerate == nil {
		return
	}
	sel := m.root.Selections()
	if len(sel) == 0 {
		m.notice = "Nothing selected to generate"
		return
	}
	if err := m.onGenerate(sel); err != nil {
		m.notice = "Generate failed: " + err.Error()
		return
	}
	m.notice = fmt.Sprintf("Generated output (%d files, %d tokens)", len(sel), m.root.TotalActiveTokens())
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
