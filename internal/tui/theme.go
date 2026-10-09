package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/bethropolis/sift/internal/highlight"
)

// ThemePreset is an instance-local color palette. Applying a preset rebuilds
// the model's own styles; it never mutates package-level state.
type ThemePreset struct {
	// ID is the stable persisted identity; Name remains the display label.
	ID          string
	Name        string
	Family      string
	Variant     string
	Description string
	Dark        bool
	// FollowTerminal marks a palette that intentionally uses the terminal's
	// ANSI colors instead of fixed RGB values.
	FollowTerminal bool

	Border   lipgloss.Color
	Title    lipgloss.Color
	Muted    lipgloss.Color
	CursorBg lipgloss.Color
	CursorFg lipgloss.Color
	Selected lipgloss.Color
	Notice   lipgloss.Color
	ModeFull lipgloss.Color
	ModeSig  lipgloss.Color
	ModeSkip lipgloss.Color
	// Highlight is retained for compatibility with legacy RenderLine callers.
	Highlight highlight.Palette
	// Syntax is the theme-independent semantic palette for cached previews.
	Syntax highlight.SyntaxPalette

	UI     UIColors
	Status StatusColors
}

// UIColors describes the semantic roles consumed by TUI renderers. Legacy
// preset fields remain supported and are normalized into this structure.
type UIColors struct {
	Background   lipgloss.Color
	Surface      lipgloss.Color
	SurfaceAlt   lipgloss.Color
	Text         lipgloss.Color
	TextMuted    lipgloss.Color
	TextSubtle   lipgloss.Color
	Accent       lipgloss.Color
	AccentSoft   lipgloss.Color
	Border       lipgloss.Color
	BorderActive lipgloss.Color
	CursorBg     lipgloss.Color
	CursorFg     lipgloss.Color
	SelectionBg  lipgloss.Color
	SelectionFg  lipgloss.Color
	TreeGuide    lipgloss.Color
	Scrollbar    lipgloss.Color
	ModeFull     lipgloss.Color
	ModeSig      lipgloss.Color
	ModeSkip     lipgloss.Color
}

// StatusColors keeps operational status separate from syntax and UI accents.
type StatusColors struct {
	Info    lipgloss.Color
	Success lipgloss.Color
	Warning lipgloss.Color
	Error   lipgloss.Color
}

// normalizeTheme fills semantic roles from the compatibility fields and
// guarantees that every renderer-facing role has a concrete fallback.
func normalizeTheme(p ThemePreset) ThemePreset {
	if p.UI.Text == "" {
		p.UI.Text = p.Title
	}
	if p.UI.TextMuted == "" {
		p.UI.TextMuted = p.Muted
	}
	if p.UI.TextSubtle == "" {
		p.UI.TextSubtle = p.Border
	}
	if p.UI.Accent == "" {
		p.UI.Accent = p.Title
	}
	if p.UI.AccentSoft == "" {
		p.UI.AccentSoft = p.Selected
	}
	if p.UI.Border == "" {
		p.UI.Border = p.Border
	}
	if p.UI.BorderActive == "" {
		p.UI.BorderActive = p.Title
	}
	if p.UI.CursorBg == "" {
		p.UI.CursorBg = p.CursorBg
	}
	if p.UI.CursorFg == "" {
		p.UI.CursorFg = p.CursorFg
	}
	if p.UI.SelectionBg == "" {
		p.UI.SelectionBg = p.CursorBg
	}
	if p.UI.SelectionFg == "" {
		p.UI.SelectionFg = p.CursorFg
	}
	if p.UI.TreeGuide == "" {
		p.UI.TreeGuide = p.Border
	}
	if p.UI.Scrollbar == "" {
		p.UI.Scrollbar = p.Muted
	}
	if p.UI.ModeFull == "" {
		p.UI.ModeFull = p.ModeFull
	}
	if p.UI.ModeSig == "" {
		p.UI.ModeSig = p.ModeSig
	}
	if p.UI.ModeSkip == "" {
		p.UI.ModeSkip = p.ModeSkip
	}
	if p.Status.Info == "" {
		p.Status.Info = p.Notice
	}
	if p.Status.Success == "" {
		p.Status.Success = p.ModeFull
	}
	if p.Status.Warning == "" {
		p.Status.Warning = p.ModeSig
	}
	if p.Status.Error == "" {
		p.Status.Error = p.ModeSkip
	}
	return p
}

