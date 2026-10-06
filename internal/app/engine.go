package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/logger"
	"github.com/bethropolis/sift/internal/scan"
	"github.com/bethropolis/sift/internal/secrets"
	"github.com/bethropolis/sift/internal/tokenize"
	"github.com/bethropolis/sift/internal/walker"
)

// Engine API shared by the CLI commands, the MCP server, and `sift serve`.
//
// The rules for this surface:
//   - explicit root: every function takes the project root as a parameter and
//     never consults the process working directory;
//   - caller context: cancellation and deadlines flow from the caller (an HTTP
//     request in serve) into the walk and the render;
//   - no side effects: nothing is written to disk or stdout, nothing is
//     printed, and os.Exit is never called. Output always goes to a buffer.
//
// The CLI keeps its existing behavior (including output files and progress
// logging) by continuing to use New; only the multi-frontend engine paths
// use NewBuffered.

// NewBuffered creates an App that renders to buffers only. Unlike New it
// never creates the configured output file and never writes to stdout, so it
// is safe to use for serving many projects inside one process.
func NewBuffered(cfg *config.Config) (*App, error) {
	cfg.ResolveColors()

	log := logger.New(os.Stderr, cfg.Verbose, cfg.UseColors)
	if cfg.LogLevel != "" {
		log.SetLevel(cfg.LogLevel)
	} else if cfg.Quiet {
		log.WithLevel(logger.LevelWarn)
	}

	return &App{
		cfg:          cfg,
		log:          log,
		output:       io.Discard,
		contentCache: scan.NewContentCache(),
	}, nil
}

// bufferedConfig clones cfg for an engine call against root: the root is
// pinned explicitly and all output sinks are disabled so the call cannot
// touch disk or stdout.
func bufferedConfig(root string, cfg *config.Config) *config.Config {
	runCfg := *cfg
	runCfg.RootDir = root
	runCfg.OutputFile = "-"
	runCfg.Clipboard = false
	runCfg.CopyOnGenerate = false
	runCfg.ShowProgress = false
	return &runCfg
}

// scanWithRoot runs the blocking collect pipeline (walk + process + rank)
// against an explicit root with the caller's context. It mirrors collect()
// but never derives a context from the config timeout alone: when
// cfg.Timeout is set it bounds the caller's context instead of replacing it.
func scanWithRoot(ctx context.Context, mode collectMode, root string, cfg *config.Config) ([]format.FileEntry, []walker.SkippedItem, error) {
	runCfg := bufferedConfig(root, cfg)
	if runCfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, runCfg.Timeout)
		defer cancel()
	}

	a, err := NewBuffered(runCfg)
	if err != nil {
		return nil, nil, err
	}

	var mu sync.Mutex
	var files []format.FileEntry
	skipped, err := a.walkAndCollect(mode, ctx, func(e format.FileEntry) error {
		mu.Lock()
		files = append(files, e)
		mu.Unlock()
		return nil
	})
	if err != nil {
		return files, skipped, err
	}

	absRootDir, absErr := a.absRoot()
	if absErr == nil {
		a.applyRank(ctx, mode, absRootDir, &files)
	} else {
		a.log.Debug("Skipping git-relevance ranking: %v", absErr)
	}
	return files, skipped, nil
}

// Scan walks, processes, and ranks the project at root in blocking (full
// content) mode, returning entries ordered by git relevance. The token budget
// is not applied; callers curate the selection first.
func Scan(ctx context.Context, root string, cfg *config.Config) ([]format.FileEntry, []walker.SkippedItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	return scanWithRoot(ctx, collectBlocking, root, cfg)
}

// ScanPicker walks, processes, and ranks the project at root in picker mode
// (full + signature content and token counts per file). It backs the TUI,
// the MCP pack_context tool, and the serve tree endpoint.
func ScanPicker(ctx context.Context, root string, cfg *config.Config) ([]format.FileEntry, []walker.SkippedItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	return scanWithRoot(ctx, collectPicker, root, cfg)
}

// Skeleton returns file metadata (no content) for root with the caller's
// context, matching the files a full ScanPicker would enrich.
func Skeleton(ctx context.Context, root string, cfg *config.Config) ([]walker.FileMeta, []walker.SkippedItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	a, err := NewBuffered(bufferedConfig(root, cfg))
	if err != nil {
		return nil, nil, err
	}
	return a.SkeletonPicker(ctx)
}

// ctxErrWriter fails writes once ctx is done, so a disconnected client stops
// the render promptly instead of formatting into a dead response.
type ctxErrWriter struct {
	ctx context.Context
	w   io.Writer
}

func (c ctxErrWriter) Write(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.w.Write(p)
}

// RenderBuffer renders exactly the given files (no token budget) into a
// buffer: secret redaction, token recount, and style rendering via the shared
// render core. Cancellation is honored between the redaction pass and the
// document write, and during the write itself.
func RenderBuffer(ctx context.Context, files []format.FileEntry, prompt string, cfg *config.Config) ([]byte, error) {
	doc, _, err := RenderBufferSections(ctx, files, prompt, cfg)
	return doc, err
}

// RenderBufferSections renders exactly the given files (no token budget)
// like RenderBuffer and additionally returns one byte range per rendered
// file for outline navigation. Offsets index the returned document.
func RenderBufferSections(ctx context.Context, files []format.FileEntry, prompt string, cfg *config.Config) ([]byte, []format.Section, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	a, err := NewBuffered(bufferedConfig("", cfg))
	if err != nil {
		return nil, nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	var buf bytes.Buffer
	sections := []format.Section{}
	if err := a.renderDocumentTo(files, prompt, ctxErrWriter{ctx: ctx, w: &buf}, &sections); err != nil {
		return nil, nil, err
	}
	return buf.Bytes(), sections, nil
}

// ReadPreview reads a single file inside root exactly as the picker would
// and returns content for mode ("full" or "sigs") capped at maxBytes with a
// truncation flag, the post-redaction token count, the redaction count, and
// the detected language. Redaction defaults on: when cfg.ForceSecrets is set
// the call is refused, so previews can never silently leak secrets.
func ReadPreview(root string, relativePath string, mode string, maxBytes int64, cfg *config.Config) (content []byte, tokens int, redactions int, language string, truncated bool, err error) {
	if cfg.ForceSecrets {
		return nil, 0, 0, "", false, fmt.Errorf("preview refuses to expose unredacted secrets")
	}
	a, err := NewBuffered(bufferedConfig(root, cfg))
	if err != nil {
		return nil, 0, 0, "", false, err
	}
	entry, err := a.ReadEntry(relativePath)
	if err != nil {
		return nil, 0, 0, "", false, err
	}

	body := entry.Content
	if mode == "sigs" && entry.SigContent != nil {
		body = entry.SigContent
	}
	if maxBytes > 0 && int64(len(body)) > maxBytes {
		body = body[:maxBytes]
		truncated = true
	}
	if cfg.SecretScan {
		var detections []secrets.Detection
		body, detections = secrets.New().RedactContent(body)
		redactions = len(detections)
	}
	tokens = entry.TokensFull
	if mode == "sigs" {
		tokens = entry.TokensSig
	}
	if redactions > 0 || truncated {
		if tz, tzErr := tokenize.New(cfg.TokenizeModel); tzErr == nil {
			if n, cntErr := tz.Count(body); cntErr == nil {
				tokens = n
			}
		}
	}
	return body, tokens, redactions, entry.Language, truncated, nil
}
