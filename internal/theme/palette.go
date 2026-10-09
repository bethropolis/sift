package theme

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/bethropolis/sift/internal/highlight"
)

// Palette is the small set of colors every editor theme defines. A preset is
// derived from it in one place (build), so every theme maps colors to UI
// roles, status colors, and syntax tokens the same way. Adding a theme means
// filling in a Palette, nothing else.
//
// All values are "#rrggbb" hex strings.
type Palette struct {
	// Surfaces and text.
	Bg    string // main background
	Alt   string // panel / secondary background
	Sel   string // cursor row and selection background
	Fg    string // primary text
	Muted string // comments and secondary text; keep >= 3:1 against Bg
	Line  string // borders, tree guides, scrollbar track

	// Hues. Every theme maps its own colors onto these seven slots.
	Red, Orange, Yellow, Green, Cyan, Blue, Purple string

	// Optional overrides. Empty values fall back to the defaults noted.
	Accent   string // chrome highlight (default Blue)
	Keyword  string // default Purple
	Function string // default Blue
	Type     string // default Yellow
	String   string // default Green
	Number   string // default Orange
	Property string // default Cyan
	Operator string // default Cyan
}

// pick returns value, or fallback when value is empty.
func pick(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func color(hex string) lipgloss.Color { return lipgloss.Color(hex) }

// syntax maps semantic token kinds to palette colors. Every TokenKind except
// TokenPlain is covered so no theme silently renders a token unstyled.
func (p Palette) syntax() highlight.SyntaxPalette {
	keyword := pick(p.Keyword, p.Purple)
	function := pick(p.Function, p.Blue)
	typeName := pick(p.Type, p.Yellow)
	str := pick(p.String, p.Green)
	number := pick(p.Number, p.Orange)
	property := pick(p.Property, p.Cyan)
	operator := pick(p.Operator, p.Cyan)

	return highlight.SyntaxPalette{
		highlight.TokenKeyword:       {Foreground: keyword, Bold: true},
		highlight.TokenString:        {Foreground: str},
		highlight.TokenStringEscape:  {Foreground: p.Cyan, Bold: true},
		highlight.TokenRegex:         {Foreground: p.Orange},
		highlight.TokenNumber:        {Foreground: number},
		highlight.TokenBool:          {Foreground: number},
		highlight.TokenNull:          {Foreground: number},
		highlight.TokenComment:       {Foreground: p.Muted, Italic: true},
		highlight.TokenDocComment:    {Foreground: p.Muted, Italic: true},
		highlight.TokenType:          {Foreground: typeName},
		highlight.TokenFunction:      {Foreground: function},
		highlight.TokenVariable:      {Foreground: p.Fg},
		highlight.TokenConstant:      {Foreground: p.Orange},
		highlight.TokenProperty:      {Foreground: property},
		highlight.TokenBuiltin:       {Foreground: function},
		highlight.TokenOperator:      {Foreground: operator},
		highlight.TokenPunctuation:   {Foreground: p.Fg, Faint: true},
		highlight.TokenAttribute:     {Foreground: p.Yellow},
		highlight.TokenDecorator:     {Foreground: p.Yellow, Bold: true},
		highlight.TokenTag:           {Foreground: p.Red},
		highlight.TokenTagAttribute:  {Foreground: p.Orange},
		highlight.TokenMarkupHeading: {Foreground: function, Bold: true},
		highlight.TokenMarkupLink:    {Foreground: p.Cyan, Underline: true},
		highlight.TokenShebang:       {Foreground: p.Muted, Italic: true},
		highlight.TokenDiffAdded:     {Foreground: p.Green},
		highlight.TokenDiffRemoved:   {Foreground: p.Red},
		highlight.TokenDiffHunk:      {Foreground: p.Blue, Bold: true},
	}
}

// build derives a complete ThemePreset from a palette. The display name is
// family plus variant ("Catppuccin Mocha"); pass an empty variant for
// single-variant families.
func build(id, family, variant string, dark bool, p Palette) ThemePreset {
	name := family
	if variant != "" {
		name += " " + variant
	}
	accent := pick(p.Accent, p.Blue)

	preset := ThemePreset{
		ID: id, Name: name, Family: family, Variant: variant, Dark: dark,

		// Legacy fields, kept for renderers that predate UIColors.
		Border: color(p.Purple), Title: color(accent), Muted: color(p.Muted),
		CursorBg: color(p.Sel), CursorFg: color(p.Fg), Selected: color(accent),
		Notice: color(p.Green), ModeFull: color(p.Green), ModeSig: color(p.Yellow), ModeSkip: color(p.Red),

		Highlight: hlPalette(
			pick(p.Keyword, p.Purple), pick(p.String, p.Green), pick(p.Number, p.Orange),
			p.Muted, pick(p.Type, p.Yellow),
		),
		Syntax: p.syntax(),

		UI: UIColors{
			Background: color(p.Bg), Surface: color(p.Alt), SurfaceAlt: color(p.Sel),
			Text: color(p.Fg), TextMuted: color(p.Muted), TextSubtle: color(p.Line),
			Accent: color(accent), AccentSoft: color(p.Sel),
			Border: color(p.Line), BorderActive: color(accent),
			CursorBg: color(p.Sel), CursorFg: color(p.Fg),
			SelectionBg: color(p.Sel), SelectionFg: color(accent),
			TreeGuide: color(p.Line), Scrollbar: color(p.Line),
			ModeFull: color(p.Green), ModeSig: color(p.Yellow), ModeSkip: color(p.Red),
		},
		Status: StatusColors{
			Info: color(p.Blue), Success: color(p.Green),
			Warning: color(p.Yellow), Error: color(p.Red),
		},
	}
	return Normalize(preset)
}
