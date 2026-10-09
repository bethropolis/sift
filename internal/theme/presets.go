package theme

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/bethropolis/sift/internal/highlight"
)

var ThemePresets = []ThemePreset{
	{
		ID: "catppuccin-mocha", Name: "Catppuccin Mocha", Family: "Catppuccin", Variant: "Mocha", Dark: true,
		Border: lipgloss.Color("#cba6f7"), Title: lipgloss.Color("#89b4fa"), Muted: lipgloss.Color("#6c7086"),
		CursorBg: lipgloss.Color("#313244"), CursorFg: lipgloss.Color("#cdd6f4"), Selected: lipgloss.Color("#89b4fa"),
		Notice: lipgloss.Color("#a6e3a1"), ModeFull: lipgloss.Color("#a6e3a1"), ModeSig: lipgloss.Color("#f9e2af"), ModeSkip: lipgloss.Color("#f38ba8"),
		Highlight: hlPalette("#cba6f7", "#a6e3a1", "#fab387", "#6c7086", "#f9e2af"),
		Syntax:    semanticPalette("#cba6f7", "#a6e3a1", "#fab387", "#6c7086", "#f9e2af", "#89b4fa", "#eba0ac", "#89dceb", "#6c7086"),
	},
	{
		ID: "tokyo-night", Name: "Tokyo Night", Family: "Tokyo Night", Variant: "Night", Dark: true,
		Border: lipgloss.Color("#7aa2f7"), Title: lipgloss.Color("#7dcfff"), Muted: lipgloss.Color("#565f89"),
		CursorBg: lipgloss.Color("#2ac3de"), CursorFg: lipgloss.Color("#1a1b26"), Selected: lipgloss.Color("#7aa2f7"),
		Notice: lipgloss.Color("#9ece6a"), ModeFull: lipgloss.Color("#9ece6a"), ModeSig: lipgloss.Color("#e0af68"), ModeSkip: lipgloss.Color("#f7768e"),
		Highlight: hlPalette("#bb9af7", "#9ece6a", "#ff9e64", "#565f89", "#7dcfff"),
		Syntax:    semanticPalette("#bb9af7", "#9ece6a", "#ff9e64", "#565f89", "#7dcfff", "#7aa2f7", "#73daca", "#89ddff", "#565f89"),
	},
	{
		ID: "dracula", Name: "Dracula", Family: "Dracula", Variant: "Dracula", Dark: true,
		Border: lipgloss.Color("#bd93f9"), Title: lipgloss.Color("#8be9fd"), Muted: lipgloss.Color("#6272a4"),
		CursorBg: lipgloss.Color("#44475a"), CursorFg: lipgloss.Color("#f8f8f2"), Selected: lipgloss.Color("#ff79c6"),
		Notice: lipgloss.Color("#50fa7b"), ModeFull: lipgloss.Color("#50fa7b"), ModeSig: lipgloss.Color("#f1fa8c"), ModeSkip: lipgloss.Color("#ff5555"),
		Highlight: hlPalette("#ff79c6", "#f1fa8c", "#bd93f9", "#6272a4", "#8be9fd"),
		Syntax:    semanticPalette("#ff79c6", "#f1fa8c", "#bd93f9", "#6272a4", "#8be9fd", "#50fa7b", "#ffb86c", "#ff79c6", "#6272a4"),
	},
	{
		ID: "gruvbox-dark", Name: "Gruvbox Dark", Family: "Gruvbox", Variant: "Dark", Dark: true,
		Border: lipgloss.Color("#d3869b"), Title: lipgloss.Color("#83a598"), Muted: lipgloss.Color("#928374"),
		CursorBg: lipgloss.Color("#3c3836"), CursorFg: lipgloss.Color("#ebdbb2"), Selected: lipgloss.Color("#83a598"),
		Notice: lipgloss.Color("#b8bb26"), ModeFull: lipgloss.Color("#b8bb26"), ModeSig: lipgloss.Color("#fabd2f"), ModeSkip: lipgloss.Color("#fb4934"),
		Highlight: hlPalette("#fb4934", "#b8bb26", "#d3869b", "#928374", "#fabd2f"),
		Syntax:    semanticPalette("#fb4934", "#b8bb26", "#d3869b", "#928374", "#fabd2f", "#83a598", "#8ec07c", "#fe8019", "#928374"),
	},
	{
		ID: "nord", Name: "Nord", Family: "Nord", Variant: "Nord", Dark: true,
		Border: lipgloss.Color("#88c0d0"), Title: lipgloss.Color("#81a1c1"), Muted: lipgloss.Color("#4c566a"),
		CursorBg: lipgloss.Color("#3b4252"), CursorFg: lipgloss.Color("#eceff4"), Selected: lipgloss.Color("#88c0d0"),
		Notice: lipgloss.Color("#a3be8c"), ModeFull: lipgloss.Color("#a3be8c"), ModeSig: lipgloss.Color("#ebcb8b"), ModeSkip: lipgloss.Color("#bf616a"),
		Highlight: hlPalette("#81a1c1", "#a3be8c", "#b48ead", "#4c566a", "#88c0d0"),
		Syntax:    semanticPalette("#81a1c1", "#a3be8c", "#b48ead", "#4c566a", "#88c0d0", "#88c0d0", "#8fbcbb", "#81a1c1", "#4c566a"),
	},
	{
		ID: "rose-pine", Name: "Rose Pine", Family: "Rosé Pine", Variant: "Main", Dark: true,
		Border: lipgloss.Color("#c4a7e7"), Title: lipgloss.Color("#e0def4"), Muted: lipgloss.Color("#6e6a86"),
		CursorBg: lipgloss.Color("#ebbcba"), CursorFg: lipgloss.Color("#191724"), Selected: lipgloss.Color("#31748f"),
		Notice: lipgloss.Color("#9ccfd8"), ModeFull: lipgloss.Color("#9ccfd8"), ModeSig: lipgloss.Color("#f6c177"), ModeSkip: lipgloss.Color("#eb6f92"),
		Highlight: hlPalette("#c4a7e7", "#9ccfd8", "#f6c177", "#6e6a86", "#ebbcba"),
		Syntax:    semanticPalette("#c4a7e7", "#9ccfd8", "#f6c177", "#6e6a86", "#ebbcba", "#31748f", "#c4a7e7", "#9ccfd8", "#6e6a86"),
	},
	{
		ID: "classic", Name: "Classic (Default)", Family: "Classic", Variant: "Default", Dark: true,
		Border: lipgloss.Color("62"), Title: lipgloss.Color("12"), Muted: lipgloss.Color("245"),
		CursorBg: lipgloss.Color("236"), CursorFg: lipgloss.Color("15"), Selected: lipgloss.Color("12"),
		Notice: lipgloss.Color("10"), ModeFull: lipgloss.Color("10"), ModeSig: lipgloss.Color("11"), ModeSkip: lipgloss.Color("9"),
		// Classic keeps the legacy ANSI palette byte-for-byte so the default
		// preview rendering is unchanged.
		Highlight: highlight.Palette{Keyword: "\033[1;34m", String: "\033[32m", Number: "\033[36m", Comment: "\033[2m", TypeName: "\033[35m"},
	},
}

