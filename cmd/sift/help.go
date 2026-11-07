package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var showAllFlags bool

func init() {
	rootCmd.PersistentFlags().BoolVar(&showAllFlags, "all", false, "Show all available flags and options in help")
	setupCustomHelp(rootCmd)
}

type helpTheme struct {
	Brand       lipgloss.Style
	BrandTag    lipgloss.Style
	Section     lipgloss.Style
	Command     lipgloss.Style
	Flag        lipgloss.Style
	FlagArg     lipgloss.Style
	Desc        lipgloss.Style
	DefaultVal  lipgloss.Style
	ExampleCmd  lipgloss.Style
	ExampleDesc lipgloss.Style
	Dim         lipgloss.Style
}

func getHelpTheme(w io.Writer) helpTheme {
	useColor := isTerminalWriter(w) && os.Getenv("NO_COLOR") == "" && !cfg.NoColor
	if !useColor {
		return helpTheme{}
	}

	return helpTheme{
		Brand:       lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#89b4fa")),
		BrandTag:    lipgloss.NewStyle().Foreground(lipgloss.Color("#a6adc8")),
		Section:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f9e2af")),
		Command:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#89dceb")),
		Flag:        lipgloss.NewStyle().Foreground(lipgloss.Color("#89dceb")),
		FlagArg:     lipgloss.NewStyle().Foreground(lipgloss.Color("#fab387")),
		Desc:        lipgloss.NewStyle().Foreground(lipgloss.Color("#cdd6f4")),
		DefaultVal:  lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086")).Italic(true),
		ExampleCmd:  lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1")),
		ExampleDesc: lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086")),
		Dim:         lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086")),
	}
}

func isTerminalWriter(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
	}
	return false
}

func setupCustomHelp(cmd *cobra.Command) {
	cmd.SetHelpFunc(renderHelp)
	cmd.SetUsageFunc(renderUsage)
}

func renderUsage(cmd *cobra.Command) error {
	w := cmd.OutOrStderr()
	t := getHelpTheme(w)

	fmt.Fprintf(w, "%s\n    %s\n\n", t.Section.Render("USAGE:"), commandUsageLine(cmd, t))
	fmt.Fprintf(w, "Run '%s' for usage and available options.\n", t.Command.Render(cmd.CommandPath()+" --help"))
	return nil
}

func renderHelp(cmd *cobra.Command, args []string) {
	w := cmd.OutOrStdout()
	t := getHelpTheme(w)

	isRoot := cmd == rootCmd || cmd.Parent() == nil

	// 1. Title / Brand Header
	versionStr := cfg.Version
	if versionStr == "" {
		versionStr = "dev"
	}
	if isRoot {
		fmt.Fprintf(w, "%s %s\n\n",
			t.Brand.Render("Sift CLI"),
			t.BrandTag.Render(fmt.Sprintf("- LLM Context Packer & Codebase Explorer (v%s)", versionStr)),
		)
	} else {
		fmt.Fprintf(w, "%s %s\n\n",
			t.Brand.Render("sift "+cmd.Name()),
			t.BrandTag.Render("- "+cmd.Short),
		)
		if cmd.Long != "" && cmd.Long != cmd.Short {
			fmt.Fprintf(w, "%s\n\n", t.Desc.Render(strings.TrimSpace(cmd.Long)))
		}
	}

	// 2. USAGE
	fmt.Fprintln(w, t.Section.Render("USAGE:"))
	if isRoot {
		fmt.Fprintf(w, "    %s\n", t.Desc.Render("sift [COMMAND] [OPTIONS]"))
		fmt.Fprintf(w, "    %s\n\n", t.Desc.Render("sift [PATH] [OPTIONS]"))
	} else {
		fmt.Fprintf(w, "    %s\n\n", t.Desc.Render(commandUsageLine(cmd, t)))
	}

	// 3. COMMANDS (for root or commands with subcommands)
	visibleCmds := getVisibleCommands(cmd)
	if len(visibleCmds) > 0 {
		fmt.Fprintln(w, t.Section.Render("COMMANDS:"))
		renderCommands(w, visibleCmds, t)
		fmt.Fprintln(w)
	}

	// 4. OPTIONS
	allRequested := showAllFlags || hasFlag(args, "--all")
	renderOptions(w, cmd, isRoot, allRequested, t)

	// 5. EXAMPLES
	examples := getCommandExamples(cmd)
	if len(examples) > 0 {
		fmt.Fprintln(w, t.Section.Render("EXAMPLES:"))
		renderExamples(w, examples, t)
		fmt.Fprintln(w)
	}

	// 6. Footer Note
	if isRoot {
		fmt.Fprintln(w, t.Desc.Render("For more information about a specific command, use:"))
		fmt.Fprintf(w, "    %s\n", t.Command.Render("sift <COMMAND> --help"))
	} else {
		fmt.Fprintln(w, t.Desc.Render("For general help and all commands, use:"))
		fmt.Fprintf(w, "    %s\n", t.Command.Render("sift --help"))
	}
}

