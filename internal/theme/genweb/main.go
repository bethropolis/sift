// Command genweb derives the web theme data and CSS from the
// internal/theme catalog, so the browser UI never hand-maintains palettes.
// Run it after touching ThemePresets:
//
//	go run ./internal/theme/genweb
//
// It rewrites web/src/lib/themes-generated.ts and
// web/src/themes-generated.css. Presets without a hex background
// (Classic ANSI, terminal-following) are skipped.
package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/bethropolis/sift/internal/highlight"
	"github.com/bethropolis/sift/internal/theme"
)

type rgb struct{ r, g, b float64 }

func parseHex(s string) (rgb, bool) {
	var r, g, b int
	if len(s) != 7 || s[0] != '#' {
		return rgb{}, false
	}
	if _, err := fmt.Sscanf(s[1:], "%02x%02x%02x", &r, &g, &b); err != nil {
		return rgb{}, false
	}
	return rgb{float64(r) / 255, float64(g) / 255, float64(b) / 255}, true
}

func hex(c rgb) string {
	cl := func(v float64) int { return int(math.Round(math.Min(1, math.Max(0, v)) * 255)) }
	return fmt.Sprintf("#%02x%02x%02x", cl(c.r), cl(c.g), cl(c.b))
}

func mix(a, b rgb, t float64) rgb {
	return rgb{a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t}
}

func white() rgb { return rgb{1, 1, 1} }

func black() rgb { return rgb{0, 0, 0} }

// shade lightens (amt > 0) or darkens (amt < 0) by amt, e.g. 0.12.
func shade(c rgb, amt float64) rgb {
	if amt >= 0 {
		return mix(c, white(), amt)
	}
	return mix(c, black(), -amt)
}

func rgba(c rgb, a float64) string {
	cl := func(v float64) int { return int(math.Round(math.Min(1, math.Max(0, v)) * 255)) }
	return fmt.Sprintf("rgba(%d, %d, %d, %g)", cl(c.r), cl(c.g), cl(c.b), a)
}

func luminance(c rgb) float64 {
	lin := func(v float64) float64 {
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.r) + 0.7152*lin(c.g) + 0.0722*lin(c.b)
}

func contrast(a, b rgb) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

var descriptions = map[string]string{
	"catppuccin-mocha":     "Soothing pastel theme for high-spirited developers",
	"catppuccin-macchiato": "Cozy mid-dark Catppuccin between mocha and frappe",
	"catppuccin-frappe":    "Soft overcast Catppuccin, brighter than mocha",
	"catppuccin-latte":     "Soothing pastel light theme for daytime coding",
	"tokyo-night":          "A clean dark theme celebrating Tokyo night lights",
	"tokyo-storm":          "Deeper storm variant of Tokyo Night",
	"tokyo-moon":           "Moonlit cooler take on Tokyo Night",
	"tokyo-day":            "Tokyo night lights inverted for bright rooms",
	"dracula":              "Famous gothic high-contrast dark theme",
	"gruvbox-dark":         "Retro groove warm and earthy dark colors",
	"gruvbox-light":        "Warm paper retro groove light colors",
	"nord":                 "An arctic, north-bluish clean and elegant palette",
	"rose-pine":            "Minimal, muted warmth and pine undertones",
	"rose-pine-moon":       "Moonlit cooler Rosé Pine",
	"rose-pine-dawn":       "Warm morning light with pine undertones",
	"one-dark":             "The iconic Atom and modern editor classic",
	"one-light":            "One Dark inverted for bright environments",
	"solarized-dark":       "Precision low-contrast dark classic",
	"solarized-light":      "Precision low-contrast light classic",
	"monokai":              "Classic vibrant high-contrast hacker palette",
	"monokai-pro":          "Softer filtered take on Monokai",
	"github-dark":          "GitHub default dark code workspace",
	"github-dimmed":        "Softer dimmed GitHub dark",
	"github-light":         "Crisp GitHub default light workspace",
	"night-owl":            "Deep blue night for late sessions",
	"light-owl":            "Night Owl inverted for daylight",
	"kanagawa-wave":        "Great wave paints muted ink and water tones",
	"kanagawa-dragon":      "Darker armor-toned Kanagawa",
	"kanagawa-lotus":       "Ink on paper Kanagawa light",
	"everforest-dark":      "Earthy greens and warm paper tones, easy on the eyes",
	"everforest-light":     "Soft paper greens for bright rooms",
	"ayu-dark":             "Bright and simple Ayu dark",
	"ayu-mirage":           "Blue-tinted Ayu middle ground",
	"ayu-light":            "Clean Ayu light",
	"material-palenight":   "Material dusky evening purple",
	"material-ocean":       "Deep Material ocean night",
	"nightfox":             "Cool dark fox night",
	"darcula":              "JetBrains Darcula IDE classic",
	"vscode-dark-plus":     "VS Code default dark",
	"vscode-light-plus":    "VS Code default light",
	"tomorrow-night":       "Tomorrow evening blue-grey dark",
	"zenburn":              "Low-contrast warm grey classic",
	"cobalt2":              "Vivid blue neon glow",
	"synthwave-84":         "Neon-soaked retro synthwave",
	"horizon":              "Warm sunset editor glow",
	"poimandres":           "Soft pastel forest night",
	"classic-dark":         "Quiet zinc and slate with warm brass accent",
	"classic-light":        "Clean paper and ink with subtle brass accent",
}