func init() {
	ThemePresets = append(ThemePresets,
		catalogTheme("gruvbox-light", "Gruvbox Light", "Gruvbox", "Light", false, "#d79921", "#458588", "#928374", "#fbf1c7", "#282828", "#bdae93", "#98971a", "#d79921", "#fb4934"),
		catalogTheme("classic-dark", "Classic Zinc (Dark)", "Classic Zinc", "Dark", true, "#d4a359", "#f0f3f8", "#525a6b", "#0b0d12", "#f0f3f8", "#d4a359", "#86efac", "#fbbf24", "#f87171"),
		catalogTheme("classic-light", "Classic Zinc (Light)", "Classic Zinc", "Light", false, "#b58532", "#111318", "#8f97a6", "#ffffff", "#111318", "#b58532", "#257035", "#8f641b", "#c53030"),
		catalogTheme("kanagawa-wave", "Kanagawa Wave", "Kanagawa", "Wave", true, "#957fb8", "#7e9cd8", "#727169", "#1f1f28", "#2a2a37", "#98bb6c", "#e6c384", "#7fb4ca", "#e82424"),
		catalogTheme("kanagawa-dragon", "Kanagawa Dragon", "Kanagawa", "Dragon", true, "#c4746e", "#8ba4b0", "#727169", "#181616", "#2d4f67", "#8ba4b0", "#c4b28a", "#8ea4a2", "#c4746e"),
		catalogTheme("kanagawa-lotus", "Kanagawa Lotus", "Kanagawa", "Lotus", false, "#4d699b", "#4d699b", "#727169", "#f2ecbc", "#e5ddb0", "#624c83", "#c84053", "#76946a", "#c84053"),
		catalogTheme("everforest-dark", "Everforest Dark", "Everforest", "Dark", true, "#83c092", "#a7c080", "#859289", "#2d353b", "#344b58", "#a7c080", "#dbbc7f", "#7fbbb3", "#e67e80"),
		catalogTheme("everforest-light", "Everforest Light", "Everforest", "Light", false, "#8da101", "#5a9200", "#939f91", "#fdf6e3", "#f4f0d9", "#8da101", "#d79784", "#35a77c", "#f85552"),
		catalogTheme("one-dark", "One Dark", "One Dark", "Dark", true, "#61afef", "#98c379", "#5c6370", "#282c34", "#3e4451", "#98c379", "#e5c07b", "#56b6c2", "#e06c75"),
		catalogTheme("solarized-dark", "Solarized Dark", "Solarized", "Dark", true, "#268bd2", "#b58900", "#586e75", "#073642", "#002b36", "#859900", "#cb4b16", "#2aa198", "#dc322f"),
		catalogTheme("solarized-light", "Solarized Light", "Solarized", "Light", false, "#268bd2", "#b58900", "#657b83", "#fdf6e3", "#eee8d5", "#859900", "#cb4b16", "#2aa198", "#dc322f"),
		catalogTheme("monokai", "Monokai", "Monokai", "Classic", true, "#f92672", "#a6e22e", "#75715e", "#272822", "#3e3d32", "#a6e22e", "#e6db74", "#66d9ef", "#f92672"),
		catalogTheme("github-dark", "GitHub Dark", "GitHub", "Dark", true, "#58a6ff", "#d2a8ff", "#8b949e", "#0d1117", "#21262d", "#3fb950", "#d29922", "#79c0ff", "#f85149"),
		catalogTheme("github-light", "GitHub Light", "GitHub", "Light", false, "#0969da", "#8250df", "#57606a", "#ffffff", "#f6f8fa", "#1a7f37", "#9a6700", "#0969da", "#cf222e"),
		catalogTheme("night-owl", "Night Owl", "Night Owl", "Dark", true, "#82aaff", "#addb67", "#637777", "#011627", "#1d3b53", "#ecc48d", "#82aaff", "#7fdbca", "#ef5350"),
		catalogTheme("catppuccin-latte", "Catppuccin Latte", "Catppuccin", "Latte", false, "#8839ef", "#1e66f5", "#9ca0b0", "#ccd0da", "#4c4f69", "#1e66f5", "#40a02b", "#df8e1d", "#d20f39"),
		catalogTheme("tokyo-day", "Tokyo Night Day", "Tokyo Night", "Day", false, "#9854f1", "#2e7de9", "#848cb5", "#e1e2e7", "#3760bf", "#2e7de9", "#587539", "#8c6c3e", "#f52a65"),
		catalogTheme("rose-pine-dawn", "Rosé Pine Dawn", "Rosé Pine", "Dawn", false, "#907aa9", "#286983", "#9893a5", "#faf4ed", "#575279", "#56949f", "#286983", "#ea9d34", "#b4637a"), catalogTheme("poimandres", "Poimandres", "Poimandres", "Dark", true, "#89ddff", "#5de4c7", "#8a9a9a", "#1b1e28", "#2a2f3a", "#5de4c7", "#fffac2", "#89ddff", "#f07178"),
		terminalTheme(),
	)
}

