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

// RenderMetrics reports what a render actually emitted after budget trimming
// and secret redaction. Callers use it for summaries and for the dump-state
// baseline, so recorded counts match the document instead of the pre-budget,
// pre-redaction collection.
type RenderMetrics struct {
	Files       int
	FullCount   int
	SigCount    int
	Tokens      int
	PatchTokens int
	Secrets     int
}

// Render applies the token budget to files and writes the rendered document.
// A non-nil runErr from Collect is reported but rendering still happens so the
// output stays well-formed (e.g. valid JSON on timeout).
func (a *App) Render(files []format.FileEntry, skippedItems []walker.SkippedItem, duration time.Duration, runErr error) error {
	_, err := a.RenderWithMetrics(files, skippedItems, duration, runErr)
	return err
}

// RenderWithMetrics renders like Render and returns the post-budget,
// post-redaction counts that were actually written.
func (a *App) RenderWithMetrics(files []format.FileEntry, skippedItems []walker.SkippedItem, duration time.Duration, runErr error) (RenderMetrics, error) {
	return a.renderWithPrompt(files, skippedItems, duration, runErr, true, a.cfg.Prompt)
}

// RenderWithMetricsCaptured is RenderWithMetrics plus a copy of the exact
// bytes delivered to the configured destination.
func (a *App) RenderWithMetricsCaptured(files []format.FileEntry, skippedItems []walker.SkippedItem, duration time.Duration, runErr error) (RenderMetrics, []byte, error) {
	return a.captureRender(func() (RenderMetrics, error) {
		return a.RenderWithMetrics(files, skippedItems, duration, runErr)
	})
}

// RenderFinalCaptured is RenderFinalWithMetrics plus the rendered bytes.
func (a *App) RenderFinalCaptured(files []format.FileEntry, skippedItems []walker.SkippedItem, duration time.Duration, runErr error) (RenderMetrics, []byte, error) {
	return a.captureRender(func() (RenderMetrics, error) {
		return a.RenderFinalWithMetrics(files, skippedItems, duration, runErr)
	})
}

// RenderEntriesWithMetricsCaptured is RenderEntriesWithMetrics plus bytes.
func (a *App) RenderEntriesWithMetricsCaptured(files []format.FileEntry, skippedItems []walker.SkippedItem, duration time.Duration, runErr error) (RenderMetrics, []byte, error) {
	return a.captureRender(func() (RenderMetrics, error) {
		return a.RenderEntriesWithMetrics(files, skippedItems, duration, runErr)
	})
}

// RenderPatchWithMetricsCaptured is RenderPatchWithMetrics plus bytes.
func (a *App) RenderPatchWithMetricsCaptured(files []format.FileEntry, patch string, skippedItems []walker.SkippedItem, duration time.Duration, runErr error) (RenderMetrics, []byte, error) {
	return a.captureRender(func() (RenderMetrics, error) {
		return a.RenderPatchWithMetrics(files, patch, skippedItems, duration, runErr)
	})
}

func (a *App) captureBytes(data []byte) {
	if a.capture == nil {
		return
	}
	_, _ = a.capture.Write(data)
}

func (a *App) captureRender(render func() (RenderMetrics, error)) (RenderMetrics, []byte, error) {
	var buf bytes.Buffer
	previous := a.capture
	a.capture = &buf
	defer func() { a.capture = previous }()

	metrics, err := render()
	if err != nil {
		return metrics, nil, err
	}
	return metrics, buf.Bytes(), nil
}

// RenderFinal renders exactly the given files without re-applying the token
// budget. It is used by the interactive picker, whose TUI owns budget
// accounting (including smart auto-selection); nothing is dropped afterwards.
func (a *App) RenderFinal(files []format.FileEntry, skippedItems []walker.SkippedItem, duration time.Duration, runErr error) error {
	_, err := a.RenderFinalWithMetrics(files, skippedItems, duration, runErr)
	return err
}

// RenderFinalWithMetrics is RenderFinal with the rendered counts exposed.
func (a *App) RenderFinalWithMetrics(files []format.FileEntry, skippedItems []walker.SkippedItem, duration time.Duration, runErr error) (RenderMetrics, error) {
	return a.renderWithPrompt(files, skippedItems, duration, runErr, false, a.cfg.Prompt)
}

// RenderFinalWithPrompt renders exactly the given files with a caller-supplied
// directive without mutating application configuration.
func (a *App) RenderFinalWithPrompt(files []format.FileEntry, skippedItems []walker.SkippedItem, duration time.Duration, runErr error, prompt string) error {
	_, err := a.RenderFinalWithPromptMetrics(files, skippedItems, duration, runErr, prompt)
	return err
}

