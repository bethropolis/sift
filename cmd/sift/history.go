package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/history"
)

// newHistoryCmd exposes the local render history: list, locate or clear it.
func newHistoryCmd(cfg *config.Config) *cobra.Command {
	var (
		jsonOutput bool
		limit      int
		pathOnly   bool
	)
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show, locate or clear recorded dump history",
		Long: `Show recent renders recorded by the opt-in local history.

History is disabled by default. Enable it with [history] enabled = true in
.sift.toml or config.toml, and use --no-history to skip a single run.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := historyStore(cfg)
			if err != nil {
				return err
			}
			if pathOnly {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), store.HistoryPath())
				return err
			}
			entries, err := store.List(limit)
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeHistoryJSON(cmd.OutOrStdout(), entries)
			}
			return writeHistoryTable(cmd.OutOrStdout(), entries)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print history as JSON")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum entries to show (0 = all)")
	cmd.Flags().BoolVar(&pathOnly, "path", false, "Print the history file path and exit")
	cmd.AddCommand(newHistoryClearCmd(cfg))
	return cmd
}

func newHistoryClearCmd(cfg *config.Config) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Remove the history log and its archived dumps",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := historyStore(cfg)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if !yes {
				if _, err := fmt.Fprint(out, "Remove all sift history and referenced archive files? [y/N] "); err != nil {
					return err
				}
				line, readErr := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
				if readErr != nil && readErr != io.EOF {
					return readErr
				}
				answer := strings.ToLower(strings.TrimSpace(line))
				if answer != "y" && answer != "yes" {
					_, err := fmt.Fprintln(out, "Cancelled.")
					return err
				}
			}
			if err := store.Clear(); err != nil {
				return err
			}
			_, err = fmt.Fprintln(out, "Cleared sift history.")
			return err
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the confirmation prompt")
	return cmd
}

func historyStore(cfg *config.Config) (*history.Store, error) {
	locations, err := historyLocations()
	if err != nil {
		return nil, err
	}
	return &history.Store{
		DataDir:  locations.dataDir,
		DumpsDir: locations.dumpsDir,
		Keep:     cfg.HistoryKeep,
		Now:      time.Now,
	}, nil
}

type historyPaths struct {
	dataDir  string
	dumpsDir string
}

// historyLocations resolves the XDG data and cache paths. The history package
// never reads environment variables; this is the command-layer boundary.
func historyLocations() (historyPaths, error) {
	var paths historyPaths
	home, homeErr := os.UserHomeDir()

	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		if homeErr != nil {
			return paths, fmt.Errorf("resolve data home: %w", homeErr)
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return paths, fmt.Errorf("resolve cache home: %w", err)
		}
		cacheHome = cache
	}

	absData, err := filepath.Abs(filepath.Join(dataHome, "sift"))
	if err != nil {
		return paths, err
	}
	absCache, err := filepath.Abs(filepath.Join(cacheHome, "sift", "dumps"))
	if err != nil {
		return paths, err
	}
	paths.dataDir = absData
	paths.dumpsDir = absCache
	return paths, nil
}

func historyEnabled(cfg *config.Config) bool {
	return cfg != nil && cfg.HistoryEnabled && !cfg.NoHistory
}

// recordHistory archives a successful render. Archive failures are reported
// to stderr and never change the render's output or exit status.
func recordHistory(cfg *config.Config, command string, rendered []byte, metrics app.RenderMetrics) {
	recordHistoryAt(cfg, cfg.RootDir, command, rendered, metrics)
}

func recordHistoryAt(cfg *config.Config, root, command string, rendered []byte, metrics app.RenderMetrics) {
	if !historyEnabled(cfg) || len(rendered) == 0 {
		return
	}
	store, err := historyStore(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sift: history: %v\n", err)
		return
	}
	if _, err := store.Record(root, command, cfg.EffectiveStyle(), rendered, metrics.Tokens, metrics.Files); err != nil {
		fmt.Fprintf(os.Stderr, "sift: history: %v\n", err)
	}
}

func writeHistoryJSON(w io.Writer, entries []history.Entry) error {
	if entries == nil {
		entries = []history.Entry{}
	}
	envelope := struct {
		V       int             `json:"v"`
		Entries []history.Entry `json:"entries"`
	}{V: history.SchemaVersion, Entries: entries}
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(envelope)
}

func writeHistoryTable(w io.Writer, entries []history.Entry) error {
	if len(entries) == 0 {
		_, err := fmt.Fprintln(w, "No dumps recorded.")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "WHEN\tCMD\tTOKENS\tSIZE\tPROJECT\tFILE"); err != nil {
		return err
	}
	for _, entry := range entries {
		when := entry.Ts
		if t, err := time.Parse(time.RFC3339, entry.Ts); err == nil {
			when = t.Local().Format("2006-01-02 15:04")
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			when, entry.Cmd, historyTokens(entry.Tokens), historyBytes(entry.Bytes), entry.Name, entry.File); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func historyTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM tok", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk tok", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d tok", n)
	}
}

func historyBytes(n int64) string {
	const (
		kib = 1024
		mib = 1024 * kib
	)
	switch {
	case n >= mib:
		return fmt.Sprintf("%.1f MB", float64(n)/mib)
	case n >= kib:
		return fmt.Sprintf("%.0f KB", float64(n)/kib)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
