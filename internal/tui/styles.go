package tui

import "github.com/charmbracelet/lipgloss"

// uiStyles holds every style the picker renders with. It is instance-local:
// a model owns its own copy so applying a theme never mutates shared package
// state and two pickers can hold different themes concurrently.
type uiStyles struct {
	title     lipgloss.Style
	hint      lipgloss.Style
	dim       lipgloss.Style
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
	// border is the neutral pane border color; the focused pane uses accent,
	// keeping focus visible on every theme.
	border lipgloss.Color
	// accent highlights the focused pane border and follows the theme title
	// color.
	accent lipgloss.Color
}

// defaultStyles reproduces the original classic palette.
func defaultStyles() uiStyles {
	return uiStyles{
		title:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")),
		hint:      lipgloss.NewStyle().Foreground(lipgloss.Color("242")),
		dim:       lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		muted:     lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Faint(true),
		treeGuide: lipgloss.NewStyle().Foreground(lipgloss.Color("239")),
		cursor:    lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("236")).Foreground(lipgloss.Color("15")),
		scrollbar: lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		selected:  lipgloss.NewStyle().Foreground(lipgloss.Color("12")),
		warning:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")),
		notice:    lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		modeFull:  lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		modeSig:   lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		modeSkip:  lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		border:    lipgloss.Color("62"),
		accent:    lipgloss.Color("12"),
	}
}
