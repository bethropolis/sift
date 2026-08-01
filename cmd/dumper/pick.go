package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/bethropolis/dir-dumper/internal/app"
	"github.com/bethropolis/dir-dumper/internal/clipboard"
	"github.com/bethropolis/dir-dumper/internal/config"
	"github.com/bethropolis/dir-dumper/internal/format"
	"github.com/bethropolis/dir-dumper/internal/rank"
	"github.com/bethropolis/dir-dumper/internal/state"
	"github.com/bethropolis/dir-dumper/internal/tui"
)

func init() {
	rootCmd.AddCommand(pickCmd)
	config.RegisterFlags(cfg, pickCmd.Flags())
}

// pickCmd interactively selects files, then renders the chosen context.
var pickCmd = &cobra.Command{
	Use:   "pick [path]",
	Short: "Interactively select files and render a context document",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPick,
}

// runPick is shared by the pick subcommand and the bare root command, which
// launches the picker when invoked without a subcommand.
func runPick(cmd *cobra.Command, args []string) error {
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		return fmt.Errorf("dumper pick requires an interactive terminal")
	}

	if len(args) > 0 {
		cfg.RootDir = args[0]
	}
	if err := applyProfile(cmd); err != nil {
		return err
	}

	start := time.Now()
	application := app.New(cfg)
	defer application.Close()

	// CollectPicker keeps both full and signature-only content so the TUI can
	// offer per-file modes and live token tallies.
	files, skipped, err := application.CollectPicker()
	if err != nil {
		return err
	}

	items := make([]tui.Item, len(files))
	for i, f := range files {
		items[i] = tui.Item{
			Path:        f.Path,
			Content:     f.Content,
			SigContent:  f.SigContent,
			TokensFull:  f.TokensFull,
			TokensSig:   f.TokensSig,
			SecretCount: f.SecretCount,
			RankScore:   f.RankScore,
		}
	}

	result, err := tui.Run(items, tui.Options{
		Budget:  cfg.Budget,
		Style:   cfg.EffectiveStyle(),
		UseNerd: !cfg.NoNerdFonts,
		OnCopy: func(sel []tui.Selection) error {
			return copySelection(application, files, sel)
		},
		Delta: buildDeltaInfo(application, files),
		OnDelta: func(sel tui.DeltaSelection) error {
			return performDelta(application, files, sel)
		},
	})
	if err != nil {
		return fmt.Errorf("dumper pick: %w", err)
	}
	if result.DeltaDone {
		// The delta dump already wrote its own output and recorded state.
		return nil
	}

	selected := result.Selections
	if len(selected) == 0 {
		return fmt.Errorf("dumper pick: no files selected")
	}
	if len(selected) == 0 {
		return fmt.Errorf("dumper pick: no files selected")
	}

	chosen := applySelection(files, selected)
	if err := application.RenderFinal(chosen, skipped, time.Since(start), nil); err != nil {
		return err
	}

	// Record the dump baseline so future delta dumps know what changed since
	// this selection. Non-fatal on failure.
	if err := recordDumpState(cfg.RootDir, chosen, "HEAD"); err != nil {
		application.LogError("Failed to record dump state: %v", err)
	}
	return nil
}

// applySelection maps the TUI's per-file modes onto the collected entries. A
// file in FULL mode keeps its raw content; SIGS uses the signature summary;
// SKIP entries are dropped.
func applySelection(files []format.FileEntry, selected []tui.Selection) []format.FileEntry {
	byPath := make(map[string]format.FileEntry, len(files))
	for _, f := range files {
		byPath[f.Path] = f
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
func copySelection(application *app.App, files []format.FileEntry, selected []tui.Selection) error {
	chosen := applySelection(files, selected)
	if len(chosen) == 0 {
		return fmt.Errorf("nothing selected")
	}
	return application.RenderToClipboard(chosen)
}

// buildDeltaInfo gathers the git delta state for the picker's delta modal.
// It returns nil when the directory is not a git repository or no dump
// baseline has been recorded for it.
func buildDeltaInfo(application *app.App, files []format.FileEntry) *tui.DeltaInfo {
	absRootDir, err := filepath.Abs(cfg.RootDir)
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
			info.FilesToken += estimateTokens(p)
		}
	}

	// Raw patch token and line estimates.
	patch := g.RawPatch(from, "HEAD")
	if patch != "" {
		info.PatchToken = countTokensString(cfg, []byte(patch))
		info.PatchLines = strings.Count(patch, "\n")
	}
	return info
}

// estimateTokens estimates a token count from the file on disk, used for
// changed paths the walker did not collect (e.g. deleted or ignored files).
func estimateTokens(path string) int {
	fi, err := os.Stat(filepath.Join(cfg.RootDir, filepath.FromSlash(path)))
	if err != nil {
		return 0
	}
	return int(fi.Size() / 4)
}

// performDelta runs a delta dump for a selection confirmed in the picker's
// delta modal. Full strategy renders only the changed files; patch strategy
// emits the raw unified diff in a context_update block.
func performDelta(application *app.App, files []format.FileEntry, sel tui.DeltaSelection) error {
	absRootDir, err := filepath.Abs(cfg.RootDir)
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
			if _, err := application.Output().Write(buf.Bytes()); err != nil {
				return err
			}
		}
		return recordDeltaState(absRootDir, sel.To, len(g.ChangedBetween(sel.From, sel.To)), countTokensString(cfg, buf.Bytes()))
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
		return application.RenderToClipboard(chosen)
	}
	if err := application.RenderFinal(chosen, nil, 0, nil); err != nil {
		return err
	}
	return recordDumpState(absRootDir, chosen, sel.To)
}
