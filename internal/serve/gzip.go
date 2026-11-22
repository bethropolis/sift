package serve

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// gzipThreshold is the response size below which compression overhead (and
// the extra buffer) is not worth it. The spec calls for 1 KiB.
const gzipThreshold = 1 << 10

var gzipPool = sync.Pool{
	New: func() any {
		w, err := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		if err != nil {
			panic(err)
		}
		return w
	},
}

// gzipAPI compresses JSON /api/* responses when the client accepts gzip. SSE
// and clone NDJSON stay streaming; 204s, pre-encoded bodies, and tiny bodies
// pass through. Vary is always set so proxies split the cache correctly.
func (s *Server) gzipAPI(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addVary(w.Header())
		if !acceptsGzip(r) || gzipStreamingPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		gw := &gzipResponseWriter{ResponseWriter: w}
		defer gw.finish()
		next.ServeHTTP(gw, r)
	})
}

func addVary(h http.Header) {
	for _, v := range h.Values("Vary") {
		for _, part := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(part), "Accept-Encoding") {
				return
			}
		}
	}
	h.Add("Vary", "Accept-Encoding")
}

// gzipStreamingPath lists streaming responses that must not be buffered.
func gzipStreamingPath(path string) bool {
	return path == "/api/app/heartbeat" || path == "/api/clone"
}

type gzipResponseWriter struct {
	http.ResponseWriter

	status     int
	wroteHead  bool
	decided    bool
	compressed bool
	buf        []byte
	gz         *gzip.Writer
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	if w.wroteHead {
		return
	}
	w.wroteHead = true
	w.status = status
}

func (w *gzipResponseWriter) Write(p []byte) (int, error) {
	if !w.wroteHead {
		w.WriteHeader(http.StatusOK)
	}
	if w.decided {
		if w.compressed {
			return w.gz.Write(p)
		}
		return w.ResponseWriter.Write(p)
	}
	if w.status == http.StatusNoContent || w.status == http.StatusNotModified {
		w.decide(false)
		return w.ResponseWriter.Write(p)
	}
	w.buf = append(w.buf, p...)
	if len(w.buf) < gzipThreshold {
		return len(p), nil
	}
	ct := strings.ToLower(w.Header().Get("Content-Type"))
	if w.Header().Get("Content-Encoding") != "" || !strings.HasPrefix(ct, "application/json") {
		w.decide(false)
		return len(p), nil
	}
	w.startGzip()
	return len(p), nil
}

// decide commits to plain output, flushing the buffered prefix.
func (w *gzipResponseWriter) decide(compress bool) {
	if w.decided {
		return
	}
	w.decided = true
	w.compressed = compress
	if compress {
		w.startGzipLocked()
		return
	}
	w.ResponseWriter.WriteHeader(w.status)
	if len(w.buf) > 0 {
		_, _ = w.ResponseWriter.Write(w.buf)
		w.buf = nil
	}
}

func (w *gzipResponseWriter) startGzip() {
	if w.decided {
		return
	}
	w.decided = true
	w.compressed = true
	w.startGzipLocked()
}

func (w *gzipResponseWriter) startGzipLocked() {
	h := w.Header()
	h.Set("Content-Encoding", "gzip")
	h.Del("Content-Length")
	w.ResponseWriter.WriteHeader(w.status)
	gz := gzipPool.Get().(*gzip.Writer)
	gz.Reset(w.ResponseWriter)
	w.gz = gz
	if len(w.buf) > 0 {
		_, _ = gz.Write(w.buf)
		w.buf = nil
	}
}

func (w *gzipResponseWriter) finish() {
	if !w.wroteHead {
		w.WriteHeader(http.StatusOK)
	}
	if !w.decided {
		ct := strings.ToLower(w.Header().Get("Content-Type"))
		compress := w.status != http.StatusNoContent && w.status != http.StatusNotModified &&
			w.Header().Get("Content-Encoding") == "" &&
			strings.HasPrefix(ct, "application/json") &&
			len(w.buf) >= gzipThreshold
		w.decide(compress)
	}
	if w.gz != nil {
		_ = w.gz.Close()
		gzipPool.Put(w.gz)
		w.gz = nil
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *gzipResponseWriter) Flush() {
	if !w.decided {
		w.decide(false)
	}
	if w.gz != nil {
		_ = w.gz.Flush()
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