func terminalTheme() ThemePreset {
	return ThemePreset{
		ID: "terminal", Name: "Terminal (Emulator)", Family: "Terminal", Variant: "Emulator", Dark: false, FollowTerminal: true,
		Description: "Use the terminal emulator's configured ANSI and 256-color palette",
		// Use named ANSI roles for chrome so common Ghostty themes retain
		// their vivid foreground/background palette instead of becoming gray.
		Border: lipgloss.Color("8"), Title: lipgloss.Color("15"), Muted: lipgloss.Color("244"),
		CursorBg: lipgloss.Color("4"), CursorFg: lipgloss.Color("0"), Selected: lipgloss.Color("14"),
		Notice: lipgloss.Color("10"), ModeFull: lipgloss.Color("10"), ModeSig: lipgloss.Color("11"), ModeSkip: lipgloss.Color("9"),
		Highlight: highlight.Palette{Keyword: "\033[1;34m", String: "\033[32m", Number: "\033[36m", Comment: "\033[2m", TypeName: "\033[35m"},
		Syntax:    terminalSemanticPalette(),
	}
}

func terminalSemanticPalette() highlight.SyntaxPalette {
	return highlight.SyntaxPalette{
		highlight.TokenKeyword:       {Foreground: "ansi:33", Bold: true},
		highlight.TokenString:        {Foreground: "ansi:42"},
		highlight.TokenStringEscape:  {Foreground: "ansi:42", Bold: true},
		highlight.TokenRegex:         {Foreground: "ansi:129"},
		highlight.TokenNumber:        {Foreground: "ansi:39"},
		highlight.TokenBool:          {Foreground: "ansi:39"},
		highlight.TokenNull:          {Foreground: "ansi:39"},
		highlight.TokenComment:       {Foreground: "ansi:245", Faint: true},
		highlight.TokenDocComment:    {Foreground: "ansi:245", Faint: true, Italic: true},
		highlight.TokenType:          {Foreground: "ansi:129"},
		highlight.TokenFunction:      {Foreground: "ansi:33"},
		highlight.TokenVariable:      {Foreground: "ansi:252"},
		highlight.TokenConstant:      {Foreground: "ansi:39"},
		highlight.TokenProperty:      {Foreground: "ansi:39"},
		highlight.TokenBuiltin:       {Foreground: "ansi:33"},
		highlight.TokenOperator:      {Foreground: "ansi:252"},
		highlight.TokenPunctuation:   {Foreground: "ansi:252"},
		highlight.TokenAttribute:     {Foreground: "ansi:129"},
		highlight.TokenDecorator:     {Foreground: "ansi:129", Bold: true},
		highlight.TokenTag:           {Foreground: "ansi:33"},
		highlight.TokenTagAttribute:  {Foreground: "ansi:39"},
		highlight.TokenMarkupHeading: {Foreground: "ansi:33", Bold: true},
		highlight.TokenMarkupLink:    {Foreground: "ansi:39", Underline: true},
		highlight.TokenShebang:       {Foreground: "ansi:245", Faint: true},
		highlight.TokenDiffAdded:     {Foreground: "ansi:42"},
		highlight.TokenDiffRemoved:   {Foreground: "ansi:203"},
		highlight.TokenDiffHunk:      {Foreground: "ansi:33", Bold: true},
	}
}

func catalogTheme(id, name, family, variant string, dark bool, border, title, muted, cursorBg, cursorFg, selected, success, warning, error string) ThemePreset {
	return ThemePreset{
		ID: id, Name: name, Family: family, Variant: variant, Dark: dark,
		Border: lipgloss.Color(border), Title: lipgloss.Color(title), Muted: lipgloss.Color(muted),
		CursorBg: lipgloss.Color(cursorBg), CursorFg: lipgloss.Color(cursorFg), Selected: lipgloss.Color(selected),
		Notice: lipgloss.Color(title), ModeFull: lipgloss.Color(success), ModeSig: lipgloss.Color(warning), ModeSkip: lipgloss.Color(error),
		Highlight: hlPalette(title, success, warning, muted, cursorFg),
		Syntax:    semanticPalette(title, success, warning, muted, cursorFg, border, selected, title, muted),
	}
}