// hlPalette builds a preview-highlight palette from theme hex colors:
// keyword (bold), string, number, comment (faint), type name.
func hlPalette(keyword, str, number, comment, typeName string) highlight.Palette {
	return highlight.Palette{
		Keyword:  highlight.ANSI(keyword, "1"),
		String:   highlight.ANSI(str),
		Number:   highlight.ANSI(number),
		Comment:  highlight.ANSI(comment, "2"),
		TypeName: highlight.ANSI(typeName),
	}
}

func semanticPalette(keyword, str, number, comment, typeName, function, property, operator, punctuation string) highlight.SyntaxPalette {
	return highlight.SyntaxPalette{
		highlight.TokenKeyword:       {Foreground: keyword, Bold: true},
		highlight.TokenString:        {Foreground: str},
		highlight.TokenStringEscape:  {Foreground: str, Bold: true},
		highlight.TokenNumber:        {Foreground: number},
		highlight.TokenBool:          {Foreground: number},
		highlight.TokenNull:          {Foreground: number},
		highlight.TokenComment:       {Foreground: comment, Faint: true},
		highlight.TokenDocComment:    {Foreground: comment, Faint: true, Italic: true},
		highlight.TokenType:          {Foreground: typeName},
		highlight.TokenFunction:      {Foreground: function},
		highlight.TokenProperty:      {Foreground: property},
		highlight.TokenBuiltin:       {Foreground: function},
		highlight.TokenOperator:      {Foreground: operator},
		highlight.TokenPunctuation:   {Foreground: punctuation},
		highlight.TokenAttribute:     {Foreground: property},
		highlight.TokenDecorator:     {Foreground: function, Bold: true},
		highlight.TokenTag:           {Foreground: typeName},
		highlight.TokenTagAttribute:  {Foreground: property},
		highlight.TokenMarkupHeading: {Foreground: function, Bold: true},
		highlight.TokenMarkupLink:    {Foreground: property, Underline: true},
		highlight.TokenDiffAdded:     {Foreground: "#a6e3a1"},
		highlight.TokenDiffRemoved:   {Foreground: "#f38ba8"},
		highlight.TokenDiffHunk:      {Foreground: "#89b4fa", Bold: true},
	}
}

// ThemePresets lists every selectable theme in the interactive t menu. The
// Classic/Default entry reproduces the original ANSI palette.
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
		catalogTheme("rose-pine-dawn", "Rosé Pine Dawn", "Rosé Pine", "Dawn", false, "#907aa9", "#286983", "#9893a5", "#faf4ed", "#575279", "#56949f", "#286983", "#ea9d34", "#b4637a"),		catalogTheme("poimandres", "Poimandres", "Poimandres", "Dark", true, "#89ddff", "#5de4c7", "#8a9a9a", "#1b1e28", "#2a2f3a", "#5de4c7", "#fffac2", "#89ddff", "#f07178"),
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

// defaultThemeIndex returns the index of the Classic (Default) preset, which
// is the initial theme.
func defaultThemeIndex() int {
	for i, p := range ThemePresets {
		if p.Name == "Classic (Default)" {
			return i
		}
	}
	return 0
}

// themeIndex resolves a persisted theme name while tolerating a missing or
// stale preference after themes are renamed or removed.
func themeID(index int) string {
	return themeIDIn(ThemePresets, index)
}

func themeIndex(name string) int {
	return themeIndexIn(ThemePresets, name)
}

func themeIndexIn(themes []ThemePreset, name string) int {
	defaultIndex := defaultThemeIndexIn(themes)
	if name != "" {
		for i, p := range themes {
			if p.ID == name || p.Name == name {
				return i
			}
		}
		aliases := map[string]string{
			"rose pine":          "rose-pine",
			"classic":            "classic",
			"classic zinc":       "classic-dark",
			"classic zinc dark":  "classic-dark",
			"classic zinc light": "classic-light",
			"zinc dark":          "classic-dark",
			"zinc light":         "classic-light",
			"catppuccin mocha":   "catppuccin-mocha",
			"tokyo night":        "tokyo-night",
			"gruvbox dark":       "gruvbox-dark",
		}
		if id, ok := aliases[strings.ToLower(strings.TrimSpace(name))]; ok {
			for i, p := range themes {
				if p.ID == id {
					return i
				}
			}
		}
	}
	return defaultIndex
}

