package serve

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/pflag"

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

// defaultWebBudget is the token budget the web client displays and sends
// when nothing else sets one (no flag, no .sift.toml, no global file, no
// persisted server default). It matches the workspace's initial value.
const defaultWebBudget = 64000

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeNDJSON writes one JSON value and a newline (the caller flushes).
func writeNDJSON(w http.ResponseWriter, v any) error {
	return json.NewEncoder(w).Encode(v)
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

// engineConfigFor resolves the engine config for one request exactly like
// the CLI resolves it inside the target directory: built-in defaults, the
// global file, the TARGET project's own .sift.toml (never the server's
// startup directory), then the operator's explicit serve flags. Output sinks
// stay disabled. Roots and selections always come from the jailed request,
// never from globals.
func (s *Server) engineConfigFor(root string) (*config.Config, error) {
	runCfg := config.New()
	fs := pflag.NewFlagSet("serve-request", pflag.ContinueOnError)
	config.RegisterFlags(runCfg, fs)
	runCfg.RootDir = root
	// Pinned to the target root: unlike localConfigPathFor there is no cwd
	// fallback, so the server's startup directory can never leak its
	// .sift.toml into projects that have none.
	if err := config.ResolveConfig(runCfg, fs, config.WithLocalConfig(filepath.Join(root, ".sift.toml"))); err != nil {
		return nil, err
	}
	for name, val := range s.cfg.EngineFlagOverrides {
		if fs.Lookup(name) == nil {
			continue
		}
		if err := fs.Set(name, val); err != nil {
			return nil, err
		}
	}
	runCfg.RootDir = root
	runCfg.OutputFile = "-"
	runCfg.Clipboard = false
	runCfg.CopyOnGenerate = false
	runCfg.ShowProgress = false
	runCfg.Quiet = true
	return runCfg, nil
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
		_, app, _ := s.auth.SessionState(r)
		meta["roots"] = s.jail.Roots()
		meta["styles"] = []string{"xml", "markdown", "plain"}
		meta["defaultBrowse"] = s.defaultBrowse()
		// app is read from the signed session, not a URL hint, so it survives
		// reloads. Drives the Quit button and the liveness stream.
		meta["app"] = app
		// Feature flags the UI uses to hide entry points that would 404.
		meta["features"] = map[string]bool{"clone": s.cloneOK}
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
	// Temporary clones vanish when the server stops; recents persist on
	// disk, so recording one would only leave a dead entry behind.
	if s.clones.owns(root) {
		w.WriteHeader(http.StatusNoContent)
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
	// Parent stays inside the jail: at an allowed root there is no way up,
	// so the client hides its parent row instead of offering a dead end.
	parent := ""
	if p := filepath.Dir(dir); p != dir {
		if _, err := s.jail.Resolve(p); err == nil {
			parent = p
		}
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
	cfg, err := s.engineConfigFor(root)
	if err != nil {
		s.log.Warn("config resolve", "root", relForm(r.URL.Query().Get("root")), "err", err)
		writeAPIError(w, http.StatusInternalServerError, "scan failed")
		return
	}
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
	// The budget rides along so the client displays and sends the resolved
	// value instead of a hardcoded guess (see resolveBudget).
	budget, source := s.resolveBudget(root, cfg)
	writeJSON(w, http.StatusOK, map[string]any{"root": root, "files": out, "budget": budget, "budgetSource": source})
}

// resolveBudget reports the token budget the web client should display and
// send back with pack/smart-select. Precedence: serve flag > project
// .sift.toml > global file > persisted server default > builtin. The
// .sift.toml wins over the persisted default so per-project files stay
// authoritative; an explicit request budget still overrides everything
// downstream. resolveBudget never fails: unreadable preferences fall back
// to the builtin.
func (s *Server) resolveBudget(root string, cfg *config.Config) (budget int, source string) {
	if _, ok := s.cfg.EngineFlagOverrides["budget"]; ok && cfg.Budget != 0 {
		return cfg.Budget, "flag"
	}
	if b, err := config.LocalBudget(filepath.Join(root, ".sift.toml")); err == nil && b != 0 {
		return b, "toml"
	}
	if cfg.Budget != 0 {
		return cfg.Budget, "default"
	}
	if prefs, err := state.LoadPreferences(); err == nil && prefs.DefaultBudget != 0 {
		return prefs.DefaultBudget, "default"
	}
	return defaultWebBudget, "default"
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
	cfg, err := s.engineConfigFor(root)
	if err != nil {
		s.log.Warn("config resolve", "root", relForm(q.Get("root")), "err", err)
		writeAPIError(w, http.StatusForbidden, "cannot read that file")
		return
	}
	content, tokens, _, language, truncated, err := app.ReadPreview(root, filepath.ToSlash(rel), mode, previewCap, cfg)
	if err != nil {
		writeAPIError(w, http.StatusForbidden, "cannot read that file")
		return
	}
	// Spans come from the same highlight engine as the TUI preview, parsed
	// on the exact redacted bytes sent below so offsets always line up.
	absPath := filepath.Join(root, rel)
	writeJSON(w, http.StatusOK, map[string]any{
		"content":   string(content),
		"tokens":    tokens,
		"language":  language,
		"truncated": truncated,
		"spans":     fileSpans(absPath, content, s.syntaxCache),
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
	cfg, err := s.engineConfigFor(root)
	if err != nil {
		s.log.Warn("config resolve", "root", relForm(body.Root), "err", err)
		writeAPIError(w, http.StatusInternalServerError, "scan failed")
		return
	}
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

	cfg, err := s.engineConfigFor(root)
	if err != nil {
		s.log.Warn("config resolve", "root", relForm(body.Root), "err", err)
		writeAPIError(w, http.StatusInternalServerError, "scan failed")
		return
	}
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
	// Always an array on the wire: a null skipped crashed the output tab.
	skipped := []string{}
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

	doc, fileSections, err := app.RenderBufferSections(r.Context(), kept, body.Prompt, cfg)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "render failed")
		return
	}
	redactions := 0
	for _, f := range kept {
		redactions += f.SecretCount
	}
	sort.Strings(skipped)
	// Sections are byte ranges into document for outline navigation. Always
	// an array on the wire, matching the skipped contract.
	outSections := make([]packSection, 0, len(fileSections))
	for _, fs := range fileSections {
		outSections = append(outSections, packSection{
			Path:   filepath.ToSlash(fs.Path),
			Tokens: fs.Tokens,
			Start:  fs.Start,
			End:    fs.End,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"document":   string(doc),
		"tokens":     used,
		"fileCount":  len(kept),
		"redactions": redactions,
		"skipped":    skipped,
		"sections":   outSections,
	})
}

// packSection is one file's byte range inside a packed document.
type packSection struct {
	Path   string `json:"path"`
	Tokens int    `json:"tokens"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
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
		"defaultBudget": firstNonZero(prefs.DefaultBudget, defaultWebBudget),
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
