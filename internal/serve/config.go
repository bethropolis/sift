package serve

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bethropolis/sift/internal/serve/auth"
	"github.com/bethropolis/sift/internal/serve/jail"
)

// Config holds the `sift serve` flags. Durations and sizes follow the spec:
// fixed loopback port, explicit opt-ins for everything remote.
type Config struct {
	Listen          string
	Roots           []string
	Deny            []string
	PasswordFile    string
	AllowRemote     bool
	TLSCert         string
	TLSKey          string
	TLSSelfSigned   bool
	BehindProxy     bool
	AllowedHosts    []string
	AllowUnredacted bool
	// Pair enables the mobile pairing flow: read-only sessions minted by
	// scanning a one-time QR. It implies the full remote posture even on a
	// loopback bind.
	Pair           bool
	PublicURL      string
	PairTTL        time.Duration
	PairSessionTTL time.Duration
	// PairMaxSessions caps concurrent paired phones. Zero resolves to the
	// default in Validate.
	PairMaxSessions int
	// AllowClone enables "clone a repository" in the web UI (needs system
	// git too). Off by default on every bind; without it the clone routes
	// are 404 and the UI hides every entry point.
	AllowClone bool
	// InsecureHTTP is pair-only: it permits an HTTP public URL. Remote
	// password mode still requires TLS or a trusted proxy.
	InsecureHTTP bool
	IdleTimeout  time.Duration
	Open         bool
	// App launches the UI in a chromeless browser window and, unless
	// KeepAlive is set, stops the server when that window goes away.
	App bool
	// KeepAlive suppresses the app-window shutdown (only meaningful with App).
	KeepAlive bool
	// EngineFlagOverrides carries operator-passed engine flags (name → value)
	// recorded before profile resolution. Each request re-applies them over
	// the target project's own .sift.toml, so CLI flag precedence holds
	// without the server's startup directory leaking into other projects.
	// Not a CLI flag.
	EngineFlagOverrides map[string]string
}

// DefaultListen is the fixed local port, keeping the browser origin stable.
const DefaultListen = "127.0.0.1:7777"

// Pairing defaults. They are deliberately short for the token and long
// enough to be useful for the read-only session.
const (
	DefaultPairTTL         = 120 * time.Second
	DefaultPairSessionTTL  = 12 * time.Hour
	DefaultPairMaxSessions = 5
)

// maxRequestBody caps API request bodies (4 MiB).
const maxRequestBody = 4 << 20

// previewCap caps file previews (1 MiB).
const previewCap = 1 << 20

// Validate checks the remote refusal matrix before binding, naming the
// missing flag. It also resolves and validates the password source.
func (c *Config) Validate() (*auth.Auth, error) {
	host, port, err := net.SplitHostPort(c.Listen)
	if err != nil {
		return nil, fmt.Errorf("bad --listen %q: %w", c.Listen, err)
	}
	remote := !isLoopbackHost(host)

	if c.InsecureHTTP && !c.Pair {
		return nil, fmt.Errorf("--insecure-http is only valid with --pair")
	}
	if c.Pair {
		return c.validatePair(remote)
	}

	if c.Open && remote {
		return nil, fmt.Errorf("--open is local-only; refusing non-loopback bind %q", c.Listen)
	}
	if c.App && remote {
		// An app window carries an auto-login token and stops the server when
		// it closes; neither belongs on a listener other machines can reach.
		return nil, fmt.Errorf("--app is local-only; refusing non-loopback bind %q", c.Listen)
	}
	if c.KeepAlive && !c.App {
		return nil, fmt.Errorf("--keep-alive only applies to --app")
	}
	if port == "0" && !remote {
		// Explicit :0 is honored (never a silent fallback); nothing to do.
	}

	if !remote {
		token, err := auth.GenerateToken()
		if err != nil {
			return nil, err
		}
		a, err := auth.NewToken(token)
		if err != nil {
			return nil, err
		}
		// A password may additionally be configured locally (same 12-char
		// minimum); either credential then unlocks the session.
		if password, err := c.resolvePassword(); err != nil {
			return nil, err
		} else if password != "" {
			if err := a.SetPassword(password); err != nil {
				return nil, fmt.Errorf("bad local password: %v (set --password-file or SIFT_SERVE_PASSWORD)", err)
			}
		}
		return a, nil
	}

	if !c.AllowRemote {
		return nil, fmt.Errorf("refusing non-loopback bind %q without --allow-remote", c.Listen)
	}
	password, err := c.resolvePassword()
	if err != nil {
		return nil, err
	}
	if password == "" {
		return nil, fmt.Errorf("remote mode needs a password: set --password-file or SIFT_SERVE_PASSWORD (min 12 characters)")
	}
	a, err := auth.NewPassword(password)
	if err != nil {
		return nil, fmt.Errorf("remote mode needs a password: %v (set --password-file or SIFT_SERVE_PASSWORD)", err)
	}
	// A generated token is not accepted remotely: password auth must be
	// explicit, so there is always a human-known credential.
	if len(c.Roots) == 0 {
		return nil, fmt.Errorf("remote mode needs at least one explicit --root")
	}
	if len(c.AllowedHosts) == 0 {
		return nil, fmt.Errorf("remote mode needs at least one --allowed-host")
	}
	tlsModes := 0
	if c.TLSCert != "" || c.TLSKey != "" {
		tlsModes++
	}
	if c.TLSSelfSigned {
		tlsModes++
	}
	if c.BehindProxy {
		tlsModes++
	}
	if tlsModes != 1 {
		return nil, fmt.Errorf("remote mode needs exactly one of --tls-cert/--tls-key, --tls-self-signed, --behind-proxy")
	}
	if c.TLSCert != "" && c.TLSKey == "" || c.TLSCert == "" && c.TLSKey != "" {
		return nil, fmt.Errorf("remote mode needs both --tls-cert and --tls-key")
	}
	return a, nil
}

