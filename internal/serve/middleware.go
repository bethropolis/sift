package serve

import (
	"net"
	"net/http"
	"strings"
)

// csp is served on every response. No inline scripts are used by the UI;
// exactly one inline <style> is allowed: the first-paint guard in
// web/index.html, pinned by sha256 hash. Recompute the hash from the BUILT
// output (Vite rewrites the block, so source bytes differ) after any change:
// gunzip -c web/dist/index.html.gz | python3 -c "import sys,re,hashlib,base64;
// m=re.search(r'<style>(.*?)</style>',sys.stdin.read(),re.S);
// print('sha256-'+base64.b64encode(hashlib.sha256(m.group(1).encode()).digest()).decode())"
// TestFirstPaintStyleHash pins the two together; edit either and it fails.
const csp = "default-src 'none'; script-src 'self'; style-src 'self' 'sha256-DOZ3BdFEccmOTseuxFlYHYD6ZM2KNwWBvAkmsBN0zOI='; img-src 'self' data:; font-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"

// securityHeaders sets the response headers from the spec on everything.
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		if s.tlsActive() {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

// hostAllowlist rejects DNS-rebinding hosts on every route, including
// static files. Anything not explicitly allowed gets 421.
func (s *Server) hostAllowlist(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.allowedHost(r.Host) {
			http.Error(w, "misdirected request", http.StatusMisdirectedRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// allowedHost reports whether the Host header is expected: the bound
// address, loopback names on the bound port, or --allowed-host entries.
func (s *Server) allowedHost(host string) bool {
	if s.hosts[host] {
		return true
	}
	// Bare names without a port are accepted for odd clients and proxies.
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return s.hosts[host]
}

// originCheck guards every non-GET: the Origin host must equal Host, and
// cross-site fetch metadata is rejected. POST/PUT must be JSON (415
// otherwise). OPTIONS is never a preflight here: no CORS, ever (405).
func originCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			if !sameOrigin(origin, r.Host) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}
		ct := r.Header.Get("Content-Type")
		if ct == "" {
			// Empty bodies (logout) are fine; JSON is required when a body
			// is present.
			if r.ContentLength != 0 {
				http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
				return
			}
		} else if !isJSONContent(ct) {
			http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func sameOrigin(origin, host string) bool {
	// Origin is scheme://host[:port]. Compare the host part exactly.
	rest, ok := strings.CutPrefix(origin, "http://")
	if !ok {
		rest, ok = strings.CutPrefix(origin, "https://")
		if !ok {
			return false
		}
	}
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		rest = rest[:i]
	}
	return rest == host
}

func isJSONContent(ct string) bool {
	media, _, _ := strings.Cut(ct, ";")
	return strings.TrimSpace(strings.ToLower(media)) == "application/json"
}

// requireSession gates every non-public route. A valid session also refreshes
// the idle timer; an expiring-soon cookie is re-issued transparently.
func (s *Server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok, reissue := s.auth.ValidSession(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if reissue {
			c := s.auth.MintCookie()
			s.secureCookie(c)
			http.SetCookie(w, c)
		}
		next.ServeHTTP(w, r)
	})
}
