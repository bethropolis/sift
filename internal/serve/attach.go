package serve

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"
)

// attachProbeTimeout bounds the "is that a sift server?" check so a bind
// conflict against a silent occupant still fails fast.
const attachProbeTimeout = 2 * time.Second

// isAddrInUse reports whether err is a bind conflict (as opposed to a bad
// address, permission problem, or anything else).
func isAddrInUse(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE)
}

// probeSiftServer asks addr for /api/meta over http, then https (the existing
// server may use an ephemeral self-signed cert), and reports the scheme when
// the response has the sift shape. Anything else — refused, non-JSON, wrong
// fields — means the occupant is not a sift server we can attach to.
func probeSiftServer(addr string) (string, bool) {
	for _, scheme := range []string{"http", "https"} {
		if probeMeta(scheme, addr) {
			return scheme, true
		}
	}
	return "", false
}

func probeMeta(scheme, addr string) bool {
	client := &http.Client{Timeout: attachProbeTimeout}
	if scheme == "https" {
		// The probe reads only public version facts over loopback and sends
		// no credentials, so skipping verification authenticates nothing and
		// risks nothing. The opened browser still shows its own TLS UI.
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec
	}
	resp, err := client.Get(scheme + "://" + addr + "/api/meta") //nolint:noctx
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var meta struct {
		Version  string `json:"version"`
		Mode     string `json:"mode"`
		AuthKind string `json:"authKind"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&meta); err != nil {
		return false
	}
	if meta.Version == "" || meta.AuthKind == "" {
		return false
	}
	return meta.Mode == "local" || meta.Mode == "remote"
}

// maybeAttach handles a bind failure when the user asked for a window. If the
// port is held by a live sift server on loopback, it opens that server and
// reports true: the process exits 0 without starting anything. Otherwise it
// reports false and the caller returns the bind error.
//
// The attached window gets the plain base URL: there is no auto-login (this
// process never knew the existing server's token) and no window-liveness
// shutdown (this process did not start that server, so closing the window
// must not stop it).
func (s *Server) maybeAttach(listenErr error) bool {
	if !isAddrInUse(listenErr) {
		return false
	}
	if s.isRemoteBind() {
		return false
	}
	if !s.cfg.Open && !s.cfg.App {
		return false
	}
	scheme, ok := probeSiftServer(s.cfg.Listen)
	if !ok {
		return false
	}
	base := scheme + "://" + s.cfg.Listen + "/"
	if s.cfg.App {
		if launched, _ := s.launchApp(base); launched {
			fmt.Fprintf(os.Stderr, "  %s %s\n", serveDim.Sprint("app →"), serveURL.Sprint(base))
		} else {
			openBrowser(base)
			fmt.Fprintf(os.Stderr, "  %s %s\n", serveDim.Sprint("open →"), serveURL.Sprint(base))
		}
	} else {
		openBrowser(base)
		fmt.Fprintf(os.Stderr, "  %s %s\n", serveDim.Sprint("open →"), serveURL.Sprint(base))
	}
	fmt.Fprintf(os.Stderr, "  %s %s\n",
		serveWarn.Sprint("already running:"),
		fmt.Sprintf("attached to the existing sift server (no new server started; log in with its token — closing this window leaves it running)"))
	return true
}

// bindHint keeps the original bind error but points at the existing server
// when the conflicting occupant proves to be sift.
func bindHint(listen string, err error) error {
	if !isAddrInUse(err) {
		return fmt.Errorf("listen %s: %w", listen, err)
	}
	host, _, serr := net.SplitHostPort(listen)
	if serr != nil || !isLoopbackHost(host) {
		return fmt.Errorf("listen %s: %w", listen, err)
	}
	if scheme, ok := probeSiftServer(listen); ok {
		return fmt.Errorf("listen %s: %w (a sift server is already running at %s://%s/; use --open to attach a browser to it)",
			listen, err, scheme, listen)
	}
	return fmt.Errorf("listen %s: %w", listen, err)
}
