package serve

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/gitclone"
)

const (
	// cloneTimeout bounds one clone, including a stalled transfer.
	cloneTimeout = 5 * time.Minute
	// maxCloneDepth is the deepest history the UI may request.
	maxCloneDepth = 1000
	// maxClones caps temporary checkouts so a session cannot fill the disk.
	maxClones = 5
)

// cloneEntry is one finished temporary clone owned by this server process.
type cloneEntry struct {
	root    string // resolved checkout path (also the jail extra root)
	name    string
	url     string // credentials stripped
	branch  string
	started time.Time
	tmp     *gitclone.Temp
}

// cloneState is the only mutable per-process state the server keeps, and it
// exists only while clones do: nothing runs when it is empty.
type cloneState struct {
	mu      sync.Mutex
	running bool
	// temps tracks every temp dir created and not yet removed, finished or
	// in flight, so shutdown cleans up even a half-done clone.
	temps   map[string]*gitclone.Temp
	entries map[string]*cloneEntry
}

func newCloneState() *cloneState {
	return &cloneState{temps: map[string]*gitclone.Temp{}, entries: map[string]*cloneEntry{}}
}

func (c *cloneState) isRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// begin claims the single clone slot. It reports why it cannot.
func (c *cloneState) begin() (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return "another clone is already running", false
	}
	if len(c.entries) >= maxClones {
		return "too many temporary clones; remove one first", false
	}
	c.running = true
	return "", true
}

func (c *cloneState) end() {
	c.mu.Lock()
	c.running = false
	c.mu.Unlock()
}

func (c *cloneState) track(t *gitclone.Temp) {
	c.mu.Lock()
	c.temps[t.Dir] = t
	c.mu.Unlock()
}

// discard removes a temp dir that never became a project.
func (c *cloneState) discard(t *gitclone.Temp) {
	_ = t.Remove()
	c.mu.Lock()
	delete(c.temps, t.Dir)
	c.mu.Unlock()
}

func (c *cloneState) add(e *cloneEntry) {
	c.mu.Lock()
	c.entries[e.root] = e
	c.mu.Unlock()
}

// owns reports whether root is a temporary clone of this process.
func (c *cloneState) owns(root string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.entries[root]
	return ok
}

// remove deletes one finished clone.
func (c *cloneState) remove(root string) (*cloneEntry, bool) {
	c.mu.Lock()
	e, ok := c.entries[root]
	if ok {
		delete(c.entries, root)
		delete(c.temps, e.tmp.Dir)
	}
	c.mu.Unlock()
	if ok {
		_ = e.tmp.Remove()
	}
	return e, ok
}

func (c *cloneState) list() []*cloneEntry {
	c.mu.Lock()
	out := make([]*cloneEntry, 0, len(c.entries))
	for _, e := range c.entries {
		out = append(out, e)
	}
	c.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].started.After(out[j].started) })
	return out
}

// removeAll deletes every temp dir. Idempotent; called on shutdown.
func (c *cloneState) removeAll() {
	c.mu.Lock()
	temps := c.temps
	c.temps = map[string]*gitclone.Temp{}
	c.entries = map[string]*cloneEntry{}
	c.mu.Unlock()
	for _, t := range temps {
		_ = t.Remove()
	}
}

// cloneAllowed decides the feature once at startup: --allow-clone plus a
// system git. The UI reads the same answer from /api/meta and hides every
// clone entry point when false.
func (s *Server) cloneAllowed() bool {
	if !s.cfg.AllowClone {
		return false
	}
	return gitclone.Available()
}

func (s *Server) cloneUnavailable(w http.ResponseWriter) bool {
	if s.cloneOK {
		return false
	}
	writeAPIError(w, http.StatusNotFound, "clone is not available")
	return true
}