type webTheme struct {
	id, name, category, bg, accent, surface, description string
	css                                                  string
	syn                                                  string
}

func convert(p theme.ThemePreset) (webTheme, bool) {
	bg, ok := parseHex(string(p.UI.Background))
	if !ok {
		return webTheme{}, false
	}
	fg, _ := parseHex(string(p.UI.Text))
	muted, _ := parseHex(string(p.UI.TextMuted))
	line, _ := parseHex(string(p.UI.Border))
	green, _ := parseHex(string(p.UI.ModeFull))
	yellow, _ := parseHex(string(p.UI.ModeSig))
	red, _ := parseHex(string(p.Status.Error))
	accent, _ := parseHex(string(p.UI.Accent))
	category := "dark"
	if !p.Dark {
		category = "light"
	}
	soft := mix(fg, muted, 0.45)
	raised := shade(mustHex(string(p.UI.Surface)), ternary(p.Dark, -0.07, 0.06))
	strong := shade(line, ternary(p.Dark, 0.16, -0.14))
	subtle := mix(line, bg, 0.5)
	hover := shade(accent, ternary(p.Dark, 0.12, -0.12))
	accentInk := "#ffffff"
	if w, _ := parseHex("#ffffff"); contrast(accent, w) < 3.0 {
		accentInk = "#1f2328"
	}
	tabActive := hex(mix(mustHex(string(p.UI.Surface)), line, 0.5))
	tabInactive := hex(muted)
	if !p.Dark {
		tabActive = hex(bg)
		tabInactive = hex(soft)
	}
	codeBg := hex(mustHex(string(p.UI.Surface)))
	codeGutter := hex(raised)
	codeDim := hex(muted)
	codeLine := hex(strong)
	if !p.Dark {
		codeBg = hex(bg)
		codeGutter = hex(mustHex(string(p.UI.Surface)))
		codeDim = hex(soft)
		codeLine = hex(muted)
	}
	logoBg := hex(raised)
	logoBorder := hex(line)
	if !p.Dark {
		logoBg = hex(fg)
		logoBorder = hex(soft)
	}
	v := func(c rgb) string { return hex(c) }
	css := fmt.Sprintf(`/* %s */
[data-theme=%q] {
  --bg: %s;
  --surface-alt: %s;
  --surface-raised: %s;
  --panel-bg: %s;
  --ink: %s;
  --ink-soft: %s;
  --ink-faint: %s;
  --border: %s;
  --border-strong: %s;
  --border-subtle: %s;
  --accent: %s;
  --accent-hover: %s;
  --accent-soft: %s;
  --accent-ink: %s;
  --logo-bg: %s;
  --logo-border: %s;
  --tab-container-bg: %s;
  --tab-active-bg: %s;
  --tab-active-text: %s;
  --tab-inactive-text: %s;
  --code-bg: %s;
  --code-text: %s;
  --code-dim: %s;
  --code-accent: %s;
  --code-border: %s;
  --code-line-number: %s;
  --code-gutter-bg: %s;
  --mode-full: %s;
  --mode-full-bg: %s;
  --mode-full-border: %s;
  --mode-sigs: %s;
  --mode-sigs-bg: %s;
  --mode-sigs-border: %s;
  --mode-skip: %s;
  --status-ok: %s;
  --status-warn: %s;
  --status-danger: %s;
}`,
		p.Name, p.ID,
		v(bg), v(mustHex(string(p.UI.Surface))), hex(raised), v(mustHex(string(p.UI.Surface))),
		v(fg), hex(soft), v(muted),
		v(line), hex(strong), hex(subtle),
		v(accent), hex(hover), rgba(accent, 0.12), accentInk,
		logoBg, logoBorder,
		v(mustHex(string(p.UI.Surface))), tabActive, v(fg), tabInactive,
		codeBg, v(fg), codeDim, v(accent), v(line), codeLine, codeGutter,
		v(green), rgba(green, 0.1), rgba(green, 0.25),
		v(yellow), rgba(yellow, 0.1), rgba(yellow, 0.25),
		v(muted),
		v(green), v(yellow), v(red),
	)
	synTok := func(k highlight.TokenKind, fallback string) string {
		if s, ok := p.Syntax[k]; ok && s.Foreground != "" {
			return s.Foreground
		}
		return fallback
	}
	fgHex := hex(fg)
	syn := fmt.Sprintf(`[data-theme=%q] {
  --syn-kw: %s;
  --syn-str: %s;
  --syn-num: %s;
  --syn-com: %s;
  --syn-typ: %s;
  --syn-fn: %s;
  --syn-var: %s;
}`,
		p.ID,
		synTok(highlight.TokenKeyword, fgHex), synTok(highlight.TokenString, fgHex),
		synTok(highlight.TokenNumber, fgHex), synTok(highlight.TokenComment, fgHex),
		synTok(highlight.TokenType, fgHex), synTok(highlight.TokenFunction, fgHex),
		synTok(highlight.TokenVariable, fgHex),
	)
	desc := descriptions[p.ID]
	if desc == "" {
		desc = p.Name + " theme"
	}
	return webTheme{
		id: p.ID, name: p.Name, category: category,
		bg: v(bg), accent: v(accent), surface: v(mustHex(string(p.UI.Surface))),
		description: desc, css: css, syn: syn,
	}, true
}

