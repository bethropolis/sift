// Package highlight provides small, terminal-safe syntax highlighting for
// human-facing previews. It deliberately does not alter source content or
// participate in scanning, token counts, or selection.
package highlight

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

type Theme string

const (
	ThemeAuto  Theme = "auto"
	ThemeNone  Theme = "none"
	ThemeDark  Theme = "dark"
	ThemeLight Theme = "light"
)

// Options controls terminal highlighting.
type Options struct {
	Enabled  bool
	Theme    Theme
	MaxBytes int
	// Palette overrides the built-in Theme palette when non-nil. The TUI
	// sets it from the active color preset so preview highlighting follows
	// the picker's theme; Theme is kept as the fallback.
	Palette *Palette
}

const defaultMaxBytes = 256 * 1024

// Render highlights complete content. It is intended for a terminal writer;
// callers must decide whether their output is a TTY before enabling it.
func Render(path string, content []byte, options Options) string {
	if !options.Enabled || options.Theme == ThemeNone || len(content) == 0 {
		return string(content)
	}
	maxBytes := options.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	if len(content) > maxBytes || !supported(path) {
		return string(content)
	}
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		lines[i] = RenderLine(path, line, options)
	}
	return strings.Join(lines, "\n")
}

// RenderLine highlights one line. Keeping this operation line-local makes it
// suitable for the TUI, which can render only the visible preview rows.
func RenderLine(path, line string, options Options) string {
	if !options.Enabled || options.Theme == ThemeNone || line == "" || !supported(path) {
		return line
	}
	styles := palette(options.Theme)
	if options.Palette != nil {
		styles = *options.Palette
	}
	keywordSet := keywords(path)
	var b strings.Builder
	for i := 0; i < len(line); {
		if strings.HasPrefix(line[i:], "//") || strings.HasPrefix(line[i:], "#") || strings.HasPrefix(line[i:], "--") {
			b.WriteString(styles.Comment)
			b.WriteString(line[i:])
			b.WriteString(reset)
			break
		}
		if line[i] == '"' || line[i] == '\'' || line[i] == '`' {
			quote := line[i]
			j := i + 1
			for j < len(line) {
				if line[j] == '\\' {
					j += 2
					if j > len(line) {
						j = len(line)
					}
					continue
				}
				if line[j] == quote {
					j++
					break
				}
				j++
			}
			b.WriteString(styles.String)
			b.WriteString(line[i:j])
			b.WriteString(reset)
			i = j
			continue
		}
		if unicode.IsDigit(rune(line[i])) {
			j := i + 1
			for j < len(line) && (unicode.IsDigit(rune(line[j])) || line[j] == '.') {
				j++
			}
			b.WriteString(styles.Number)
			b.WriteString(line[i:j])
			b.WriteString(reset)
			i = j
			continue
		}
		if isIdentStart(line[i]) {
			j := i + 1
			for j < len(line) && isIdentPart(line[j]) {
				j++
			}
			word := line[i:j]
			style := ""
			switch {
			case keywordSet[word]:
				style = styles.Keyword
			case isTypeName(word):
				style = styles.TypeName
			}
			if style != "" {
				b.WriteString(style)
				b.WriteString(word)
				b.WriteString(reset)
			} else {
				b.WriteString(word)
			}
			i = j
			continue
		}
		b.WriteByte(line[i])
		i++
	}
	return b.String()
}

const reset = "\033[0m"

// Palette holds the SGR sequences for each token class. The TUI builds one
// per color preset so preview highlighting matches the picker's theme.
type Palette struct {
	Keyword, String, Number, Comment, TypeName string
}

func palette(theme Theme) Palette {
	// Auto intentionally uses the terminal's ANSI palette. Dark/light are
	// explicit but remain conservative so they work on most terminals.
	if theme == ThemeDark {
		return Palette{"\033[1;34m", "\033[32m", "\033[36m", "\033[2;37m", "\033[35m"}
	}
	if theme == ThemeLight {
		return Palette{"\033[1;34m", "\033[31m", "\033[35m", "\033[2;30m", "\033[34m"}
	}
	return Palette{"\033[1;34m", "\033[32m", "\033[36m", "\033[2m", "\033[35m"}
}

// ANSI builds a truecolor SGR sequence for hex (e.g. "#cba6f7" or "cba6f7")
// with optional SGR modifiers ("1" for bold, "2" for faint). It returns ""
// for unparseable input so callers degrade to unstyled text.
func ANSI(hex string, mods ...string) string {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) != 6 {
		return ""
	}
	rgb := make([]int64, 3)
	for i := range rgb {
		v, err := strconv.ParseUint(hex[2*i:2*i+2], 16, 8)
		if err != nil {
			return ""
		}
		rgb[i] = int64(v)
	}
	code := strings.Join(append(mods, fmt.Sprintf("38;2;%d;%d;%d", rgb[0], rgb[1], rgb[2])), ";")
	return "\033[" + code + "m"
}

func supported(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".rs", ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".py", ".pyi", ".java", ".kt", ".kts", ".cs", ".c", ".h", ".cc", ".cpp", ".cxx", ".rb", ".php", ".swift", ".json", ".yaml", ".yml", ".toml", ".sh", ".bash", ".zsh", ".sql", ".html", ".css", ".scss", ".vue", ".svelte", ".astro":
		return true
	default:
		return false
	}
}

func keywords(path string) map[string]bool {
	base := map[string]bool{}
	for _, word := range strings.Fields("if else for range switch case return func package import type struct interface class def return const var let function export async await try catch throw new public private protected static void int string bool fn impl trait use mod match pub enum where namespace using") {
		base[word] = true
	}
	if strings.HasSuffix(strings.ToLower(path), ".py") {
		for _, word := range strings.Fields("from as in is None True False with yield lambda") {
			base[word] = true
		}
	}
	return base
}

func isTypeName(word string) bool {
	return len(word) > 0 && unicode.IsUpper(rune(word[0]))
}

func isIdentStart(b byte) bool { return b == '_' || unicode.IsLetter(rune(b)) }
func isIdentPart(b byte) bool  { return isIdentStart(b) || unicode.IsDigit(rune(b)) }