// validatePair enforces the pairing refusal matrix. Pair is read-only,
// always redacted, and always remote-gated, regardless of the bind address.
func (c *Config) validatePair(remote bool) (*auth.Auth, error) {
	if len(c.Roots) == 0 {
		return nil, fmt.Errorf("--pair needs at least one explicit --root")
	}
	if c.AllowClone {
		return nil, fmt.Errorf("--pair cannot be combined with --allow-clone")
	}
	if c.AllowUnredacted {
		return nil, fmt.Errorf("--pair cannot be combined with --allow-unredacted")
	}
	if c.App {
		return nil, fmt.Errorf("--pair cannot be combined with --app")
	}
	if c.PublicURL == "" && !c.InsecureHTTP {
		return nil, fmt.Errorf("--pair needs --public-url (HTTPS) or --insecure-http")
	}
	if c.PublicURL != "" {
		u, err := url.Parse(c.PublicURL)
		if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil ||
			u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
			return nil, fmt.Errorf("bad --public-url %q: need an absolute http(s) URL without userinfo, query, or fragment", c.PublicURL)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return nil, fmt.Errorf("bad --public-url %q: scheme must be http or https", c.PublicURL)
		}
		if u.Scheme == "http" && !c.InsecureHTTP {
			return nil, fmt.Errorf("--public-url uses http://; pass --insecure-http to allow plain HTTP pairing")
		}
		c.PublicURL = strings.TrimRight(c.PublicURL, "/")
		if u.Scheme == "https" && !remote {
			// A loopback bind behind a TLS tunnel is the recommended setup.
			c.BehindProxy = true
		}
	}
	if remote && !c.BehindProxy && !c.InsecureHTTP && c.TLSCert == "" && !c.TLSSelfSigned {
		return nil, fmt.Errorf("--pair on a non-loopback bind needs --behind-proxy, --insecure-http, or a TLS certificate")
	}
	if c.PairTTL <= 0 {
		c.PairTTL = DefaultPairTTL
	}
	if c.PairSessionTTL <= 0 {
		c.PairSessionTTL = DefaultPairSessionTTL
	}
	if c.PairMaxSessions <= 0 {
		c.PairMaxSessions = DefaultPairMaxSessions
	}
	// The startup token/password paths are intentionally absent. Pair mode
	// has exactly one credential: the one-time token.
	return auth.NewPair()
}

// resolvePassword reads the password from --password-file or
// SIFT_SERVE_PASSWORD. The file must not be group/world-readable.
func (c *Config) resolvePassword() (string, error) {
	if c.PasswordFile != "" {
		info, err := os.Stat(c.PasswordFile)
		if err != nil {
			return "", fmt.Errorf("cannot stat --password-file: %w", err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			return "", fmt.Errorf("--password-file %q is group/world-readable; chmod 600 it", c.PasswordFile)
		}
		data, err := os.ReadFile(c.PasswordFile)
		if err != nil {
			return "", fmt.Errorf("cannot read --password-file: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	return strings.TrimSpace(os.Getenv("SIFT_SERVE_PASSWORD")), nil
}

// isLoopbackHost reports whether host is a loopback name or IP. Empty host
// (all interfaces, e.g. ":7777" or "0.0.0.0") is remote.
func isLoopbackHost(host string) bool {
	switch strings.ToLower(strings.Trim(host, "[]")) {
	case "", "0.0.0.0", "::", "localhost":
		if host == "" || host == "0.0.0.0" || host == "::" {
			return false
		}
		return true
	case "127.0.0.1", "::1":
		return true
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// defaultRoots returns $HOME for local mode.
func defaultRoots() []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	return []string{home}
}

// buildJail resolves the effective roots (explicit or $HOME default) into a
// jail. In remote mode explicit roots were already enforced by Validate.
func buildJail(c *Config) (*jail.Jail, error) {
	roots := c.Roots
	if len(roots) == 0 {
		roots = defaultRoots()
	}
	if len(roots) == 0 {
		return nil, fmt.Errorf("no allowed roots and $HOME is unknown; pass --root")
	}
	// Resolve ~ prefixes for convenience.
	for i, r := range roots {
		if r == "~" || strings.HasPrefix(r, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				roots[i] = filepath.Join(home, strings.TrimPrefix(r, "~"))
			}
		}
	}
	return jail.New(roots, c.Deny)
}
