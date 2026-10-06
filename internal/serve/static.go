package serve

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"net/http"
	"os/exec"
	"path"
	"runtime"
	"strings"

	"github.com/bethropolis/sift/web"
)

// handleStatic serves the embedded Svelte app. Assets are stored
// precompressed (file.gz): clients accepting gzip get them as-is with
// Content-Encoding: gzip, and the rare client without gzip support gets a
// decompressed stream. index.html is no-cache; hashed assets are immutable.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		// Only / and /assets/* exist; the hash router owns the rest, so any
		// other path serves the app shell. Unknown /assets/* paths 404.
		if !strings.HasPrefix(r.URL.Path, "/assets/") {
			serveGzFile(w, r, "dist/index.html", "text/html; charset=utf-8", false)
			return
		}
	}
	name := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	if name == "" || name == "." {
		name = "index.html"
	}
	asset := "dist/" + name
	contentType := contentTypeFor(name)
	immutable := strings.HasPrefix(r.URL.Path, "/assets/")
	serveGzFile(w, r, asset, contentType, immutable)
}

func serveGzFile(w http.ResponseWriter, r *http.Request, asset, contentType string, immutable bool) {
	data, err := webDist().Open(asset + ".gz")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer data.Close()
	info, err := data.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}

	h := w.Header()
	h.Set("Content-Type", contentType)
	if immutable {
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		h.Set("Cache-Control", "no-cache")
	}

	if acceptsGzip(r) {
		h.Set("Content-Encoding", "gzip")
		http.ServeContent(w, r, asset, info.ModTime(), data.(io.ReadSeeker))
		return
	}
	// No gzip support: decompress in memory (assets are small).
	gz, err := gzip.NewReader(data)
	if err != nil {
		http.Error(w, "cannot decompress asset", http.StatusInternalServerError)
		return
	}
	raw, err := io.ReadAll(gz)
	_ = gz.Close()
	if err != nil {
		http.Error(w, "cannot decompress asset", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, asset, info.ModTime(), bytes.NewReader(raw))
}

func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		if strings.TrimSpace(strings.SplitN(part, ";", 2)[0]) == "gzip" {
			return true
		}
	}
	return false
}

func contentTypeFor(name string) string {
	switch {
	case strings.HasSuffix(name, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(name, ".js"):
		return "text/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(name, ".json"):
		return "application/json"
	case strings.HasSuffix(name, ".webmanifest"):
		return "application/manifest+json"
	default:
		return "application/octet-stream"
	}
}

// openBrowser opens the login URL for --open (local only; enforced by
// Validate). Failures only log: the URL is already printed.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

// webDist exposes the embedded filesystem for testing.
func webDist() fs.FS {
	return web.DistFS
}
