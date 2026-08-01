package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"time"

	"github.com/bethropolis/sift/internal/clipboard"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/summary"
	"github.com/bethropolis/sift/internal/tokenize"
	"github.com/bethropolis/sift/internal/walker"
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
	return a.render(files, skippedItems, duration, runErr, false)
}

// RenderToClipboard renders the given files and copies the result to the
// system clipboard, without writing to the configured output destination. It
// backs the picker's live copy action.
func (a *App) RenderToClipboard(files []format.FileEntry) error {
	renderer, err := format.NewRenderer(format.ParseStyle(a.cfg.EffectiveStyle()), a.cfg.UseColors)
	if err != nil {
		return err
	}

	total := 0
	paths := make([]string, 0, len(files))
	for _, f := range files {
		total += f.Tokens
		paths = append(paths, f.Path)
	}
	doc := &format.Document{
		DirectoryTree: format.BuildTree(paths),
		Files:         files,
		TotalTokens:   total,
		Instructions:  a.cfg.Prompt,
	}

	var buf bytes.Buffer
	if err := renderer.Render(doc, &buf); err != nil {
		return err
	}
	return clipboard.Copy(buf.Bytes())
}

func (a *App) render(files []format.FileEntry, skippedItems []walker.SkippedItem, duration time.Duration, runErr error, applyBudget bool) error {
	// --- Create the renderer ---
	renderer, err := format.NewRenderer(format.ParseStyle(a.cfg.EffectiveStyle()), a.cfg.UseColors)
	if err != nil {
		return err
	}
	a.log.Debug("Output style: %s", a.cfg.EffectiveStyle())

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

	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	doc := &format.Document{
		DirectoryTree: format.BuildTree(paths),
		Files:         files,
		TotalTokens:   int(tokenTotal),
		Instructions:  a.cfg.Prompt,
	}

	// --- Render output ---
	if a.cfg.Clipboard {
		var buf bytes.Buffer
		renderErr := renderer.Render(doc, &buf)
		if renderErr != nil {
			a.log.Error("Error rendering output: %v", renderErr)
		} else if err := clipboard.Copy(buf.Bytes()); err != nil {
			a.log.Error("Failed to copy output to clipboard: %v", err)
		} else {
			a.infoLog("Copied %d files (%d tokens) to clipboard.", len(files), tokenTotal)
		}
	} else if renderErr := renderer.Render(doc, a.output); renderErr != nil {
		a.log.Error("Error rendering output: %v", renderErr)
	}

	// --- Handle walk errors ---
	if runErr != nil {
		if errors.Is(runErr, context.DeadlineExceeded) {
			a.log.Warn("Timeout of %v reached. Scan stopped.", a.cfg.Timeout)
		} else {
			a.log.Error("Critical error during directory walk: %v", runErr)
		}
		return runErr
	}

	// --- Show results summary ---
	summary.DisplayResults(a.log, int64(len(files)), tokenTotal, duration, a.cfg.Quiet)

	// --- Show Skipped Items (if requested) ---
	if a.cfg.ShowSkipped {
		summary.DisplaySkippedItems(a.log, skippedItems, os.Stderr, a.cfg.Quiet)
	}

	return nil
}