func commandUsageLine(cmd *cobra.Command, t helpTheme) string {
	if cmd.UseLine() != "" {
		return cmd.UseLine()
	}
	return cmd.CommandPath() + " [OPTIONS]"
}

type commandItem struct {
	Name  string
	Short string
}

func getVisibleCommands(parent *cobra.Command) []commandItem {
	var items []commandItem
	for _, c := range parent.Commands() {
		if c.Hidden || c.Name() == "help" {
			continue
		}
		items = append(items, commandItem{
			Name:  c.Name(),
			Short: c.Short,
		})
	}

	// Sort commands so core ones are prominently ordered
	order := map[string]int{
		"dump":       1,
		"pick":       2,
		"select":     3,
		"diff":       4,
		"delta":      5,
		"follow":     6,
		"clone":      7,
		"copy":       8,
		"watch":      9,
		"serve":      10,
		"mcp":        11,
		"desktop":    12,
		"version":    13,
		"completion": 14,
	}

	sort.SliceStable(items, func(i, j int) bool {
		oi, oki := order[items[i].Name]
		oj, okj := order[items[j].Name]
		if oki && okj {
			return oi < oj
		}
		if oki {
			return true
		}
		if okj {
			return false
		}
		return items[i].Name < items[j].Name
	})

	return items
}

func renderCommands(w io.Writer, cmds []commandItem, t helpTheme) {
	maxName := 0
	for _, c := range cmds {
		if len(c.Name) > maxName {
			maxName = len(c.Name)
		}
	}

	for _, c := range cmds {
		pad := strings.Repeat(" ", maxName-len(c.Name)+4)
		fmt.Fprintf(w, "    %s%s%s\n",
			t.Command.Render(c.Name),
			pad,
			t.Desc.Render(c.Short),
		)
	}
}

type flagItem struct {
	Flag      *pflag.Flag
	LeftCol   string
	RightDesc string
	Category  string
}

func renderOptions(w io.Writer, cmd *cobra.Command, isRoot bool, showAll bool, t helpTheme) {
	fs := cmd.Flags()
	if fs == nil {
		return
	}

	var allItems []flagItem
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		allItems = append(allItems, buildFlagItem(f))
	})

	if len(allItems) == 0 {
		return
	}

	if !showAll {
		// Focused friendly options list
		renderFocusedOptions(w, cmd, allItems, isRoot, t)
	} else {
		// Full categorized options list
		renderCategorizedOptions(w, cmd, allItems, t)
	}
}

func buildFlagItem(f *pflag.Flag) flagItem {
	left := ""
	if f.Shorthand != "" {
		left = fmt.Sprintf("-%s, --%s", f.Shorthand, f.Name)
	} else {
		left = fmt.Sprintf("    --%s", f.Name)
	}

	argType := flagArgPlaceholder(f)
	if argType != "" {
		left += " " + argType
	}

	desc := f.Usage
	if f.Name == "help" {
		desc = "Show help information"
	}
	def := f.DefValue
	// Clean up default string for display
	if !strings.Contains(desc, "default") {
		if def != "" && def != "false" && def != "0" && def != `""` && def != "[]" {
			if f.Value.Type() == "string" && !strings.HasPrefix(def, `"`) {
				desc += fmt.Sprintf(" (default %q)", def)
			} else {
				desc += fmt.Sprintf(" (default %s)", def)
			}
		} else if def == "true" {
			desc += " (default true)"
		}
	}

	cat := getFlagCategory(f.Name)

	return flagItem{
		Flag:      f,
		LeftCol:   left,
		RightDesc: desc,
		Category:  cat,
	}
}