// handleClone clones a repository into a private temp directory and streams
// NDJSON: {"type":"progress","phase","percent"} lines, then exactly one
// {"type":"done",...} or {"type":"error",...}. Aborting the request cancels
// the clone and removes the checkout.
func (s *Server) handleClone(w http.ResponseWriter, r *http.Request) {
	if s.cloneUnavailable(w) {
		return
	}
	var body struct {
		URL    string `json:"url"`
		Branch string `json:"branch"`
		Depth  int    `json:"depth"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if body.Depth == 0 {
		body.Depth = 1
	}
	if body.Depth < 1 || body.Depth > maxCloneDepth {
		writeAPIError(w, http.StatusBadRequest, "depth must be between 1 and 1000")
		return
	}
	if err := gitclone.ValidateURL(body.URL); err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := gitclone.ValidateRef(body.Branch); err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	if msg, ok := s.clones.begin(); !ok {
		writeAPIError(w, http.StatusConflict, msg)
		return
	}
	defer func() {
		s.clones.end()
		// A long clone is activity: do not let the idle timer count it as quiet.
		s.resetIdle()
	}()

	name := gitclone.RepoName(body.URL)
	tmp, err := gitclone.NewTemp(name)
	if err != nil {
		s.log.Error("clone temp dir", "err", err)
		writeAPIError(w, http.StatusInternalServerError, "cannot create a temporary directory")
		return
	}
	s.clones.track(tmp)
	redacted := gitclone.Redact(body.URL)
	s.log.Info("clone started", "url", redacted, "branch", body.Branch, "depth", body.Depth)

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	rc := http.NewResponseController(w)
	send := func(v map[string]any) {
		if err := writeNDJSON(w, v); err == nil {
			_ = rc.Flush()
		}
	}
	_ = rc.Flush()

	ctx, cancel := context.WithTimeout(r.Context(), cloneTimeout)
	defer cancel()
	// Server shutdown cancels an in-flight clone; this goroutine ends with it.
	go func() {
		select {
		case <-s.done:
			cancel()
		case <-ctx.Done():
		}
	}()

	var last gitclone.Progress
	err = gitclone.Clone(ctx, gitclone.Options{
		URL:    body.URL,
		Branch: body.Branch,
		Depth:  body.Depth,
		Dest:   tmp.Repo,
		OnProgress: func(p gitclone.Progress) {
			if p == last {
				return
			}
			last = p
			send(map[string]any{"type": "progress", "phase": p.Phase, "percent": p.Percent})
		},
	})
	if err != nil {
		s.clones.discard(tmp)
		if errors.Is(err, context.Canceled) {
			s.log.Info("clone cancelled", "url", redacted)
			return // client gone or server stopping: nobody to tell
		}
		s.log.Warn("clone failed", "url", redacted, "err", err)
		send(cloneErrorEvent(err))
		return
	}

	root, err := s.jail.AddRoot(tmp.Repo)
	if err != nil {
		s.clones.discard(tmp)
		s.log.Error("clone root", "err", err)
		send(map[string]any{"type": "error", "message": "cannot open the cloned repository"})
		return
	}
	// A repository can carry its own .sift.toml; refuse it here, at the
	// moment of cloning, rather than as a confusing 403 on open.
	if issues := config.CheckProjectConfig(root); len(issues) > 0 {
		s.jail.RemoveRoot(root)
		s.clones.discard(tmp)
		s.log.Warn("clone refused (config)", "url", redacted)
		send(map[string]any{"type": "error", "message": "this repository's .sift.toml is not allowed here"})
		return
	}
	s.clones.add(&cloneEntry{root: root, name: name, url: redacted, branch: body.Branch, started: time.Now(), tmp: tmp})
	s.log.Info("clone finished", "url", redacted, "name", name)
	send(map[string]any{"type": "done", "root": root, "name": name, "url": redacted})
}

// cloneErrorEvent turns a clone failure into the terminal NDJSON event.
func cloneErrorEvent(err error) map[string]any {
	ev := map[string]any{"type": "error"}
	var f *gitclone.Failure
	switch {
	case errors.Is(err, gitclone.ErrNoGit):
		ev["message"] = "git is not installed on the server"
	case errors.Is(err, context.DeadlineExceeded):
		ev["message"] = "The clone took too long and was stopped (limit 5 minutes)."
	case errors.As(err, &f):
		ev["message"] = "git could not clone the repository."
		if f.Detail != "" {
			ev["detail"] = f.Detail
		}
	default:
		ev["message"] = err.Error()
	}
	return ev
}

// handleClonesGet lists this session's temporary clones, newest first.
func (s *Server) handleClonesGet(w http.ResponseWriter, r *http.Request) {
	if s.cloneUnavailable(w) {
		return
	}
	type item struct {
		Root    string `json:"root"`
		Name    string `json:"name"`
		URL     string `json:"url"`
		Branch  string `json:"branch,omitempty"`
		Started int64  `json:"started"`
	}
	out := []item{}
	for _, e := range s.clones.list() {
		out = append(out, item{Root: e.root, Name: e.name, URL: e.url, Branch: e.branch, Started: e.started.UnixMilli()})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleCloneDelete removes one temporary clone. Only checkouts this server
// created can be named; anything else is a 404 and nothing is touched.
func (s *Server) handleCloneDelete(w http.ResponseWriter, r *http.Request) {
	if s.cloneUnavailable(w) {
		return
	}
	root := r.URL.Query().Get("root")
	e, ok := s.clones.remove(root)
	if !ok {
		writeAPIError(w, http.StatusNotFound, "no such clone")
		return
	}
	s.jail.RemoveRoot(e.root)
	s.log.Info("clone removed", "name", e.name)
	w.WriteHeader(http.StatusNoContent)
}
