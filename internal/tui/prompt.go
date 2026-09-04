package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// PromptPreset is a reusable task directive available from the prompt modal.
type PromptPreset struct {
	Name string
	Text string
}

var promptPresets = []PromptPreset{
	{Name: "Code Review & Security Audit", Text: "Review this codebase for correctness, security issues, race conditions, and maintainability problems. Identify concrete findings with file paths and recommended fixes."},
	{Name: "Refactor & Clean Code", Text: "Refactor this codebase for clarity, maintainability, and simplicity. Identify code smells and propose focused improvements without changing behavior unnecessarily."},
	{Name: "Bug Investigation / Root Cause Analysis", Text: "Investigate this codebase for the root cause of the reported bug. Trace the relevant execution paths, explain the failure, and propose a targeted fix."},
	{Name: "Generate Unit Tests", Text: "Design and generate a comprehensive unit test plan for this codebase, prioritizing important behavior, edge cases, and regression coverage."},
	{Name: "Architecture Explanation", Text: "Explain the architecture of this codebase, including its major components, data flow, boundaries, and the most important design decisions."},
}

func (m *model) openPrompt() {
	m.promptOpen = true
	m.promptInput = m.prompt
	m.promptCustom = m.prompt != "" && promptPresetIndex(m.prompt) < 0
	m.promptCursor = 0
	if idx := promptPresetIndex(m.prompt); idx >= 0 {
		m.promptCursor = idx
	}
}

func promptPresetIndex(prompt string) int {
	for i, preset := range promptPresets {
		if preset.Text == prompt {
			return i
		}
	}
	return -1
}

func (m *model) commitPrompt() {
	if m.promptCustom {
		m.prompt = strings.TrimSpace(m.promptInput)
		return
	}
	if m.promptCursor >= 0 && m.promptCursor < len(promptPresets) {
		m.prompt = promptPresets[m.promptCursor].Text
	}
}

func (m *model) promptMove(delta int) {
	m.promptCursor += delta
	if m.promptCursor < 0 {
		m.promptCursor = 0
	}
	if m.promptCursor >= len(promptPresets) {
		m.promptCursor = len(promptPresets) - 1
	}
	m.promptCustom = false
}

func (m *model) renderPromptModal(view string, width, height int) string {
	modalWidth := min(width, 76)
	if modalWidth < 36 {
		modalWidth = 36
	}
	bodyRows := max(1, min(len(promptPresets), height-9))
	modalHeight := min(max(10, height-2), bodyRows+8)
	var b strings.Builder

	title := fmt.Sprintf(" %sTask Directive ", m.glyphs.Prompt)
	b.WriteString(m.styles.title.Render(ansi.Truncate(title, modalWidth-4, "…")))
	b.WriteString("\n")
	b.WriteString(m.styles.dim.Render("Pick a preset or press c to write a custom directive."))
	b.WriteString("\n")

	for i := 0; i < bodyRows && i < len(promptPresets); i++ {
		cur := "  "
		if i == m.promptCursor && !m.promptCustom {
			cur = m.styles.title.Render("❯ ")
		}
		line := fmt.Sprintf("%s%d. %s", cur, i+1, promptPresets[i].Name)
		line = ansi.Truncate(line, modalWidth-4, "…")
		if i == m.promptCursor && !m.promptCustom {
			b.WriteString(m.styles.cursor.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	// Custom input row
	var customLine string
	if m.promptCustom {
		customLine = m.styles.title.Render("❯ ") + m.styles.hint.Render("Custom: ") +
			m.promptInput + m.styles.title.Render("▌")
	} else {
		customLine = m.styles.dim.Render("  Custom: ") + m.styles.dim.Render(ansi.Truncate(m.promptInput, modalWidth-14, "…"))
	}
	b.WriteString(ansi.Truncate(customLine, modalWidth-4, "…"))
	b.WriteString("\n")

	// Active prompt preview
	activeText := m.prompt
	if activeText == "" {
		activeText = "(none)"
	}
	preview := ansi.Truncate("Active: "+activeText, modalWidth-4, "…")
	b.WriteString(m.styles.dim.Render(preview))
	b.WriteString("\n")

	b.WriteString(m.styles.hint.Render("Enter apply · c custom · g generate · y copy · Esc close"))
	modal := m.boxStyle(modalWidth).Height(max(1, modalHeight-2)).Render(b.String())
	return m.overlay(view, modal, width, height)
}