func flagArgPlaceholder(f *pflag.Flag) string {
	if f.Value.Type() == "bool" {
		return ""
	}
	switch f.Name {
	case "format", "style":
		return "<style>"
	case "output":
		return "<file>"
	case "budget":
		return "<tokens>"
	case "mode":
		return "<mode>"
	case "prompt":
		return "<text>"
	case "ext":
		return "<exts>"
	case "ignore":
		return "<patterns>"
	case "dir":
		return "<path>"
	case "theme":
		return "<name>"
	case "timeout", "idle-timeout":
		return "<duration>"
	case "workers", "max-size", "max-depth", "smart-max-tokens", "highlight-max-bytes",
		"depth", "full-depth", "follow-depth", "follow-full-depth":
		return "<int>"
	case "log-level":
		return "<level>"
	case "target", "profile":
		return "<name>"
	case "selection-format":
		return "<format>"
	case "follow":
		return "<file>"
	case "branch":
		return "<branch>"
	case "since":
		return "<ref>"
	case "direction", "follow-direction":
		return "<direction>"
	case "listen":
		return "<addr>"
	case "root", "deny":
		return "<path>"
	case "password-file", "tls-cert", "tls-key":
		return "<file>"
	case "allowed-host":
		return "<host>"
	case "port":
		return "<port>"
	case "host":
		return "<host>"
	default:
		return "<" + f.Value.Type() + ">"
	}
}

var primaryFlags = map[string]bool{
	"format":    true,
	"output":    true,
	"budget":    true,
	"mode":      true,
	"prompt":    true,
	"clipboard": true,
	"ext":       true,
	"ignore":    true,
	"smart":     true,
	"theme":     true,
	"verbose":   true,
	"quiet":     true,
	"no-color":  true,
	"help":      true,
}

var primaryFlagOrder = map[string]int{
	"format":    1,
	"style":     2,
	"output":    3,
	"budget":    4,
	"mode":      5,
	"prompt":    6,
	"clipboard": 7,
	"ext":       8,
	"ignore":    9,
	"smart":     10,
	"theme":     11,
	"verbose":   12,
	"quiet":     13,
	"no-color":  14,
	"help":      15,
}

var cmdSpecificPriority = map[string]int{
	// serve
	"app": 1, "open": 2, "keep-alive": 3, "listen": 4, "root": 5, "allow-remote": 6, "password-file": 7,
	// delta
	"patch": 1, "since": 2,
	// clone & follow
	"branch": 1, "depth": 2, "here": 3, "direction": 1, "full-depth": 3,
	// select
	"print-selection": 1, "selection-only": 2, "selection-format": 3, "include-skipped": 4, "follow": 5,
}

func renderFocusedOptions(w io.Writer, cmd *cobra.Command, allItems []flagItem, isRoot bool, t helpTheme) {
	// For subcommands: separate local/custom flags from shared common flags
	localFlags := cmd.LocalFlags()
	var cmdSpecific []flagItem
	var commonItems []flagItem

	for _, item := range allItems {
		isLocal := localFlags != nil && localFlags.Lookup(item.Flag.Name) != nil && isCommandSpecificFlag(item.Flag.Name)
		if isLocal {
			cmdSpecific = append(cmdSpecific, item)
		} else if primaryFlags[item.Flag.Name] {
			commonItems = append(commonItems, item)
		}
	}

	sort.SliceStable(cmdSpecific, func(i, j int) bool {
		pi := cmdSpecificPriority[cmdSpecific[i].Flag.Name]
		pj := cmdSpecificPriority[cmdSpecific[j].Flag.Name]
		if pi > 0 && pj > 0 {
			return pi < pj
		}
		if pi > 0 {
			return true
		}
		if pj > 0 {
			return false
		}
		return cmdSpecific[i].Flag.Name < cmdSpecific[j].Flag.Name
	})

	sort.SliceStable(commonItems, func(i, j int) bool {
		oi := primaryFlagOrder[commonItems[i].Flag.Name]
		oj := primaryFlagOrder[commonItems[j].Flag.Name]
		if oi == 0 {
			oi = 99
		}
		if oj == 0 {
			oj = 99
		}
		return oi < oj
	})

	if len(cmdSpecific) > 0 {
		fmt.Fprintln(w, t.Section.Render("COMMAND OPTIONS:"))
		printFlagList(w, cmdSpecific, t)
		fmt.Fprintln(w)
		fmt.Fprintln(w, t.Section.Render("OPTIONS:"))
	} else {
		fmt.Fprintln(w, t.Section.Render("OPTIONS:"))
	}

	// Always add the --all flag hint
	allItem := flagItem{
		LeftCol:   "    --all",
		RightDesc: "Show all available flags and options",
	}
	displayCommon := append(commonItems, allItem)
	printFlagList(w, displayCommon, t)
	fmt.Fprintln(w)
}

