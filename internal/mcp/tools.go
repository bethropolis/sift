package mcp

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/rank"
	"github.com/bethropolis/sift/internal/selection"
	"github.com/bethropolis/sift/internal/state"
)

// Statelessness rule: sift delta's baseline (~/.config/sift/state.json) is
// the one piece of cross-invocation state sift has. These handlers must
// never read or write it — pack_diff takes explicit refs only, pack_context
// never records — so an agent hammering the tools cannot perturb what a
// human's next pick session considers "the last dump."

func toolDefinitions() []Tool {
	str := func(desc string) Property { return Property{Type: "string", Description: desc} }
	return []Tool{
		{
			Name:        "pack_context",
			Description: "Pack a project directory into one ranked, token-budgeted LLM-ready context document.",
			InputSchema: Schema{Type: "object", Properties: map[string]Property{
				"budget": {Type: "number", Description: "Maximum tokens for the document."},
				"prompt": str("Optional task directive prepended to the output."),
				"mode":   {Type: "string", Description: "Context density.", Enum: []string{"full", "signatures"}},
				"style":  str("Output style."),
				"ext":    str("Only include these extensions (comma-separated, e.g. 'go,md')."),
				"ignore": str("Extra ignore patterns (comma-separated, gitignore syntax)."),
			}},
		},
		{
			Name:        "pack_diff",
			Description: "Pack only what changed: uncommitted working-tree changes by default, or the range between a git ref and HEAD.",
			InputSchema: Schema{Type: "object", Properties: map[string]Property{
				"from":  str("Git ref to compare from; empty means uncommitted working-tree changes."),
				"patch": {Type: "boolean", Description: "Return a raw unified diff patch instead of full file contents."},
			}},
		},
		{
			Name:        "list_tree",
			Description: "List the repository structure (paths only, no content) for cheap orientation before a bigger call.",
			InputSchema: Schema{Type: "object", Properties: map[string]Property{
				"ext":    str("Only include these extensions (comma-separated, e.g. 'go,md')."),
				"ignore": str("Extra ignore patterns (comma-separated, gitignore syntax)."),
			}},
		},
	}
}

// callTool runs one tool by name. ok is false for unknown tools.
func callTool(ctx context.Context, name string, args map[string]any, cfg *config.Config) (text string, err error, ok bool) {
	switch name {
	case "pack_context":
		text, err := handlePackContext(ctx, args, cfg)
		return text, err, true
	case "pack_diff":
		text, err := handlePackDiff(ctx, args, cfg)
		return text, err, true
	case "list_tree":
		text, err := handleListTree(ctx, args, cfg)
		return text, err, true
	default:
		return "", nil, false
	}
}

// handlePackContext mirrors runSelect minus the file writes: collect with
// picker enrichment, rank, run the budget optimizer, render to a buffer.
func handlePackContext(ctx context.Context, args map[string]any, cfg *config.Config) (string, error) {
	_ = ctx
	runCfg := *cfg
	intArg(args, "budget", &runCfg.Budget)
	stringArg(args, "prompt", &runCfg.Prompt)
	stringArg(args, "mode", &runCfg.Mode)
	stringArg(args, "style", &runCfg.Style)
	stringArg(args, "ext", &runCfg.Extensions)
	stringArg(args, "ignore", &runCfg.CustomIgnore)
	if runCfg.Mode != "" && runCfg.Mode != "full" && runCfg.Mode != "signatures" {
		return "", fmt.Errorf("invalid mode %q: want full or signatures", runCfg.Mode)
	}
	runCfg.SmartFilter = true
	runCfg.OutputFile = "-" // never touch disk

	application, err := app.New(&runCfg)
	if err != nil {
		return "", err
	}
	defer application.Close()

	files, _, err := application.CollectPicker()
	if err != nil {
		return "", err
	}
	absRoot, err := state.CanonicalRoot(runCfg.RootDir)
	if err != nil {
		return "", err
	}
	preferred := map[string]rank.FileScoreResult{}
	ranker := app.NewRankerWithWeights(absRoot, app.WeightsFromScoring(runCfg.Scoring))
	preferred = ranker.Rank(files)

	candidates := make([]selection.Candidate, 0, len(files))
	for _, file := range files {
		sc := preferred[file.Path]
		candidates = append(candidates, selection.Candidate{
			File:          file,
			PreferredMode: sc.PreferredMode,
			Signals: selection.Signals{
				Recency:    sc.Signals.Recency,
				Churn:      sc.Signals.Churn,
				Centrality: sc.Signals.Centrality,
				Role:       sc.Signals.Role,
			},
		})
	}
	result := selection.Select(candidates, selection.Request{
		Budget: runCfg.Budget,
		Prompt: runCfg.Prompt,
		Tuning: app.TuningFromScoring(runCfg.Scoring),
	})

	buf, err := application.RenderFinalToBuffer(result.Selected, runCfg.Prompt)
	return string(buf), err
}

