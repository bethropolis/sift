package mcp

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/lang"
	"github.com/bethropolis/sift/internal/rank"
	"github.com/bethropolis/sift/internal/scan"
	"github.com/bethropolis/sift/internal/selection"
	"github.com/bethropolis/sift/internal/serve/jail"
	"github.com/bethropolis/sift/internal/state"
	embedded "github.com/bethropolis/sift/skills"
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
				"path":   str("Optional subdirectory, relative to the active root or absolute inside it."),
			}},
		},
		{
			Name:        "pack_diff",
			Description: "Pack only what changed: uncommitted working-tree changes by default, or the range between a git ref and HEAD.",
			InputSchema: Schema{Type: "object", Properties: map[string]Property{
				"from":          str("Git ref to compare from; empty means uncommitted working-tree changes."),
				"patch":         {Type: "boolean", Description: "Return a raw unified diff patch instead of full file contents."},
				"with_context":  {Type: "boolean", Description: "Emit the unified diff plus import-graph neighbors in signatures mode."},
				"context_depth": {Type: "number", Description: "Import hops to include around changed files (default 1)."},
			}},
		},
		{
			Name:        "pack_follow",
			Description: "Pack one file plus the files connected to it through imports, followed transitively. Dependents answers what could break if the file changes; deps answers what the file needs.",
			InputSchema: Schema{Type: "object", Properties: map[string]Property{
				"file":       str("Seed file, relative to the project root or absolute."),
				"direction":  {Type: "string", Description: "Which way to walk the import graph.", Enum: []string{"deps", "dependents", "both"}},
				"depth":      {Type: "number", Description: "Max import hops from the seed (-1 = unlimited, 0 = seed only)."},
				"full_depth": {Type: "number", Description: "Files within N hops render in full; farther files render as signatures."},
				"budget":     {Type: "number", Description: "Maximum tokens for the document."},
				"mode":       {Type: "string", Description: "Context density.", Enum: []string{"full", "signatures"}},
				"path":       str("Optional subdirectory, relative to the active root or absolute inside it."),
			}},
		},
		{
			Name:        "list_tree",
			Description: "List the repository structure (paths only, no content) for cheap orientation before a bigger call.",
			InputSchema: Schema{Type: "object", Properties: map[string]Property{
				"ext":         str("Only include these extensions (comma-separated, e.g. 'go,md')."),
				"ignore":      str("Extra ignore patterns (comma-separated, gitignore syntax)."),
				"path":        str("Optional subdirectory, relative to the active root or absolute inside it."),
				"with_tokens": {Type: "boolean", Description: "Show role tags, approximate token counts, and directory totals."},
			}},
		},
		{
			Name:        "pack_files",
			Description: "Pack an explicit list of files, each with its own full/signatures/outline mode and optional symbol or line-range focus.",
			InputSchema: Schema{Type: "object", Properties: map[string]Property{
				"files": {
					Type:        "array",
					Description: "Files to pack.",
					Items: &Schema{Type: "object", Properties: map[string]Property{
						"path":   str("File path, relative to the active root or absolute inside it."),
						"mode":   {Type: "string", Description: "Per-file density.", Enum: []string{"full", "signatures", "outline"}},
						"symbol": str("Optional declaration name to expand in full (e.g. Config or Type.Method)."),
						"lines":  str("Optional 1-based line range, e.g. 10-25."),
					}, Required: []string{"path"}},
				},
				"style":  str("Output style."),
				"prompt": str("Optional task directive prepended to the output."),
				"path":   str("Optional subdirectory, relative to the active root or absolute inside it."),
			}, Required: []string{"files"}},
		},
		{
			Name:        "get_skill",
			Description: "Retrieve instructions and guidelines for an embedded Sift agent skill (e.g. 'sift-project-dotfile' for authoring .sift.toml).",
			InputSchema: Schema{Type: "object", Properties: map[string]Property{
				"name": str("Name of the skill to retrieve (e.g. 'sift-project-dotfile'). Empty retrieves the default dotfile skill."),
			}},
		},
	}
}

