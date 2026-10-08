// Package gitclone is the one place sift runs `git clone`. The CLI
// (`sift clone`) and the web server (`POST /api/clone`) both go through it, so
// URL validation, hardening, and git invocation have a single copy to audit.
//
// Only remote repositories are accepted: https, http, ssh, git, and scp-like
// `user@host:path`. Local paths and file:// URLs are refused everywhere (there
// is no legitimate need to "clone" a directory that is already on disk, and a
// web endpoint must never be talked into reading one), as are git's
// `<helper>::<address>` remote-helper syntax (`ext::` runs commands) and
// anything that could be mistaken for a command-line option.
//
// Authentication is left entirely to the system git: credential helpers,
// ~/.gitconfig, and the ssh agent all work as they do in a terminal.
package gitclone

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// allowedProtocols is exported to git as GIT_ALLOW_PROTOCOL so a hostile
// repository cannot pull in file:// or ext:: submodules or redirects.
const allowedProtocols = "http:https:ssh:git"

// maxURLLen bounds what we hand to git.
const maxURLLen = 2048

// tailLines is how many non-progress stderr lines are kept for error detail.
const tailLines = 6

// ErrNoGit means the git executable is not on PATH.
var ErrNoGit = errors.New("git is not installed or not on PATH")

// Available reports whether a usable git executable exists. The web UI hides
// every clone entry point when it does not.
func Available() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

var (
	helperSyntax = regexp.MustCompile(`^[A-Za-z0-9+.-]+::`)
	scpHost      = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	progressLine = regexp.MustCompile(`^(.*?):\s+(\d{1,3})%`)
)

// ValidateURL accepts only remote repository URLs. The returned message is
// safe to show to the user.
func ValidateURL(raw string) error {
	s := strings.TrimSpace(raw)
	switch {
	case s == "":
		return errors.New("enter a repository URL")
	case len(s) > maxURLLen:
		return errors.New("repository URL is too long")
	case strings.HasPrefix(s, "-"):
		return errors.New("repository URL cannot start with '-'")
	}
	for _, r := range s {
		if r <= ' ' || r == 0x7f {
			return errors.New("repository URL cannot contain spaces or control characters")
		}
	}
	if helperSyntax.MatchString(s) {
		return errors.New("remote-helper URLs (transport::address) are not supported")
	}

	if strings.Contains(s, "://") {
		u, err := url.Parse(s)
		if err != nil {
			return errors.New("not a valid repository URL")
		}
		switch strings.ToLower(u.Scheme) {
		case "https", "http", "ssh", "git":
		case "file":
			return errors.New("local repositories are not supported; use a remote URL")
		default:
			return fmt.Errorf("unsupported URL scheme %q (use https, ssh, or git)", u.Scheme)
		}
		if u.Hostname() == "" || strings.HasPrefix(u.Hostname(), "-") {
			return errors.New("repository URL has no valid host")
		}
		if _, hasPassword := u.User.Password(); hasPassword {
			return errors.New("don't put credentials in the URL; use a git credential helper or ssh")
		}
		return nil
	}

	// scp-like [user@]host:path. Git's own rule: a colon before any slash.
	colon := strings.Index(s, ":")
	slash := strings.Index(s, "/")
	if colon <= 0 || (slash >= 0 && slash < colon) {
		return errors.New("local paths are not supported; use a remote URL like https://host/owner/repo")
	}
	host := s[:colon]
	if at := strings.LastIndex(host, "@"); at >= 0 {
		host = host[at+1:]
	}
	if host == "" || strings.HasPrefix(host, "-") || !scpHost.MatchString(host) {
		return errors.New("repository URL has no valid host")
	}
	if colon == len(s)-1 {
		return errors.New("repository URL has no path")
	}
	return nil
}

// ValidateRef checks a branch or tag name before it is passed to --branch.
func ValidateRef(ref string) error {
	if ref == "" {
		return nil
	}
	if len(ref) > 255 || strings.HasPrefix(ref, "-") {
		return errors.New("invalid branch or tag name")
	}
	for _, r := range ref {
		if r <= ' ' || r == 0x7f {
			return errors.New("invalid branch or tag name")
		}
	}
	return nil
}

// Redact strips credentials from a URL for logs and UI. Anything that fails
// to parse is returned without its userinfo as best effort.
func Redact(raw string) string {
	s := strings.TrimSpace(raw)
	if !strings.Contains(s, "://") {
		return s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "(unparseable url)"
	}
	u.User = nil
	return u.String()
}

// RepoName derives a short directory/display name from a repository URL
// ("https://host/owner/repo.git" → "repo"), falling back to "repo".
func RepoName(raw string) string {
	s := strings.TrimSpace(raw)
	path := s
	if strings.Contains(s, "://") {
		if u, err := url.Parse(s); err == nil {
			path = u.Path
		}
	} else if i := strings.Index(s, ":"); i >= 0 {
		path = s[i+1:]
	}
	name := filepath.Base(strings.TrimRight(filepath.ToSlash(path), "/"))
	name = strings.TrimSuffix(name, ".git")
	name = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			return r
		}
		return '_'
	}, name)
	name = strings.TrimLeft(name, ".")
	if name == "" || name == "_" {
		return "repo"
	}
	return name
}

