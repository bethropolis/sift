package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/clipboard"
	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/rank"
	"github.com/bethropolis/sift/internal/state"
	"github.com/bethropolis/sift/internal/tokenize"
)

var (
	deltaSince string
	deltaPatch bool
)

func init() {
	deltaCmd.Flags().StringVar(&deltaSince, "since", "", "Compare against a ref instead of the last recorded dump")
	deltaCmd.Flags().BoolVar(&deltaPatch, "patch", false, "Output a raw unified diff patch instead of full file contents")
	rootCmd.AddCommand(deltaCmd)
	config.RegisterFlags(cfg, deltaCmd.Flags())
}

// deltaCmd dumps only the changes since the last recorded dump (or a ref).
var deltaCmd = &cobra.Command{
	Use:   "delta [path]",
	Short: "Dump only the changes since the last recorded dump",
	Long: `Dump a context document containing only what changed since the last
recorded dump, feeding an LLM incremental context instead of the whole
repository.

By default the full content of every changed file is rendered. With --patch
the raw unified git diff is emitted instead, wrapped in a context_update
block, which is dramatically more token-efficient for iterative coding.

The comparison base is the commit recorded by the most recent dump, pick,
diff, or delta for this project, or --since to override it.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			cfg.RootDir = args[0]
		}

		rootDir := cfg.RootDir
		absRootDir, err := state.CanonicalRoot(rootDir)
		if err != nil {
			return fmt.Errorf("invalid root directory path '%s': %w", rootDir, err)
		}

		g := rank.New(absRootDir)
		if !g.Available() {
			return fmt.Errorf("sift delta: %q is not inside a git repository", absRootDir)
		}

		from := deltaSince
		if from == "" {
			st, err := state.Load()
			if err != nil {
				return fmt.Errorf("sift delta: %w", err)
			}
			key := state.GetProjectKey(absRootDir)
			if rec, ok := st.Get(key); ok && rec.LastCommitHash != "" {
				from = rec.LastCommitHash
			} else {
				return fmt.Errorf("sift delta: no previous dump recorded for %q; run \"sift dump\" first or pass --since <ref>", absRootDir)
			}
		}

		to := "HEAD"
		if err := applyProfile(cmd); err != nil {
			return err
		}

		// Resolve HEAD to a concrete short hash so the envelope and the
		// recorded baseline name an actual commit, not a symbolic ref.
		to, _ = g.Head()
		if to == "" {
			return fmt.Errorf("sift delta: cannot resolve HEAD")
		}

		changed := g.ChangedBetween(from, to)
		if len(changed) == 0 {
			return fmt.Errorf("sift delta: no files changed since %s", from)
		}

		if deltaPatch {
			return runDeltaPatch(g, absRootDir, from, to, changed)
		}
		return runDeltaFull(absRootDir, from, to, changed)
	},
}

// runDeltaFull renders the full content of the files changed since from.
func runDeltaFull(absRootDir, from, to string, changed []string) error {
	application, err := app.New(cfg)
	if err != nil {
		return err
	}
	defer application.Close()

	only := make(map[string]bool, len(changed))
	for _, p := range changed {
		only[filepath.ToSlash(p)] = true
	}
	application.OnlyPaths = only

	start := time.Now()
	files, skipped, err := application.Collect()
	if err != nil {
		return err
	}
	if err := application.Render(files, skipped, time.Since(start), err); err != nil {
		return err
	}

	if err := recordDumpState(absRootDir, files, to); err != nil {
		application.LogError("Failed to record dump state: %v", err)
	}
	return nil
}

// runDeltaPatch emits the raw unified diff wrapped in a context_update block.
func runDeltaPatch(g *rank.Git, absRootDir, from, to string, changed []string) error {
	patch := g.RawPatch(from, to)
	patch = strings.TrimSpace(patch)
	if patch == "" {
		return fmt.Errorf("sift delta: no changes between %s and %s", from, to)
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "<context_update type=\"delta_patch\" from_commit=\"%s\" to_commit=\"%s\">\n", from, to)
	buf.WriteString("<![CDATA[\n")
	buf.WriteString(patch)
	buf.WriteString("\n]]>\n</context_update>\n")
	body := buf.Bytes()

	if cfg.Clipboard {
		if err := clipboard.Copy(body); err != nil {
			return fmt.Errorf("sift delta: %w", err)
		}
	} else {
		application, err := app.New(cfg)
		if err != nil {
			return err
		}
		defer application.Close()
		if _, err := application.Output().Write(body); err != nil {
			return fmt.Errorf("sift delta: %w", err)
		}
	}

	tokens := countTokensString(cfg, body)
	if err := recordDeltaState(absRootDir, to, len(changed), tokens); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: failed to record dump state: %v\n", err)
	}
	return nil
}

// countTokensString counts tokens for a raw byte payload (e.g. a diff patch).
func countTokensString(cfg *config.Config, content []byte) int {
	tokenizer, err := tokenize.New(cfg.TokenizeModel)
	if err != nil {
		return 0
	}
	tokens, err := tokenizer.Count(content)
	if err != nil {
		return 0
	}
	return tokens
}