// callTool runs one tool by name. ok is false for unknown tools.
// session is one MCP server process's shared state: the configuration and a
// cache reused across tool calls so repeat requests skip tree-sitter and BPE
// work.
type session struct {
	cfg   *config.Config
	cache *scan.ContentCache

	// roots are the active project roots. They start from cfg.RootDir and are
	// replaced when the client advertises the roots capability.
	roots []string
	jail  *jail.Jail

	rootsSupported bool
	rootsRequested bool
	nextRootsID    int
	pendingRootsID string
}

func newSession(cfg *config.Config) *session {
	s := &session{cfg: cfg, cache: app.NewContentCache(cfg)}
	if abs, err := filepath.Abs(cfg.RootDir); err == nil {
		if j, err := jail.New([]string{abs}, nil); err == nil {
			s.jail = j
			s.roots = j.Roots()
		}
	}
	if len(s.roots) == 0 {
		s.roots = []string{cfg.RootDir}
	}
	return s
}

func callTool(ctx context.Context, name string, args map[string]any, s *session) (text string, err error, ok bool) {
	switch name {
	case "pack_context":
		text, err := handlePackContextSession(ctx, args, s)
		return text, err, true
	case "pack_diff":
		text, err := handlePackDiffSession(ctx, args, s)
		return text, err, true
	case "pack_follow":
		text, err := handlePackFollowSession(ctx, args, s)
		return text, err, true
	case "list_tree":
		text, err := handleListTreeSession(ctx, args, s)
		return text, err, true
	case "pack_files":
		text, err := handlePackFilesSession(ctx, args, s)
		return text, err, true
	case "get_skill":
		text, err := handleGetSkill(args)
		return text, err, true
	default:
		return "", nil, false
	}
}

// handlePackContext mirrors runSelect minus the file writes: collect with
// picker enrichment, rank, run the budget optimizer, render to a buffer.
func handlePackContextSession(ctx context.Context, args map[string]any, s *session) (string, error) {
	cfg := s.cfg
	_ = ctx
	runCfg := *cfg
	intArg(args, "budget", &runCfg.Budget)
	stringArg(args, "prompt", &runCfg.Prompt)
	stringArg(args, "mode", &runCfg.Mode)
	if _, ok := args["style"]; ok {
		// An explicit style must beat the server's legacy --json/--markdown
		// flags, which EffectiveStyle consults first.
		runCfg.JSONOutput = false
		runCfg.MarkdownOutput = false
	}
	stringArg(args, "style", &runCfg.Style)
	stringArg(args, "ext", &runCfg.Extensions)
	stringArg(args, "ignore", &runCfg.CustomIgnore)
	_, modeGiven := args["mode"]
	if runCfg.Mode != "" && runCfg.Mode != "full" && runCfg.Mode != "signatures" {
		return "", fmt.Errorf("invalid mode %q: want full or signatures", runCfg.Mode)
	}
	runCfg.SmartFilter = true
	runCfg.OutputFile = "-" // never touch disk
	root, err := s.scopeRoot(args)
	if err != nil {
		return "", err
	}
	runCfg.RootDir = root

	files, _, err := app.ScanPickerWithCache(ctx, root, &runCfg, s.cache)
	if err != nil {
		return "", err
	}
	absRoot, err := state.CanonicalRoot(runCfg.RootDir)
	if err != nil {
		return "", err
	}
	preferred := map[string]rank.FileScoreResult{}
	graph := map[string][]string{}
	ranker := app.NewRankerWithWeights(absRoot, app.WeightsFromScoring(runCfg.Scoring))
	preferred, graph = ranker.RankGraph(files)
	affinity := app.RelatedTestAffinity(files, graph)

	candidates := make([]selection.Candidate, 0, len(files))
	for _, file := range files {
		sc := preferred[file.Path]
		candidates = append(candidates, selection.Candidate{
			File:          file,
			PreferredMode: sc.PreferredMode,
			Signals: selection.Signals{
				Recency:      sc.Signals.Recency,
				Churn:        sc.Signals.Churn,
				Centrality:   sc.Signals.Centrality,
				Role:         sc.Signals.Role,
				TestAffinity: affinity[file.Path],
			},
		})
	}
	result := app.SelectWithDependencies(app.DependencyRequest{
		Candidates: candidates,
		Files:      files,
		Graph:      graph,
		Preferred:  preferred,
		Budget:     runCfg.Budget,
		Overhead:   app.EstimatedDocumentOverhead(&runCfg, runCfg.Prompt, files),
		Prompt:     runCfg.Prompt,
		Tuning:     app.TuningFromScoring(runCfg.Scoring),
		MaxDepth:   runCfg.MaxDepth,
	})

	selected := result.Selected
	if modeGiven {
		selected = forceSelectionMode(selected, runCfg.Mode, runCfg.Budget)
	}

	buf, metrics, err := app.RenderBufferWithMetrics(ctx, selected, runCfg.Prompt, &runCfg)
	if err != nil {
		return "", err
	}
	return string(buf) + omissionManifest(files, selected, runCfg.Budget, metrics), nil
}

