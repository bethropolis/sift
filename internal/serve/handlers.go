package serve

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bethropolis/sift/internal/app"
	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/selection"
	"github.com/bethropolis/sift/internal/serve/auth"
	"github.com/bethropolis/sift/internal/state"
	"github.com/bethropolis/sift/internal/tokenize"
)

// browseCap bounds directory listings.
const browseCap = 5000

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeAPIError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// decodeBody parses a size-capped JSON body.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

// engineConfigFor returns a per-request engine config: the shared defaults
// with output sinks disabled. Roots and selections always come from the
// jailed request, never from globals.
func (s *Server) engineConfigFor(root string) *config.Config {
	cfg := *s.engineCfg
	cfg.RootDir = root
	cfg.OutputFile = "-"
	cfg.Clipboard = false
	cfg.CopyOnGenerate = false
	cfg.ShowProgress = false
	cfg.Quiet = true
	return &cfg
}

// handleMeta serves version/mode/TLS/auth facts. Unauthenticated callers get
// the minimal set (no paths); the UI status pill and TLS banner read it.
func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	authed, _ := s.auth.ValidSession(r)
	meta := map[string]any{
		"version":       s.engineCfg.Version,
		"mode":          "local",
		"tls":           s.tls,
		"authKind":      s.auth.Kind(),
		"authenticated": authed,
	}
	if s.remote {
		meta["mode"] = "remote"
	}
	if authed {
		meta["roots"] = s.jail.Roots()
		meta["styles"] = []string{"xml", "markdown", "plain"}
		meta["defaultBrowse"] = s.defaultBrowse()
	}
	writeJSON(w, http.StatusOK, meta)
}

// defaultBrowse prefers ~/Projects when it exists inside the jail, falling
// back to the first allowed root so the folder browser never opens on a
// missing directory.
func (s *Server) defaultBrowse() string {
	roots := s.jail.Roots()
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if proj, err := s.jail.Resolve(filepath.Join(home, "Projects")); err == nil {
			return proj
		}
	}
	if len(roots) > 0 {
		return roots[0]
	}
	return ""
}

// handleLogin exchanges a token or password for a session cookie. The token
// travels in the POST body (the URL fragment never reaches the server).
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
		Token    string `json:"token"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	ip := auth.ClientIP(r, s.cfg.BehindProxy)
	if retry, ok := s.auth.CheckLogin(ip); !ok {
		s.log.Warn("login throttled", "ip", ip)
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"retryAfter": retry})
		return
	}

	valid := false
	if body.Token != "" {
		valid = s.auth.VerifyToken(body.Token)
	} else {
		valid = s.auth.VerifyPassword(body.Password)
	}
	if !valid {
		wait := s.auth.RecordResult(ip, false)
		s.log.Warn("login failed", "ip", ip)
		if wait > 0 {
			writeJSON(w, http.StatusTooManyRequests, map[string]any{"retryAfter": wait})
			return
		}
		writeAPIError(w, http.StatusUnauthorized, "wrong credentials")
		return
	}
	s.auth.RecordResult(ip, true)
	s.log.Info("login success", "ip", ip)
	c := s.auth.MintCookie()
	s.secureCookie(c)
	if s.secureProxyRequest(r) {
		c.Secure = true
	}
	http.SetCookie(w, c)
	w.WriteHeader(http.StatusNoContent)
}

// handleLogout clears the session cookie.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	c := &http.Cookie{Name: auth.CookieName, Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode}
	http.SetCookie(w, c)
	w.WriteHeader(http.StatusNoContent)
}

// handleRecentsGet lists shared recents (most recent first, cap 25).
func (s *Server) handleRecentsGet(w http.ResponseWriter, r *http.Request) {
	recents, err := state.ListRecents()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "cannot list recents")
		return
	}
	out := make([]map[string]string, 0, len(recents))
	for _, rc := range recents {
		out = append(out, map[string]string{
			"root":       rc.Root,
			"name":       rc.Name,
			"lastOpened": rc.LastOpened,
			"branch":     rc.Branch,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleRecentsPost records an open. GETs never mutate; the root must pass
// the jail and the project config check.
func (s *Server) handleRecentsPost(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Root string `json:"root"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	root, err := s.jail.Resolve(body.Root)
	if err != nil {
		s.log.Warn("denied project open", "root", relForm(body.Root))
		writeAPIError(w, http.StatusForbidden, "cannot open that path")
		return
	}
	if issues := config.CheckProjectConfig(root); len(issues) > 0 {
		s.log.Warn("denied project open (config)", "root", relForm(root))
		writeAPIError(w, http.StatusForbidden, "cannot open that path")
		return
	}
	state.RecordRecent(root)
	w.WriteHeader(http.StatusNoContent)
}