// RenderFinalWithPromptMetrics is RenderFinalWithPrompt with metrics.
func (a *App) RenderFinalWithPromptMetrics(files []format.FileEntry, skippedItems []walker.SkippedItem, duration time.Duration, runErr error, prompt string) (RenderMetrics, error) {
	return a.renderWithPrompt(files, skippedItems, duration, runErr, false, prompt)
}

// RenderEntriesWithMetrics applies the token budget and renders an explicit
// entry set through this App's output destination. Multi-root commands use it
// after prefixing each root's files with a unique label.
func (a *App) RenderEntriesWithMetrics(files []format.FileEntry, skippedItems []walker.SkippedItem, duration time.Duration, runErr error) (RenderMetrics, error) {
	return a.renderWithPrompt(files, skippedItems, duration, runErr, true, a.cfg.Prompt)
}

// RenderPatchWithMetrics renders a diff plus its signature-mode neighbors.
// The patch is charged against the budget before any file body is considered.
func (a *App) RenderPatchWithMetrics(files []format.FileEntry, patch string, skippedItems []walker.SkippedItem, duration time.Duration, runErr error) (RenderMetrics, error) {
	a.patch = patch
	defer func() { a.patch = "" }()
	return a.renderWithPrompt(files, skippedItems, duration, runErr, true, a.cfg.Prompt)
}

// RenderPatchBuffer renders a diff plus neighbors (honoring the configured
// budget) into a new buffer for MCP callers.
func (a *App) RenderPatchBuffer(files []format.FileEntry, patch, prompt string) ([]byte, error) {
	doc, _, _, err := a.RenderPatchBufferSections(context.Background(), files, patch, prompt)
	return doc, err
}

// RenderPatchBufferSections is RenderPatchBuffer with per-file byte ranges and
// post-budget metrics for the serve diff endpoint.
func (a *App) RenderPatchBufferSections(ctx context.Context, files []format.FileEntry, patch, prompt string) ([]byte, []format.Section, RenderMetrics, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, RenderMetrics{}, err
	}
	a.patch = patch
	defer func() { a.patch = "" }()
	a.prepareArchitecture(files)
	if a.cfg.Budget > 0 {
		files = a.fitToBudget(files, prompt)
	}
	var buf bytes.Buffer
	sections := []format.Section{}
	metrics, err := a.renderDocumentTo(files, prompt, ctxErrWriter{ctx: ctx, w: &buf}, &sections)
	if err != nil {
		return nil, nil, RenderMetrics{}, err
	}
	return buf.Bytes(), sections, metrics, nil
}

// RenderFilesBufferSections renders files without a patch, applying the same
// budget and metrics as RenderPatchBufferSections. The serve diff endpoint
// uses it when patch=false.
func (a *App) RenderFilesBufferSections(ctx context.Context, files []format.FileEntry, prompt string) ([]byte, []format.Section, RenderMetrics, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, RenderMetrics{}, err
	}
	a.prepareArchitecture(files)
	if a.cfg.Budget > 0 {
		files = a.fitToBudget(files, prompt)
	}
	var buf bytes.Buffer
	sections := []format.Section{}
	metrics, err := a.renderDocumentTo(files, prompt, ctxErrWriter{ctx: ctx, w: &buf}, &sections)
	if err != nil {
		return nil, nil, RenderMetrics{}, err
	}
	return buf.Bytes(), sections, metrics, nil
}

// RenderToClipboard renders the given files and copies the result to the
// system clipboard, without writing to the configured output destination. It
// backs the picker's live copy action.
func (a *App) RenderToClipboard(files []format.FileEntry) error {
	return a.RenderToClipboardWithPrompt(files, a.cfg.Prompt)
}