func defaultThemeIndexIn(themes []ThemePreset) int {
	for i, p := range themes {
		if p.ID == "classic" || p.Name == "Classic (Default)" {
			return i
		}
	}
	return 0
}

func themeIDIn(themes []ThemePreset, index int) string {
	if index < 0 || index >= len(themes) {
		return themes[defaultThemeIndexIn(themes)].ID
	}
	return themes[index].ID
}

// applyTheme rebuilds the model's own styles from the preset. Because the
// styles live on the model, two pickers never share theme state and applying
// a theme cannot race another render. The preset's highlight palette is
// installed alongside the chrome styles so preview syntax highlighting
// follows the theme; highlighting stays off entirely when disabled.
func (m *model) applyTheme(p ThemePreset) {
	p = normalizeTheme(p)
	m.styles = uiStyles{
		appTitle:   lipgloss.NewStyle().Bold(true).Foreground(p.Title),
		title:      lipgloss.NewStyle().Bold(true).Foreground(p.Title),
		hint:       lipgloss.NewStyle().Foreground(p.Muted),
		dim:        lipgloss.NewStyle().Foreground(p.Border).Faint(true),
		subtle:     lipgloss.NewStyle().Foreground(p.Border).Faint(true),
		muted:      lipgloss.NewStyle().Foreground(p.Muted),
		treeGuide:  lipgloss.NewStyle().Foreground(p.Border),
		cursor:     lipgloss.NewStyle().Bold(true).Background(p.CursorBg).Foreground(p.CursorFg),
		scrollbar:  lipgloss.NewStyle().Foreground(p.Muted),
		selected:   lipgloss.NewStyle().Foreground(p.Selected),
		warning:    lipgloss.NewStyle().Bold(true).Foreground(p.Status.Warning),
		notice:     lipgloss.NewStyle().Foreground(p.Status.Info),
		modeFull:   lipgloss.NewStyle().Foreground(p.UI.ModeFull),
		modeSig:    lipgloss.NewStyle().Foreground(p.UI.ModeSig),
		modeSkip:   lipgloss.NewStyle().Foreground(p.UI.ModeSkip),
		danger:     lipgloss.NewStyle().Foreground(p.Status.Error),
		success:    lipgloss.NewStyle().Foreground(p.Status.Success),
		border:     p.UI.Border,
		accent:     p.UI.Accent,
		accentSoft: p.UI.AccentSoft,
	}
	m.highlight.Palette = &p.Highlight
	m.highlight.Syntax = &p.Syntax
}

func themePreviewStyles(p ThemePreset) uiStyles {
	p = normalizeTheme(p)
	return uiStyles{
		appTitle:   lipgloss.NewStyle().Bold(true).Foreground(p.UI.Accent),
		title:      lipgloss.NewStyle().Bold(true).Foreground(p.UI.Accent),
		hint:       lipgloss.NewStyle().Foreground(p.UI.TextMuted),
		muted:      lipgloss.NewStyle().Foreground(p.UI.TextMuted),
		subtle:     lipgloss.NewStyle().Foreground(p.UI.TextSubtle),
		dim:        lipgloss.NewStyle().Foreground(p.UI.TextSubtle).Faint(true),
		treeGuide:  lipgloss.NewStyle().Foreground(p.UI.TreeGuide),
		cursor:     lipgloss.NewStyle().Bold(true).Background(p.UI.CursorBg).Foreground(p.UI.CursorFg),
		scrollbar:  lipgloss.NewStyle().Foreground(p.UI.Scrollbar),
		selected:   lipgloss.NewStyle().Foreground(p.UI.AccentSoft),
		warning:    lipgloss.NewStyle().Bold(true).Foreground(p.Status.Warning),
		notice:     lipgloss.NewStyle().Foreground(p.Status.Info),
		modeFull:   lipgloss.NewStyle().Foreground(p.UI.ModeFull),
		modeSig:    lipgloss.NewStyle().Foreground(p.UI.ModeSig),
		modeSkip:   lipgloss.NewStyle().Foreground(p.UI.ModeSkip),
		danger:     lipgloss.NewStyle().Foreground(p.Status.Error),
		success:    lipgloss.NewStyle().Foreground(p.Status.Success),
		border:     p.UI.Border,
		accent:     p.UI.Accent,
		accentSoft: p.UI.AccentSoft,
	}
}

