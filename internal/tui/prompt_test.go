package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPromptPresetReachesGenerateCallback(t *testing.T) {
	var got string
	m := newModel(BuildTree([]Item{{Path: "main.go", Content: []byte("package main"), TokensFull: 2}}), Options{
		OnGeneratePrompt: func(_ []Selection, prompt string) error {
			got = prompt
			return nil
		},
	})
	m.root.setSelected(true)
	m = updateKey(m, tea.KeyRunes, 'p')
	if !m.promptOpen {
		t.Fatal("p did not open prompt modal")
	}
	m = updateKey(m, tea.KeyDown)
	m = updateKey(m, tea.KeyEnter)
	if m.promptOpen || m.prompt != promptPresets[1].Text {
		t.Fatalf("prompt after preset selection = %q, open=%v", m.prompt, m.promptOpen)
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = updated.(model)
	m, _ = runGenJob(t, m, cmd)
	if got != promptPresets[1].Text {
		t.Errorf("callback prompt = %q, want %q", got, promptPresets[1].Text)
	}
}

func TestPromptCustomDirectiveReachesCopyCallback(t *testing.T) {
	var got string
	m := newModel(BuildTree([]Item{{Path: "main.go", Content: []byte("package main"), TokensFull: 2}}), Options{
		OnCopyPrompt: func(_ []Selection, prompt string) error {
			got = prompt
			return nil
		},
	})
	m.root.setSelected(true)
	m = updateKey(m, tea.KeyRunes, 'p')
	m = updateKey(m, tea.KeyRunes, 'c')
	m = updateKey(m, tea.KeyRunes, 'c')
	for _, r := range "heck tests" {
		m = updateKey(m, tea.KeyRunes, r)
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(model)
	if m.promptOpen {
		t.Error("y did not close prompt modal")
	}
	m, _ = runGenJob(t, m, cmd)
	if got != "check tests" {
		t.Errorf("custom callback prompt = %q, want %q", got, "check tests")
	}
}
