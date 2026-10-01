package picker

import (
	"fmt"
	"os"
	"time"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/clipboard"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/tui"
	"github.com/bethropolis/sift/internal/walker"
)

// buildSkeleton maps the metadata walk into the TUI's initial items and a
// token-estimate entry list used by the delta modal. Files carry a byte-based
// token estimate (size/4) shown with a "~" until exact counts stream in.
func buildSkeleton(metas []walker.FileMeta, preferredModes map[string]string) ([]tui.Item, []format.FileEntry) {
	items := make([]tui.Item, 0, len(metas))
	deltaFiles := make([]format.FileEntry, 0, len(metas))
	for _, m := range metas {
		approx := int(m.Size / 4)
		items = append(items, tui.Item{
			Path:          m.Path,
			IsDir:         m.IsDir,
			ApproxTokens:  approx,
			PreferredMode: tui.CompressMode(preferredModes[m.Path]),
			Hidden:        m.Hidden,
			GitIgnored:    m.GitIgnored,
		})
		deltaFiles = append(deltaFiles, format.FileEntry{
			Path:       m.Path,
			TokensFull: approx,
			Tokens:     approx,
		})
	}
	return items, deltaFiles
}

// applySelection maps the TUI's per-file modes onto the collected entries. A
// file in FULL mode keeps its raw content; SIGS uses the signature summary;
// SKIP entries are dropped. Selections whose content has not streamed in yet
// fall back to a fresh read.
func applySelection(application *app.App, files []format.FileEntry, selected []tui.Selection) []format.FileEntry {
	byPath := make(map[string]format.FileEntry, len(files))
	for _, f := range files {
		byPath[f.Path] = f
	}

	chosen := make([]format.FileEntry, 0, len(selected))
	for _, sel := range selected {
		f, ok := byPath[sel.Path]
		if !ok {
			e, err := application.ReadEntry(sel.Path)
			if err != nil {
				application.LogError("Failed to read %s: %v", sel.Path, err)
				continue
			}
			f = e
		}
		switch sel.Mode {
		case tui.ModeSignatures:
			if f.SigContent != nil {
				f.Content = f.SigContent
				f.Tokens = f.TokensSig
				f.IsCompressed = true
			} else {
				f.Tokens = f.TokensFull
			}
		case tui.ModeSkip:
			continue
		default: // ModeFull
			f.Tokens = f.TokensFull
			f.IsCompressed = false
		}
		chosen = append(chosen, f)
	}
	return chosen
}

// copySelection renders the current selection to the system clipboard without
// touching the picker's output destination.
func (s *service) copySelection(files []format.FileEntry, selected []tui.Selection) error {
	return s.copySelectionWithPrompt(files, selected, s.cfg.Prompt)
}

func (s *service) copySelectionWithPrompt(files []format.FileEntry, selected []tui.Selection, prompt string) error {
	chosen := applySelection(s.env.App, files, selected)
	if len(chosen) == 0 {
		return fmt.Errorf("nothing selected")
	}
	return s.env.App.RenderToClipboardWithPrompt(chosen, prompt)
}

// generateSelection renders the current selection to the output document
// without leaving the picker (pressing g). The output file is truncated first
// so repeated generates replace the previous dump instead of appending to it.
func (s *service) generateSelection(files []format.FileEntry, skipped []walker.SkippedItem, start time.Time, selected []tui.Selection) error {
	return s.generateSelectionWithPrompt(files, skipped, start, selected, s.cfg.Prompt)
}

func (s *service) generateSelectionWithPrompt(files []format.FileEntry, skipped []walker.SkippedItem, start time.Time, selected []tui.Selection, prompt string) error {
	chosen := applySelection(s.env.App, files, selected)
	chosen = app.ExpandChosen(files, chosen, s.cfg.RootDir, s.cfg.Budget, s.cfg.MaxDepth)
	if len(chosen) == 0 {
		return fmt.Errorf("nothing selected")
	}

	// Render fully into memory first so a failed render (disk full, bad
	// content, etc.) never destroys the previous dump. Only after the document
	// is complete do we replace the output file's contents.
	rendered, err := s.env.App.RenderFinalToBuffer(chosen, prompt)
	if err != nil {
		return fmt.Errorf("render output: %w", err)
	}

	out := s.env.App.Output()
	if f, ok := out.(*os.File); ok && f != os.Stdout && f != os.Stderr {
		// Truncate after a successful render so repeated generates replace the
		// previous dump instead of appending to it, without risking data loss
		// on failure.
		if err := f.Truncate(0); err != nil {
			return err
		}
		if _, err := f.Seek(0, 0); err != nil {
			return err
		}
	}
	if _, err := out.Write(rendered); err != nil {
		return err
	}
	if s.cfg.CopyOnGenerate {
		if err := clipboard.Copy(rendered); err != nil {
			return fmt.Errorf("copy generated output to clipboard: %w", err)
		}
	}

	// Update the baseline so a later delta dump knows what was just rendered.
	return s.env.RecordDump(s.cfg.RootDir, chosen, "HEAD")
}
