package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpOutputContainsSections(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	renderHelp(rootCmd, nil)
	out := buf.String()

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
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	renderHelp(rootCmd, []string{"--all"})
	out := buf.String()

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
	var buf bytes.Buffer
	dumpCmd.SetOut(&buf)
	dumpCmd.SetErr(&buf)

	renderHelp(dumpCmd, nil)
	out := buf.String()

	if !strings.Contains(out, "EXAMPLES:") {
		t.Fatalf("dump help missing EXAMPLES section:\n%s", out)
	}

	if !strings.Contains(out, "sift dump ./src -f xml") {
		t.Errorf("dump help missing expected example:\n%s", out)
	}
}

func TestSubcommandHelpShowsCommandSpecificOptions(t *testing.T) {
	var buf bytes.Buffer
	selectCmd.SetOut(&buf)
	selectCmd.SetErr(&buf)

	renderHelp(selectCmd, nil)
	out := buf.String()

	if !strings.Contains(out, "COMMAND OPTIONS:") {
		t.Fatalf("select help missing COMMAND OPTIONS section:\n%s", out)
	}

	if !strings.Contains(out, "--print-selection") {
		t.Errorf("select help missing --print-selection option:\n%s", out)
	}
}

func TestHelpNoAnsiWhenNotTerminal(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	renderHelp(rootCmd, nil)
	out := buf.String()

	// A bytes.Buffer is not a terminal file descriptor, so getHelpTheme should return no ANSI escapes
	if strings.Contains(out, "\x1b[") {
		t.Errorf("help output written to non-terminal buffer contains raw ANSI escape codes")
	}
}
