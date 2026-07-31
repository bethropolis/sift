// Package tui renders an interactive file picker for the pick subcommand.
// It lets the user choose files by relevance/token weight before rendering a
// context document.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Item is one selectable file.
type Item struct {
	// Path is the file's path relative to the scanned root.
	Path string
	// Tokens is the token count of the file's content.
	Tokens int
}

// Run displays the picker for items and returns the selected paths in the
// order they were presented. An empty selection yields an empty slice. The
// picker takes over its own screen via the alternate buffer and restores the
// terminal on exit.
func Run(items []Item) ([]string, error) {
	if len(items) == 0 {
		return nil, nil
	}
	m := newModel(items)
	final, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	if err != nil {
		return nil, err
	}
	mm, ok := final.(model)
	if !ok {
		return nil, fmt.Errorf("tui: unexpected final model %T", final)
	}
	return mm.selectedPaths(), nil
}

type model struct {
	items    []Item
	selected map[int]bool
	cursor   int
	height   int
	width    int
	offset   int
}

func newModel(items []Item) model {
	return model{items: items, selected: map[int]bool{}, height: 24, width: 60}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height, m.width = msg.Height, msg.Width
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyCtrlQ:
			return m, tea.Quit
		case tea.KeyEnter:
			return m, tea.Quit
		case tea.KeyUp, tea.KeyShiftTab:
			m.cursor = max(0, m.cursor-1)
		case tea.KeyDown, tea.KeyTab:
			m.cursor = min(len(m.items)-1, m.cursor+1)
		case tea.KeySpace:
			if len(m.items) > 0 {
				m.selected[m.cursor] = !m.selected[m.cursor]
			}
		case tea.KeyRunes:
			switch string(msg.Runes) {
			case "k":
				m.cursor = max(0, m.cursor-1)
			case "j":
				m.cursor = min(len(m.items)-1, m.cursor+1)
			case "a":
				if len(m.selected) == len(m.items) {
					m.selected = map[int]bool{}
				} else {
					for i := range m.items {
						m.selected[i] = true
					}
				}
			}
		}
		m.clampOffset()
	}
	return m, nil
}

// clampOffset keeps the cursor inside the visible window.
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

const headerLines = 2
const footerLines = 2

func (m model) View() string {
	width := max(0, min(m.width, 120))
	var b strings.Builder
	b.WriteString(titleStyle.Render(" dumper pick "))
	b.WriteString(hintStyle.Render(fmt.Sprintf(" (%d files)", len(m.items))))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", width))
	b.WriteString("\n")

	rows := m.height - headerLines - footerLines
	if rows < 1 {
		rows = 1
	}
	end := min(len(m.items), m.offset+rows)
	for i := m.offset; i < end; i++ {
		b.WriteString(m.row(i, width))
		b.WriteString("\n")
	}

	b.WriteString(strings.Repeat("─", width))
	b.WriteString("\n")
	selected, tokens := m.stats()
	b.WriteString(hintStyle.Render(fmt.Sprintf("space toggle   a all/none   enter done   %d selected, %d tokens", selected, tokens)))
	b.WriteString("\n")
	return b.String()
}

func (m model) row(i int, width int) string {
	mark := "[ ]"
	if m.selected[i] {
		mark = "[x]"
	}
	pad := max(20, width-18)
	row := fmt.Sprintf(" %s %-*s %6d tokens", mark, pad, m.items[i].Path, m.items[i].Tokens)
	if i == m.cursor {
		return cursorStyle.Render(row)
	}
	if m.selected[i] {
		return selectedStyle.Render(row)
	}
	return row
}

func (m model) stats() (int, int) {
	selected, tokens := 0, 0
	for i, sel := range m.selected {
		if sel {
			selected++
			tokens += m.items[i].Tokens
		}
	}
	return selected, tokens
}

// selectedPaths returns the selected paths in presentation order.
func (m model) selectedPaths() []string {
	var paths []string
	for i := range m.items {
		if m.selected[i] {
			paths = append(paths, m.items[i].Path)
		}
	}
	return paths
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	hintStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	cursorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
)
