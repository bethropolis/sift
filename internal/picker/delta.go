package picker

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bethropolis/sift/internal/clipboard"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/rank"
	"github.com/bethropolis/sift/internal/state"
	"github.com/bethropolis/sift/internal/tui"
)

// buildDeltaInfo gathers the git delta state for the picker's delta modal.
// It returns nil when the directory is not a git repository or no dump
// baseline has been recorded for it.
func (s *service) buildDeltaInfo(files []format.FileEntry) *tui.DeltaInfo {
	absRootDir, err := filepath.Abs(s.cfg.RootDir)
	if err != nil {
		return nil
	}
	g := rank.New(absRootDir)
	if !g.Available() {
		return nil
	}

	st, err := state.Load()
	if err != nil {
		return nil
	}
	rec, ok := st.Get(state.GetProjectKey(absRootDir))
	if !ok || rec.LastCommitHash == "" {
		return nil
	}
	from := rec.LastCommitHash
	headHash, headMsg := g.Head()
	if headHash == "" {
		return nil
	}

	commits := g.CommitsBetween(from, "HEAD")
	info := &tui.DeltaInfo{
		RootDir:  absRootDir,
		FromHash: from,
		FromMsg:  rec.LastCommitMsg,
		HeadHash: headHash,
		HeadMsg:  headMsg,
	}

	// Changed files and their full-content token estimate. Files already
	// collected by the picker contribute real token counts; anything else
	// (deleted, ignored) falls back to a size estimate.
	tokensByPath := make(map[string]int, len(files))
	for _, f := range files {
		tokensByPath[f.Path] = f.TokensFull
	}
	for _, c := range commits {
		info.Commits = append(info.Commits, tui.DeltaCommit{Short: c.Short, Subject: c.Subject, Checked: true})
	}
	changed := g.ChangedBetween(from, "HEAD")
	for _, p := range changed {
		info.Files = append(info.Files, p)
		if t, ok := tokensByPath[p]; ok {
			info.FilesToken += t
		} else {
			info.FilesToken += s.estimateTokens(p)
		}
	}

	// Raw patch token and line estimates.
	patch := g.RawPatch(from, "HEAD")
	if patch != "" {
		info.PatchToken = s.env.CountTokens([]byte(patch))
		info.PatchLines = strings.Count(patch, "\n")
	}
	return info
}

// estimateTokens estimates a token count from the file on disk, used for
// changed paths the walker did not collect (e.g. deleted or ignored files).
func (s *service) estimateTokens(path string) int {
	fi, err := os.Stat(filepath.Join(s.cfg.RootDir, filepath.FromSlash(path)))
	if err != nil {
		return 0
	}
	return int(fi.Size() / 4)
}

// performDelta runs a delta dump for a selection confirmed in the picker's
// delta modal. Full strategy renders only the changed files; patch strategy
// emits the raw unified diff in a context_update block.
func (s *service) performDelta(files []format.FileEntry, sel tui.DeltaSelection) error {
	absRootDir, err := filepath.Abs(s.cfg.RootDir)
	if err != nil {
		return err
	}
	g := rank.New(absRootDir)
	if !g.Available() {
		return fmt.Errorf("not inside a git repository")
	}

	if sel.Strategy == tui.DeltaPatch {
		patch := g.RawPatch(sel.From, sel.To)
		if strings.TrimSpace(patch) == "" {
			return fmt.Errorf("no changes between %s and %s", sel.From, sel.To)
		}
		var buf bytes.Buffer
		fmt.Fprintf(&buf, "<context_update type=\"delta_patch\" from_commit=\"%s\" to_commit=\"%s\">\n", sel.From, sel.To)
		buf.WriteString("<![CDATA[\n")
		buf.WriteString(patch)
		buf.WriteString("\n]]>\n</context_update>\n")

		if sel.Clipboard {
			if err := clipboard.Copy(buf.Bytes()); err != nil {
				return err
			}
		} else {
			if _, err := s.env.App.Output().Write(buf.Bytes()); err != nil {
				return err
			}
		}
		return s.env.RecordDelta(absRootDir, sel.To, len(g.ChangedBetween(sel.From, sel.To)), s.env.CountTokens(buf.Bytes()))
	}

	// Full strategy: keep only files changed in the range.
	changed := g.ChangedBetween(sel.From, sel.To)
	only := make(map[string]bool, len(changed))
	for _, p := range changed {
		only[filepath.ToSlash(p)] = true
	}
	chosen := make([]format.FileEntry, 0, len(changed))
	for _, f := range files {
		if only[f.Path] {
			f.Tokens = f.TokensFull
			f.IsCompressed = false
			chosen = append(chosen, f)
		}
	}
	if len(chosen) == 0 {
		return fmt.Errorf("no collected files changed between %s and %s", sel.From, sel.To)
	}

	if sel.Clipboard {
		return s.env.App.RenderToClipboard(chosen)
	}
	if err := s.env.App.RenderFinal(chosen, nil, 0, nil); err != nil {
		return err
	}
	return s.env.RecordDump(absRootDir, chosen, sel.To)
}
