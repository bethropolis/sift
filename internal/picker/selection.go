package picker

import (
	"fmt"
	"sync"
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

	// Files that have not streamed in yet need a disk read plus processing.
	// Do those in a small pool; selected order must still be preserved.
	missing := make([]int, 0, len(selected))
	for i, sel := range selected {
		if _, ok := byPath[sel.Path]; !ok {
			missing = append(missing, i)
		}
	}
	if len(missing) > 0 {
		type readResult struct {
			entry format.FileEntry
			err   error
		}
		results := make([]readResult, len(missing))
		jobs := make(chan int)
		workers := min(4, len(missing))
		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range jobs {
					entry, err := application.ReadEntry(selected[missing[j]].Path)
					results[j] = readResult{entry: entry, err: err}
				}
			}()
		}
		for j := range missing {
			jobs <- j
		}
		close(jobs)
		wg.Wait()
		for j, res := range results {
			if res.err != nil {
				application.LogError("Failed to read %s: %v", selected[missing[j]].Path, res.err)
				continue
			}
			byPath[res.entry.Path] = res.entry
		}
	}

	chosen := make([]format.FileEntry, 0, len(selected))
	for _, sel := range selected {
		f, ok := byPath[sel.Path]
		if !ok {
			continue
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
func (s *service) generateSelection(files []format.FileEntry, graph map[string][]string, skipped []walker.SkippedItem, start time.Time, selected []tui.Selection) error {
	_, err := s.generateSelectionWithPrompt(files, graph, skipped, start, selected, s.cfg.Prompt)
	return err
}

// generateSelectionWithPrompt renders the selection once and returns the
// rendered bytes alongside writing them to the output destination. Callers
// that also need a clipboard copy reuse the returned buffer instead of
// re-reading the file or rendering the whole selection a second time.
//
// When the destination is stdout (`--output -`) the write is skipped: the
// picker owns the terminal through Bubble Tea's alternate screen, so flushing
// a whole document there would corrupt the interface. The bytes are still
// returned so Y can copy them, and `g` reports the destination in its notice.
func (s *service) generateSelectionWithPrompt(files []format.FileEntry, graph map[string][]string, skipped []walker.SkippedItem, start time.Time, selected []tui.Selection, prompt string) ([]byte, error) {
	selectionStart := time.Now()
	chosen := applySelection(s.env.App, files, selected)
	s.env.App.LogDebug("picker selection timing: %s selected=%d", time.Since(selectionStart).Round(time.Millisecond), len(selected))
	expandStart := time.Now()
	chosen = app.ExpandChosenWithGraph(files, chosen, graph, s.cfg.RootDir, s.cfg.Budget, s.cfg.MaxDepth)
	s.env.App.LogDebug("picker expand timing: %s chosen=%d", time.Since(expandStart).Round(time.Millisecond), len(chosen))
	if len(chosen) == 0 {
		return nil, fmt.Errorf("nothing selected")
	}

	// Render fully into memory first so a failed render (disk full, bad
	// content, etc.) never destroys the previous dump. Only after the document
	// is complete do we replace the output file's contents.
	renderStart := time.Now()
	rendered, err := s.env.App.RenderFinalToBuffer(chosen, prompt)
	s.env.App.LogDebug("picker render timing: %s bytes=%d", time.Since(renderStart).Round(time.Millisecond), len(rendered))
	if err != nil {
		return nil, fmt.Errorf("render output: %w", err)
	}

	// An empty OutputPath means the destination is stdout, which the picker
	// cannot write to while its alternate screen is active.
	if s.env.App.OutputPath() == "" {
		if s.cfg.CopyOnGenerate {
			copyStart := time.Now()
			if err := clipboard.Copy(rendered); err != nil {
				return nil, fmt.Errorf("copy generated output to clipboard: %w", err)
			}
			s.env.App.LogDebug("picker clipboard timing: %s bytes=%d", time.Since(copyStart).Round(time.Millisecond), len(rendered))
		}
		if err := s.env.RecordDump(s.cfg.RootDir, chosen, "HEAD"); err != nil {
			return rendered, err
		}
		return rendered, nil
	}

	// Replace the output document atomically: a failed or partial write must
	// never leave the previous dump truncated.
	writeStart := time.Now()
	if err := s.env.App.WriteOutput(rendered); err != nil {
		return nil, err
	}
	s.env.App.LogDebug("picker write timing: %s bytes=%d", time.Since(writeStart).Round(time.Millisecond), len(rendered))
	if s.cfg.CopyOnGenerate {
		copyStart := time.Now()
		if err := clipboard.Copy(rendered); err != nil {
			return nil, fmt.Errorf("copy generated output to clipboard: %w", err)
		}
		s.env.App.LogDebug("picker clipboard timing: %s bytes=%d", time.Since(copyStart).Round(time.Millisecond), len(rendered))
	}

	// Update the baseline so a later delta dump knows what was just rendered.
	if err := s.env.RecordDump(s.cfg.RootDir, chosen, "HEAD"); err != nil {
		return rendered, err
	}
	return rendered, nil
}
