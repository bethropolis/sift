package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

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

// Normalize fills semantic roles from the compatibility fields and
// guarantees that every renderer-facing role has a concrete fallback.
func Normalize(p ThemePreset) ThemePreset {
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
// DefaultIndex returns the index of the Classic (Default) preset in Presets,
// which is the initial theme.
func DefaultIndex() int {
	return DefaultIndexIn(ThemePresets)
}

// IndexOf resolves a persisted theme id or name to an index, tolerating a
// missing or stale preference after themes are renamed or removed.
func IndexOf(themes []ThemePreset, name string) int {
	defaultIndex := DefaultIndexIn(themes)
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

// DefaultIndexIn returns the index of the Classic (Default) preset in themes.
func DefaultIndexIn(themes []ThemePreset) int {
	for i, p := range themes {
		if p.ID == "classic" || p.Name == "Classic (Default)" {
			return i
		}
	}
	return 0
}

// IDAt returns the stable id of the preset at index, falling back to the
// default when out of range.
func IDAt(themes []ThemePreset, index int) string {
	if index < 0 || index >= len(themes) {
		return themes[DefaultIndexIn(themes)].ID
	}
	return themes[index].ID
}
