package main

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/lang"
)

type statsOptions struct {
	top        int
	signatures bool
}

type statsBucket struct {
	Name   string `json:"name"`
	Files  int    `json:"files"`
	Tokens int    `json:"tokens"`
}

type statsReport struct {
	Files          int           `json:"files"`
	Skipped        int           `json:"skipped"`
	Tokens         int           `json:"tokens"`
	SignatureToken int           `json:"signature_tokens,omitempty"`
	SavingsPct     float64       `json:"savings_percent,omitempty"`
	Directories    []statsBucket `json:"directories"`
	Languages      []statsBucket `json:"languages"`
	Roles          []statsBucket `json:"roles"`
	TopFiles       []statsBucket `json:"top_files"`
}

func newStatsCmd(cfg *config.Config) *cobra.Command {
	opts := &statsOptions{top: 10}
	cmd := &cobra.Command{
		Use:   "stats [path]",
		Short: "Show token distribution before dumping",
		Long: `Scan a project and report where its tokens go: totals, top-level
directories, languages, architectural roles, and the heaviest files.

Use --signatures to price the signature-mode document as well.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				cfg.RootDir = args[0]
			}
			if err := applyProfile(cfg, cmd); err != nil {
				return err
			}
			if opts.top <= 0 {
				opts.top = 10
			}
			cfg.OutputFile = "-"
			application, err := app.NewBuffered(cfg)
			if err != nil {
				return err
			}
			defer application.Close()

			var files []format.FileEntry
			var skipped int
			if opts.signatures {
				picked, skippedItems, err := application.CollectPicker()
				if err != nil {
					return err
				}
				files, skipped = picked, len(skippedItems)
			} else {
				collected, skippedItems, err := application.Collect()
				if err != nil {
					return err
				}
				files, skipped = collected, len(skippedItems)
			}
			report := buildStats(files, skipped, opts.signatures)
			if opts.top < len(report.TopFiles) {
				report.TopFiles = report.TopFiles[:opts.top]
			}
			out := cmd.OutOrStdout()
			if cfg.JSONOutput {
				data, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(data))
				return nil
			}
			printStats(out, report, opts.top, opts.signatures)
			return nil
		},
	}
	cmd.Flags().IntVar(&opts.top, "top", opts.top, "Number of heaviest files to list")
	cmd.Flags().BoolVar(&opts.signatures, "signatures", opts.signatures, "Also price signature-mode output")
	config.RegisterFlags(cfg, cmd.Flags())
	return cmd
}

func buildStats(files []format.FileEntry, skipped int, withSignatures bool) statsReport {
	report := statsReport{Files: len(files), Skipped: skipped}
	dirs := map[string]*statsBucket{}
	langs := map[string]*statsBucket{}
	roles := map[string]*statsBucket{}
	add := func(m map[string]*statsBucket, name string, tokens int) {
		if name == "" {
			name = "(none)"
		}
		b := m[name]
		if b == nil {
			b = &statsBucket{Name: name}
			m[name] = b
		}
		b.Files++
		b.Tokens += tokens
	}
	for _, f := range files {
		report.Tokens += f.TokensFull
		report.SignatureToken += f.TokensSig
		dir := strings.SplitN(filepath.ToSlash(f.Path), "/", 2)[0]
		if !strings.Contains(filepath.ToSlash(f.Path), "/") {
			dir = "(root)"
		}
		add(dirs, dir, f.TokensFull)
		language := f.Language
		if language == "" {
			language = strings.TrimPrefix(strings.ToLower(filepath.Ext(f.Path)), ".")
		}
		add(langs, language, f.TokensFull)
		add(roles, string(lang.Classify(f.Path).Role), f.TokensFull)
		report.TopFiles = append(report.TopFiles, statsBucket{Name: f.Path, Files: 1, Tokens: f.TokensFull})
	}
	if withSignatures && report.Tokens > 0 {
		report.SavingsPct = float64(report.Tokens-report.SignatureToken) * 100 / float64(report.Tokens)
	}
	report.Directories = sortedBuckets(dirs)
	report.Languages = sortedBuckets(langs)
	report.Roles = sortedBuckets(roles)
	sort.SliceStable(report.TopFiles, func(i, j int) bool {
		if report.TopFiles[i].Tokens != report.TopFiles[j].Tokens {
			return report.TopFiles[i].Tokens > report.TopFiles[j].Tokens
		}
		return report.TopFiles[i].Name < report.TopFiles[j].Name
	})
	return report
}

func sortedBuckets(m map[string]*statsBucket) []statsBucket {
	out := make([]statsBucket, 0, len(m))
	for _, b := range m {
		out = append(out, *b)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Tokens != out[j].Tokens {
			return out[i].Tokens > out[j].Tokens
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func printStats(w io.Writer, report statsReport, top int, withSignatures bool) {
	fmt.Fprintf(w, "%d files, %d tokens", report.Files, report.Tokens)
	if withSignatures {
		fmt.Fprintf(w, "; signatures %d (%.1f%% saved)", report.SignatureToken, report.SavingsPct)
	}
	fmt.Fprintf(w, "; %d skipped\n", report.Skipped)
	sections := []struct {
		title   string
		buckets []statsBucket
	}{
		{"Directories", report.Directories},
		{"Languages", report.Languages},
		{"Roles", report.Roles},
	}
	for _, section := range sections {
		fmt.Fprintf(w, "\n%s\n", section.title)
		for _, b := range section.buckets {
			fmt.Fprintf(w, "  %-24s %8d  %5.1f%%  %d files\n",
				b.Name, b.Tokens, percent(b.Tokens, report.Tokens), b.Files)
		}
	}
	if top > len(report.TopFiles) {
		top = len(report.TopFiles)
	}
	fmt.Fprintf(w, "\nHeaviest files\n")
	for _, f := range report.TopFiles[:top] {
		fmt.Fprintf(w, "  %8d  %s\n", f.Tokens, f.Name)
	}
}

func percent(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) * 100 / float64(total)
}
