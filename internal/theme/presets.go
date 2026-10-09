package theme

import (
	"sort"

	"github.com/charmbracelet/lipgloss"

	"github.com/bethropolis/sift/internal/highlight"
)

// ThemePresets lists every selectable theme in the interactive t menu, in
// menu order: Classic first (the default), then the catalog grouped by
// family, then the terminal-following theme.
var ThemePresets = sortedPresets()

func sortedPresets() []ThemePreset {
	all := append(append([]ThemePreset{classicTheme()}, catalog()...), terminalTheme())
	sort.SliceStable(all, func(i, j int) bool { return all[i].Name < all[j].Name })
	return all
}

// classicTheme keeps the legacy ANSI palette byte-for-byte so the default
// preview rendering is unchanged.
func classicTheme() ThemePreset {
	return ThemePreset{
		ID: "classic", Name: "Classic (Default)", Family: "Classic", Variant: "Default", Dark: true,
		Border: lipgloss.Color("62"), Title: lipgloss.Color("12"), Muted: lipgloss.Color("245"),
		CursorBg: lipgloss.Color("236"), CursorFg: lipgloss.Color("15"), Selected: lipgloss.Color("12"),
		Notice: lipgloss.Color("10"), ModeFull: lipgloss.Color("10"), ModeSig: lipgloss.Color("11"), ModeSkip: lipgloss.Color("9"),
		Highlight: highlight.Palette{Keyword: "\033[1;34m", String: "\033[32m", Number: "\033[36m", Comment: "\033[2m", TypeName: "\033[35m"},
	}
}