// handleRecentsDelete removes one recent.
func (s *Server) handleRecentsDelete(w http.ResponseWriter, r *http.Request) {
	root := r.URL.Query().Get("root")
	if root == "" {
		writeAPIError(w, http.StatusBadRequest, "missing root")
		return
	}
	if err := state.RemoveRecent(root); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "cannot remove recent")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleBrowse lists directories only, capped at 5000 entries. isGitRepo is
// decided by lstat(.git): no git subprocess, no writes. Dotfiles are hidden
// unless ?hidden=1 (the folder browser passes the ShowHidden setting).
func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	dir, err := s.jail.Resolve(r.URL.Query().Get("path"))
	if err != nil {
		s.log.Warn("denied browse", "path", relForm(r.URL.Query().Get("path")))
		writeAPIError(w, http.StatusForbidden, "cannot read that folder")
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		writeAPIError(w, http.StatusForbidden, "cannot read that folder")
		return
	}
	showHidden := r.URL.Query().Get("hidden") == "1"
	type entry struct {
		Name      string `json:"name"`
		IsDir     bool   `json:"isDir"`
		IsGitRepo bool   `json:"isGitRepo"`
		ModTime   int64  `json:"modTime"`
	}
	out := make([]entry, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if !showHidden && strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if len(out) >= browseCap {
			break
		}
		full := filepath.Join(dir, e.Name())
		gitMarker := filepath.Join(full, ".git")
		_, statErr := os.Lstat(gitMarker)
		var modTime int64
		if info, infoErr := e.Info(); infoErr == nil {
			modTime = info.ModTime().UnixMilli()
		}
		out = append(out, entry{Name: e.Name(), IsDir: true, IsGitRepo: statErr == nil, ModTime: modTime})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	parent := ""
	if p := filepath.Dir(dir); p != dir {
		parent = p
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": dir, "parent": parent, "entries": out})
}

// openProject resolves and validates a ?root= for the data endpoints.
func (s *Server) openProject(rootParam string) (string, error) {
	root, err := s.jail.Resolve(rootParam)
	if err != nil {
		return "", err
	}
	if issues := config.CheckProjectConfig(root); len(issues) > 0 {
		return "", fmt.Errorf("config escapes")
	}
	return root, nil
}

// handleTree returns files with TUI-identical tokens and scores.
func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	root, err := s.openProject(r.URL.Query().Get("root"))
	if err != nil {
		s.log.Warn("denied tree", "root", relForm(r.URL.Query().Get("root")))
		writeAPIError(w, http.StatusForbidden, "cannot open that project")
		return
	}
	cfg := s.engineConfigFor(root)
	files, _, err := app.ScanPicker(r.Context(), root, cfg)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "scan failed")
		return
	}
	type file struct {
		Path     string  `json:"path"`
		Size     int     `json:"size"`
		Tokens   int     `json:"tokens"`
		Language string  `json:"language"`
		Score    float64 `json:"score"`
	}
	out := make([]file, 0, len(files))
	for _, f := range files {
		out = append(out, file{
			Path:     filepath.ToSlash(f.Path),
			Size:     len(f.Content),
			Tokens:   f.TokensFull,
			Language: f.Language,
			Score:    f.RankScore,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"root": root, "files": out})
}

