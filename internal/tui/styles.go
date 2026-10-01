package tui

import "github.com/charmbracelet/lipgloss"

// uiStyles holds every style the picker renders with. It is instance-local:
// a model owns its own copy so applying a theme never mutates shared package
// state and two pickers can hold different themes concurrently. The fields are
// semantic roles rather than view-specific colors so renderers can evolve
// without hard-coding palette decisions in every component.
type uiStyles struct {
	appTitle  lipgloss.Style
	title     lipgloss.Style
	hint      lipgloss.Style
	dim       lipgloss.Style
	subtle    lipgloss.Style
	muted     lipgloss.Style
	treeGuide lipgloss.Style
	cursor    lipgloss.Style
	scrollbar lipgloss.Style
	selected  lipgloss.Style
	warning   lipgloss.Style
	notice    lipgloss.Style
	modeFull  lipgloss.Style
	modeSig   lipgloss.Style
	modeSkip  lipgloss.Style
	danger    lipgloss.Style
	success   lipgloss.Style

	border     lipgloss.Color
	accent     lipgloss.Color
	accentSoft lipgloss.Color
}

// defaultStyles reproduces the original classic palette while establishing
// the semantic roles used by the polished renderer.
func defaultStyles() uiStyles {
	return uiStyles{
		appTitle:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")),
		title:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")),
		hint:       lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		dim:        lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		subtle:     lipgloss.NewStyle().Foreground(lipgloss.Color("242")),
		muted:      lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		treeGuide:  lipgloss.NewStyle().Foreground(lipgloss.Color("239")),
		cursor:     lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("236")).Foreground(lipgloss.Color("15")),
		scrollbar:  lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		selected:   lipgloss.NewStyle().Foreground(lipgloss.Color("12")),
		warning:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")),
		notice:     lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		modeFull:   lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		modeSig:    lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		modeSkip:   lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		danger:     lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		success:    lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		border:     lipgloss.Color("62"),
		accent:     lipgloss.Color("12"),
		accentSoft: lipgloss.Color("245"),
	}
}