// forceSelectionMode rewrites the optimizer's picks to an explicit caller mode.
// Files that cannot be compressed fall back to full content; entries that would
// exceed the budget are dropped so the document never overshoots.
func forceSelectionMode(files []format.FileEntry, mode string, budget int) []format.FileEntry {
	out := make([]format.FileEntry, 0, len(files))
	used := 0
	for _, f := range files {
		switch mode {
		case "signatures":
			if len(f.SigContent) > 0 {
				f.Content = f.SigContent
				f.Tokens = f.TokensSig
				f.IsCompressed = true
			} else {
				f.Tokens = f.TokensFull
				f.IsCompressed = false
			}
		case "full":
			f.Tokens = f.TokensFull
			f.IsCompressed = false
		}
		if budget > 0 && used+f.Tokens > budget {
			continue
		}
		used += f.Tokens
		out = append(out, f)
	}
	return out
}

// handlePackDiff packs what changed. from="" means uncommitted working-tree
// changes (ChangedSinceRef("HEAD")); otherwise the from..HEAD range. It
// never consults the recorded delta baseline.
func handlePackDiffSession(ctx context.Context, args map[string]any, s *session) (string, error) {
	cfg := s.cfg
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
	var patch, withContext bool
	boolArg(args, "patch", &patch)
	boolArg(args, "with_context", &withContext)
	contextDepth := 1
	intArg(args, "context_depth", &contextDepth)
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

	if patch || withContext {
		var diff string
		if uncommitted {
			diff = strings.TrimSpace(g.RawWorktreePatch("HEAD"))
		} else {
			diff = strings.TrimSpace(g.RawPatch(from, head))
		}
		if patch {
			return format.RenderContextUpdate(from, to, diff), nil
		}
		if contextDepth <= 0 {
			contextDepth = 1
		}
		runCfg := *cfg
		runCfg.RootDir = absRoot
		runCfg.OutputFile = "-"
		files, _, err := app.ScanPickerWithCache(ctx, absRoot, &runCfg, s.cache)
		if err != nil {
			return "", err
		}
		neighbors := app.DiffNeighborFiles(&runCfg, files, changed, contextDepth)
		application, err := app.NewBufferedWithCache(&runCfg, s.cache)
		if err != nil {
			return "", err
		}
		defer application.Close()
		buf, err := application.RenderPatchBuffer(neighbors, diff, runCfg.Prompt)
		return string(buf), err
	}

	runCfg := *cfg
	runCfg.OutputFile = "-"

	only := make(map[string]bool, len(changed))
	for _, p := range changed {
		only[filepath.ToSlash(p)] = true
	}

	// OnlyPaths is an App-level restriction; apply it through a buffered
	// app since the engine Scan always walks the whole root.
	application, err := app.NewBufferedWithCache(&runCfg, s.cache)
	if err != nil {
		return "", err
	}
	defer application.Close()
	application.OnlyPaths = only

	files, _, err := application.Collect()
	if err != nil {
		return "", err
	}
	buf, err := app.RenderBuffer(ctx, files, "", &runCfg)
	return string(buf), err
}

