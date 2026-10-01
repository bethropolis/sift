package tui

// Glyphs holds the decorative symbols used by the picker. Two variants exist:
// Nerd Fonts (rich, Unicode) and ASCII (fallback for plain terminals). The
// ASCII fallback is selected with --no-nerd-fonts.
type Glyphs struct {
	FolderClosed string
	FolderOpen   string
	File         string
	Warning      string
	CheckFull    string
	CheckPartial string
	CheckNone    string
	Cursor       string
	Relevant     string
	Success      string
	Refresh      string
	ModeFull     string
	ModeSigns    string
	ModeSkip     string

	TreePipe   string
	TreeMiddle string
	TreeLast   string
	TreeSpace  string

	Theme  string
	Delta  string
	Prompt string
	Help   string
}

// NewNerdFontGlyphs returns the Nerd Font glyph set.
func NewNerdFontGlyphs() Glyphs {
	return Glyphs{
		FolderClosed: "\uf07b ",
		FolderOpen:   "\uf07c ",
		File:         "\U000f0219 ",
		Warning:      "\uf071 ",
		CheckFull:    "●",
		CheckPartial: "◐",
		CheckNone:    "○",
		Cursor:       "▸ ",
		Relevant:     "◆",
		Success:      "✓",
		Refresh:      "↻",
		ModeFull:     "[FULL]",
		ModeSigns:    "[SIGS]",
		ModeSkip:     "[SKIP]",

		TreePipe:   "│   ",
		TreeMiddle: "├── ",
		TreeLast:   "└── ",
		TreeSpace:  "    ",

		Theme:  "󰸌 ",
		Delta:  "⟳ ",
		Prompt: "󰞋 ",
		Help:   "󰋖 ",
	}
}

// NewASCIIGlyphs returns a plain ASCII fallback glyph set.
func NewASCIIGlyphs() Glyphs {
	return Glyphs{
		FolderClosed: "+ ",
		FolderOpen:   "- ",
		File:         "  ",
		Warning:      "! ",
		CheckFull:    "[x]",
		CheckPartial: "[-]",
		CheckNone:    "[ ]",
		Cursor:       "> ",
		Relevant:     "*",
		Success:      "+",
		Refresh:      "~",
		ModeFull:     "[FULL]",
		ModeSigns:    "[SIGS]",
		ModeSkip:     "[SKIP]",

		TreePipe:   "|   ",
		TreeMiddle: "|-- ",
		TreeLast:   "`-- ",
		TreeSpace:  "    ",

		Theme:  "* ",
		Delta:  "^ ",
		Prompt: "> ",
		Help:   "? ",
	}
}