var engineFlags = map[string]bool{
	"dir":                 true,
	"verbose":             true,
	"quiet":               true,
	"log-level":           true,
	"concurrent":          true,
	"workers":             true,
	"max-size":            true,
	"hidden":              true,
	"git":                 true,
	"ignore":              true,
	"ext":                 true,
	"binary":              true,
	"smart":               true,
	"smart-max-tokens":    true,
	"no-color":            true,
	"highlight":           true,
	"no-highlight":        true,
	"theme":               true,
	"ui-theme-file":       true,
	"highlight-max-bytes": true,
	"window-title":        true,
	"no-window-title":     true,
	"output":              true,
	"progress":            true,
	"timeout":             true,
	"show-skipped":        true,
	"version":             true,
	"json":                true,
	"markdown":            true,
	"style":               true,
	"format":              true,
	"no-nerd-fonts":       true,
	"profile":             true,
	"target":              true,
	"budget":              true,
	"tokenize-model":      true,
	"mode":                true,
	"max-depth":           true,
	"prompt":              true,
	"secrets":             true,
	"force-secrets":       true,
	"clipboard":           true,
	"copy-on-generate":    true,
	"all":                 true,
	"help":                true,
}

func isCommandSpecificFlag(name string) bool {
	return !engineFlags[name]
}

func renderCategorizedOptions(w io.Writer, cmd *cobra.Command, allItems []flagItem, t helpTheme) {
	categories := []struct {
		ID    string
		Title string
	}{
		{"cmd", "COMMAND OPTIONS"},
		{"output", "OUTPUT & FORMATTING"},
		{"filter", "FILTERING & SCOPE"},
		{"config", "EXECUTION & PERFORMANCE"},
		{"ui", "UI & DISPLAY"},
		{"general", "LOGGING & GENERAL"},
	}

	byCat := make(map[string][]flagItem)
	for _, item := range allItems {
		catID := item.Category
		if isCommandSpecificFlag(item.Flag.Name) {
			catID = "cmd"
		}
		byCat[catID] = append(byCat[catID], item)
	}

	for _, cat := range categories {
		items := byCat[cat.ID]
		if len(items) == 0 {
			continue
		}
		fmt.Fprintln(w, t.Section.Render(cat.Title+":"))
		printFlagList(w, items, t)
		fmt.Fprintln(w)
	}
}

func getFlagCategory(name string) string {
	switch name {
	case "output", "style", "format", "budget", "mode", "prompt", "clipboard",
		"copy-on-generate", "secrets", "force-secrets", "json", "markdown":
		return "output"
	case "ext", "ignore", "smart", "smart-max-tokens", "hidden", "git", "binary", "max-size", "max-depth":
		return "filter"
	case "dir", "profile", "target", "timeout", "workers", "concurrent", "tokenize-model":
		return "config"
	case "theme", "ui-theme-file", "highlight", "no-highlight", "highlight-max-bytes",
		"no-nerd-fonts", "window-title", "no-window-title", "progress", "show-skipped":
		return "ui"
	default:
		return "general"
	}
}

func printFlagList(w io.Writer, items []flagItem, t helpTheme) {
	maxLeft := 0
	for _, it := range items {
		if len(it.LeftCol) > maxLeft {
			maxLeft = len(it.LeftCol)
		}
	}
	if maxLeft < 14 {
		maxLeft = 14
	}

	for _, it := range items {
		pad := strings.Repeat(" ", maxLeft-len(it.LeftCol)+3)

		// Format flag name and arg with separate styling
		styledLeft := formatFlagLeft(it.LeftCol, t)
		styledDesc := formatFlagDesc(it.RightDesc, t)

		fmt.Fprintf(w, "    %s%s%s\n", styledLeft, pad, styledDesc)
	}
}

