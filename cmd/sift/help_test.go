package main

import (
	"bytes"
	"strings"
	"testing"
)

// helpFor renders help for the named subcommand ("" for the root) through the
// same custom help function the CLI installs.
func helpFor(t *testing.T, name string, all bool) string {
	t.Helper()
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)

	target := root
	if name != "" {
		found, _, err := root.Find([]string{name})
		if err != nil || found == nil {
			t.Fatalf("subcommand %q not found: %v", name, err)
		}
		target = found
		target.SetOut(&buf)
		target.SetErr(&buf)
	}
	if all {
		if err := root.PersistentFlags().Set("all", "true"); err != nil {
			t.Fatal(err)
		}
	}
	if err := target.Help(); err != nil {
		t.Fatalf("help: %v", err)
	}
	return buf.String()
}

func TestHelpOutputContainsSections(t *testing.T) {
	out := helpFor(t, "", false)

	requiredSections := []string{
		"USAGE:",
		"COMMANDS:",
		"OPTIONS:",
		"EXAMPLES:",
		"sift <COMMAND> --help",
	}

	for _, sec := range requiredSections {
		if !strings.Contains(out, sec) {
			t.Errorf("root help output missing section %q:\n%s", sec, out)
		}
	}

	// Verify key commands appear
	for _, cmdName := range []string{"dump", "pick", "select", "diff", "serve"} {
		if !strings.Contains(out, cmdName) {
			t.Errorf("root help output missing command %q", cmdName)
		}
	}

	// Verify key options appear
	for _, opt := range []string{"--format", "--output", "--budget", "--prompt", "--clipboard", "--help"} {
		if !strings.Contains(out, opt) {
			t.Errorf("root help output missing option %q", opt)
		}
	}
}

func TestHelpWithAllShowsCategorizedOptions(t *testing.T) {
	out := helpFor(t, "", true)

	categorizedSections := []string{
		"OUTPUT & FORMATTING:",
		"FILTERING & SCOPE:",
		"EXECUTION & PERFORMANCE:",
		"UI & DISPLAY:",
		"LOGGING & GENERAL:",
	}

	for _, sec := range categorizedSections {
		if !strings.Contains(out, sec) {
			t.Errorf("help with --all missing category %q:\n%s", sec, out)
		}
	}
}

func TestSubcommandHelpShowsExamples(t *testing.T) {
	out := helpFor(t, "dump", false)

	if !strings.Contains(out, "EXAMPLES:") {
		t.Fatalf("dump help missing EXAMPLES section:\n%s", out)
	}

	if !strings.Contains(out, "sift dump ./src -f xml") {
		t.Errorf("dump help missing expected example:\n%s", out)
	}
}

func TestSubcommandHelpShowsCommandSpecificOptions(t *testing.T) {
	out := helpFor(t, "select", false)

	if !strings.Contains(out, "COMMAND OPTIONS:") {
		t.Fatalf("select help missing COMMAND OPTIONS section:\n%s", out)
	}

	if !strings.Contains(out, "--print-selection") {
		t.Errorf("select help missing --print-selection option:\n%s", out)
	}
}

func TestServeHelpShowsAppAndCommandOptions(t *testing.T) {
	out := helpFor(t, "serve", false)

	if !strings.Contains(out, "COMMAND OPTIONS:") {
		t.Fatalf("serve help missing COMMAND OPTIONS section:\n%s", out)
	}

	for _, opt := range []string{"--app", "--open", "--listen", "--root", "--allow-remote"} {
		if !strings.Contains(out, opt) {
			t.Errorf("serve help missing option %q:\n%s", opt, out)
		}
	}

	if !strings.Contains(out, "sift serve --app") {
		t.Errorf("serve help missing 'sift serve --app' example:\n%s", out)
	}
}

func TestDeltaHelpShowsPatchAndSinceAndMultipleExamples(t *testing.T) {
	out := helpFor(t, "delta", false)

	if !strings.Contains(out, "COMMAND OPTIONS:") {
		t.Fatalf("delta help missing COMMAND OPTIONS section:\n%s", out)
	}

	for _, opt := range []string{"--patch", "--since"} {
		if !strings.Contains(out, opt) {
			t.Errorf("delta help missing option %q:\n%s", opt, out)
		}
	}

	for _, ex := range []string{"sift delta --patch", "sift delta --since HEAD~2", "sift delta --budget 40k"} {
		if !strings.Contains(out, ex) {
			t.Errorf("delta help missing example %q:\n%s", ex, out)
		}
	}
}

func TestHelpNoAnsiWhenNotTerminal(t *testing.T) {
	out := helpFor(t, "", false)

	// A bytes.Buffer is not a terminal file descriptor, so getHelpTheme should return no ANSI escapes
	if strings.Contains(out, "\x1b[") {
		t.Errorf("help output written to non-terminal buffer contains raw ANSI escape codes")
	}
}

// TestRootCommandsRunConcurrently builds and executes independent command
// trees in parallel. Run with -race: it guards against package-level config or
// flag state leaking across invocations.
func TestRootCommandsRunConcurrently(t *testing.T) {
	for i := 0; i < 6; i++ {
		i := i
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			t.Parallel()
			root := newRootCmd()
			var buf bytes.Buffer
			root.SetOut(&buf)
			root.SetErr(&buf)
			root.SetArgs([]string{"dump", "--help"})
			if err := root.Execute(); err != nil {
				t.Fatalf("execute: %v", err)
			}
			if !strings.Contains(buf.String(), "USAGE:") {
				t.Fatalf("sequential help output mismatch:\n%s", buf.String())
			}
		})
	}
}

// TestIndependentRootsDoNotShareState proves two command trees can be built and
// parsed without leaking flags into each other.
func TestIndependentRootsDoNotShareState(t *testing.T) {
	first := newRootCmd()
	second := newRootCmd()

	dumpFirst, _, err := first.Find([]string{"dump"})
	if err != nil {
		t.Fatal(err)
	}
	dumpSecond, _, err := second.Find([]string{"dump"})
	if err != nil {
		t.Fatal(err)
	}
	if err := dumpFirst.Flags().Parse([]string{"--style", "xml"}); err != nil {
		t.Fatal(err)
	}
	if dumpSecond.Flags().Changed("style") {
		t.Fatal("flag change leaked between independent roots")
	}
}
