package app

import (
	"fmt"

	"github.com/bethropolis/sift/internal/compress"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/tokenize"
)

// OutlineEntry rewrites an entry to the comment-free outline rendering. It
// returns the entry unchanged when no summarizer applies.
func OutlineEntry(entry format.FileEntry, tokenizeModel string) (format.FileEntry, error) {
	compressor := compress.New()
	var out string
	var ok bool
	if id, has := compressor.LanguageForPath(entry.Path); has {
		out, ok = compressor.CompressOutline(entry.Content, id)
	} else {
		out, ok = compress.SummarizePath(entry.Content, entry.Path)
	}
	if !ok {
		return entry, nil
	}
	content := []byte(out)
	tz, err := tokenize.New(tokenizeModel)
	if err != nil {
		return entry, err
	}
	tokens, err := tz.Count(content)
	if err != nil {
		return entry, err
	}
	entry.Content = content
	entry.Tokens = tokens
	entry.IsCompressed = true
	entry.Language = languageForEntry(entry)
	return entry, nil
}

func languageForEntry(entry format.FileEntry) string {
	if id, ok := compress.New().LanguageForPath(entry.Path); ok {
		return string(id)
	}
	return entry.Language
}

// FocusEntry rewrites an entry to a focused rendering: one declaration in full
// with siblings as signatures, or a marked line slice. Token counts are
// recomputed so budgeting sees the focused size.
func FocusEntry(entry format.FileEntry, target Target, tokenizeModel string) (format.FileEntry, error) {
	if !target.IsFocused() {
		return entry, nil
	}
	content, err := compress.New().Focus(entry.Content, entry.Path, target.Symbol, target.Start, target.End)
	if err != nil {
		return entry, fmt.Errorf("focus %s: %w", entry.Path, err)
	}
	tz, err := tokenize.New(tokenizeModel)
	if err != nil {
		return entry, err
	}
	tokens, err := tz.Count(content)
	if err != nil {
		return entry, fmt.Errorf("focus %s: count tokens: %w", entry.Path, err)
	}
	entry.Content = content
	entry.Tokens = tokens
	entry.TokensFull = tokens
	entry.TokensSig = tokens
	entry.SigContent = nil
	entry.IsCompressed = false
	return entry, nil
}
