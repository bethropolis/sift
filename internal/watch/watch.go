// Package watch re-renders a directory's context document when files change.
package watch

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// DefaultInterval is the debounce window applied to bursts of file events.
const DefaultInterval = 500 * time.Millisecond

// Options configures a Watcher.
type Options struct {
	// Root is the directory tree to watch.
	Root string
	// Ignore reports whether path should not trigger a re-render or be
	// followed as a directory. Called with absolute paths.
	Ignore func(path string) bool
	// Interval is the debounce window; zero uses DefaultInterval.
	Interval time.Duration
	// OnChange re-renders the document after a quiet period. Called from the
	// watch loop, so it must not block indefinitely.
	OnChange func() error
}

// Run watches the tree rooted at Root and calls OnChange after each burst of
// file events, blocking until ctx is canceled or an unrecoverable error
// occurs.
func Run(ctx context.Context, opts Options) error {
	interval := opts.Interval
	if interval <= 0 {
		interval = DefaultInterval
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	if err := addTree(watcher, opts.Root, opts.Ignore); err != nil {
		return err
	}

	var timer *time.Timer
	var timerC <-chan time.Time
	reset := func() {
		if timer != nil {
			timer.Stop()
		}
		timer = time.NewTimer(interval)
		timerC = timer.C
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timerC:
			timerC = nil
			if err := opts.OnChange(); err != nil {
				return err
			}
		case ev, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if opts.Ignore != nil && opts.Ignore(ev.Name) {
				continue
			}
			if !isRelevant(ev) {
				continue
			}
			// Follow newly created directories so deeper changes are seen.
			if ev.Op&fsnotify.Create != 0 {
				if info, statErr := os.Stat(ev.Name); statErr == nil && info.IsDir() {
					_ = watcher.Add(ev.Name)
				}
			}
			reset()
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			if err != nil {
				return err
			}
		}
	}
}

// addTree registers every directory under root with the watcher.
func addTree(w *fsnotify.Watcher, root string, ignore func(string) bool) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if ignore != nil && ignore(path) {
			return filepath.SkipDir
		}
		return w.Add(path)
	})
}

func isRelevant(ev fsnotify.Event) bool {
	return ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0
}
