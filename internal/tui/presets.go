package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/bethropolis/sift/internal/state"
)

func (m *model) presetRoot() string {
	root := m.projectPath
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return root
	}
	return abs
}

func (m *model) reloadPresets() {
	presets, err := state.ListPresets(m.presetRoot())
	if err != nil {
		m.presetError = err.Error()
		return
	}
	m.presets = presets
	m.presetError = ""
	if m.presetCursor >= len(m.presets) {
		m.presetCursor = max(0, len(m.presets)-1)
	}
}

func (m *model) openPresets() tea.Cmd {
	m.presetsOpen = true
	m.presetSaving = false
	m.presetInput = ""
	m.presetCursor = 0
	m.presetOffset = 0
	m.reloadPresets()
	return nil
}

func (m model) updatePresetKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.presetSaving {
		switch msg.Type {
		case tea.KeyEsc:
			m.presetSaving = false
			m.presetInput = ""
		case tea.KeyBackspace, tea.KeyDelete:
			if len(m.presetInput) > 0 {
				m.presetInput = m.presetInput[:len(m.presetInput)-1]
			}
		case tea.KeyEnter:
			m.savePreset()
		case tea.KeyRunes:
			m.presetInput += string(msg.Runes)
		}
		return m, nil
	}
	switch msg.Type {
	case tea.KeyEsc, tea.KeyCtrlC, tea.KeyCtrlQ:
		m.presetsOpen = false
	case tea.KeyUp:
		if m.presetCursor > 0 {
			m.presetCursor--
		}
	case tea.KeyDown:
		if m.presetCursor < len(m.presets)-1 {
			m.presetCursor++
		}
	case tea.KeyEnter:
		m.applyPreset()
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "j":
			if m.presetCursor < len(m.presets)-1 {
				m.presetCursor++
			}
		case "k":
			if m.presetCursor > 0 {
				m.presetCursor--
			}
		case "s":
			m.presetSaving = true
			m.presetInput = ""
		case "d":
			m.deletePreset()
		case "q":
			m.presetsOpen = false
		}
	}
	return m, nil
}

func (m *model) savePreset() {
	name := strings.TrimSpace(m.presetInput)
	if name == "" {
		m.presetError = "preset name is required"
		return
	}
	selections := map[string]string{}
	for _, sel := range m.root.Selections() {
		selections[sel.Path] = string(sel.Mode)
	}
	if len(selections) == 0 {
		m.presetError = "nothing selected"
		return
	}
	if _, err := state.SavePreset(m.presetRoot(), state.Preset{
		Name:       name,
		Selections: selections,
		Prompt:     m.prompt,
		Style:      m.style,
		Budget:     m.budget,
	}); err != nil {
		m.presetError = err.Error()
		return
	}
	m.presetSaving = false
	m.presetInput = ""
	m.reloadPresets()
}

func (m *model) applyPreset() {
	if m.presetCursor < 0 || m.presetCursor >= len(m.presets) {
		return
	}
	preset := m.presets[m.presetCursor]
	for _, node := range m.nodeIndex {
		if node.Kind != KindFile {
			continue
		}
		node.SelectState = Unselected
		node.Mode = ModeFull
	}
	for path, raw := range preset.Selections {
		node := m.nodeIndex[path]
		if node == nil || node.Kind != KindFile {
			continue // vanished file: ignore gracefully
		}
		switch CompressMode(raw) {
		case ModeSkip:
			node.Mode = ModeSkip
			node.SelectState = Unselected
		case ModeSignatures:
			node.Mode = ModeSignatures
			node.SelectState = Selected
		default:
			node.Mode = ModeFull
			node.SelectState = Selected
		}
	}
	if preset.Prompt != "" {
		m.prompt = preset.Prompt
	}
	if preset.Style != "" {
		m.style = preset.Style
	}
	if preset.Budget > 0 {
		m.budget = preset.Budget
	}
	m.root.recompute()
	m.presetsOpen = false
	m.setNotice(fmt.Sprintf("Applied preset %q", preset.Name))
}

func (m *model) deletePreset() {
	if m.presetCursor < 0 || m.presetCursor >= len(m.presets) {
		return
	}
	name := m.presets[m.presetCursor].Name
	if err := state.DeletePreset(m.presetRoot(), name); err != nil {
		m.presetError = err.Error()
		return
	}
	m.reloadPresets()
}

func (m model) renderPresetModal(view string, width, height int) string {
	modalWidth := min(width, 64)
	if modalWidth < 30 {
		modalWidth = 30
	}
	var b strings.Builder
	b.WriteString(m.modalTitle(m.glyphs.Help, "Selection Presets", modalWidth))
	if m.presetSaving {
		b.WriteString("\n")
		b.WriteString(ansi.Truncate("Name: "+m.presetInput+"▌", modalWidth-4, "…"))
		b.WriteString("\n")
		b.WriteString(m.styles.hint.Render("Enter save · Esc cancel"))
	} else if len(m.presets) == 0 {
		b.WriteString("\n")
		b.WriteString(m.styles.muted.Render("No saved presets"))
		b.WriteString("\n")
		b.WriteString(m.styles.hint.Render("s save current · Esc close"))
	} else {
		bodyRows := max(1, min(len(m.presets), height-8))
		if m.presetCursor < m.presetOffset {
			m.presetOffset = m.presetCursor
		}
		if m.presetCursor >= m.presetOffset+bodyRows {
			m.presetOffset = m.presetCursor - bodyRows + 1
		}
		for i := m.presetOffset; i < m.presetOffset+bodyRows && i < len(m.presets); i++ {
			b.WriteString("\n")
			line := fmt.Sprintf("  %s  %d files", m.presets[i].Name, len(m.presets[i].Selections))
			if i == m.presetCursor {
				b.WriteString(m.styles.cursor.Render(ansi.Truncate("❯"+line[1:], modalWidth-4, "…")))
			} else {
				b.WriteString(ansi.Truncate(line, modalWidth-4, "…"))
			}
		}
		b.WriteString("\n")
		b.WriteString(m.styles.hint.Render("↑/↓ move · Enter apply · s save · d delete · Esc close"))
	}
	if m.presetError != "" {
		b.WriteString("\n")
		b.WriteString(m.styles.warning.Render(ansi.Truncate(m.presetError, modalWidth-4, "…")))
	}
	modalHeight := max(3, lipgloss.Height(b.String())+2)
	return m.overlay(view, m.modalStyle(modalWidth).Height(modalHeight).Render(b.String()), width, height)
}
