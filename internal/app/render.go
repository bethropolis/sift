package app

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/bethropolis/sift/internal/clipboard"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/highlight"
	"github.com/bethropolis/sift/internal/secrets"
	"github.com/bethropolis/sift/internal/summary"
	"github.com/bethropolis/sift/internal/tokenize"
	"github.com/bethropolis/sift/internal/walker"
	"github.com/muesli/termenv"
)

// Render applies the token budget to files and writes the rendered document.
// A non-nil runErr from Collect is reported but rendering still happens so the
// output stays well-formed (e.g. valid JSON on timeout).
func (a *App) Render(files []format.FileEntry, skippedItems []walker.SkippedItem, duration time.Duration, runErr error) error {
	return a.render(files, skippedItems, duration, runErr, true)
}

// RenderFinal renders exactly the given files without re-applying the token
// budget. It is used by the interactive picker, whose TUI owns budget
// accounting (including smart auto-selection); nothing is dropped afterwards.
func (a *App) RenderFinal(files []format.FileEntry, skippedItems []walker.SkippedItem, duration time.Duration, runErr error) error {
	return a.RenderFinalWithPrompt(files, skippedItems, duration, runErr, a.cfg.Prompt)
}

// RenderFinalWithPrompt renders picker output with a session-specific
// directive without mutating application configuration.
func (a *App) RenderFinalWithPrompt(files []format.FileEntry, skippedItems []walker.SkippedItem, duration time.Duration, runErr error, prompt string) error {
	return a.renderWithPrompt(files, skippedItems, duration, runErr, false, prompt)
}

// RenderToClipboard renders the given files and copies the result to the
// system clipboard, without writing to the configured output destination. It
// backs the picker's live copy action.
func (a *App) RenderToClipboard(files []format.FileEntry) error {
	return a.RenderToClipboardWithPrompt(files, a.cfg.Prompt)
}

// RenderToClipboardWithPrompt renders clipboard output with a session-specific
// directive without mutating application configuration.
func (a *App) RenderToClipboardWithPrompt(files []format.FileEntry, prompt string) error {
	var buf bytes.Buffer
	if err := a.renderDocumentTo(files, prompt, &buf); err != nil {
		return fmt.Errorf("render output: %w", err)
	}
	return clipboard.Copy(buf.Bytes())
}