func formatFlagLeft(left string, t helpTheme) string {
	parts := strings.SplitN(left, " <", 2)
	if len(parts) == 2 {
		return t.Flag.Render(parts[0]) + " " + t.FlagArg.Render("<"+parts[1])
	}
	return t.Flag.Render(left)
}

func formatFlagDesc(desc string, t helpTheme) string {
	idx := strings.Index(desc, " (default")
	if idx != -1 {
		mainPart := desc[:idx]
		defPart := desc[idx:]
		return t.Desc.Render(mainPart) + t.DefaultVal.Render(defPart)
	}
	return t.Desc.Render(desc)
}

type exampleItem struct {
	Command string
	Desc    string
}

func getCommandExamples(cmd *cobra.Command) []exampleItem {
	switch cmd.Name() {
	case "sift":
		return []exampleItem{
			{"sift", "Launch the interactive TUI picker (default)"},
			{"sift dump", "Dump current directory to codebase.md"},
			{"sift dump -f xml --budget 100k", "Dump as XML within a 100k token budget"},
			{"sift dump src/ --ext go,ts", "Dump only Go and TypeScript files in src/"},
			{"sift dump --clipboard", "Copy dump directly to system clipboard"},
			{"sift select --budget 50k", "Auto-select most relevant files using git signals"},
			{"sift diff HEAD~1", "Render files modified in the last commit"},
			{"sift delta --patch", "Dump incremental unified diff since last dump"},
			{"sift follow cmd/sift/main.go", "Render file plus its import-graph dependencies"},
			{"sift serve --app", "Launch browser UI in a dedicated chromeless window"},
			{"sift copy", "Copy the last generated document to clipboard"},
		}
	case "dump":
		return []exampleItem{
			{"sift dump", "Dump current directory to codebase.md"},
			{"sift dump ./src -f xml", "Dump ./src as XML to codebase.xml"},
			{"sift dump -f xml --budget 120k", "Fit dump within a 120,000 token budget"},
			{"sift dump --mode signatures", "Extract signatures and drop function bodies"},
			{"sift dump --ext go,ts,py", "Only scan Go, TypeScript, and Python files"},
			{"sift dump -p \"Review security\"", "Prepend task instructions to output"},
			{"sift dump --clipboard", "Copy output directly to clipboard"},
			{"sift dump -o -", "Write document directly to stdout"},
			{"sift dump --smart", "Skip minified, lockfile, and oversized files"},
		}
	case "pick":
		return []exampleItem{
			{"sift pick", "Open interactive file picker TUI"},
			{"sift pick ./src", "Open picker rooted at ./src"},
			{"sift pick --budget 64k", "Open picker with 64,000 token budget meter"},
			{"sift pick --theme dark", "Open picker with dark syntax highlighting theme"},
			{"sift pick -f xml", "Pick files and render output as XML"},
			{"sift pick --no-nerd-fonts", "Use plain ASCII glyphs instead of Nerd Fonts"},
		}
	case "select":
		return []exampleItem{
			{"sift select", "Auto-select files using git relevance signals"},
			{"sift select --budget 50k", "Fill context up to 50,000 tokens"},
			{"sift select --print-selection", "Display automatic full/signatures/skip decisions"},
			{"sift select --selection-only", "Print decisions without generating codebase document"},
			{"sift select --selection-format json", "Output selection decisions as JSON"},
			{"sift select --follow main.go", "Restrict candidates to seed file dependencies"},
			{"sift select --ext go,ts --clipboard", "Select Go/TS files and copy directly to clipboard"},
		}
	case "diff":
		return []exampleItem{
			{"sift diff HEAD~1", "Dump files changed in the previous commit"},
			{"sift diff main", "Dump files changed compared to main branch"},
			{"sift diff v1.0.0", "Dump files changed since release tag v1.0.0"},
			{"sift diff HEAD~2 --budget 60k", "Dump recent commit changes within 60k token budget"},
			{"sift diff main -f xml --clipboard", "Dump branch diff as XML directly to clipboard"},
			{"sift diff origin/main --mode signatures", "Extract signatures for modified files"},
		}
	case "delta":
		return []exampleItem{
			{"sift delta", "Dump files modified since last sift dump"},
			{"sift delta --patch", "Output a raw unified diff patch instead of full file contents"},
			{"sift delta --patch --clipboard", "Copy delta patch directly to system clipboard"},
			{"sift delta --since HEAD~2", "Compare against git revision instead of last dump"},
			{"sift delta --since main -f xml", "Compare against main branch and format as XML"},
			{"sift delta -o patch.diff", "Save delta changes to a specific file"},
			{"sift delta --budget 40k", "Fit delta changes within a 40k token budget"},
		}
	case "follow":
		return []exampleItem{
			{"sift follow cmd/sift/main.go", "Render seed file plus its direct import dependencies"},
			{"sift follow main.go --direction deps", "Only follow imported dependencies"},
			{"sift follow main.go --direction dependents", "Only follow files that import this file"},
			{"sift follow main.go --direction both", "Follow both dependencies and dependents"},
			{"sift follow main.go --depth 3", "Traverse imports up to 3 hops away"},
			{"sift follow main.go --full-depth 1", "Render 1 hop in full and deeper hops as signatures"},
			{"sift follow main.go -f xml --budget 80k", "Dump import graph as XML within 80k token budget"},
		}
	case "clone":
		return []exampleItem{
			{"sift clone github.com/user/repo", "Shallow-clone repository and dump its context"},
			{"sift clone https://github.com/... -b v1.2", "Clone specific branch or tag"},
			{"sift clone github.com/user/repo --depth 5", "Clone with git history depth of 5 commits"},
			{"sift clone github.com/user/repo --here", "Clone into current directory instead of temporary folder"},
			{"sift clone github.com/user/repo -f xml", "Clone and output context directly as XML"},
			{"sift clone github.com/user/repo --budget 100k", "Clone and pack repository within 100k tokens"},
		}
	case "copy":
		return []exampleItem{
			{"sift copy", "Copy last generated context document to clipboard"},
			{"sift copy codebase.xml", "Copy specific file to clipboard"},
			{"sift copy custom/context.txt", "Copy context from custom path"},
		}
	case "serve":
		return []exampleItem{
			{"sift serve", "Start browser file picker UI on localhost:5173"},
			{"sift serve --app", "Open in chromeless app window and stop server on close"},
			{"sift serve --open", "Start server and automatically open default browser"},
			{"sift serve --listen :8080", "Bind server to port 8080 on loopback"},
			{"sift serve --listen 0.0.0.0:8080 --allow-remote", "Bind to all interfaces for remote access"},
			{"sift serve --password-file ./pw --tls-self-signed", "Secure remote instance with TLS and password"},
			{"sift serve --root /path/to/project", "Restrict allowed filesystem roots"},
			{"sift serve --app --keep-alive", "Keep server running even after app window closes"},
		}
	case "watch":
		return []exampleItem{
			{"sift watch", "Re-render context document on file changes"},
			{"sift watch ./src -f xml", "Watch ./src and update codebase.xml"},
			{"sift watch --clipboard", "Copy updated context to clipboard on every change"},
			{"sift watch --ext go,ts", "Only monitor and update Go and TypeScript files"},
			{"sift watch -o -", "Stream updated context directly to stdout"},
		}
	case "mcp":
		return []exampleItem{
			{"sift mcp", "Start Model Context Protocol server over stdio for LLM agents"},
			{"sift mcp ./src", "Start MCP server scoped to ./src directory"},
			{"sift mcp --budget 120k", "Start MCP server with default 120k token limit"},
		}
	case "desktop":
		return []exampleItem{
			{"sift desktop install", "Install desktop launcher and application icon"},
			{"sift desktop uninstall", "Remove desktop launcher"},
			{"sift desktop status", "Check whether desktop launcher is installed"},
		}
	default:
		return nil
	}
}

func renderExamples(w io.Writer, examples []exampleItem, t helpTheme) {
	maxCmd := 0
	for _, ex := range examples {
		if len(ex.Command) > maxCmd {
			maxCmd = len(ex.Command)
		}
	}

	for _, ex := range examples {
		pad := strings.Repeat(" ", maxCmd-len(ex.Command)+4)
		fmt.Fprintf(w, "    %s%s%s\n",
			t.ExampleCmd.Render(ex.Command),
			pad,
			t.ExampleDesc.Render("# "+ex.Desc),
		)
	}
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag || a == "--all=true" {
			return true
		}
	}
	return false
}