// catalog holds the palette-driven themes. Hex values follow each theme's
// published palette; Muted is nudged lighter or darker where the canonical
// comment color fails a 3:1 contrast check (see presets_test.go).
func catalog() []ThemePreset {
	return []ThemePreset{
		// Catppuccin
		build("catppuccin-mocha", "Catppuccin", "Mocha", true, Palette{
			Bg: "#1e1e2e", Alt: "#181825", Sel: "#313244", Fg: "#cdd6f4", Muted: "#7f849c", Line: "#45475a",
			Red: "#f38ba8", Orange: "#fab387", Yellow: "#f9e2af", Green: "#a6e3a1", Cyan: "#94e2d5", Blue: "#89b4fa", Purple: "#cba6f7",
			Accent: "#cba6f7",
		}),
		build("catppuccin-macchiato", "Catppuccin", "Macchiato", true, Palette{
			Bg: "#24273a", Alt: "#1e2030", Sel: "#363a4f", Fg: "#cad3f5", Muted: "#8087a2", Line: "#494d64",
			Red: "#ed8796", Orange: "#f5a97f", Yellow: "#eed49f", Green: "#a6da95", Cyan: "#8bd5ca", Blue: "#8aadf4", Purple: "#c6a0f6",
			Accent: "#c6a0f6",
		}),
		build("catppuccin-frappe", "Catppuccin", "Frappé", true, Palette{
			Bg: "#303446", Alt: "#292c3c", Sel: "#414559", Fg: "#c6d0f5", Muted: "#838ba7", Line: "#51576d",
			Red: "#e78284", Orange: "#ef9f76", Yellow: "#e5c890", Green: "#a6d189", Cyan: "#81c8be", Blue: "#8caaee", Purple: "#ca9ee6",
			Accent: "#ca9ee6",
		}),
		build("catppuccin-latte", "Catppuccin", "Latte", false, Palette{
			Bg: "#eff1f5", Alt: "#e6e9ef", Sel: "#ccd0da", Fg: "#4c4f69", Muted: "#7c7f93", Line: "#bcc0cc",
			Red: "#d20f39", Orange: "#fe640b", Yellow: "#b5710f", Green: "#37882a", Cyan: "#179299", Blue: "#1e66f5", Purple: "#8839ef",
			Accent: "#8839ef",
		}),

		// Tokyo Night
		build("tokyo-night", "Tokyo Night", "", true, Palette{
			Bg: "#1a1b26", Alt: "#16161e", Sel: "#283457", Fg: "#c0caf5", Muted: "#6b7394", Line: "#3b4261",
			Red: "#f7768e", Orange: "#ff9e64", Yellow: "#e0af68", Green: "#9ece6a", Cyan: "#7dcfff", Blue: "#7aa2f7", Purple: "#bb9af7",
			Property: "#73daca", Operator: "#89ddff",
		}),
		build("tokyo-storm", "Tokyo Night", "Storm", true, Palette{
			Bg: "#24283b", Alt: "#1f2335", Sel: "#2e3c64", Fg: "#c0caf5", Muted: "#7079a1", Line: "#3b4261",
			Red: "#f7768e", Orange: "#ff9e64", Yellow: "#e0af68", Green: "#9ece6a", Cyan: "#7dcfff", Blue: "#7aa2f7", Purple: "#bb9af7",
			Property: "#73daca", Operator: "#89ddff",
		}),
		build("tokyo-moon", "Tokyo Night", "Moon", true, Palette{
			Bg: "#222436", Alt: "#1e2030", Sel: "#2d3f76", Fg: "#c8d3f5", Muted: "#7a88cf", Line: "#444a73",
			Red: "#ff757f", Orange: "#ff966c", Yellow: "#ffc777", Green: "#c3e88d", Cyan: "#86e1fc", Blue: "#82aaff", Purple: "#c099ff",
			Property: "#4fd6be", Operator: "#89ddff",
		}),
		build("tokyo-day", "Tokyo Night", "Day", false, Palette{
			Bg: "#e1e2e7", Alt: "#d0d5e3", Sel: "#b7c1e3", Fg: "#3760bf", Muted: "#6172b0", Line: "#a8aecb",
			Red: "#f52a65", Orange: "#b15c00", Yellow: "#8c6c3e", Green: "#587539", Cyan: "#007197", Blue: "#2e7de9", Purple: "#9854f1",
			Property: "#387068",
		}),

		// Dracula
		build("dracula", "Dracula", "", true, Palette{
			Bg: "#282a36", Alt: "#21222c", Sel: "#44475a", Fg: "#f8f8f2", Muted: "#7082b6", Line: "#44475a",
			Red: "#ff5555", Orange: "#ffb86c", Yellow: "#f1fa8c", Green: "#50fa7b", Cyan: "#8be9fd", Blue: "#bd93f9", Purple: "#bd93f9",
			Accent: "#ff79c6", Keyword: "#ff79c6", Function: "#50fa7b", Type: "#8be9fd", String: "#f1fa8c", Number: "#bd93f9", Operator: "#ff79c6",
		}),

		// Gruvbox
		build("gruvbox-dark", "Gruvbox", "Dark", true, Palette{
			Bg: "#282828", Alt: "#1d2021", Sel: "#3c3836", Fg: "#ebdbb2", Muted: "#928374", Line: "#504945",
			Red: "#fb4934", Orange: "#fe8019", Yellow: "#fabd2f", Green: "#b8bb26", Cyan: "#8ec07c", Blue: "#83a598", Purple: "#d3869b",
			Keyword: "#fb4934", Operator: "#fe8019",
		}),
		build("gruvbox-light", "Gruvbox", "Light", false, Palette{
			Bg: "#fbf1c7", Alt: "#f2e5bc", Sel: "#ebdbb2", Fg: "#3c3836", Muted: "#7c6f64", Line: "#d5c4a1",
			Red: "#9d0006", Orange: "#af3a03", Yellow: "#b57614", Green: "#79740e", Cyan: "#427b58", Blue: "#076678", Purple: "#8f3f71",
			Keyword: "#9d0006", Operator: "#af3a03",
		}),

		// Nord
		build("nord", "Nord", "", true, Palette{
			Bg: "#2e3440", Alt: "#242933", Sel: "#434c5e", Fg: "#d8dee9", Muted: "#7b88a1", Line: "#4c566a",
			Red: "#bf616a", Orange: "#d08770", Yellow: "#ebcb8b", Green: "#a3be8c", Cyan: "#88c0d0", Blue: "#81a1c1", Purple: "#b48ead",
			Accent: "#88c0d0", Keyword: "#81a1c1", Function: "#88c0d0", Type: "#8fbcbb",
		}),

		// Rosé Pine
		build("rose-pine", "Rosé Pine", "", true, Palette{
			Bg: "#191724", Alt: "#1f1d2e", Sel: "#26233a", Fg: "#e0def4", Muted: "#8a86a3", Line: "#403d52",
			Red: "#eb6f92", Orange: "#f6c177", Yellow: "#f6c177", Green: "#9ccfd8", Cyan: "#9ccfd8", Blue: "#31748f", Purple: "#c4a7e7",
			Accent: "#c4a7e7", Keyword: "#31748f", Function: "#ebbcba", Type: "#9ccfd8", String: "#f6c177",
		}),
		build("rose-pine-moon", "Rosé Pine", "Moon", true, Palette{
			Bg: "#232136", Alt: "#2a273f", Sel: "#393552", Fg: "#e0def4", Muted: "#908caa", Line: "#44415a",
			Red: "#eb6f92", Orange: "#f6c177", Yellow: "#f6c177", Green: "#9ccfd8", Cyan: "#9ccfd8", Blue: "#3e8fb0", Purple: "#c4a7e7",
			Accent: "#c4a7e7", Keyword: "#3e8fb0", Function: "#ea9a97", Type: "#9ccfd8", String: "#f6c177",
		}),
		build("rose-pine-dawn", "Rosé Pine", "Dawn", false, Palette{
			Bg: "#faf4ed", Alt: "#fffaf3", Sel: "#f2e9e1", Fg: "#575279", Muted: "#797593", Line: "#dfdad9",
			Red: "#b4637a", Orange: "#ea9d34", Yellow: "#c27a1a", Green: "#56949f", Cyan: "#56949f", Blue: "#286983", Purple: "#907aa9",
			Accent: "#907aa9", Keyword: "#286983", Function: "#d7827e", Type: "#56949f", String: "#ea9d34",
		}),

		// One
		build("one-dark", "One", "Dark", true, Palette{
			Bg: "#282c34", Alt: "#21252b", Sel: "#3e4451", Fg: "#abb2bf", Muted: "#7f848e", Line: "#3b4048",
			Red: "#e06c75", Orange: "#d19a66", Yellow: "#e5c07b", Green: "#98c379", Cyan: "#56b6c2", Blue: "#61afef", Purple: "#c678dd",
			Property: "#e06c75",
		}),
		build("one-light", "One", "Light", false, Palette{
			Bg: "#fafafa", Alt: "#f0f0f1", Sel: "#e5e5e6", Fg: "#383a42", Muted: "#6b6d75", Line: "#dbdbdc",
			Red: "#e45649", Orange: "#986801", Yellow: "#c18401", Green: "#50a14f", Cyan: "#0184bc", Blue: "#4078f2", Purple: "#a626a4",
			Property: "#e45649",
		}),

		// Solarized
		build("solarized-dark", "Solarized", "Dark", true, Palette{
			Bg: "#002b36", Alt: "#073642", Sel: "#0b4756", Fg: "#93a1a1", Muted: "#6c8289", Line: "#1a4e5c",
			Red: "#dc322f", Orange: "#cb4b16", Yellow: "#b58900", Green: "#859900", Cyan: "#2aa198", Blue: "#268bd2", Purple: "#6c71c4",
			Keyword: "#859900",
		}),
		build("solarized-light", "Solarized", "Light", false, Palette{
			Bg: "#fdf6e3", Alt: "#eee8d5", Sel: "#e4ddc8", Fg: "#586e75", Muted: "#6f8087", Line: "#d3cbb7",
			Red: "#dc322f", Orange: "#cb4b16", Yellow: "#a07700", Green: "#778800", Cyan: "#2aa198", Blue: "#268bd2", Purple: "#6c71c4",
			Keyword: "#859900",
		}),

		// Monokai
		build("monokai", "Monokai", "", true, Palette{
			Bg: "#272822", Alt: "#1e1f1c", Sel: "#3e3d32", Fg: "#f8f8f2", Muted: "#8f8b73", Line: "#49483e",
			Red: "#f92672", Orange: "#fd971f", Yellow: "#e6db74", Green: "#a6e22e", Cyan: "#66d9ef", Blue: "#66d9ef", Purple: "#ae81ff",
			Keyword: "#f92672", Function: "#a6e22e", Type: "#66d9ef", String: "#e6db74", Number: "#ae81ff", Operator: "#f92672",
		}),
		build("monokai-pro", "Monokai", "Pro", true, Palette{
			Bg: "#2d2a2e", Alt: "#221f22", Sel: "#403e41", Fg: "#fcfcfa", Muted: "#939293", Line: "#5b595c",
			Red: "#ff6188", Orange: "#fc9867", Yellow: "#ffd866", Green: "#a9dc76", Cyan: "#78dce8", Blue: "#78dce8", Purple: "#ab9df2",
			Keyword: "#ff6188", Function: "#a9dc76", Type: "#78dce8", String: "#ffd866", Number: "#ab9df2", Operator: "#ff6188",
		}),

		// GitHub
		build("github-dark", "GitHub", "Dark", true, Palette{
			Bg: "#0d1117", Alt: "#161b22", Sel: "#21262d", Fg: "#e6edf3", Muted: "#8b949e", Line: "#30363d",
			Red: "#ff7b72", Orange: "#ffa657", Yellow: "#e3b341", Green: "#3fb950", Cyan: "#a5d6ff", Blue: "#79c0ff", Purple: "#d2a8ff",
			Accent: "#58a6ff", Keyword: "#ff7b72", Function: "#d2a8ff", Type: "#ffa657", String: "#a5d6ff", Number: "#79c0ff", Property: "#79c0ff",
		}),
		build("github-dimmed", "GitHub", "Dark Dimmed", true, Palette{
			Bg: "#22272e", Alt: "#1c2128", Sel: "#2d333b", Fg: "#adbac7", Muted: "#768390", Line: "#444c56",
			Red: "#f47067", Orange: "#f69d50", Yellow: "#c69026", Green: "#57ab5a", Cyan: "#96d0ff", Blue: "#6cb6ff", Purple: "#dcbdfb",
			Accent: "#539bf5", Keyword: "#f47067", Function: "#dcbdfb", Type: "#f69d50", String: "#96d0ff", Number: "#6cb6ff", Property: "#6cb6ff",
		}),
		build("github-light", "GitHub", "Light", false, Palette{
			Bg: "#ffffff", Alt: "#f6f8fa", Sel: "#eaeef2", Fg: "#1f2328", Muted: "#6e7781", Line: "#d0d7de",
			Red: "#cf222e", Orange: "#953800", Yellow: "#9a6700", Green: "#1a7f37", Cyan: "#0a3069", Blue: "#0550ae", Purple: "#8250df",
			Accent: "#0969da", Keyword: "#cf222e", Function: "#8250df", Type: "#953800", String: "#0a3069", Number: "#0550ae", Property: "#0550ae",
		}),

		// Night Owl
		build("night-owl", "Night Owl", "", true, Palette{
			Bg: "#011627", Alt: "#01111d", Sel: "#0b2942", Fg: "#d6deeb", Muted: "#637777", Line: "#1d3b53",
			Red: "#ef5350", Orange: "#f78c6c", Yellow: "#ecc48d", Green: "#addb67", Cyan: "#7fdbca", Blue: "#82aaff", Purple: "#c792ea",
			Type: "#ffcb8b", String: "#ecc48d", Operator: "#c792ea",
		}),
		build("light-owl", "Light Owl", "", false, Palette{
			Bg: "#fbfbfb", Alt: "#f0f0f0", Sel: "#e0e7ea", Fg: "#403f53", Muted: "#6a7088", Line: "#d9d9d9",
			Red: "#d3423e", Orange: "#c96765", Yellow: "#b6881a", Green: "#08916a", Cyan: "#0c969b", Blue: "#4876d6", Purple: "#994cc3",
			String: "#c96765", Number: "#aa0982",
		}),

		// Kanagawa
		build("kanagawa-wave", "Kanagawa", "Wave", true, Palette{
			Bg: "#1f1f28", Alt: "#16161d", Sel: "#2a2a37", Fg: "#dcd7ba", Muted: "#8a8980", Line: "#54546d",
			Red: "#e46876", Orange: "#ffa066", Yellow: "#e6c384", Green: "#98bb6c", Cyan: "#7aa89f", Blue: "#7e9cd8", Purple: "#957fb8",
			Type: "#7aa89f", Number: "#d27e99", Operator: "#c0a36e",
		}),
		build("kanagawa-dragon", "Kanagawa", "Dragon", true, Palette{
			Bg: "#181616", Alt: "#0d0c0c", Sel: "#282727", Fg: "#c5c9c5", Muted: "#8a8980", Line: "#393836",
			Red: "#c4746e", Orange: "#b6927b", Yellow: "#c4b28a", Green: "#8a9a7b", Cyan: "#8ea4a2", Blue: "#8ba4b0", Purple: "#a292a3",
		}),
		build("kanagawa-lotus", "Kanagawa", "Lotus", false, Palette{
			Bg: "#f2ecbc", Alt: "#e7dba0", Sel: "#e4d794", Fg: "#545464", Muted: "#6d6c63", Line: "#d5cea3",
			Red: "#c84053", Orange: "#cc6d00", Yellow: "#77713f", Green: "#6f894e", Cyan: "#597b75", Blue: "#4d699b", Purple: "#624c83",
		}),

		// Everforest
		build("everforest-dark", "Everforest", "Dark", true, Palette{
			Bg: "#2d353b", Alt: "#232a2e", Sel: "#3d484d", Fg: "#d3c6aa", Muted: "#859289", Line: "#475258",
			Red: "#e67e80", Orange: "#e69875", Yellow: "#dbbc7f", Green: "#a7c080", Cyan: "#83c092", Blue: "#7fbbb3", Purple: "#d699b6",
			Keyword: "#e67e80", Function: "#a7c080",
		}),
		build("everforest-light", "Everforest", "Light", false, Palette{
			Bg: "#fdf6e3", Alt: "#f4f0d9", Sel: "#efebd4", Fg: "#5c6a72", Muted: "#7b8883", Line: "#e0dcc7",
			Red: "#f85552", Orange: "#f57d26", Yellow: "#b07b00", Green: "#6f8200", Cyan: "#35a77c", Blue: "#3a94c5", Purple: "#df69ba",
			Keyword: "#f85552", Function: "#8da101",
		}),

		// Ayu
		build("ayu-dark", "Ayu", "Dark", true, Palette{
			Bg: "#0b0e14", Alt: "#0f131a", Sel: "#273747", Fg: "#bfbdb6", Muted: "#6c7886", Line: "#1c212a",
			Red: "#f07178", Orange: "#ff8f40", Yellow: "#e6b450", Green: "#aad94c", Cyan: "#95e6cb", Blue: "#59c2ff", Purple: "#d2a6ff",
			Accent: "#e6b450", Keyword: "#ff8f40", Function: "#ffb454", Type: "#59c2ff", Number: "#d2a6ff",
		}),
		build("ayu-mirage", "Ayu", "Mirage", true, Palette{
			Bg: "#1f2430", Alt: "#1a1f29", Sel: "#33415e", Fg: "#cccac2", Muted: "#7b8598", Line: "#2a3141",
			Red: "#f28779", Orange: "#ffa759", Yellow: "#ffcc66", Green: "#d5ff80", Cyan: "#95e6cb", Blue: "#73d0ff", Purple: "#dfbfff",
			Accent: "#ffcc66", Keyword: "#ffa759", Function: "#ffd173", Type: "#73d0ff", Number: "#dfbfff",
		}),
		build("ayu-light", "Ayu", "Light", false, Palette{
			Bg: "#fcfcfc", Alt: "#f3f4f5", Sel: "#e7eaed", Fg: "#5c6166", Muted: "#757c85", Line: "#e6e8ea",
			Red: "#e65050", Orange: "#d9661a", Yellow: "#a8730a", Green: "#6c8f00", Cyan: "#2f9c7b", Blue: "#2a85c4", Purple: "#8a5db3",
			Accent: "#d9661a", Keyword: "#d9661a", Function: "#a8730a", Type: "#2a85c4", Number: "#8a5db3",
		}),

		// Material
		build("material-palenight", "Material", "Palenight", true, Palette{
			Bg: "#292d3e", Alt: "#202331", Sel: "#32374d", Fg: "#a6accd", Muted: "#7982b4", Line: "#3a3f58",
			Red: "#f07178", Orange: "#f78c6c", Yellow: "#ffcb6b", Green: "#c3e88d", Cyan: "#89ddff", Blue: "#82aaff", Purple: "#c792ea",
			Accent: "#c792ea",
		}),
		build("material-ocean", "Material", "Ocean", true, Palette{
			Bg: "#0f111a", Alt: "#090b10", Sel: "#1f2233", Fg: "#a6accd", Muted: "#6c7596", Line: "#232635",
			Red: "#f07178", Orange: "#f78c6c", Yellow: "#ffcb6b", Green: "#c3e88d", Cyan: "#89ddff", Blue: "#82aaff", Purple: "#c792ea",
			Accent: "#82aaff",
		}),

		// Editor classics
		build("nightfox", "Nightfox", "", true, Palette{
			Bg: "#192330", Alt: "#131a24", Sel: "#2b3b51", Fg: "#cdcecf", Muted: "#738091", Line: "#39506d",
			Red: "#c94f6d", Orange: "#f4a261", Yellow: "#dbc074", Green: "#81b29a", Cyan: "#63cdcf", Blue: "#719cd6", Purple: "#9d79d6",
			Type: "#f4a261",
		}),
		build("darcula", "Darcula", "", true, Palette{
			Bg: "#2b2b2b", Alt: "#313335", Sel: "#214283", Fg: "#a9b7c6", Muted: "#808080", Line: "#3c3f41",
			Red: "#ff6b68", Orange: "#cc7832", Yellow: "#ffc66d", Green: "#6a8759", Cyan: "#20999d", Blue: "#6897bb", Purple: "#9876aa",
			Keyword: "#cc7832", Function: "#ffc66d", Type: "#a9b7c6", Number: "#6897bb", Property: "#9876aa",
		}),
		build("vscode-dark-plus", "VS Code", "Dark+", true, Palette{
			Bg: "#1e1e1e", Alt: "#252526", Sel: "#264f78", Fg: "#d4d4d4", Muted: "#6a9955", Line: "#3c3c3c",
			Red: "#f44747", Orange: "#ce9178", Yellow: "#dcdcaa", Green: "#6a9955", Cyan: "#9cdcfe", Blue: "#569cd6", Purple: "#c586c0",
			Keyword: "#569cd6", Function: "#dcdcaa", Type: "#4ec9b0", String: "#ce9178", Number: "#b5cea8",
		}),
		build("vscode-light-plus", "VS Code", "Light+", false, Palette{
			Bg: "#ffffff", Alt: "#f3f3f3", Sel: "#add6ff", Fg: "#1f1f1f", Muted: "#4f7f4f", Line: "#e5e5e5",
			Red: "#cd3131", Orange: "#a31515", Yellow: "#795e26", Green: "#008000", Cyan: "#001080", Blue: "#0451a5", Purple: "#af00db",
			Keyword: "#0000ff", Function: "#795e26", Type: "#267f99", String: "#a31515", Number: "#098658",
		}),
		build("tomorrow-night", "Tomorrow Night", "", true, Palette{
			Bg: "#1d1f21", Alt: "#151718", Sel: "#373b41", Fg: "#c5c8c6", Muted: "#8d908e", Line: "#373b41",
			Red: "#cc6666", Orange: "#de935f", Yellow: "#f0c674", Green: "#b5bd68", Cyan: "#8abeb7", Blue: "#81a2be", Purple: "#b294bb",
		}),
		build("zenburn", "Zenburn", "", true, Palette{
			Bg: "#3f3f3f", Alt: "#2b2b2b", Sel: "#4f4f4f", Fg: "#dcdccc", Muted: "#9fbf9f", Line: "#5f5f5f",
			Red: "#cc9393", Orange: "#dfaf8f", Yellow: "#f0dfaf", Green: "#7f9f7f", Cyan: "#93e0e3", Blue: "#8cd0d3", Purple: "#dc8cc3",
			Keyword: "#f0dfaf", Function: "#efef8f", Type: "#93e0e3", String: "#cc9393", Number: "#8cd0d3",
		}),

		// Vivid dark themes
		build("cobalt2", "Cobalt2", "", true, Palette{
			Bg: "#193549", Alt: "#15232d", Sel: "#0d3a58", Fg: "#ffffff", Muted: "#6fa8dc", Line: "#2a4d66",
			Red: "#ff628c", Orange: "#ff9d00", Yellow: "#ffc600", Green: "#3ad900", Cyan: "#80ffbb", Blue: "#0088ff", Purple: "#fb94ff",
			Accent: "#ffc600", Keyword: "#ff9d00", Function: "#ffc600", Type: "#80ffbb", Number: "#ff628c",
		}),
		build("synthwave-84", "Synthwave '84", "", true, Palette{
			Bg: "#262335", Alt: "#241b2f", Sel: "#34294f", Fg: "#f0e6ff", Muted: "#9199cc", Line: "#495495",
			Red: "#fe4450", Orange: "#f97e72", Yellow: "#fede5d", Green: "#72f1b8", Cyan: "#36f9f6", Blue: "#03edf9", Purple: "#b893ce",
			Accent: "#ff7edb", Keyword: "#fede5d", Function: "#36f9f6", Type: "#ff7edb", String: "#ff8b39", Number: "#f97e72",
		}),
		build("horizon", "Horizon", "", true, Palette{
			Bg: "#1c1e26", Alt: "#16161c", Sel: "#2e303e", Fg: "#d5d8da", Muted: "#7f83a8", Line: "#2e303e",
			Red: "#e95678", Orange: "#fab795", Yellow: "#fac29a", Green: "#29d398", Cyan: "#59e1e3", Blue: "#26bbd9", Purple: "#b877db",
			Accent: "#e95678", Function: "#fac29a", String: "#fab795",
		}),
		build("poimandres", "Poimandres", "", true, Palette{
			Bg: "#1b1e28", Alt: "#171922", Sel: "#303340", Fg: "#e4f0fb", Muted: "#767c9d", Line: "#303340",
			Red: "#f07178", Orange: "#ffa07a", Yellow: "#fffac2", Green: "#5de4c7", Cyan: "#89ddff", Blue: "#91b4d5", Purple: "#a6accd",
			Accent: "#5de4c7", Keyword: "#add7ff", Function: "#89ddff", Type: "#fffac2", Number: "#5de4c7",
		}),

		// Classic Zinc: sift's own neutral palettes.
		build("classic-dark", "Classic Zinc", "(Dark)", true, Palette{
			Bg: "#0b0d12", Alt: "#12151c", Sel: "#1c212b", Fg: "#f0f3f8", Muted: "#7a8294", Line: "#2a303c",
			Red: "#f87171", Orange: "#fb923c", Yellow: "#fbbf24", Green: "#86efac", Cyan: "#7dd3fc", Blue: "#93c5fd", Purple: "#c4b5fd",
			Accent: "#d4a359",
		}),
		build("classic-light", "Classic Zinc", "(Light)", false, Palette{
			Bg: "#ffffff", Alt: "#f5f6f8", Sel: "#e9ecf1", Fg: "#111318", Muted: "#5f6779", Line: "#d5d9e0",
			Red: "#c53030", Orange: "#b45309", Yellow: "#8f641b", Green: "#257035", Cyan: "#0e7490", Blue: "#1d4ed8", Purple: "#6d28d9",
			Accent: "#b58532",
		}),
	}
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
