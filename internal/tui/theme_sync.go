package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bethropolis/sift/internal/state"
	"github.com/bethropolis/sift/internal/theme"
)

// themePollInterval is how often the picker re-reads the shared preferences
// file so a web theme change applies live. The read is a single small JSON
// file; the model only restyles when the id actually differs.
const themePollInterval = time.Second

type themePollMsg struct {
	id string
}

func pollThemeCmd() tea.Cmd {
	return tea.Tick(themePollInterval, func(time.Time) tea.Msg {
		prefs, err := state.LoadPreferences()
		if err != nil {
			return themePollMsg{}
		}
		return themePollMsg{id: state.EffectiveTheme(prefs)}
	})
}

// syncExternalTheme applies a theme id written by the other frontend (web
// settings land in the same preferences file). Unknown ids and "system"
// never match a picker preset, so they leave the current theme alone. It
// reports whether the model changed.
func (m *model) syncExternalTheme(id string) bool {
	if id == "" || id == "system" {
		return false
	}
	if id == theme.IDAt(m.themes, m.themeIndex) {
		return false
	}
	if !containsThemeID(m.themes, id) {
		return false
	}
	idx := theme.IndexOf(m.themes, id)
	m.themeIndex = idx
	m.themeCursor = idx
	m.applyTheme(m.themes[idx])
	return true
}