// handleFile returns a redacted, capped preview with a truncation flag.
func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	root, err := s.openProject(q.Get("root"))
	if err != nil {
		s.log.Warn("denied file", "root", relForm(q.Get("root")))
		writeAPIError(w, http.StatusForbidden, "cannot open that project")
		return
	}
	mode := q.Get("mode")
	if mode != "full" && mode != "sigs" {
		writeAPIError(w, http.StatusBadRequest, "mode must be full or sigs")
		return
	}
	rel, err := filepath.Rel(root, mustResolve(s, root, q.Get("path")))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		writeAPIError(w, http.StatusForbidden, "cannot read that file")
		return
	}
	cfg := s.engineConfigFor(root)
	content, tokens, _, language, truncated, err := app.ReadPreview(root, filepath.ToSlash(rel), mode, previewCap, cfg)
	if err != nil {
		writeAPIError(w, http.StatusForbidden, "cannot read that file")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"content":   string(content),
		"tokens":    tokens,
		"language":  language,
		"truncated": truncated,
	})
}

// mustResolve resolves a file path for the file endpoint: absolute paths go
// through the jail directly, project-relative paths are joined to the
// already-jailed root first. Failures yield "".
func mustResolve(s *Server, root, p string) string {
	if p == "" {
		return ""
	}
	if !filepath.IsAbs(filepath.FromSlash(p)) {
		p = filepath.Join(root, filepath.FromSlash(p))
	}
	resolved, err := s.jail.Resolve(p)
	if err != nil {
		return ""
	}
	return resolved
}

// handleSmartSelect ranks the project and returns per-file selections.
func (s *Server) handleSmartSelect(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Root   string `json:"root"`
		Budget int    `json:"budget"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	root, err := s.openProject(body.Root)
	if err != nil {
		s.log.Warn("denied smart-select", "root", relForm(body.Root))
		writeAPIError(w, http.StatusForbidden, "cannot open that project")
		return
	}
	cfg := s.engineConfigFor(root)
	if body.Budget > 0 {
		cfg.Budget = body.Budget
	}
	files, _, err := app.ScanPicker(r.Context(), root, cfg)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "scan failed")
		return
	}
	absRoot := root
	ranker := app.NewRankerWithWeights(absRoot, app.WeightsFromScoring(cfg.Scoring))
	preferred, graph := ranker.RankGraph(files)

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
	result := app.SelectWithDependencies(app.DependencyRequest{
		Candidates: candidates,
		Files:      files,
		Graph:      graph,
		Preferred:  preferred,
		Budget:     cfg.Budget,
		Tuning:     app.TuningFromScoring(cfg.Scoring),
		MaxDepth:   cfg.MaxDepth,
	})
	selections := make(map[string]string, len(result.Decisions))
	for _, d := range result.Decisions {
		mode := string(d.Mode)
		if mode == "signatures" {
			mode = "sigs"
		}
		selections[filepath.ToSlash(d.Path)] = mode
	}
	writeJSON(w, http.StatusOK, map[string]any{"selections": selections})
}

// handlePack renders the browser's explicit selections into the document.
// At most 2 packs run concurrently; request cancellation stops the work.
func (s *Server) handlePack(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Root       string            `json:"root"`
		Selections map[string]string `json:"selections"`
		Budget     int               `json:"budget"`
		Style      string            `json:"style"`
		Prompt     string            `json:"prompt"`
		Redact     *bool             `json:"redact"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	root, err := s.openProject(body.Root)
	if err != nil {
		s.log.Warn("denied pack", "root", relForm(body.Root))
		writeAPIError(w, http.StatusForbidden, "cannot open that project")
		return
	}
	redact := true
	if body.Redact != nil {
		redact = *body.Redact
	}
	if !redact {
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

	cfg := s.engineConfigFor(root)
	if body.Style != "" {
		cfg.Style = body.Style
	}
	if !redact {
		cfg.SecretScan = false
	}
	files, _, err := app.ScanPicker(r.Context(), root, cfg)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "scan failed")
		return
	}

	// Map the browser's per-file modes onto scanned entries (TUI parity).
	byPath := make(map[string]format.FileEntry, len(files))
	for _, f := range files {
		byPath[filepath.ToSlash(f.Path)] = f
	}
	chosen := make([]format.FileEntry, 0, len(body.Selections))
	var skipped []string
	for path, mode := range body.Selections {
		if r.Context().Err() != nil {
			writeAPIError(w, 499, "client closed request")
			return
		}
		f, ok := byPath[path]
		if !ok {
			skipped = append(skipped, path)
			continue
		}
		switch strings.ToLower(mode) {
		case "sigs":
			if f.SigContent != nil {
				f.Content = f.SigContent
				f.Tokens = f.TokensSig
				f.IsCompressed = true
			} else {
				f.Tokens = f.TokensFull
			}
		case "skip", "":
			skipped = append(skipped, path)
			continue
		default: // full
			f.Tokens = f.TokensFull
			f.IsCompressed = false
		}
		chosen = append(chosen, f)
	}

	kept := chosen
	used := 0
	for _, f := range chosen {
		used += f.Tokens
	}
	if body.Budget > 0 {
		kept, used = tokenize.FitToBudget(chosen, body.Budget)
		if len(kept) < len(chosen) {
			have := map[string]bool{}
			for _, f := range kept {
				have[filepath.ToSlash(f.Path)] = true
			}
			for _, f := range chosen {
				if !have[filepath.ToSlash(f.Path)] {
					skipped = append(skipped, filepath.ToSlash(f.Path))
				}
			}
		}
	}

	doc, err := app.RenderBuffer(r.Context(), kept, body.Prompt, cfg)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "render failed")
		return
	}
	redactions := 0
	for _, f := range kept {
		redactions += f.SecretCount
	}
	sort.Strings(skipped)
	writeJSON(w, http.StatusOK, map[string]any{
		"document":   string(doc),
		"tokens":     used,
		"fileCount":  len(kept),
		"redactions": redactions,
		"skipped":    skipped,
	})
}

