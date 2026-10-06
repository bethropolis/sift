package serve

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/serve/auth"
	"github.com/bethropolis/sift/internal/serve/jail"
	"github.com/bethropolis/sift/web"
	"github.com/fatih/color"
)

// Serve banner colors via fatih/color: auto-disabled under NO_COLOR,
// TERM=dumb, or piped output, so no manual checks are needed here.
var (
	serveBold = color.New(color.Bold)
	serveDim  = color.New(color.Faint)
	serveURL  = color.New(color.FgCyan)
	serveWarn = color.New(color.FgYellow)
)

// Server is a configured `sift serve` instance. It holds no per-project
// state: the browser sends the selection with each request, and only the
// auth key plus small state-dir files live server-side.
type Server struct {
	cfg       *Config
	engineCfg *config.Config
	auth      *auth.Auth
	jail      *jail.Jail
	log       *slog.Logger
	hosts     map[string]bool
	remote    bool
	tls       bool

	mux     *http.ServeMux
	packSem chan struct{}

	httpSrv *http.Server

	idleMu    sync.Mutex
	idleTimer *time.Timer
}

// route describes one registered route for the enumeration test.
type route struct {
	method string
	path   string
	public bool
}

// New validates the config (refusing bad remote setups before binding),
// builds the jail, and registers every route from one table.
func New(cfg *Config, engineCfg *config.Config, log *slog.Logger) (*Server, error) {
	a, err := cfg.Validate()
	if err != nil {
		return nil, err
	}
	j, err := buildJail(cfg)
	if err != nil {
		return nil, err
	}
	if log == nil {
		log = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	s := &Server{
		cfg:       cfg,
		engineCfg: engineCfg,
		auth:      a,
		jail:      j,
		log:       log,
		packSem:   make(chan struct{}, 2),
	}
	s.remote = s.isRemoteBind()
	s.tls = cfg.TLSCert != "" || cfg.TLSSelfSigned
	s.hosts = s.buildHostSet()
	s.registerRoutes()
	return s, nil
}

// routes enumerates every route; the security test asserts the public set.
func (s *Server) routes() []route {
	return []route{
		{"GET", "/", true},
		{"GET", "/assets/", true},
		{"GET", "/api/meta", true},
		{"POST", "/api/login", true},
		{"POST", "/api/logout", false},
		{"GET", "/api/recents", false},
		{"POST", "/api/recents", false},
		{"DELETE", "/api/recents", false},
		{"GET", "/api/browse", false},
		{"GET", "/api/tree", false},
		{"GET", "/api/file", false},
		{"POST", "/api/smart-select", false},
		{"POST", "/api/pack", false},
		{"GET", "/api/settings", false},
		{"PUT", "/api/settings", false},
	}
}

func (s *Server) registerRoutes() {
	mux := http.NewServeMux()

	// Static UI.
	mux.Handle("GET /", s.securityHeaders(s.hostAllowlist(http.HandlerFunc(s.handleStatic))))

	// Public API.
	mux.Handle("GET /api/meta", s.apiChain(http.HandlerFunc(s.handleMeta), true))
	mux.Handle("POST /api/login", s.apiChain(http.HandlerFunc(s.handleLogin), true))

	// Session API.
	mux.Handle("POST /api/logout", s.apiChain(http.HandlerFunc(s.handleLogout), false))
	mux.Handle("GET /api/recents", s.apiChain(http.HandlerFunc(s.handleRecentsGet), false))
	mux.Handle("POST /api/recents", s.apiChain(http.HandlerFunc(s.handleRecentsPost), false))
	mux.Handle("DELETE /api/recents", s.apiChain(http.HandlerFunc(s.handleRecentsDelete), false))
	mux.Handle("GET /api/browse", s.apiChain(http.HandlerFunc(s.handleBrowse), false))
	mux.Handle("GET /api/tree", s.apiChain(http.HandlerFunc(s.handleTree), false))
	mux.Handle("GET /api/file", s.apiChain(http.HandlerFunc(s.handleFile), false))
	mux.Handle("POST /api/smart-select", s.apiChain(http.HandlerFunc(s.handleSmartSelect), false))
	mux.Handle("POST /api/pack", s.apiChain(http.HandlerFunc(s.handlePack), false))
	mux.Handle("GET /api/settings", s.apiChain(http.HandlerFunc(s.handleSettingsGet), false))
	mux.Handle("PUT /api/settings", s.apiChain(http.HandlerFunc(s.handleSettingsPut), false))

	s.mux = mux
}

// apiChain wraps API handlers: host check, headers, idle reset, origin
// check, then the session gate for non-public routes.
func (s *Server) apiChain(next http.Handler, public bool) http.Handler {
	h := next
	if !public {
		h = s.requireSession(h)
	}
	h = originCheck(h)
	h = s.idleReset(h)
	h = s.securityHeaders(h)
	h = s.hostAllowlist(h)
	return h
}

// isRemoteBind reports whether the listen address is non-loopback.
func (s *Server) isRemoteBind() bool {
	host, _, err := net.SplitHostPort(s.cfg.Listen)
	if err != nil {
		return true
	}
	return !isLoopbackHost(host)
}

// buildHostSet computes the Host allowlist: the bound address, loopback
// names on the bound port (and bare, for odd clients), plus --allowed-host.
func (s *Server) buildHostSet() map[string]bool {
	set := map[string]bool{}
	host, port, err := net.SplitHostPort(s.cfg.Listen)
	if err != nil {
		return set
	}
	withPort := func(h string) string {
		if strings.Contains(h, ":") && !strings.HasPrefix(h, "[") {
			h = "[" + h + "]"
		}
		return h + ":" + port
	}
	set[s.cfg.Listen] = true
	for _, loop := range []string{"127.0.0.1", "localhost", "::1"} {
		set[withPort(loop)] = true
		set[loop] = true
	}
	if host != "" {
		set[host] = true
		set[withPort(host)] = true
	}
	for _, h := range s.cfg.AllowedHosts {
		set[h] = true
	}
	return set
}

func (s *Server) tlsActive() bool { return s.tls }

// secureCookie applies the Secure flag when the session travels over TLS:
// direct TLS, or a trusted proxy reporting https.
func (s *Server) secureCookie(c *http.Cookie) {
	if s.tls {
		c.Secure = true
	}
}

// secureProxyRequest reports an https proto from the trusted proxy.
func (s *Server) secureProxyRequest(r *http.Request) bool {
	return s.cfg.BehindProxy && strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// idleReset restarts the one-shot idle timer on every request. The timer is
// the only idle-state in the process; there are no tickers or pollers.
func (s *Server) idleReset(next http.Handler) http.Handler {
	if s.cfg.IdleTimeout <= 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.idleMu.Lock()
		if s.idleTimer != nil {
			s.idleTimer.Stop()
			s.idleTimer = time.AfterFunc(s.cfg.IdleTimeout, func() {
				s.log.Info("idle timeout reached, exiting")
				s.shutdown()
			})
		}
		s.idleMu.Unlock()
		next.ServeHTTP(w, r)
	})
}

