package main

import (
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/rank"
	"github.com/bethropolis/sift/internal/state"
)

// recordDumpState updates the persisted per-project dump record after a
// successful dump of the given file entries. It is a no-op outside a git
// repository, and failures are returned so callers can log them without
// aborting the dump. ref is the git ref the dump covered (default HEAD); the
// state stores that ref as the next delta baseline.
func recordDumpState(rootDir string, files []format.FileEntry, ref string) error {
	tokens := 0
	for _, f := range files {
		tokens += f.Tokens
	}
	return recordState(rootDir, ref, len(files), tokens)
}

// recordDeltaState is recordDumpState for delta dumps, which may not carry
// file entries (the raw-patch strategy).
func recordDeltaState(rootDir, ref string, files, tokens int) error {
	return recordState(rootDir, ref, files, tokens)
}

func recordState(rootDir, ref string, files, tokens int) error {
	absRootDir, err := state.CanonicalRoot(rootDir)
	if err != nil {
		return err
	}
	g := rank.New(absRootDir)
	if !g.Available() {
		return nil
	}

	hash := ""
	subject := ""
	if ref == "" || ref == "HEAD" {
		hash, subject = g.Head()
	} else {
		hash, subject = g.Ref(ref)
	}
	if hash == "" {
		return nil
	}

	key := state.GetProjectKey(absRootDir)
	return state.MutateState(func(st *state.AppState) error {
		st.Record(key, hash, subject, files, tokens)
		return nil
	})
}