// handleSettingsGet returns shared defaults.
func (s *Server) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	prefs, err := state.LoadPreferences()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "cannot load settings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"defaultStyle":  firstNonEmpty(prefs.DefaultStyle, "xml"),
		"defaultBudget": firstNonZero(prefs.DefaultBudget, 64000),
		"theme":         firstNonEmpty(state.EffectiveTheme(prefs), "system"),
		"showHidden":    prefs.ShowHidden,
		"fileSort":      firstNonEmpty(prefs.FileSort, "name"),
	})
}

// handleSettingsPut validates and stores shared defaults.
func (s *Server) handleSettingsPut(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DefaultStyle  string `json:"defaultStyle"`
		DefaultBudget int    `json:"defaultBudget"`
		Theme         string `json:"theme"`
		ShowHidden    *bool  `json:"showHidden"`
		FileSort      string `json:"fileSort"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	prefs, err := state.LoadPreferences()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "cannot load settings")
		return
	}
	if body.DefaultStyle != "" {
		prefs.DefaultStyle = body.DefaultStyle
	}
	if body.DefaultBudget != 0 {
		if body.DefaultBudget < 1000 || body.DefaultBudget > 4000000 {
			writeAPIError(w, http.StatusBadRequest, "defaultBudget out of range")
			return
		}
		prefs.DefaultBudget = body.DefaultBudget
	}
	if body.Theme != "" {
		// One id drives both frontends: mirror it to the legacy TUI field
		// so older builds follow a browser theme change.
		prefs.Theme = body.Theme
		prefs.UITheme = body.Theme
	}
	if body.ShowHidden != nil {
		prefs.ShowHidden = *body.ShowHidden
	}
	if body.FileSort != "" {
		prefs.FileSort = body.FileSort
	}
	prefs = state.SanitizePreferences(prefs)
	if body.Theme != "" && prefs.Theme == "" {
		writeAPIError(w, http.StatusBadRequest, "bad theme id")
		return
	}
	if body.FileSort != "" && prefs.FileSort == "" {
		writeAPIError(w, http.StatusBadRequest, "bad fileSort")
		return
	}
	if err := state.SavePreferences(prefs); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "cannot save settings")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// relForm trims absolute paths in logs to their last two segments so denied
// paths never leak directory structure into stderr.
func relForm(p string) string {
	p = filepath.ToSlash(p)
	if i := strings.LastIndex(p, "/"); i >= 0 {
		if j := strings.LastIndex(p[:i], "/"); j >= 0 {
			return "…" + p[j:]
		}
		return p[i+1:]
	}
	return p
}

func firstNonEmpty(v, def string) string {
	if v != "" {
		return v
	}
	return def
}

func firstNonZero(v, def int) int {
	if v != 0 {
		return v
	}
	return def
}