// Temp is a throwaway checkout location, created like the CLI always has: a
// private sift-clone-* directory under the OS temp dir holding one repo dir.
// An OS temp cleaner (or reboot) reclaims anything a crash leaves behind.
type Temp struct {
	// Dir is the private parent directory that Remove deletes.
	Dir string
	// Repo is the (not yet existing) checkout path inside Dir.
	Repo string
}

// NewTemp creates the private parent directory (mode 0700).
func NewTemp(name string) (*Temp, error) {
	dir, err := os.MkdirTemp("", "sift-clone-")
	if err != nil {
		return nil, fmt.Errorf("create temporary clone directory: %w", err)
	}
	if name == "" {
		name = "repo"
	}
	return &Temp{Dir: dir, Repo: filepath.Join(dir, name)}, nil
}

// Remove deletes the checkout and its parent. Safe to call more than once.
func (t *Temp) Remove() error { return os.RemoveAll(t.Dir) }

// Options configures one clone.
type Options struct {
	URL    string
	Branch string
	// Depth is the history depth; values below 1 are rejected.
	Depth int
	// Dest is the checkout path; it must not exist (or be empty).
	Dest string
	// Interactive lets git use the caller's terminal for prompts (the CLI).
	// Otherwise git never prompts and runs without a controlling terminal, so
	// a repository that needs credentials fails fast instead of hanging.
	Interactive bool
	Stdin       io.Reader
	// Output receives git's raw stderr/stdout (the CLI passes the terminal).
	Output io.Writer
	// OnProgress receives parsed "Receiving objects: 42%" style updates.
	OnProgress func(Progress)
}

// Progress is one parsed git progress update.
type Progress struct {
	Phase   string
	Percent int
}

// Failure is a git exit failure carrying git's last messages.
type Failure struct {
	Err error
	// Detail is the tail of git's non-progress output, for display.
	Detail string
}

func (f *Failure) Error() string { return "git clone: " + f.Err.Error() }
func (f *Failure) Unwrap() error { return f.Err }

// Args builds the git argument list. Exported for tests.
func Args(o Options) []string {
	args := []string{
		// Applied to this process only (not written into the checkout).
		"-c", "core.fsmonitor=false",
		"-c", "core.hooksPath=" + os.DevNull,
		"clone",
		fmt.Sprintf("--depth=%d", o.Depth),
		"--single-branch",
	}
	if o.Branch != "" {
		args = append(args, "--branch", o.Branch)
	}
	if o.OnProgress != nil {
		// stderr is a pipe, so git only reports progress when asked.
		args = append(args, "--progress")
	}
	return append(args, "--", strings.TrimSpace(o.URL), o.Dest)
}

// Clone validates o and runs git clone, honoring ctx cancellation.
func Clone(ctx context.Context, o Options) error {
	if err := ValidateURL(o.URL); err != nil {
		return err
	}
	if err := ValidateRef(o.Branch); err != nil {
		return err
	}
	if o.Depth < 1 {
		return errors.New("depth must be at least 1")
	}
	if o.Dest == "" {
		return errors.New("no destination")
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return ErrNoGit
	}

	cmd := exec.CommandContext(ctx, gitPath, Args(o)...)
	cmd.Env = append(os.Environ(), "GIT_ALLOW_PROTOCOL="+allowedProtocols)
	cmd.WaitDelay = 3 * time.Second
	if o.Interactive {
		cmd.Stdin = o.Stdin
	} else {
		cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=0")
		detach(cmd)
	}

	tail := &tailBuffer{}
	lines := &lineWriter{fn: func(line string) {
		m := progressLine.FindStringSubmatch(line)
		if m == nil {
			tail.add(line)
			return
		}
		if o.OnProgress != nil {
			pct, _ := strconv.Atoi(m[2])
			o.OnProgress(Progress{Phase: strings.TrimPrefix(strings.TrimSpace(m[1]), "remote: "), Percent: min(pct, 100)})
		}
	}}
	var sink io.Writer = lines
	if o.Output != nil {
		sink = io.MultiWriter(o.Output, lines)
	}
	cmd.Stdout = sink
	cmd.Stderr = sink

	runErr := cmd.Run()
	lines.flush()
	if runErr == nil {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return &Failure{Err: runErr, Detail: tail.String()}
}

// lineWriter splits a byte stream on \r and \n (git redraws progress with
// bare carriage returns) and reports each non-empty line.
type lineWriter struct {
	buf []byte
	fn  func(string)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	for _, b := range p {
		if b == '\r' || b == '\n' {
			w.flush()
			continue
		}
		if len(w.buf) < 4096 {
			w.buf = append(w.buf, b)
		}
	}
	return len(p), nil
}

func (w *lineWriter) flush() {
	if len(w.buf) == 0 {
		return
	}
	line := strings.TrimSpace(string(w.buf))
	w.buf = w.buf[:0]
	if line != "" {
		w.fn(line)
	}
}

// tailBuffer keeps the last few lines.
type tailBuffer struct{ lines []string }

func (t *tailBuffer) add(line string) {
	if len(line) > 300 {
		line = line[:300]
	}
	t.lines = append(t.lines, line)
	if len(t.lines) > tailLines {
		t.lines = t.lines[len(t.lines)-tailLines:]
	}
}

func (t *tailBuffer) String() string { return strings.Join(t.lines, "\n") }