// themePreviewLines renders a miniature Sift surface for a candidate theme
// without mutating the active model. It deliberately uses the same semantic
// palette and ANSI renderer as the live preview.
func themePreviewLines(p ThemePreset, width int) []string {
	p = normalizeTheme(p)
	styles := themePreviewStyles(p)
	inner := max(10, width-4)
	lines := []string{
		styles.title.Render("Theme Preview"),
		styles.hint.Render(p.Family + " · " + p.Variant + " · " + p.ID),
		styles.treeGuide.Render("  " + string(p.UI.BorderActive) + " internal/"),
		styles.cursor.Render("▸ app.go") + "  " + styles.modeFull.Render("[FULL]") + "  " + styles.hint.Render("1.8k"),
		styles.selected.Render("  config.go") + "  " + styles.modeSig.Render("[SIG]") + "  " + styles.hint.Render("0.7k"),
	}
	doc := highlight.Parse("preview.go", []byte("func greet() string {\n\treturn \"ready\"\n}"), highlight.Options{Enabled: true, Theme: highlight.ThemeAuto, Syntax: &p.Syntax})
	code := []string{
		highlight.RenderDocumentLine(doc, 0, highlight.Options{Enabled: true, Theme: highlight.ThemeAuto, Syntax: &p.Syntax}),
		highlight.RenderDocumentLine(doc, 1, highlight.Options{Enabled: true, Theme: highlight.ThemeAuto, Syntax: &p.Syntax}),
	}
	lines = append(lines, code...)
	lines = append(lines, styles.success.Render("✓ Ready")+"  "+styles.warning.Render("⚠ Warning"))
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], inner, "…")
	}
	return lines
}
func (m *model) renderThemeModal(view string, width, height int) string {
	modalWidth := min(width, 68)
	if modalWidth < 30 {
		modalWidth = 30
	}
	showPreview := modalWidth >= 52 && height >= 18
	previewRows := 0
	if showPreview {
		previewRows = min(9, max(0, height-9))
	}
	bodyRows := max(1, min(len(m.themes), height-6-previewRows))
	maxOffset := max(0, len(m.themes)-bodyRows)
	if m.themeOffset > maxOffset {
		m.themeOffset = maxOffset
	}
	if m.themeOffset < 0 {
		m.themeOffset = 0
	}
	if m.themeCursor < m.themeOffset {
		m.themeOffset = m.themeCursor
	}
	if m.themeCursor >= m.themeOffset+bodyRows {
		m.themeOffset = m.themeCursor - bodyRows + 1
	}

	var b strings.Builder
	b.WriteString(m.modalTitle(m.glyphs.Theme, "Color Theme", modalWidth))
	for i := m.themeOffset; i < m.themeOffset+bodyRows && i < len(m.themes); i++ {
		b.WriteString("\n")
		p := m.themes[i]
		mark := "  "
		if i == m.themeIndex {
			mark = m.styles.notice.Render("●")
		}
		cur := "  "
		if i == m.themeCursor {
			cur = m.styles.title.Render("❯")
		}
		// Three color swatches: border, title, selected
		swatch := lipgloss.NewStyle().Foreground(p.Border).Render("■") +
			lipgloss.NewStyle().Foreground(p.Title).Render("■") +
			lipgloss.NewStyle().Foreground(p.Selected).Render("■")

		name := ansi.Truncate(p.Name, modalWidth-14, "…")
		line := fmt.Sprintf("%s %s %s %s", cur, mark, swatch, name)
		if i == m.themeCursor {
			b.WriteString(m.styles.cursor.Render(ansi.Truncate(line, modalWidth-4, "…")))
		} else {
			b.WriteString(ansi.Truncate(line, modalWidth-4, "…"))
		}
	}
	if showPreview {
		b.WriteString("\n")
		preview := themePreviewLines(m.themes[m.themeCursor], modalWidth)
		for i := 0; i < previewRows && i < len(preview); i++ {
			b.WriteString("\n")
			b.WriteString(preview[i])
		}
	}
	b.WriteString("\n")
	b.WriteString(m.styles.hint.Render("↑/↓ move · Enter/Space apply · Esc/q close"))
	modalHeight := max(1, height-4)
	if showPreview {
		modalHeight = min(modalHeight, lipgloss.Height(b.String())+2)
	}
	return m.overlay(view, m.modalStyle(modalWidth).Height(modalHeight).Render(b.String()), width, height)
}
