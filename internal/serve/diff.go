package serve

import (
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/rank"
)

// handleDiff renders a context document for changed files. It shares the pack
// semaphore and request cancellation, and never passes a raw ref to git: the
// hardened rank.Git resolver validates it first.
func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Root        string `json:"root"`
		From        string `json:"from"`
		Patch       *bool  `json:"patch"`
		WithContext bool   `json:"withContext"`
		Budget      int    `json:"budget"`
		Style       string `json:"style"`
		Prompt      string `json:"prompt"`
		Redact      *bool  `json:"redact"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	root, err := s.openProject(body.Root)
	if err != nil {
		s.log.Warn("denied diff", "root", relForm(body.Root))
		writeAPIError(w, http.StatusForbidden, "cannot open that project")
		return
	}
	redact := true
	if body.Redact != nil {
		redact = *body.Redact
	}
	if !redact {
		if s.cfg.Pair {
			writeAPIError(w, http.StatusBadRequest, "unredacted output is disabled on this server")
			return
		}
		if s.remote && !s.cfg.AllowUnredacted {
			writeAPIError(w, http.StatusBadRequest, "unredacted output is disabled on this server")
			return
		}
	}

	select {
	case s.packSem <- struct{}{}:
		defer func() { <-s.packSem }()
	default:
		writeAPIError(w, http.StatusServiceUnavailable, "server busy, try again")
		return
	}

	includePatch := true
	if body.Patch != nil {
		includePatch = *body.Patch
	}
	from := strings.TrimSpace(body.From)
	if strings.HasPrefix(from, "-") {
		writeAPIError(w, http.StatusBadRequest, "invalid git ref")
		return
	}
	g := rank.New(root)
	if !g.Available() {
		writeAPIError(w, http.StatusBadRequest, "not a git repository")
		return
	}
	var changed []string
	var patch string
	if from != "" {
		if _, ok := g.ResolveRef(from); !ok {
			writeAPIError(w, http.StatusBadRequest, "unknown git ref")
			return
		}
		changed = g.ChangedBetween(from, "HEAD")
		if includePatch {
			patch = g.RawPatch(from, "HEAD")
		}
	} else {
		changed = g.ChangedSinceRef("HEAD")
		if includePatch {
			patch = g.RawWorktreePatch("HEAD")
		}
	}
	if len(changed) == 0 && patch == "" {
		writeEmptyDiff(w)
		return
	}

	cfg, err := s.engineConfigFor(root)
	if err != nil {
		s.log.Warn("config resolve", "root", relForm(body.Root), "err", err)
		writeAPIError(w, http.StatusInternalServerError, "scan failed")
		return
	}
	if body.Style != "" {
		cfg.Style = body.Style
	}
	if body.Budget > 0 {
		cfg.Budget = body.Budget
	}
	if !redact {
		cfg.SecretScan = false
	}

	var files []format.FileEntry
	if !includePatch || body.WithContext {
		files, _, err = app.ScanPickerWithCache(r.Context(), root, cfg, s.contentCache)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "scan failed")
			return
		}
		if r.Context().Err() != nil {
			writeAPIError(w, 499, "client closed request")
			return
		}
	}

	var chosen []format.FileEntry
	if !includePatch {
		byPath := make(map[string]format.FileEntry, len(files))
		for _, f := range files {
			byPath[filepath.ToSlash(f.Path)] = f
		}
		for _, p := range changed {
			if f, ok := byPath[filepath.ToSlash(p)]; ok {
				f.Tokens = f.TokensFull
				f.IsCompressed = false
				chosen = append(chosen, f)
			}
		}
	}
	var neighbors []format.FileEntry
	if includePatch && body.WithContext {
		neighbors = app.DiffNeighborFiles(cfg, files, changed, 1)
	} else if !includePatch && body.WithContext {
		neighbors = app.DiffNeighborFiles(cfg, files, changed, 1)
		chosen = append(chosen, neighbors...)
	}

	application, err := app.NewBufferedWithCache(cfg, s.contentCache)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "render failed")
		return
	}
	defer application.Close()

	var doc []byte
	var sections []format.Section
	var metrics app.RenderMetrics
	if includePatch {
		doc, sections, metrics, err = application.RenderPatchBufferSections(r.Context(), neighbors, patch, body.Prompt)
	} else {
		doc, sections, metrics, err = application.RenderFilesBufferSections(r.Context(), chosen, body.Prompt)
	}
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "render failed")
		return
	}

	skipped := []string{}
	if len(files) > 0 {
		have := make(map[string]bool, len(files))
		for _, f := range files {
			have[filepath.ToSlash(f.Path)] = true
		}
		for _, p := range changed {
			p = filepath.ToSlash(p)
			if !have[p] {
				skipped = append(skipped, p)
			}
		}
	}
	sort.Strings(skipped)
	outSections := make([]packSection, 0, len(sections))
	for _, fs := range sections {
		outSections = append(outSections, packSection{
			Path:   filepath.ToSlash(fs.Path),
			Tokens: fs.Tokens,
			Start:  fs.Start,
			End:    fs.End,
		})
	}
	fileCount := metrics.Files
	if includePatch {
		fileCount += len(changed)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"document":   string(doc),
		"tokens":     metrics.Tokens + metrics.PatchTokens,
		"fileCount":  fileCount,
		"redactions": metrics.Secrets,
		"skipped":    skipped,
		"sections":   outSections,
	})
}

func writeEmptyDiff(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, map[string]any{
		"document":   "",
		"tokens":     0,
		"fileCount":  0,
		"redactions": 0,
		"skipped":    []string{},
		"sections":   []packSection{},
	})
}
