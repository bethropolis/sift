//go:build cgo

package scan

import "testing"

// TestContentCacheLanguageSeparation verifies identical bytes under different
// extensions do not share signature results or language metadata.
func TestContentCacheLanguageSeparation(t *testing.T) {
	cache := NewContentCache()
	p := newCacheTestProcessor(t, true, cache)
	src := []byte("package main\n\nfunc F() {}\n")

	goEntry, err := p.Process("main.go", src, ModePicker)
	if err != nil {
		t.Fatal(err)
	}
	if goEntry.SigContent == nil || goEntry.Language != "go" {
		t.Fatalf("go entry not compressed: %+v", goEntry)
	}

	txtEntry, err := p.Process("notes.txt", src, ModePicker)
	if err != nil {
		t.Fatal(err)
	}
	if txtEntry.SigContent != nil {
		t.Errorf("txt entry inherited SigContent: %+v", txtEntry)
	}
	if txtEntry.Language != "" {
		t.Errorf("txt entry inherited language %q", txtEntry.Language)
	}
	if txtEntry.TokensSig != txtEntry.TokensFull {
		t.Errorf("txt entry token counts differ: %d vs %d", txtEntry.TokensSig, txtEntry.TokensFull)
	}
}