// directive without mutating application configuration.
func (a *App) RenderToClipboardWithPrompt(files []format.FileEntry, prompt string) error {
	a.prepareArchitecture(files)
	var buf bytes.Buffer
	if _, err := a.renderDocumentTo(files, prompt, &buf, nil); err != nil {
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
	doc, _, err := a.RenderFinalToBufferWithMetrics(files, prompt)
	return doc, err
}

// RenderFinalToBufferWithMetrics is RenderFinalToBuffer with the rendered
// counts exposed. It backs the picker, which needs both for history.
func (a *App) RenderFinalToBufferWithMetrics(files []format.FileEntry, prompt string) ([]byte, RenderMetrics, error) {
	a.prepareArchitecture(files)
	var buf bytes.Buffer
	metrics, err := a.renderDocumentTo(files, prompt, &buf, nil)
	if err != nil {
		return nil, metrics, err
	}
	return buf.Bytes(), metrics, nil
}

// renderDocumentTo builds the output document for the given files (applying
// secret redaction, promotion, and no token budget) and renders it to w. It is
// the shared render core for file, clipboard, and buffer output. When
// sections is non-nil it collects one byte range per rendered file.
func (a *App) renderDocumentTo(files []format.FileEntry, prompt string, w io.Writer, sections *[]format.Section) (RenderMetrics, error) {
	return a.renderDocument(files, prompt, w, sections, true)
}

// renderDocumentNoScan is renderDocument with secret scanning disabled. Budget
// verification uses it to avoid repeating the expensive redaction pass.
func (a *App) renderDocumentNoScan(files []format.FileEntry, prompt string, w io.Writer, sections *[]format.Section) (RenderMetrics, error) {
	return a.renderDocument(files, prompt, w, sections, false)
}

func (a *App) renderDocument(files []format.FileEntry, prompt string, w io.Writer, sections *[]format.Section, scanSecrets bool) (RenderMetrics, error) {
	if a.archGraph == nil && a.cfg.IncludeGraph {
		a.prepareArchitecture(files)
	}
	var metrics RenderMetrics
	renderer, err := format.NewRendererWithOptions(format.ParseStyle(a.cfg.EffectiveStyle()), format.RenderOptions{
		UseColors: a.cfg.UseColors,
		Highlight: highlight.Options{
			Enabled: a.cfg.Highlight && !a.cfg.NoHighlight && a.cfg.UseColors,
			Theme:   highlight.Theme(a.cfg.Theme), MaxBytes: a.cfg.HighlightMaxBytes,
			Profile: terminalHighlightProfile(a.cfg.UseColors),
		},
		Sections: sections,
	})
	if err != nil {
		return metrics, err
	}

	// Scan secrets ONLY on the selected files and patch, right before
	// rendering. Scanning is deliberately kept out of the walker so the
	// directory scan stays fast; only the bytes actually emitted are inspected.
	patch := a.patch
	patchSecrets := 0
	if scanSecrets && a.cfg.SecretScan && !a.cfg.ForceSecrets {
		start := time.Now()
		files = secrets.Default().RedactSelectedFiles(files)
		// Redaction shortens content, so the pre-redaction token count is stale.
		// Recount redacted files so the totals match what is emitted.
		a.recountRedactedTokens(files)
		if patch != "" {
			redacted, detections := secrets.Default().RedactContent([]byte(patch))
			patch = string(redacted)
			patchSecrets = len(detections)
		}
		a.LogDebug("secrets timing: %s files=%d", time.Since(start).Round(time.Millisecond), len(files))
	}

	// Metrics are computed from the final (post-redaction) slice so summaries
	// and recorded dump state agree with the document bytes.
	for _, f := range files {
		metrics.Files++
		metrics.Tokens += f.Tokens
		metrics.Secrets += f.SecretCount
		if f.IsCompressed {
			metrics.SigCount++
		} else {
			metrics.FullCount++
		}
	}
	metrics.Secrets += patchSecrets
	if patch != "" {
		if tz, err := tokenize.New(a.cfg.TokenizeModel); err == nil {
			if n, err := tz.Count([]byte(patch)); err == nil {
				metrics.PatchTokens = n
			}
		}
	}

	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	doc := &format.Document{
		DirectoryTree:     format.BuildTree(paths),
		Files:             files,
		TotalTokens:       metrics.Tokens,
		Instructions:      prompt,
		ArchitectureGraph: a.archGraph,
		Patch:             patch,
	}
	start := time.Now()
	err = renderer.Render(doc, w)
	a.LogDebug("render timing: %s files=%d", time.Since(start).Round(time.Millisecond), len(files))
	return metrics, err
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

func (a *App) renderWithPrompt(files []format.FileEntry, skippedItems []walker.SkippedItem, duration time.Duration, runErr error, applyBudget bool, prompt string) (RenderMetrics, error) {
	a.prepareArchitecture(files)
	if applyBudget && a.cfg.Budget > 0 {
		start := time.Now()
		a.infoLog("Applying token budget: %d", a.cfg.Budget)
		files = a.fitToBudget(files, prompt)
		a.LogDebug("budget timing: %s files=%d", time.Since(start).Round(time.Millisecond), len(files))
		if len(files) == 0 {
			a.log.Warn("Token budget %d is too small for any file.", a.cfg.Budget)
		}
	}

	start := time.Now()
	metrics, writeErr := a.renderToDestination(files, prompt)
	a.LogDebug("destination timing: %s files=%d", time.Since(start).Round(time.Millisecond), len(files))

	if runErr != nil {
		if errors.Is(runErr, context.DeadlineExceeded) {
			a.log.Warn("Timeout of %v reached. Scan stopped.", a.cfg.Timeout)
		} else {
			a.log.Error("Critical error during directory walk: %v", runErr)
		}
		if writeErr != nil {
			return metrics, errors.Join(runErr, writeErr)
		}
		return metrics, runErr
	}
	if writeErr != nil {
		return metrics, writeErr
	}

	stats := summary.DumpStats{
		FileCount:         metrics.Files,
		FullCount:         metrics.FullCount,
		SigCount:          metrics.SigCount,
		SkippedCount:      len(skippedItems),
		TotalTokens:       int64(metrics.Tokens),
		Budget:            a.cfg.Budget,
		Duration:          duration,
		OutputFile:        a.cfg.OutputFile,
		OutputStyle:       a.cfg.EffectiveStyle(),
		IsClipboard:       a.cfg.Clipboard,
		SecretCount:       metrics.Secrets,
		FallbackMode:      a.cfg.FallbackMode,
		FallbackThreshold: a.cfg.FallbackThreshold,
		FallbackApplied:   metrics.SigCount > 0 && a.cfg.FallbackMode == "signatures",
	}

	// When dumping to stdout (-o - or empty output), send the human summary to stderr
	// so the document stream on stdout stays pristine. When dumping to a file or clipboard,
	// send the formatted summary to standard output.
	summaryOut := os.Stdout
	if (a.cfg.OutputFile == "" || a.cfg.OutputFile == "-") && !a.cfg.Clipboard {
		summaryOut = os.Stderr
	}

	summary.PrintDumpSummary(summaryOut, stats, a.cfg.UseColors, a.cfg.Quiet)
	summary.DisplayResults(a.log, int64(metrics.Files), int64(metrics.Tokens), duration, a.cfg.Quiet)

	if a.cfg.ShowSkipped {
		summary.DisplaySkippedItems(a.log, skippedItems, summaryOut, a.cfg.Quiet)
	}

	return metrics, nil
}

// renderToDestination writes the document to the configured sink and returns
// the counts that were emitted. File output is replaced atomically: watch
// re-renders and repeated generates never append to a previous document.
func (a *App) renderToDestination(files []format.FileEntry, prompt string) (RenderMetrics, error) {
	switch {
	case a.cfg.Clipboard:
		var buf bytes.Buffer
		metrics, err := a.renderDocumentTo(files, prompt, &buf, nil)
		if err != nil {
			return metrics, fmt.Errorf("render output: %w", err)
		}
		a.captureBytes(buf.Bytes())
		start := time.Now()
		if err := clipboard.Copy(buf.Bytes()); err != nil {
			return metrics, fmt.Errorf("copy output to clipboard: %w", err)
		}
		a.LogDebug("clipboard timing: %s bytes=%d", time.Since(start).Round(time.Millisecond), buf.Len())
		a.infoLog("Copied %d files (%d tokens) to clipboard.", metrics.Files, metrics.Tokens)
		return metrics, nil

	case a.cfg.CopyOnGenerate:
		var buf bytes.Buffer
		metrics, err := a.renderDocumentTo(files, prompt, &buf, nil)
		if err != nil {
			return metrics, fmt.Errorf("render output: %w", err)
		}
		a.captureBytes(buf.Bytes())
		if err := a.WriteOutput(buf.Bytes()); err != nil {
			return metrics, err
		}
		copyStart := time.Now()
		if err := clipboard.Copy(buf.Bytes()); err != nil {
			return metrics, fmt.Errorf("copy generated output to clipboard: %w", err)
		}
		a.LogDebug("clipboard timing: %s bytes=%d", time.Since(copyStart).Round(time.Millisecond), buf.Len())
		return metrics, nil

	default:
		if a.outputPath == "" || a.cfg.OutputFile == "-" {
			out, err := a.ensureOutput()
			if err != nil {
				return RenderMetrics{}, err
			}
			// Buffer the render: renderers emit one fmt.Fprintf per line/fence,
			// which would otherwise be a write syscall per call on large dumps.
			bufw := bufio.NewWriterSize(out, 256<<10)
			destination := io.Writer(bufw)
			if a.capture != nil {
				destination = io.MultiWriter(bufw, a.capture)
			}
			start := time.Now()
			metrics, renderErr := a.renderDocumentTo(files, prompt, destination, nil)
			if renderErr != nil {
				return metrics, fmt.Errorf("render output: %w", renderErr)
			}
			if flushErr := bufw.Flush(); flushErr != nil {
				return metrics, fmt.Errorf("write output: %w", flushErr)
			}
			a.LogDebug("write timing: %s", time.Since(start).Round(time.Millisecond))
			return metrics, nil
		}

		var metrics RenderMetrics
		start := time.Now()
		err := writeFileAtomic(a.outputPath, func(w io.Writer) error {
			var renderErr error
			destination := io.Writer(w)
			if a.capture != nil {
				destination = io.MultiWriter(w, a.capture)
			}
			metrics, renderErr = a.renderDocumentTo(files, prompt, destination, nil)
			if renderErr != nil {
				return fmt.Errorf("render output: %w", renderErr)
			}
			return nil
		})
		a.LogDebug("write timing: %s path=%s", time.Since(start).Round(time.Millisecond), a.outputPath)
		return metrics, err
	}
}
