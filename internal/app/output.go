package app

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// writeFileAtomic replaces path with the bytes produced by render. The render
// runs against a temporary file in the same directory, which is flushed and
// renamed into place only after it is complete. A failed render therefore
// leaves the previous document untouched, and external readers never observe a
// partially written file. Context documents are regenerable, so they skip the
// durability fsync: close + rename already prevents torn reads.
func writeFileAtomic(path string, render func(io.Writer) error) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".sift-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	tmpName := tmp.Name()
	// Remove the temp file on every failure path; after a successful rename
	// it no longer exists and this is a no-op.
	defer func() { _ = os.Remove(tmpName) }()

	// Preserve the previous file's permissions when replacing it.
	if info, statErr := os.Stat(path); statErr == nil {
		_ = tmp.Chmod(info.Mode().Perm())
	}

	bufw := bufio.NewWriterSize(tmp, 256<<10)
	if err := render(bufw); err != nil {
		tmp.Close()
		return err
	}
	if err := bufw.Flush(); err != nil {
		tmp.Close()
		return fmt.Errorf("write output: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		// Windows cannot rename over an existing file. Removing the target
		// first gives up atomicity there but still replaces rather than
		// appends, which is the behavior this function exists to guarantee.
		if _, statErr := os.Stat(path); statErr == nil {
			if rmErr := os.Remove(path); rmErr == nil {
				if renameErr := os.Rename(tmpName, path); renameErr == nil {
					return nil
				}
			}
		}
		return fmt.Errorf("replace output %s: %w", path, err)
	}
	return nil
}

// WriteOutput writes data to the configured destination, replacing a file
// rather than appending to a previously opened handle. It backs the delta
// patch paths, which produce output outside the standard renderer.
func (a *App) WriteOutput(data []byte) error {
	if a.outputPath == "" || a.cfg.OutputFile == "-" {
		w, err := a.ensureOutput()
		if err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}
	return writeFileAtomic(a.outputPath, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}