// handlePackDiff packs what changed. from="" means uncommitted working-tree
// changes (ChangedSinceRef("HEAD")); otherwise the from..HEAD range. It
// never consults the recorded delta baseline.
func handlePackDiff(ctx context.Context, args map[string]any, cfg *config.Config) (string, error) {
	_ = ctx
	absRoot, err := state.CanonicalRoot(cfg.RootDir)
	if err != nil {
		return "", err
	}
	g := rank.New(absRoot)
	if !g.Available() {
		return "", fmt.Errorf("not a git repository")
	}

	var from string
	stringArg(args, "from", &from)
	var patch bool
	boolArg(args, "patch", &patch)
	head, _ := g.Head()

	var changed []string
	uncommitted := from == ""
	to := head
	if uncommitted {
		changed = g.ChangedSinceRef("HEAD")
		from = "HEAD"
		to = "WORKTREE"
	} else {
		changed = g.ChangedBetween(from, head)
	}
	if len(changed) == 0 {
		return "No files changed.", nil
	}

	if patch {
		var diff string
		if uncommitted {
			diff = strings.TrimSpace(g.RawWorktreePatch("HEAD"))
		} else {
			diff = strings.TrimSpace(g.RawPatch(from, head))
		}
		return fmt.Sprintf("<context_update type=\"delta_patch\" from_commit=%q to_commit=%q>\n<![CDATA[\n%s\n]]>\n</context_update>\n", from, to, diff), nil
	}

	runCfg := *cfg
	runCfg.OutputFile = "-"
	application, err := app.New(&runCfg)
	if err != nil {
		return "", err
	}
	defer application.Close()

	only := make(map[string]bool, len(changed))
	for _, p := range changed {
		only[filepath.ToSlash(p)] = true
	}
	application.OnlyPaths = only

	files, _, err := application.Collect()
	if err != nil {
		return "", err
	}
	buf, err := application.RenderFinalToBuffer(files, "")
	return string(buf), err
}

// handleListTree renders the repository structure without reading file
// contents: indented paths with trailing slashes on directories, plus a
// file/dir count line.
func handleListTree(ctx context.Context, args map[string]any, cfg *config.Config) (string, error) {
	runCfg := *cfg
	stringArg(args, "ext", &runCfg.Extensions)
	stringArg(args, "ignore", &runCfg.CustomIgnore)
	runCfg.OutputFile = "-" // app.New must not create codebase.md

	application, err := app.New(&runCfg)
	if err != nil {
		return "", err
	}
	defer application.Close()

	metas, _, err := application.SkeletonPicker(ctx)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	files, dirs := 0, 0
	for _, m := range metas {
		depth := strings.Count(m.Path, "/")
		name := filepath.Base(m.Path)
		b.WriteString(strings.Repeat("  ", depth) + name)
		if m.IsDir {
			b.WriteString("/")
			dirs++
		} else {
			files++
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "\n%d files, %d dirs", files, dirs)
	return b.String(), nil
}