// shutdown gracefully stops the server with a 5s deadline, cancelling
// in-flight work via request contexts.
func (s *Server) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.httpSrv.Shutdown(ctx)
}

// Run binds (refusing to start on any misconfiguration), prints the startup
// banner with the login URL, and serves until SIGINT/SIGTERM or the idle
// timeout. Missing web assets are checked before binding or printing any
// token.
func (s *Server) Run() error {
	if err := checkWebUI(); err != nil {
		return err
	}

	ln, err := net.Listen("tcp", s.cfg.Listen)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.cfg.Listen, err)
	}

	tlsConfig, fingerprint, err := s.tlsConfig()
	if err != nil {
		return err
	}

	s.httpSrv = &http.Server{
		Handler:           s.mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	if s.cfg.IdleTimeout > 0 {
		s.idleMu.Lock()
		s.idleTimer = time.AfterFunc(s.cfg.IdleTimeout, func() {
			s.log.Info("idle timeout reached, exiting")
			s.shutdown()
		})
		s.idleMu.Unlock()
	}

	addr := ln.Addr().String()
	scheme := "http"
	if tlsConfig != nil {
		scheme = "https"
		s.tls = true
	}
	mode := "local"
	if s.remote {
		mode = "remote"
	}
	fmt.Fprintf(os.Stderr, "%s %s · %s · %s://%s · %s auth\n",
		serveBold.Sprint("sift serve"),
		serveDim.Sprint(s.engineCfg.Version),
		mode,
		scheme, addr,
		s.auth.Kind(),
	)
	s.log.Info("sift serve starting",
		"mode", mode,
		"addr", addr,
		"tls", scheme == "https",
		"auth", s.auth.Kind(),
		"roots", s.jail.Roots(),
		"version", s.engineCfg.Version,
	)
	if fingerprint != "" {
		fmt.Fprintf(os.Stderr, "  %s %s\n",
			serveWarn.Sprint("tls fingerprint (SHA-256):"),
			fingerprint)
	}
	if !s.remote {
		fmt.Fprintf(os.Stderr, "  %s %s\n",
			serveDim.Sprint("open →"),
			serveURL.Sprintf("%s://%s/#/login?token=%s", scheme, addr, s.auth.Token()))
	}

	if s.cfg.Open {
		openBrowser(scheme + "://" + addr + "/#/login?token=" + s.auth.Token())
	}

	if tlsConfig != nil {
		tlsLn := tls.NewListener(ln, tlsConfig)
		return s.serveWithSignals(tlsLn)
	}
	return s.serveWithSignals(ln)
}

func (s *Server) serveWithSignals(ln net.Listener) error {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.httpSrv.Serve(ln)
	}()

	select {
	case sig := <-stop:
		s.log.Info("shutting down", "signal", sig)
		s.shutdown()
		<-errCh
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

// tlsConfig returns a TLS config for --tls-cert/key or an ephemeral
// in-memory ECDSA P-256 self-signed certificate.
func (s *Server) tlsConfig() (*tls.Config, string, error) {
	if s.cfg.TLSCert != "" {
		cert, err := tls.LoadX509KeyPair(s.cfg.TLSCert, s.cfg.TLSKey)
		if err != nil {
			return nil, "", err
		}
		return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}, "", nil
	}
	if !s.cfg.TLSSelfSigned {
		return nil, "", nil
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, "", err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, "", err
	}
	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "sift serve"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, "", err
	}
	cert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	sum := sha256.Sum256(der)
	return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12},
		hex.EncodeToString(sum[:]), nil
}

// checkWebUI refuses to start when the embedded UI is absent (plain clones,
// @main installs): the failure names the fix.
func checkWebUI() error {
	if _, err := web.DistFS.Open("dist/index.html.gz"); err != nil {
		return fmt.Errorf("this build has no web UI. Install a release binary (see https://bethropolis.github.io/sift/install/) or build it with \"just web\"")
	}
	return nil
}