// handlePackFollow packs one seed file plus its import-graph neighbors. It
// mirrors runFollow minus the file writes: collect, walk from the seed,
// run the budget optimizer over the walk, render to a buffer. It never
// touches the delta baseline.
func handlePackFollowSession(ctx context.Context, args map[string]any, s *session) (string, error) {
	cfg := s.cfg
	var file string
	stringArg(args, "file", &file)
	if strings.TrimSpace(file) == "" {
		return "", fmt.Errorf("file is required")
	}
	target, err := app.ParseTarget(file)
	if err != nil {
		return "", err
	}
	var rawDirection string
	stringArg(args, "direction", &rawDirection)
	if rawDirection == "" {
		rawDirection = app.FollowDependents
	}
	direction, err := app.ParseFollowDirection(rawDirection)
	if err != nil {
		return "", err
	}
	depth := 2
	intArg(args, "depth", &depth)
	fullDepth := 1
	intArg(args, "full_depth", &fullDepth)

	runCfg := *cfg
	intArg(args, "budget", &runCfg.Budget)
	stringArg(args, "mode", &runCfg.Mode)
	if runCfg.Mode != "" && runCfg.Mode != "full" && runCfg.Mode != "signatures" {
		return "", fmt.Errorf("invalid mode %q: want full or signatures", runCfg.Mode)
	}
	_, modeGiven := args["mode"]
	runCfg.SmartFilter = true
	runCfg.OutputFile = "-" // never touch disk

	root, err := s.scopeRoot(args)
	if err != nil {
		return "", err
	}
	runCfg.RootDir = root
	absRoot, err := state.CanonicalRoot(root)
	if err != nil {
		return "", err
	}
	seed, err := app.ResolveSeed(absRoot, target.Path)
	if err != nil {
		return "", err
	}
	files, _, err := app.ScanPickerWithCache(ctx, root, &runCfg, s.cache)
	if err != nil {
		return "", err
	}
	seedRel, err := s.resolveFile(root, seed)
	if err != nil {
		return "", err
	}
	seed = seedRel
	seedPresent := false
	for _, f := range files {
		if filepath.ToSlash(f.Path) == seed {
			seedPresent = true
		}
	}
	if !seedPresent {
		return "", fmt.Errorf("seed %q is ignored or outside the collected tree", file)
	}
	if !lang.HasImportScanner(seed) {
		return "", fmt.Errorf("no import resolver for %q; supported: %s",
			filepath.Ext(seed), strings.Join(lang.ImportExtensions(), ", "))
	}

	ranker := app.NewRankerWithWeights(absRoot, app.WeightsFromScoring(runCfg.Scoring))
	preferred, graph := ranker.RankGraph(files)
	if target.IsFocused() {
		for i := range files {
			if filepath.ToSlash(files[i].Path) == seed {
				files[i], err = app.FocusEntry(files[i], target, runCfg.TokenizeModel)
				if err != nil {
					return "", err
				}
				break
			}
		}
	}
	modeOverride := ""
	if modeGiven {
		modeOverride = runCfg.Mode
	}
	sel, err := app.FollowCandidates(files, graph, preferred, seed, direction, depth, fullDepth, modeOverride)
	if err != nil {
		return "", err
	}
	result := selection.Select(sel.Candidates, selection.Request{
		Budget:   runCfg.Budget,
		Overhead: app.EstimatedDocumentOverhead(&runCfg, runCfg.Prompt, files),
		Prompt:   runCfg.Prompt,
		Tuning:   app.TuningFromScoring(runCfg.Scoring),
	})
	wantMode := modeOverride
	if wantMode == "" {
		wantMode = sel.Modes[seed]
	}
	for _, d := range result.Decisions {
		if filepath.ToSlash(d.Path) == seed && d.Selected && string(d.Mode) == wantMode {
			buf, metrics, err := app.RenderBufferWithMetrics(ctx, result.Selected, runCfg.Prompt, &runCfg)
			if err != nil {
				return "", err
			}
			candidates := make([]format.FileEntry, 0, len(sel.Candidates))
			for _, c := range sel.Candidates {
				candidates = append(candidates, c.File)
			}
			return string(buf) + omissionManifest(candidates, result.Selected, runCfg.Budget, metrics), nil
		}
	}
	return "", fmt.Errorf("seed %q alone exceeds the token budget (%d)", file, runCfg.Budget)
}