func ternary(cond bool, a, b float64) float64 {
	if cond {
		return a
	}
	return b
}

func mustHex(s string) rgb {
	c, _ := parseHex(s)
	return c
}

func main() {
	_, file, _, _ := runtime.Caller(0)
	webDir := filepath.Join(filepath.Dir(file), "..", "..", "..", "web", "src")
	var themes []webTheme
	for _, p := range theme.ThemePresets {
		t, ok := convert(p)
		if !ok {
			continue
		}
		themes = append(themes, t)
	}
	sort.SliceStable(themes, func(i, j int) bool { return themes[i].name < themes[j].name })

	var ts strings.Builder
	ts.WriteString("import type { ThemeDefinition } from './themes';\n\n")
	ts.WriteString("// Generated by go run ./internal/theme/genweb — do not edit.\n")
	ts.WriteString("export const GENERATED_THEMES: ThemeDefinition[] = [\n")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "\\'") + "'" }
	for _, t := range themes {
		fmt.Fprintf(&ts, "  {\n    id: %s,\n    name: %s,\n    category: '%s',\n    bg: '%s',\n    accent: '%s',\n    surface: '%s',\n    description: %s,\n  },\n",
			quote(t.id), quote(t.name), t.category, t.bg, t.accent, t.surface, quote(t.description))
	}
	ts.WriteString("];\n")
	if err := os.WriteFile(filepath.Join(webDir, "lib", "themes-generated.ts"), []byte(ts.String()), 0o644); err != nil {
		panic(err)
	}

	var css strings.Builder
	css.WriteString("/* Generated by go run ./internal/theme/genweb — do not edit. */\n")
	for _, t := range themes {
		css.WriteString("\n" + t.css + "\n")
	}
	css.WriteString("\n/* Preview syntax tokens, mirroring the TUI Syntax palettes. */\n")
	for _, t := range themes {
		css.WriteString("\n" + t.syn + "\n")
	}
	if err := os.WriteFile(filepath.Join(webDir, "themes-generated.css"), []byte(css.String()), 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("generated %d web themes\n", len(themes))
}
