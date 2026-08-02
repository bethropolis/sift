package picker

import (
	"context"
	"sync"
	"time"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/tui"
	"github.com/bethropolis/sift/internal/walker"
)

const (
	// streamBatchSize caps each NodesMsg so the picker's Update loop handles
	// small, frequent batches instead of one giant insert.
	streamBatchSize = 128
	// streamProgressInterval is how often the picker footer refreshes.
	streamProgressInterval = 250 * time.Millisecond
)

// pickStream is the bridge between the background walk and the TUI. The TUI
// only reads; the scan goroutine owns every channel and closes them once the
// walk (and final rank patch) finishes.
type pickStream struct {
	nodes    chan tui.NodesMsg
	progress chan tui.ProgressMsg
	errCh    chan error
}

func newPickStream() pickStream {
	return pickStream{
		nodes:    make(chan tui.NodesMsg, 16),
		progress: make(chan tui.ProgressMsg, 4),
		errCh:    make(chan error, 1),
	}
}

func (s pickStream) tuiStream() tui.Stream {
	return tui.Stream{Nodes: s.nodes, Progress: s.progress, Err: s.errCh}
}

// streamScan runs the picker's background walk. It streams enriched entries
// to nodes in batches, reports progress on a ticker, appends every entry to
// the shared collection state, and finally patches git-relevance ranks now
// that every path is known. skeletonFiles lists the file paths the skeleton
// showed; any that never stream in were rejected by the full walk (generated
// headers, minified bundles) and are removed so the picker never offers files
// that can never be enriched. It never touches the TUI's data directly.
func streamScan(ctx context.Context, application *app.App, preferredModes map[string]string,
	absRoot string, skeletonFiles []string, stateMu *sync.Mutex, collected *[]format.FileEntry,
	skippedMu *sync.Mutex, skipped *[]walker.SkippedItem,
	totalFiles, totalDirs int, s pickStream) {

	defer close(s.nodes)
	defer close(s.progress)
	defer close(s.errCh)

	// The walker may call emit concurrently, so batching state needs its own
	// lock. Entries still stream in the same order the walker finishes them.
	var mu sync.Mutex
	var batch []tui.Item
	processed := 0

	// Cancellation-aware sends: every send yields to ctx.Done so a picker that
	// quit without draining can never leave the producer blocked on a full
	// channel. Deferred closes are safe because no sender survives a cancelled
	// context (each returns and unwinds to the deferred closes).
	sendNodes := func(msg tui.NodesMsg) bool {
		select {
		case s.nodes <- msg:
			return true
		case <-ctx.Done():
			return false
		}
	}
	sendProgress := func(msg tui.ProgressMsg) bool {
		select {
		case s.progress <- msg:
			return true
		case <-ctx.Done():
			return false
		}
	}
	sendErr := func(err error) bool {
		select {
		case s.errCh <- err:
			return true
		case <-ctx.Done():
			return false
		}
	}

	flush := func(force bool) bool {
		mu.Lock()
		if len(batch) == 0 || (!force && len(batch) < streamBatchSize) {
			mu.Unlock()
			return true
		}
		items := batch
		batch = nil
		mu.Unlock()
		return sendNodes(tui.NodesMsg{Items: items})
	}

	// Throttled footer progress, independent of the walker.
	progressTicker := time.NewTicker(streamProgressInterval)
	defer progressTicker.Stop()
	stopProgress := make(chan struct{})
	progressDone := make(chan struct{})
	go func() {
		defer close(progressDone)
		for {
			select {
			case <-stopProgress:
				return
			case <-ctx.Done():
				return
			case <-progressTicker.C:
				mu.Lock()
				p := processed
				mu.Unlock()
				if !sendProgress(tui.ProgressMsg{Files: totalFiles, Dirs: totalDirs, Processed: p}) {
					return
				}
			}
		}
	}()

	walkSkipped, scanErr := application.StreamPicker(ctx, func(e format.FileEntry) error {
		it := tui.Item{
			Path:          e.Path,
			Content:       e.Content,
			SigContent:    e.SigContent,
			TokensFull:    e.TokensFull,
			TokensSig:     e.TokensSig,
			SecretCount:   e.SecretCount,
			PreferredMode: tui.CompressMode(preferredModes[e.Path]),
		}
		mu.Lock()
		batch = append(batch, it)
		processed++
		mu.Unlock()
		if !flush(false) {
			// The picker is gone; abort the walk so the whole stream unwinds.
			return ctx.Err()
		}

		stateMu.Lock()
		*collected = append(*collected, e)
		stateMu.Unlock()
		return nil
	})

	// Flush any remaining batch; a false result means the context ended, which
	// supersedes a nil walk error.
	if !flush(true) {
		scanErr = ctx.Err()
	}
	close(stopProgress)
	<-progressDone

	if scanErr != nil {
		sendErr(scanErr)
		return
	}

	// Merge the walker's skip list into the skeleton's. Both passes now
	// enforce the same size and binary rules, so the same path can appear
	// twice; keep the first (metadata) reason.
	skippedMu.Lock()
	mergeSkipped(skipped, walkSkipped)
	skippedMu.Unlock()

	// Patch unified relevance ranks now that every path is known. Non-git
	// repos keep the skeleton's zero scores; the picker still works path-ordered.
	if absRoot != "" {
		if r := app.NewRanker(absRoot); r.Available() {
			stateMu.Lock()
			snapshot := append([]format.FileEntry(nil), *collected...)
			stateMu.Unlock()

			scores := r.Rank(snapshot)
			stateMu.Lock()
			*collected = snapshot
			stateMu.Unlock()

			rankBatch := make([]tui.Item, 0, len(snapshot))
			for _, f := range snapshot {
				res := scores[f.Path]
				rankBatch = append(rankBatch, tui.Item{
					Path:          f.Path,
					RankScore:     f.RankScore,
					PreferredMode: tui.CompressMode(res.PreferredMode),
				})
			}
			if len(rankBatch) > 0 {
				if !sendNodes(tui.NodesMsg{Items: rankBatch}) {
					return
				}
			}
		}
	}

	// Drop skeleton files the full walk rejected: they passed the metadata
	// name rules but failed content checks (generated headers, minified
	// bundles), so no enrichment will ever arrive for them.
	stateMu.Lock()
	have := make(map[string]bool, len(*collected))
	for _, f := range *collected {
		have[f.Path] = true
	}
	stateMu.Unlock()
	var removed []string
	for _, p := range skeletonFiles {
		if !have[p] {
			removed = append(removed, p)
		}
	}
	if len(removed) > 0 {
		if !sendNodes(tui.NodesMsg{Remove: removed}) {
			return
		}
	}

	if !sendProgress(tui.ProgressMsg{Files: totalFiles, Dirs: totalDirs, Processed: processed, Done: true}) {
		return
	}
}

// mergeSkipped appends src items whose path is not already present, preserving
// the first-seen reason (the metadata pass runs first). Size and binary skips
// produce identical reasons in both passes, so the first entry is sufficient.
func mergeSkipped(dst *[]walker.SkippedItem, src []walker.SkippedItem) {
	if len(src) == 0 {
		return
	}
	seen := make(map[string]bool, len(*dst)+len(src))
	for _, it := range *dst {
		seen[it.Path] = true
	}
	for _, it := range src {
		if !seen[it.Path] {
			*dst = append(*dst, it)
			seen[it.Path] = true
		}
	}
}