// handleListTree renders the repository structure without reading file
// contents: indented paths with trailing slashes on directories, plus a
// file/dir count line. WalkMeta only emits files, so directory entries are
// derived from the files' ancestor paths — the tree shows exactly the
// structure sift would pack, honoring every ignore and ext filter.
func handleListTreeSession(ctx context.Context, args map[string]any, s *session) (string, error) {
	cfg := s.cfg
	runCfg := *cfg
	stringArg(args, "ext", &runCfg.Extensions)
	stringArg(args, "ignore", &runCfg.CustomIgnore)
	runCfg.OutputFile = "-" // app.New must not create codebase.md
	root, err := s.scopeRoot(args)
	if err != nil {
		return "", err
	}
	runCfg.RootDir = root

	metas, _, err := app.SkeletonWithCache(ctx, root, &runCfg, s.cache)
	if err != nil {
		return "", err
	}
	var withTokens bool
	boolArg(args, "with_tokens", &withTokens)
	dirs := map[string]bool{}
	for _, m := range metas {
		for d := filepath.Dir(m.Path); d != "."; d = filepath.Dir(d) {
			dirs[filepath.ToSlash(d)] = true
		}
	}
	paths := make([]string, 0, len(metas)+len(dirs))
	for _, m := range metas {
		paths = append(paths, m.Path)
	}
	for d := range dirs {
		paths = append(paths, d)
	}
	sort.Strings(paths) // lexical order puts parents before children

	tokens := map[string]int{}
	if withTokens {
		for _, m := range metas {
			t := int(m.Size) / 4
			if t < 1 {
				t = 1
			}
			tokens[filepath.ToSlash(m.Path)] += t
			tokens["."] += t
			for d := filepath.Dir(filepath.ToSlash(m.Path)); d != "."; d = filepath.Dir(d) {
				tokens[filepath.ToSlash(d)] += t
			}
		}
	}

	var b strings.Builder
	for _, p := range paths {
		depth := strings.Count(p, "/")
		name := filepath.Base(p)
		b.WriteString(strings.Repeat("  ", depth) + name)
		if dirs[p] {
			b.WriteString("/")
			if withTokens {
				fmt.Fprintf(&b, "  ~%d tokens", tokens[p])
			}
		} else if withTokens {
			fmt.Fprintf(&b, "  [%s] ~%d tokens", lang.Classify(p).Role, tokens[p])
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "\n%d files, %d dirs", len(metas), len(dirs))
	if withTokens {
		fmt.Fprintf(&b, ", ~%d tokens", tokens["."])
	}
	return b.String(), nil
}

// The non-session handlers preserve the original call shape for tests and
// one-shot callers; the server always uses the session variants.

func handlePackContext(ctx context.Context, args map[string]any, cfg *config.Config) (string, error) {
	return handlePackContextSession(ctx, args, newSession(cfg))
}

func handlePackDiff(ctx context.Context, args map[string]any, cfg *config.Config) (string, error) {
	return handlePackDiffSession(ctx, args, newSession(cfg))
}

func handlePackFollow(ctx context.Context, args map[string]any, cfg *config.Config) (string, error) {
	return handlePackFollowSession(ctx, args, newSession(cfg))
}

func handleListTree(ctx context.Context, args map[string]any, cfg *config.Config) (string, error) {
	return handleListTreeSession(ctx, args, newSession(cfg))
}

func handleGetSkill(args map[string]any) (string, error) {
	name := "sift-project-dotfile"
	stringArg(args, "name", &name)
	if name == "" {
		name = "sift-project-dotfile"
	}
	content, err := embedded.ReadSkill(name)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// handlePackFilesSession packs an explicit file list. Each file can select its
// own density and an optional symbol or line range; every path is jailed and
// processed through the shared cache before one formatted document is
// rendered.
func handlePackFilesSession(ctx context.Context, args map[string]any, s *session) (string, error) {
	runCfg := *s.cfg
	if _, ok := args["style"]; ok {
		runCfg.JSONOutput = false
		runCfg.MarkdownOutput = false
	}
	stringArg(args, "style", &runCfg.Style)
	stringArg(args, "prompt", &runCfg.Prompt)
	runCfg.OutputFile = "-"

	root, err := s.scopeRoot(args)
	if err != nil {
		return "", err
	}
	runCfg.RootDir = root

	raw, _ := args["files"].([]any)
	if len(raw) == 0 {
		return "", fmt.Errorf("files is required and must be a non-empty array")
	}

	application, err := app.NewBufferedWithCache(&runCfg, s.cache)
	if err != nil {
		return "", err
	}
	defer application.Close()

	entries := make([]format.FileEntry, 0, len(raw))
	for i, item := range raw {
		spec, ok := item.(map[string]any)
		if !ok {
			return "", fmt.Errorf("files[%d] must be an object", i)
		}
		var path, mode, symbol, lines string
		stringArg(spec, "path", &path)
		stringArg(spec, "mode", &mode)
		stringArg(spec, "symbol", &symbol)
		stringArg(spec, "lines", &lines)
		if path == "" {
			return "", fmt.Errorf("files[%d].path is required", i)
		}
		if symbol != "" && lines != "" {
			return "", fmt.Errorf("files[%d]: specify either symbol or lines, not both", i)
		}
		if mode == "" {
			mode = "full"
		}
		if mode != "full" && mode != "signatures" && mode != "outline" {
			return "", fmt.Errorf("files[%d].mode %q: want full, signatures, or outline", i, mode)
		}
		rel, err := s.resolveFile(root, path)
		if err != nil {
			return "", err
		}
		entry, err := application.ReadEntry(rel)
		if err != nil {
			return "", fmt.Errorf("%s: %w", path, err)
		}

		switch mode {
		case "signatures":
			if len(entry.SigContent) > 0 {
				entry.Content = entry.SigContent
				entry.Tokens = entry.TokensSig
				entry.IsCompressed = true
			}
		case "outline":
			entry, err = app.OutlineEntry(entry, runCfg.TokenizeModel)
			if err != nil {
				return "", err
			}
		}

		if symbol != "" || lines != "" {
			spec := rel
			if symbol != "" {
				spec += "#" + symbol
			} else {
				spec += ":" + lines
			}
			target, err := app.ParseTarget(spec)
			if err != nil {
				return "", fmt.Errorf("%s: %w", path, err)
			}
			entry, err = app.FocusEntry(entry, target, runCfg.TokenizeModel)
			if err != nil {
				return "", err
			}
		}
		entries = append(entries, entry)
	}

	buf, _, err := app.RenderBufferWithMetrics(ctx, entries, runCfg.Prompt, &runCfg)
	return string(buf), err
}

// omissionManifest summarizes what a budgeted response included and cut, so an
// agent can ask for a specific omitted file with pack_files.
func omissionManifest(all, selected []format.FileEntry, budget int, metrics app.RenderMetrics) string {
	selectedPaths := make(map[string]bool, len(selected))
	full, sigs := 0, 0
	for _, f := range selected {
		selectedPaths[filepath.ToSlash(f.Path)] = true
		if f.IsCompressed {
			sigs++
		} else {
			full++
		}
	}
	var omitted []format.FileEntry
	for _, f := range all {
		if !selectedPaths[filepath.ToSlash(f.Path)] {
			omitted = append(omitted, f)
		}
	}
	sort.SliceStable(omitted, func(i, j int) bool {
		if omitted[i].TokensFull != omitted[j].TokensFull {
			return omitted[i].TokensFull > omitted[j].TokensFull
		}
		return omitted[i].Path < omitted[j].Path
	})
	var b strings.Builder
	fmt.Fprintf(&b, "\n\n[manifest] included: %d full, %d signatures; tokens: %d", full, sigs, metrics.Tokens)
	if budget > 0 {
		fmt.Fprintf(&b, "/%d", budget)
	}
	fmt.Fprintf(&b, "; secrets redacted: %d; omitted: %d", metrics.Secrets, len(omitted))
	if len(omitted) > 0 {
		b.WriteString("; largest omitted:")
		limit := len(omitted)
		if limit > 8 {
			limit = 8
		}
		for _, f := range omitted[:limit] {
			fmt.Fprintf(&b, " %s (%d)", f.Path, f.TokensFull)
		}
	}
	return b.String()
}