// RenderFinalToBuffer renders exactly the given files (no token budget) into a
// newly allocated byte slice without touching the configured output
// destination. The interactive picker uses it to replace an output file
// non-destructively: the render must fully succeed before the previous dump is
// truncated, so a failed render never destroys the last good document.
func (a *App) RenderFinalToBuffer(files []format.FileEntry, prompt string) ([]byte, error) {
	var buf bytes.Buffer
	if err := a.renderDocumentTo(files, prompt, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// renderDocumentTo builds the output document for the given files (applying
// secret redaction, promotion, and no token budget) and renders it to w. It is
// the shared render core for file, clipboard, and buffer output.
func (a *App) renderDocumentTo(files []format.FileEntry, prompt string, w io.Writer) error {
	renderer, err := format.NewRendererWithOptions(format.ParseStyle(a.cfg.EffectiveStyle()), format.RenderOptions{
		UseColors: a.cfg.UseColors,
		Highlight: highlight.Options{
			Enabled: a.cfg.Highlight && !a.cfg.NoHighlight && a.cfg.UseColors,
			Theme:   highlight.Theme(a.cfg.Theme), MaxBytes: a.cfg.HighlightMaxBytes,
			Profile: terminalHighlightProfile(a.cfg.UseColors),
		},
	})
	if err != nil {
		return err
	}

	// Scan secrets ONLY on the selected files, right before rendering.
	// Scanning is deliberately kept out of the walker so the directory scan
	// stays fast; only the files that will actually be emitted are inspected.
	if a.cfg.SecretScan && !a.cfg.ForceSecrets {
		files = secrets.New().RedactSelectedFiles(files)
		// Redaction shortens content, so the pre-redaction token count is stale.
		// Recount redacted files so Doc.TotalTokens matches what is emitted.
		a.recountRedactedTokens(files)
	}

	usedTokens := 0
	for _, f := range files {
		usedTokens += f.Tokens
	}
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	doc := &format.Document{
		DirectoryTree: format.BuildTree(paths),
		Files:         files,
		TotalTokens:   int(usedTokens),
		Instructions:  prompt,
	}
	return renderer.Render(doc, w)
}

func terminalHighlightProfile(enabled bool) highlight.ColorProfile {
	if !enabled {
		return highlight.ProfileNone
	}
	switch termenv.EnvColorProfile() {
	case termenv.TrueColor:
		return highlight.ProfileTrueColor
	case termenv.ANSI256:
		return highlight.ProfileANSI256
	case termenv.ANSI:
		return highlight.ProfileANSI16
	default:
		return highlight.ProfileNone
	}
}

// recountRedactedTokens refreshes the token count of files whose content changed
// during secret redaction so reported totals match what is actually emitted. Only
// files with detections are recounted, and the tokenizer is built lazily, so a
// scan whose redaction changed nothing pays no counting cost.
func (a *App) recountRedactedTokens(files []format.FileEntry) {
	var tz *tokenize.Tokenizer
	for i := range files {
		if files[i].SecretCount == 0 {
			continue
		}
		if tz == nil {
			var err error
			if tz, err = tokenize.New(a.cfg.TokenizeModel); err != nil {
				a.log.Warn("Unable to recount redacted tokens: %v", err)
				return
			}
		}
		if n, err := tz.Count(files[i].Content); err == nil {
			files[i].Tokens = n
		}
	}
}

func (a *App) render(files []format.FileEntry, skippedItems []walker.SkippedItem, duration time.Duration, runErr error, applyBudget bool) error {
	return a.renderWithPrompt(files, skippedItems, duration, runErr, applyBudget, a.cfg.Prompt)
}

func (a *App) renderWithPrompt(files []format.FileEntry, skippedItems []walker.SkippedItem, duration time.Duration, runErr error, applyBudget bool, prompt string) error {
	// Apply the token budget, keeping the highest-priority files.
	var usedTokens int
	if applyBudget && a.cfg.Budget > 0 {
		a.infoLog("Applying token budget: %d", a.cfg.Budget)
		var kept []format.FileEntry
		kept, usedTokens = tokenize.FitToBudget(files, a.cfg.Budget)
		files = kept
		if len(files) == 0 {
			a.log.Warn("Token budget %d is too small for any file.", a.cfg.Budget)
		}
	} else {
		for _, f := range files {
			usedTokens += f.Tokens
		}
	}
	tokenTotal := int64(usedTokens)

	// --- Render output ---
	var writeErr error
	if a.cfg.Clipboard {
		var buf bytes.Buffer
		if renderErr := a.renderDocumentTo(files, prompt, &buf); renderErr != nil {
			writeErr = fmt.Errorf("render output: %w", renderErr)
		} else if err := clipboard.Copy(buf.Bytes()); err != nil {
			writeErr = fmt.Errorf("copy output to clipboard: %w", err)
		} else {
			a.infoLog("Copied %d files (%d tokens) to clipboard.", len(files), tokenTotal)
		}
	} else if a.cfg.CopyOnGenerate {
		var buf bytes.Buffer
		if renderErr := a.renderDocumentTo(files, prompt, &buf); renderErr != nil {
			writeErr = fmt.Errorf("render output: %w", renderErr)
		} else if _, err := a.output.Write(buf.Bytes()); err != nil {
			writeErr = fmt.Errorf("write output: %w", err)
		} else if err := clipboard.Copy(buf.Bytes()); err != nil {
			writeErr = fmt.Errorf("copy generated output to clipboard: %w", err)
		}
	} else {
		// Buffer the render: renderers emit one fmt.Fprintf per line/fence,
		// which would otherwise be a write syscall per call on large dumps.
		bufw := bufio.NewWriterSize(a.output, 256<<10)
		if renderErr := a.renderDocumentTo(files, prompt, bufw); renderErr != nil {
			writeErr = fmt.Errorf("render output: %w", renderErr)
		} else if flushErr := bufw.Flush(); flushErr != nil {
			writeErr = fmt.Errorf("write output: %w", flushErr)
		}
	}

	// --- Handle walk errors ---
	if runErr != nil {
		if errors.Is(runErr, context.DeadlineExceeded) {
			a.log.Warn("Timeout of %v reached. Scan stopped.", a.cfg.Timeout)
		} else {
			a.log.Error("Critical error during directory walk: %v", runErr)
		}
		if writeErr != nil {
			return errors.Join(runErr, writeErr)
		}
		return runErr
	}
	if writeErr != nil {
		return writeErr
	}

	// --- Show results summary ---
	summary.DisplayResults(a.log, int64(len(files)), tokenTotal, duration, a.cfg.Quiet)

	// --- Show Skipped Items (if requested) ---
	if a.cfg.ShowSkipped {
		summary.DisplaySkippedItems(a.log, skippedItems, os.Stderr, a.cfg.Quiet)
	}

	return nil
}
