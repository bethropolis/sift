package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	hintStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	treeGuideStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("239"))
	cursorStyle    = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("236")).Foreground(lipgloss.Color("15"))
	scrollbarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	selectedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	warningStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
	noticeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))

	modeFullStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // Green
	modeSigStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // Yellow
	modeSkipStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))  // Red
)
